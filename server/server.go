package server

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
)

const (
	// ACCEPT_MIN_SLEEP is the minimum acceptable sleep times on temporary errors.
	ACCEPT_MIN_SLEEP = 100 * time.Millisecond
	// ACCEPT_MAX_SLEEP is the maximum acceptable sleep times on temporary errors
	ACCEPT_MAX_SLEEP = 10 * time.Second
	// DEFAULT_ROUTE_CONNECT Route solicitation intervals.
	DEFAULT_ROUTE_CONNECT = 2 * time.Second
	// DEFAULT_TLS_TIMEOUT
	DEFAULT_TLS_TIMEOUT = 5 * time.Second
)

type Info struct {
	Host      string
	Port      string
	Cluster   ClusterInfo
	TlsInfo   TLSInfo
	TlsHost   string
	TlsPort   string
	TLSConfig *tls.Config
}
type TLSInfo struct {
	Verify   bool
	CaFile   string
	CertFile string
	KeyFile  string
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
	gmu           sync.Mutex
	listener      net.Listener
	routeListener net.Listener
	clients       map[string]*client
	routers       map[string]*client
	remotes       map[string]*client
	queues        map[string]int
	sl            *Sublist
	rl            *RetainList
}

func New(info *Info) *Server {
	return &Server{
		ID:      GenUniqueId(),
		info:    info,
		clients: make(map[string]*client),
		routers: make(map[string]*client),
		remotes: make(map[string]*client),
		queues:  make(map[string]int),
		sl:      NewSublist(),
		rl:      NewRetainList(),
	}
}

func (s *Server) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	if s.info.Port != "" {
		s.startGoRoutine(func() {
			s.AcceptClientsLoop(false)
		})
	}

	if s.info.TlsPort != "" {
		s.startGoRoutine(func() {
			s.AcceptClientsLoop(true)
		})
	}

	if s.info.Cluster.Port != "" {
		s.startGoRoutine(func() {
			s.AcceptRoutersLoop()
		})
	}
	if len(s.info.Cluster.Routers) > 0 {
		s.startGoRoutine(func() {
			s.ConnectToRouters()
		})
	}

	go s.StaticInfo()
	// <-make(chan bool)

}

func (s *Server) StaticInfo() {
	timeTicker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-timeTicker.C:
			// log.Info("client Num: ", len(s.clients), " ,router Num: ", len(s.routers), " ,remote Num: ", len(s.remotes))
			log.Info("client Num: ", len(s.clients))
		}
	}
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
			log.Error("Error trying to connect to route: ", err)
			select {
			case <-time.After(DEFAULT_ROUTE_CONNECT):
				log.Debug("\tserver/server.go:Connect to route timeout ,retry...")
				continue
			}
		}
		route := &Route{
			tlsRequired: false,
			remoteID:    remoteID,
			remoteUrl:   url,
		}
		s.createRemote(conn, route)
		return
	}
}

func (s *Server) AcceptClientsLoop(tlsRequire bool) {
	var hp string

	if tlsRequire {
		hp = s.info.TlsHost + ":" + s.info.TlsPort
		log.Info("\tListen clients on tls port: ", hp)
	} else {
		hp = s.info.Host + ":" + s.info.Port
		log.Info("\tListen clients on port: ", hp)
	}

	l, e := net.Listen("tcp", hp)
	if e != nil {
		log.Error("Error listening on ", hp, e)
		return
	}

	tmpDelay := 10 * ACCEPT_MIN_SLEEP
	for {
		conn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Error("Temporary Client Accept Error(%v), sleeping %dms",
					ne, tmpDelay/time.Millisecond)
				time.Sleep(tmpDelay)
				tmpDelay *= 2
				if tmpDelay > ACCEPT_MAX_SLEEP {
					tmpDelay = ACCEPT_MAX_SLEEP
				}
			} else {
				log.Error("Accept error: %v", err)
			}
			continue
		}
		tmpDelay = ACCEPT_MIN_SLEEP
		s.startGoRoutine(func() {
			s.createClient(conn, tlsRequire)
		})
	}
}

func (s *Server) createClient(conn net.Conn, tlsRequire bool) *client {
	c := &client{srv: s, nc: conn, typ: CLIENT, tlsRequired: tlsRequire}
	c.initClient()

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return c
	}
	s.mu.Unlock()

	// Re-Grab lock
	c.mu.Lock()

	if c.tlsRequired {
		log.Info("statting TLS Client connection handshake")
		c.nc = tls.Server(c.nc, s.info.TLSConfig)
		conn := c.nc.(*tls.Conn)

		// Setup the timeout
		time.AfterFunc(DEFAULT_TLS_TIMEOUT, func() { TlsTimeout(c, conn) })
		conn.SetReadDeadline(time.Now().Add(DEFAULT_TLS_TIMEOUT))

		// Force handshake
		c.mu.Unlock()
		if err := conn.Handshake(); err != nil {
			log.Error("TLS handshake error, ", err)
			return nil
		}
		conn.SetReadDeadline(time.Time{})
		c.mu.Lock()
	}

	// The connection may have been closed
	if c.nc == nil {
		c.mu.Unlock()
		return c
	}

	s.startGoRoutine(func() { c.readLoop() })

	// if c.info.tlsRequire {
	// 	log.Debugf("TLS handshake complete")
	// 	cs := c.nc.(*tls.Conn).ConnectionState()
	// 	// log.Debugf("TLS version %s, cipher suite %s", tlsVersion(cs.Version), tlsCipher(cs.CipherSuite))
	// }

	c.mu.Unlock()
	return c

}

func (s *Server) AcceptRoutersLoop() {
	var hp string
	hp = s.info.Cluster.Host + ":" + s.info.Cluster.Port
	log.Info("\tListen on route port: ", hp)

	l, e := net.Listen("tcp", hp)
	if e != nil {
		log.Error("\tserver/router.go: Error listening on ", hp, e)
		return
	}

	tmpDelay := 10 * ACCEPT_MIN_SLEEP
	for {
		conn, err := l.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				log.Error("\tserver/router.go: Temporary Client Accept Error(%v), sleeping %dms",
					ne, tmpDelay/time.Millisecond)
				time.Sleep(tmpDelay)
				tmpDelay *= 2
				if tmpDelay > ACCEPT_MAX_SLEEP {
					tmpDelay = ACCEPT_MAX_SLEEP
				}
			} else {
				log.Error("\tserver/router.go: Accept error: %v", err)
			}
			continue
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

	s.startGoRoutine(func() {
		c.readLoop()
	})

	c.SendConnect()
	c.SendInfo()

	s.startGoRoutine(func() {
		c.StartPing()
	})

	return c
}

func TlsTimeout(c *client, conn *tls.Conn) {
	nc := c.nc
	// Check if already closed
	if nc == nil {
		return
	}
	cs := conn.ConnectionState()
	if !cs.HandshakeComplete {
		log.Error("TLS handshake timeout")
		c.Close()
	}
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
	s.gmu.Lock()
	if s.running {
		go f()
	}
	s.gmu.Unlock()
}

func (s *Server) removeClient(c *client) {
	cid := c.clientID
	typ := c.typ

	s.mu.Lock()
	switch typ {
	case CLIENT:
		delete(s.clients, cid)
	case ROUTER:
		delete(s.routers, cid)
	case REMOTE:
		delete(s.remotes, cid)
	}
	s.mu.Unlock()
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
	s.mu.Lock()
	exist := false
	for _, v := range s.remotes {
		if v.route.remoteUrl == url {
			if v.route.remoteID == "" || v.route.remoteID != remoteID {
				v.route.remoteID = remoteID
			}
			exist = true
		}
	}
	s.mu.Unlock()
	if !exist {
		s.startGoRoutine(func() {
			s.connectRouter(url, remoteID)
		})
	}
	// log.Info("ValidAndProcessRemoteInfo success ")
}

func (s *Server) BroadcastInfoMessage(remoteID string, msg message.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.remotes {
		if r.route.remoteID == remoteID {
			continue
		}
		r.writeMessage(msg)
	}
	// log.Info("BroadcastInfoMessage success ")
}

func (s *Server) BroadcastSubscribeMessage(buf []byte) {

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.remotes {
		r.writeBuffer(buf)
	}
	// log.Info("BroadcastSubscribeMessage remotes: ", s.remotes)
}

func (s *Server) BroadcastUnSubscribeMessage(buf []byte) {
	// log.Info("remotes: ", s.remotes)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.remotes {
		r.writeBuffer(buf)
	}
	// log.Info("BroadcastUnSubscribeMessage remotes: ", s.remotes)
}

func (s *Server) BroadcastUnSubscribe(sub *subscription) {

	var topic []byte
	if sub.queue {
		front := []byte("$queue/")
		topic = append(front, sub.topic...)
	}
	ubsub := message.NewUnsubscribeMessage()
	ubsub.AddTopic(topic)

	s.mu.Lock()
	for _, r := range s.remotes {
		r.writeMessage(ubsub)
	}
	s.mu.Unlock()
}

func (s *Server) SendLocalSubsToRouter(c *client) {
	s.mu.Lock()
	if len(s.clients) < 1 {
		s.mu.Unlock()
		return
	}
	subMsg := message.NewSubscribeMessage()
	for _, client := range s.clients {
		client.mu.Lock()
		subs := make([]*subscription, 0, len(client.subs))
		for _, sub := range client.subs {
			subs = append(subs, sub)
		}
		client.mu.Unlock()
		for _, sub := range subs {
			var topic []byte
			if sub.queue {
				front := []byte("$queue/")
				topic = append(front, sub.topic...)
			}
			subMsg.AddTopic(topic, sub.qos)
		}
	}
	s.mu.Unlock()

	err := c.writeMessage(subMsg)
	if err != nil {
		log.Error("Send localsubs To Router error :", err)
	}
}
