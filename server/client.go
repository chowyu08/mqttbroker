package server

import (
	"net"
	"sync"
	"sync/atomic"
)

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
	srv *Server
	nc  net.Conn
	mu  sync.Mutex
}
type subscription struct {
	client  *client
	subject []byte
	queue   []byte
	sid     []byte
}

func (c *client) initClient() {
	s := c.srv
	c.cid = atomic.AddUint64(&s.gcid, 1)
}
func (c *client) readLoop() {
	if c.nc == nil {
		return
	}
	for {
		buf, err := getMessageBuffer(c.nc)
		if err != nil {
			c.closeConnection()
			return
		}
		c.parse(buf)
	}
}
func (c *client) closeConnection() {

}
