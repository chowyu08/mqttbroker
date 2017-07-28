package server

import (
	"fmt"
	"net"
	"time"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
	"github.com/tidwall/gjson"
)

func (s *Server) AcceptRoutersLoop() {
	var hp string
	hp = s.info.Cluster.Host + ":" + s.info.Cluster.Port
	log.Info("\tListen on route port: ", hp)

	l, e := net.Listen("tcp", hp)
	if e != nil {
		log.Error("\tserver/server.go: Error listening on ", hp, e)
		return
	}

	tmpDelay := 10 * ACCEPT_MIN_SLEEP
	for {
		conn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Error("\tserver/server.go: Temporary Client Accept Error(%v), sleeping %dms",
					ne, tmpDelay/time.Millisecond)
				time.Sleep(tmpDelay)
				tmpDelay *= 2
				if tmpDelay > ACCEPT_MAX_SLEEP {
					tmpDelay = ACCEPT_MAX_SLEEP
				}
				continue
			} else {
				log.Error("\tserver/server.go: Accept error: %v", err)
				return
			}
		}
		tmpDelay = ACCEPT_MIN_SLEEP
		s.startGoRoutine(func() {
			s.createRoute(conn)
		})
	}
}

func (s *Server) createRoute(conn net.Conn) *client {
	c := &client{srv: s, nc: conn, typ: ROUTER}
	c.initClient()

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return c
	}
	s.mu.Unlock()

	s.startGoRoutine(func() { c.readLoop() })

	return c
}

func (s *Server) createRemote(conn net.Conn, route *Route) *client {
	c := &client{srv: s, nc: conn, typ: REMOTE, route: route}
	c.initClient()

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return c
	}
	s.mu.Unlock()

	s.startGoRoutine(func() { c.readLoop() })

	c.SendConnect()
	c.SendInfo()

	return c
}

func (c *client) ProcessInfo(msg *message.PublishMessage) {

	c.mu.Lock()
	if c.nc == nil {
		c.mu.Unlock()
		return
	}
	s := c.srv
	log.Info("recv remoteInfo: ", string(msg.Payload()))

	rid := gjson.GetBytes(msg.Payload(), "remoteID").String()
	rurl := gjson.GetBytes(msg.Payload(), "url").String()
	isForward := gjson.GetBytes(msg.Payload(), "isForward ").Bool()

	if rid == "" {
		log.Error("\tserver/client.go: receive info message error with remoteID is null")
		return
	}

	if !isForward {
		route := &Route{
			remoteurl: rurl,
			remoteID:  rid,
		}
		c.route = route

		// if info is self.
		if c.route.remoteID == s.ID {
			c.mu.Unlock()
			c.Close()
			return
		}
		info := fmt.Sprintf(`{"remoteID":"%s","url":"%s","isForward ":true}`, rid, rurl)
		msg.SetPayload([]byte(info))
		s.BroadcastInfoMessage(msg)
	}
	// s.ValidAndProcessRemoteInfo(rid, rurl)

}
