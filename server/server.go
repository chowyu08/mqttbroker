package server

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net"
	"sync"
	"time"

	log "github.com/cihub/seelog"
)

const (
	// ACCEPT_MIN_SLEEP is the minimum acceptable sleep times on temporary errors.
	ACCEPT_MIN_SLEEP = 100 * time.Millisecond
	// ACCEPT_MAX_SLEEP is the maximum acceptable sleep times on temporary errors
	ACCEPT_MAX_SLEEP = 10 * time.Second
	// DEFAULT_ROUTE_CONNECT Route solicitation intervals.
	DEFAULT_ROUTE_CONNECT = 2 * time.Second
)

type Info struct {
	Host    string
	Port    string
	Cluster ClusterInfo
	TLS     TLSConfig
}
type TLSConfig struct {
	Host        string
	Port        string
	TLSVerify   bool
	TLSRequired bool
	CaFile      string
	CertFile    string
	KeyFile     string
}
type ClusterInfo struct {
	Host    string
	Port    string
	Routers []string
}

type Server struct {
	ID            string
	info          *Info
	running       bool
	mu            sync.Mutex
	listener      net.Listener
	routeListener net.Listener
	clients       map[string]*client
	routers       map[string]*client
	remotes       map[string]*client
	sl            *Sublist
}

func New(info *Info) *Server {
	return &Server{
		ID:      GenUniqueId(),
		info:    info,
		clients: make(map[string]*client),
		routers: make(map[string]*client),
		remotes: make(map[string]*client),
		sl:      NewSublist(),
	}
}

func (s *Server) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	if s.info.Port != "" {
		s.startGoRoutine(func() {
			s.AcceptLoop(CLIENT)
		})
	}
	if s.info.Cluster.Port != "" {
		s.startGoRoutine(func() {
			s.AcceptLoop(ROUTER)
		})
	}
	if len(s.info.Cluster.Routers) > 0 {
		s.startGoRoutine(func() {
			s.ConnectToRouters()
		})
	}
	<-make(chan bool)

}
func (s *Server) ConnectToRouters() {
	for i := 0; i < len(s.info.Cluster.Routers); i++ {
		url := s.info.Cluster.Routers[i]
		s.startGoRoutine(func() {
			s.connectRouter(url, "")
		})
	}
}
func (s *Server) connectRouter(url, remoteID string) {
	for s.running {
		conn, err := net.Dial("tcp", url)
		if err != nil {
			log.Error("\tserver/server.go: Error trying to connect to route: ", err)
			select {
			case <-time.After(DEFAULT_ROUTE_CONNECT):
				continue
			}
		}
		s.createClient(conn, REMOTE, url, remoteID)
		return
	}
}
func (s *Server) AcceptLoop(typ int) {
	var hp string
	if typ == CLIENT {
		hp = s.info.Host + ":" + s.info.Port
		log.Info("\tListen on client port: ", hp)
	} else if typ == ROUTER {
		hp = s.info.Cluster.Host + ":" + s.info.Cluster.Port
		log.Info("\tListen on cluster port: ", hp)
	}
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
			} else if s.running {
				log.Error("\tserver/server.go: Accept error: %v", err)
			}
			continue
		}
		tmpDelay = ACCEPT_MIN_SLEEP
		s.startGoRoutine(func() {
			s.createClient(conn, typ, "", "")
		})
	}
}
func (s *Server) createClient(conn net.Conn, typ int, url, remoteID string) *client {
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
	if c.typ == CLIENT {
		tlsRequired := s.info.TLS.TLSRequired
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
	if c.typ == REMOTE {
		c.remote.url = url
		c.remote.remoteID = remoteID
		c.SendConnect()
		c.SendInfo()
	}
	s.startGoRoutine(func() { c.readLoop() })
	c.mu.Unlock()
	return c

}
func (s *Server) ReadLocalBrokerIP() []string {
	var ip net.IP
	urls := make([]string, 0, 1)
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// Skip non global unicast addresses
			if !ip.IsGlobalUnicast() || ip.IsUnspecified() {
				ip = nil
				continue
			}
			urls = append(urls, net.JoinHostPort(ip.String(), s.info.Cluster.Port))
		}
	}
	return urls
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

func (s *Server) ValidAndProcessRemoteInfo(remoteID, url string) {
	exist := false
	for _, v := range s.remotes {
		if v.remote.url == url {
			if v.remote.remoteID == "" || v.remote.remoteID != remoteID {
				v.remote.remoteID = remoteID
			}
			exist = true
		}
	}
	if !exist {
		s.startGoRoutine(func() {
			s.connectRouter(url, remoteID)
		})
	}
}
func (s *Server) BroadcastSubscribeMessage(buf []byte) {
	// log.Info("remotes: ", s.remotes)
	for _, r := range s.remotes {
		r.nc.Write(buf)
	}
}
