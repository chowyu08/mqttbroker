package server

import "net"

// Type of client connection.
const (
	// CLIENT is an end user.
	CLIENT = iota
	// ROUTER is another router in the cluster.
	ROUTER
)

type client struct {
	typ int
	cid uint64
	srv Server
	nc  net.Conn
}
type subscription struct {
	client  *client
	subject []byte
	queue   []byte
	sid     []byte
}
