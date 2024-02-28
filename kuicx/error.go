package kuicx

import (
	"errors"
	"strconv"
)

const (
	_ DialErrno = iota // successful
	ErrPortUnreachable
	ErrHandshakePacket
	ErrEstablished
)

var (
	ErrServerClosed   = errors.New("vnet: Server closed")
	ErrListenerClosed = errors.New("vnet: Listener closed")
)

type DialErrno byte

func (d DialErrno) Error() string {
	if d == 0 {
		return "<nil>"
	} else if errors.Is(d, ErrPortUnreachable) {
		return "port unreachable"
	} else if errors.Is(d, ErrHandshakePacket) {
		return "handshake packet"
	} else if errors.Is(d, ErrEstablished) {
		return "establish failed"
	}
	str := strconv.FormatInt(int64(d), 10)

	return "unknown errno: " + str
}
