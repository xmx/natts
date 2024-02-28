package kuicx

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
)

type streamListener struct {
	mux    *StreamMux
	port   uint16
	addr   net.Addr
	queue  chan *streamConn
	closed atomic.Bool
	ctx    context.Context
	cancel context.CancelFunc
}

func (sl *streamListener) Accept() (net.Conn, error) {
	select {
	case stm := <-sl.queue:
		return stm, nil
	case <-sl.ctx.Done():
		return nil, &net.OpError{
			Op:   "accept",
			Net:  "vnet",
			Addr: sl.addr,
			Err:  ErrListenerClosed,
		}
	}
}

func (sl *streamListener) Close() error {
	if !sl.closed.CompareAndSwap(false, true) {
		return &net.OpError{
			Op:   "close",
			Net:  "vnet",
			Addr: sl.addr,
			Err:  ErrListenerClosed,
		}
	}

	sl.cancel()
	if mux := sl.mux; mux != nil {
		mux.unregisterListener(sl.port)
	}

	return nil
}

func (sl *streamListener) Addr() net.Addr {
	return sl.addr
}

func (sl *streamListener) establish(sc *streamConn) error {
	opErr := &net.OpError{
		Op:   "establish",
		Net:  "vnet",
		Addr: sl.addr,
	}
	if sc == nil {
		opErr.Err = errors.New("establish conn or stream is nil")
		return opErr
	}

	// 先判断 Listener 是否已经关闭
	opErr.Err = ErrListenerClosed
	if sl.closed.Load() {
		return opErr
	}

	select {
	case sl.queue <- sc:
		return nil
	case <-sl.ctx.Done():
		return opErr
	}
}
