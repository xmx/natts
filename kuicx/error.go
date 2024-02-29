package kuicx

import (
	"errors"
	"strconv"
)

const (
	_ Errno = iota // successful
	ErrPortUnreachable
	ErrHandshakePacket
)

var (
	ErrServerClosed   = errors.New("vnet: Server closed")
	ErrListenerClosed = errors.New("vnet: Listener closed")

	errnoStrings = []string{
		ErrPortUnreachable: "vnet port unreachable",
		ErrHandshakePacket: "vnet bad handshake packet body",
	}
)

type Errno byte

func (e Errno) Error() string {
	ie := int(e)
	sz := len(errnoStrings)
	if e == 0 || ie > sz {
		str := strconv.FormatInt(int64(sz), 10)
		return "unknown vnet error code: " + str
	}

	return errnoStrings[e]
}
