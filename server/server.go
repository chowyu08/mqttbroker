package server

import (
	"broker/acl"
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
	DEFAULT_ROUTE_CONNECT = 5 * time.Second
	// DEFAULT_TLS_TIMEOUT
	DEFAULT_TLS_TIMEOUT = 5 * time.Second
)

type Info struct {
	Host    string      `json:"host"`
	Port    string      `json:"port"`
	Cluster ClusterInfo `json:"cluster"`
	TlsInfo TLSInfo     `json:"tlsInfo"`
	WsPath  string      `json:"wsPath"`
	WsPort  string      `json:"wsPort"`
	WsTLS   bool        `json:"wsTLS"`
	TlsHost string      `json:"tlsHost"`
	TlsPort string      `json:"tlsPort"`
	Acl     bool        `json:"acl"`
	AclConf string      `json:"aclConf"`
}

type TLSInfo struct {
	Verify   bool   `json:"verify"`
	CaFile   string `json:"caFile"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type ClusterInfo struct {
	Host    string   `json:"host"`
	Port    string   `json:"port"`
	Routers []string `json:"routers"`
}

type Server struct {
	ID            string
	info          *Info
	running       bool
	mu            sync.Mutex
	listener      net.Listener
	routeListener net.Listener
	TLSConfig     *tls.Config
	AclConfig     *acl.ACLConfig
	clients       ClientMap
	routers       ClientMap
	remotes       ClientMap
	qmu           sync.Mutex
	queues        map[string]int
	sl            *Sublist
	rl            *RetainList
}

func New(info *Info) (*Server, error) {
	server := &Server{
		ID:      GenUniqueId(),
		info:    info,
		clients: NewClientMap(),
		routers: NewClientMap(),
		remotes: NewClientMap(),
		queues:  make(map[string]int),
		sl:      NewSublist(),
		rl:      NewRetainList(),
	}
	if server.info.TlsPort != "" {
		tlsconfig, err := NewTLSConfig(server.info.TlsInfo)
		if err != nil {
			log.Error("new tlsConfig error: ", err)
			return nil, err
		}
		server.TLSConfig = tlsconfig
	}
	if server.info.Acl {
		aclconfig, err := acl.AclConfigLoad(server.info.AclConf)
		if err != nil {
			log.Error("Load acl conf error: ", err)
			return nil, err
		}
		server.AclConfig = aclconfig
	}
	return server, nil
}

func (s *Server) Start() {
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

	if s.info.WsPort != "" {
		s.startGoRoutine(func() {
			s.AcceptWSLoop()
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

	s.startGoRoutine(func() {
		s.StaticInfo()
	})

}

func (s *Server) StaticInfo() {
	timeTicker := time.NewTicker(time.Second * 5)
	for {
		select {
		case <-timeTicker.C:
			// log.Info("client Num: ", len(s.clients), " ,router Num: ", len(s.routers), " ,remote Num: ", len(s.remotes))
			log.Info("client Num: ", s.clients.Count())
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
	for {
		conn, err := net.Dial("tcp", url)
		if err != nil {
			log.Error("Error trying to connect to route: ", err)
			select {
			case <-time.After(DEFAULT_ROUTE_CONNECT):
				log.Debug("Connect to route timeout ,retry...")
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

func (s *Server) createClient(conn net.Conn, tlsRequire bool) {
	c := &client{srv: s, nc: conn, typ: CLIENT, tlsRequired: tlsRequire, isWs: false}
	c.initClient()

	// Re-Grab lock
	c.mu.Lock()

	if c.tlsRequired {
		log.Info("statting TLS Client connection handshake")
		c.nc = tls.Server(c.nc, s.TLSConfig)
		conn := c.nc.(*tls.Conn)

		// Setup the timeout
		time.AfterFunc(DEFAULT_TLS_TIMEOUT, func() { TlsTimeout(c, conn) })
		conn.SetReadDeadline(time.Now().Add(DEFAULT_TLS_TIMEOUT))

		// Force handshake
		c.mu.Unlock()
		if err := conn.Handshake(); err != nil {
			log.Error("TLS handshake error, ", err)
			return
		}
		conn.SetReadDeadline(time.Time{})
		c.mu.Lock()
	}

	// The connection may have been closed
	if c.nc == nil {
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	s.startGoRoutine(func() { c.readLoop() })

	// if c.info.tlsRequire {
	// 	log.Debugf("TLS handshake complete")
	// 	cs := c.nc.(*tls.Conn).ConnectionState()
	// 	// log.Debugf("TLS version %s, cipher suite %s", tlsVersion(cs.Version), tlsCipher(cs.CipherSuite))
	// }

	return

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

func (s *Server) createRoute(conn net.Conn) {
	c := &client{srv: s, nc: conn, typ: ROUTER, isWs: false}
	c.initClient()

	s.startGoRoutine(func() { c.readLoop() })

	return
}

func (s *Server) createRemote(conn net.Conn, route *Route) {
	c := &client{srv: s, nc: conn, typ: REMOTE, route: route, isWs: false}

	c.initClient()
	c.clientID = GenUniqueId()

	//save remote info and send local subs
	// old, exist := srv.remotes.Get(c.clientID)
	// if exist {
	// 	log.Warn("remotes exists, close old...")
	// 	old.Close()
	// }
	s.remotes.Set(c.clientID, c)

	s.startGoRoutine(func() {
		c.readLoop()
	})

	c.SendConnect()
	c.SendInfo()

	s.startGoRoutine(func() {
		c.StartPing()
	})
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
	go f()
}

func (s *Server) removeClient(c *client) {
	cid := c.clientID
	typ := c.typ

	switch typ {
	case CLIENT:
		s.clients.Remove(cid)
	case ROUTER:
		s.routers.Remove(cid)
	case REMOTE:
		s.remotes.Remove(cid)
	}
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

func (s *Server) CheckRemoteExist(remoteID, url string) bool {
	remotes := s.remotes.Items()
	exist := false
	for _, v := range remotes {
		if v.route.remoteUrl == url {
			// if v.route.remoteID == "" || v.route.remoteID != remoteID {
			v.route.remoteID = remoteID
			// }
			exist = true
			break
		}
	}
	return exist
}

func (s *Server) BroadcastInfoMessage(remoteID string, msg message.Message) {
	remotes := s.remotes.Items()
	for _, r := range remotes {
		if r.route.remoteID == remoteID {
			continue
		}
		r.writeMessage(msg)
	}
	// log.Info("BroadcastInfoMessage success ")
}

func (s *Server) BroadcastSubscribeMessage(buf []byte) {

	remotes := s.remotes.Items()
	for _, r := range remotes {
		r.writeBuffer(buf)
	}
	// log.Info("BroadcastSubscribeMessage remotes: ", s.remotes)
}

func (s *Server) BroadcastUnSubscribeMessage(buf []byte) {
	// log.Info("remotes: ", s.remotes)
	remotes := s.remotes.Items()
	for _, r := range remotes {
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

	remotes := s.remotes.Items()
	for _, r := range remotes {
		r.writeMessage(ubsub)
	}
}

func (s *Server) SendLocalSubsToRouter(c *client) {

	clients := s.clients.Items()
	subMsg := message.NewSubscribeMessage()
	for _, client := range clients {
		client.mu.Lock()
		subs := client.subs
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

	err := c.writeMessage(subMsg)
	if err != nil {
		log.Error("Send localsubs To Router error :", err)
	}
}
