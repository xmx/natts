package kuicx

import (
	"context"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

type contextKey struct {
	name string
}

var vnetClientInfoKey = &contextKey{name: "vnet-client-key"}

type infoConn struct {
	conn quic.Connection
	info *ClientInfo
}

type streamConn struct {
	localAddr  net.Addr
	remoteAddr net.Addr
	info       *ClientInfo
	stm        quic.Stream
}

func (sc *streamConn) Read(b []byte) (int, error) {
	return sc.stm.Read(b)
}

func (sc *streamConn) Write(b []byte) (int, error) {
	return sc.stm.Write(b)
}

func (sc *streamConn) Close() error {
	return sc.stm.Close()
}

func (sc *streamConn) LocalAddr() net.Addr {
	return sc.localAddr
}

func (sc *streamConn) RemoteAddr() net.Addr {
	return sc.remoteAddr
}

func (sc *streamConn) SetDeadline(t time.Time) error {
	return sc.stm.SetDeadline(t)
}

func (sc *streamConn) SetReadDeadline(t time.Time) error {
	return sc.stm.SetReadDeadline(t)
}

func (sc *streamConn) SetWriteDeadline(t time.Time) error {
	return sc.stm.SetWriteDeadline(t)
}

func FromConn(c net.Conn) *ClientInfo {
	if sc, _ := c.(*streamConn); sc != nil {
		return sc.info
	}
	return nil
}

func FromContext(ctx context.Context) *ClientInfo {
	if ctx == nil {
		return nil
	}

	val := ctx.Value(vnetClientInfoKey)
	if sc, _ := val.(*streamConn); sc != nil {
		return sc.info
	}

	return nil
}

func ConnContext(ctx context.Context, c net.Conn) context.Context {
	if _, ok := c.(*streamConn); ok {
		return context.WithValue(ctx, vnetClientInfoKey, c)
	}

	return ctx
}
