package server

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	log "github.com/cihub/seelog"
)

const (
	// ACCEPT_MIN_SLEEP is the minimum acceptable sleep times on temporary errors.
	ACCEPT_MIN_SLEEP = 100 * time.Millisecond

	// ACCEPT_MAX_SLEEP is the maximum acceptable sleep times on temporary errors
	ACCEPT_MAX_SLEEP = 10 * time.Second
)

type Info struct {
	ID          string
	Port        uint64
	ClusterPort uint64
	TLSRequired bool
	TLSVerify   bool
}

type Server struct {
	gcid          uint64
	grid          uint64
	info          Info
	running       bool
	mu            sync.Mutex
	listener      net.Listener
	routeListener net.Listener
	clients       map[uint64]*client
	routes        map[uint64]*client
	sl            *Sublist
}

func New() *Server {
	info := Info{
		ID:          GenUniqueId(),
		Port:        1883,
		ClusterPort: 1993,
		TLSVerify:   false,
		TLSRequired: false,
	}
	return &Server{
		info:    info,
		clients: make(map[uint64]*client),
		sl:      NewSublist(),
	}
}

func (s *Server) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	if s.info.Port != 0 {
		s.startGoRoutine(func() {
			s.AcceptLoop(CLIENT)
		})
	}
	if s.info.ClusterPort != 0 {
		s.startGoRoutine(func() {
			s.AcceptLoop(ROUTER)
		})
	}
	<-make(chan bool)

}
func (s *Server) AcceptLoop(typ int) {
	var hp string
	if typ == CLIENT {
		hp = ":" + strconv.FormatUint(s.info.Port, 10)
	} else if typ == ROUTER {
		hp = ":" + strconv.FormatUint(s.info.ClusterPort, 10)
	}

	l, e := net.Listen("tcp", hp)
	if e != nil {
		log.Error("\tserver/server.go: Error listening on port: %s, %q", hp, e)
		return
	}
	log.Info("\tListen on port: ", hp)
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
			} else if s.running {
				log.Error("\tserver/server.go: Accept error: %v", err)
			}
			continue
		}
		tmpDelay = ACCEPT_MIN_SLEEP
		s.startGoRoutine(func() {
			s.createClient(conn, typ)
		})
	}
}
func (s *Server) createClient(conn net.Conn, typ int) *client {
	c := &client{srv: s, nc: conn, typ: typ}
	c.initClient()
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return c
	}
	// s.clients[c.cid] = c
	s.mu.Unlock()

	// Re-Grab lock
	c.mu.Lock()
	if c.typ != ROUTER {
		tlsRequired := s.info.TLSRequired
		if tlsRequired {
			// c.nc = tls.Server(c.nc, s.opts.TLSConfig)
			// conn := c.nc.(*tls.Conn)

			// // Setup the timeout
			// ttl := secondsToDuration(s.opts.TLSTimeout)
			// time.AfterFunc(ttl, func() { tlsTimeout(c, conn) })
			// conn.SetReadDeadline(time.Now().Add(ttl))

			// // Force handshake
			// c.mu.Unlock()
			// if err := conn.Handshake(); err != nil {
			// 	c.Debugf("TLS handshake error: %v", err)
			// 	c.sendErr("Secure Connection - TLS Required")
			// 	c.closeConnection()
			// 	return nil
			// }
		}
	}

	// The connection may have been closed
	if c.nc == nil {
		c.mu.Unlock()
		return c
	}
	s.startGoRoutine(func() { c.readLoop() })
	c.mu.Unlock()
	return c

}
func (s *Server) startGoRoutine(f func()) {
	go f()
}
func GenUniqueId() string {
	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	h := md5.New()
	h.Write([]byte(base64.URLEncoding.EncodeToString(b)))
	return hex.EncodeToString(h.Sum(nil))
	// return GetMd5String()
}
