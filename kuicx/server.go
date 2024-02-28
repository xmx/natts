package kuicx

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
)

type Handler interface {
	Handle(conn quic.Connection)
}

type Server struct {
	Addr             string
	Handler          Handler
	TLSConfig        *tls.Config
	QUICConfig       *quic.Config
	HandshakeTimeout time.Duration

	mutex   sync.Mutex
	closers map[io.Closer]struct{}
	closed  atomic.Bool
}

func (srv *Server) ListenAndServe(ctx context.Context) error {
	if srv.isClosed() {
		return ErrServerClosed
	}
	addr := srv.Addr
	if addr == "" {
		addr = ":https"
	}

	ln, err := quic.ListenAddr(addr, srv.TLSConfig, srv.QUICConfig)
	if err != nil {
		return err
	}

	return srv.Serve(ctx, ln)
}

func (srv *Server) Serve(ctx context.Context, ln *quic.Listener) error {
	closer := srv.newOnceCloser(ln)
	//goland:noinspection GoUnhandledErrorResult
	defer closer.Close()

	if !srv.storeCloser(closer) {
		return ErrServerClosed
	}
	defer srv.removeCloser(closer)

	var tempDelay time.Duration // how long to sleep on accept failure
	for {
		conn, err := ln.Accept(ctx)
		if err != nil {
			if srv.isClosed() {
				return ErrServerClosed
			}

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

			return err
		}

		trunk := srv.newTrunkConn(conn)
		go trunk.serve()
	}
}

func (srv *Server) Close() error {
	if !srv.closed.CompareAndSwap(false, true) {
		return ErrServerClosed
	}

	return srv.closeCloser()
}

func (srv *Server) newTrunkConn(conn quic.Connection) *trunkConn {
	return &trunkConn{conn: conn, srv: srv}
}

func (srv *Server) closeCloser() error {
	var err error
	srv.mutex.Lock()
	defer srv.mutex.Unlock()

	for c := range srv.closers {
		if exx := c.Close(); exx != nil && err == nil {
			err = exx
		}
	}

	return err
}

func (srv *Server) storeCloser(c io.Closer) bool {
	srv.mutex.Lock()
	defer srv.mutex.Unlock()

	if srv.closers == nil {
		srv.closers = make(map[io.Closer]struct{})
	}
	if srv.isClosed() {
		return false
	}
	srv.closers[c] = struct{}{}

	return true
}

func (srv *Server) removeCloser(c io.Closer) {
	srv.mutex.Lock()
	delete(srv.closers, c)
	srv.mutex.Unlock()
}

func (srv *Server) isClosed() bool {
	return srv.closed.Load()
}

func (srv *Server) newOnceCloser(c io.Closer) io.Closer {
	once := sync.OnceValue(c.Close)
	return &onceCloser{once: once}
}

type onceCloser struct {
	once func() error
}

func (oc *onceCloser) Close() error {
	return oc.once()
}
