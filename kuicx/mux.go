package kuicx

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

type Multiplexer interface {
	Listen(port uint16) (net.Listener, error)
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func NewStreamMux() *StreamMux {
	return &StreamMux{}
}

type StreamMux struct {
	Timeout     time.Duration
	lmu         sync.RWMutex
	listens     map[uint16]*streamListener
	cmu         sync.RWMutex
	connections map[string]*infoConn
}

func (sm *StreamMux) Handle(conn quic.Connection) {
	info, err := sm.handshakeConn(conn)
	if err != nil {
		_ = conn.CloseWithError(0, err.Error())
		return
	}

	//goland:noinspection GoUnhandledErrorResult
	defer conn.CloseWithError(0, "")

	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		stm, err := conn.AcceptStream(context.Background())
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if maximum := time.Second; tempDelay > maximum {
					tempDelay = maximum
				}
				time.Sleep(tempDelay)
				continue
			}
			break
		}

		go sm.serveStream(conn, stm, info)
	}
}

func (sm *StreamMux) Listen(port uint16) (net.Listener, error) {
	lis := sm.newStreamListener(port)
	if !sm.registerListener(lis) {
		err := &net.OpError{
			Op:   "listen",
			Net:  "vnet",
			Addr: lis.addr,
			Err:  errors.New("port already in use"),
		}
		return nil, err
	}

	return lis, nil
}

func (sm *StreamMux) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Err: err}
	}
	port, err := strconv.ParseInt(portStr, 10, 64)
	if err != nil || port < 0 || port > 65535 {
		return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("port" + portStr + "out of range")}
	}

	ic := sm.lookupConnection(host)
	if ic == nil {
		return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("no route to host: " + host)}
	}
	conn := ic.conn
	stm, err := conn.OpenStream()
	if err != nil {
		return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("can not open stream: " + address)}
	}

	data := make([]byte, 2)
	binary.BigEndian.PutUint16(data, uint16(port))

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}
	_ = stm.SetDeadline(deadline)

	if _, err = stm.Write(data); err != nil {
		_ = stm.Close()
		return nil, &net.OpError{Op: "dial", Net: network, Err: err}
	}
	resp := make([]byte, 1) // 读取响应消息
	if _, err = stm.Read(resp); err != nil || resp[0] != 0 {
		_ = stm.Close()
		if err == nil && resp[0] != 0 {
			err = DialErrno(resp[0])
		}
		return nil, &net.OpError{Op: "dial", Net: network, Err: err}
	}

	sc := sm.newStreamConn(conn, stm, ic.info)

	return sc, nil
}

func (sm *StreamMux) lookupConnection(id string) *infoConn {
	sm.cmu.RLock()
	defer sm.cmu.RUnlock()

	if cs := sm.connections; cs != nil {
		return cs[id]
	}

	return nil
}

func (sm *StreamMux) newStreamListener(port uint16) *streamListener {
	ctx, cancel := context.WithCancel(context.Background())

	return &streamListener{
		mux:    sm,
		port:   port,
		addr:   &net.UDPAddr{Port: int(port)},
		queue:  make(chan *streamConn),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (sm *StreamMux) registerListener(lis *streamListener) (registered bool) {
	port := lis.port
	sm.lmu.Lock()
	if sm.listens == nil {
		sm.listens = make(map[uint16]*streamListener, 32)
	}
	_, exist := sm.listens[port]
	if registered = !exist; registered {
		sm.listens[port] = lis
	}
	sm.lmu.Unlock()

	return
}

func (sm *StreamMux) unregisterListener(port uint16) {
	sm.lmu.Lock()
	delete(sm.listens, port)
	sm.lmu.Unlock()
}

func (sm *StreamMux) lookupListener(port uint16) *streamListener {
	sm.lmu.RLock()
	defer sm.lmu.RUnlock()

	if listens := sm.listens; listens != nil {
		return listens[port]
	}

	return nil
}

func (sm *StreamMux) handshakeConn(conn quic.Connection) (*ClientInfo, error) {
	var ctx context.Context
	if timeout := sm.Timeout; timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	} else {
		ctx = context.Background()
	}

	stm, err := conn.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	//goland:noinspection GoUnhandledErrorResult
	defer stm.Close()

	info := new(ClientInfo)
	if err = json.NewDecoder(stm).Decode(info); err != nil {
		return nil, err
	}

	resp := &HandshakeResult{Successful: true}
	if info.ID == "" {
		resp.Successful = false
		resp.Message = "id 必须填写"
	} else if info.Inet.IsUnspecified() {
		resp.Successful = false
		resp.Message = "inet 无效"
	}
	if err = json.NewEncoder(stm).Encode(resp); err != nil {
		return nil, err
	}

	return info, nil
}

func (sm *StreamMux) serveStream(conn quic.Connection, stm quic.Stream, info *ClientInfo) {
	lis, err := sm.handshakeStream(stm)
	if err != nil || lis == nil {
		_ = stm.Close()
		return
	}

	sc := sm.newStreamConn(conn, stm, info)
	if err = lis.establish(sc); err != nil {
		_ = stm.Close()
	}
}

func (sm *StreamMux) handshakeStream(stm quic.Stream) (*streamListener, error) {
	deadline := time.Now().Add(10 * time.Second)
	_ = stm.SetWriteDeadline(deadline)

	// 读取 port
	head := make([]byte, 2)
	n, err := stm.Read(head)
	if err != nil || n != 2 {
		_, _ = stm.Write([]byte{byte(ErrHandshakePacket)})
		if err == nil {
			err = ErrHandshakePacket
		}
		return nil, err
	}

	port := binary.BigEndian.Uint16(head)
	lis := sm.lookupListener(port)
	if lis == nil {
		_, _ = stm.Write([]byte{byte(ErrPortUnreachable)})
	} else {
		_, err = stm.Write([]byte{0}) // success flag
	}
	if err != nil {
		return nil, err
	}

	return lis, nil
}

func (sm *StreamMux) newStreamConn(conn quic.Connection, stm quic.Stream, info *ClientInfo) *streamConn {
	localAddr, remoteAddr := conn.LocalAddr(), conn.RemoteAddr()
	return &streamConn{localAddr: localAddr, remoteAddr: remoteAddr, stm: stm, info: info}
}
