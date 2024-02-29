package kuicx

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/quic-go/quic-go"
)

func Dial(ctx context.Context, addr string) (http.RoundTripper, error) {
	cfg := &tls.Config{NextProtos: []string{"vnet"}}
	conn, err := quic.DialAddr(ctx, addr, cfg, nil)
	if err != nil {
		return nil, err
	}

	stm, err := conn.OpenStream()
	if err != nil {
		return nil, err
	}

	req := &ClientInfo{ID: "10001", Inet: net.IP{172, 31, 61, 168}}
	if err = json.NewEncoder(stm).Encode(req); err != nil {
		return nil, err
	}
	resp := new(HandshakeResult)
	if err = json.NewDecoder(stm).Decode(resp); err != nil {
		return nil, err
	}
	_ = stm.Close()

	cc := &clientConn{conn: conn, info: req}
	trip := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			num, _ := strconv.ParseInt(port, 10, 64)

			conn, err := cc.DialContext(ctx, uint16(num))
			if err != nil {
				return nil, err
			}

			return conn, nil
		},
	}

	return trip, nil
}

type Tunneler interface {
	DialContext(ctx context.Context, port uint16) (net.Conn, error)
	ClientInfo() *ClientInfo
}

type clientConn struct {
	conn quic.Connection
	info *ClientInfo
}

func (cc *clientConn) DialContext(ctx context.Context, port uint16) (net.Conn, error) {
	stm, err := cc.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			_ = stm.Close()
		}
	}()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(10 * time.Second)
	}
	if err = stm.SetDeadline(deadline); err != nil {
		return nil, err
	}

	head := make([]byte, 2)
	binary.BigEndian.PutUint16(head, port)
	if _, err = stm.Write(head); err != nil {
		return nil, err
	}
	result := make([]byte, 1)
	if _, err = stm.Read(result); err != nil {
		return nil, err
	}
	if errno := result[0]; errno != 0 {
		err = Errno(errno)
		return nil, err
	}

	sc := cc.newStreamConn(stm)

	return sc, nil
}

func (cc *clientConn) newStreamConn(stm quic.Stream) *streamConn {
	c := cc.conn

	return &streamConn{
		localAddr:  c.LocalAddr(),
		remoteAddr: c.RemoteAddr(),
		info:       cc.info,
		stm:        stm,
	}
}
