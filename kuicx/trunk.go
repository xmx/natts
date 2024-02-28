package kuicx

import "github.com/quic-go/quic-go"

type trunkConn struct {
	srv  *Server
	conn quic.Connection
}

func (tc *trunkConn) serve() {
}
