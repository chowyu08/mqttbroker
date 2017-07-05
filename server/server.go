package server

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net"
	"sync"
)

type Info struct {
	ID          string
	Port        uint64
	ClusterPort uint64
	TLSRequired bool
	TLSVerify   bool
}

type Server struct {
	info          Info
	running       bool
	mu            sync.Mutex
	listener      net.Listener
	routeListener net.Listener
	clients       map[uint64]client
	sl            Sublist
}

func New() *Server {
	info = Info{
		ID:          GenUniqueId(),
		Port:        1883,
		ClusterPort: 1993,
		TLSVerify:   false,
		TLSRequired: false,
	}
	return &Server{
		info:    info,
		clients: make(map[uint64]client),
		sl:      NewSublist(),
	}
}

func (s *Server) Start() {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()
	if s.info.ClusterPort != 0 {
		s.startGoRoutine(func() {
			s.StartRouting()
		})
	}
}
func (s *Server) startGoRoutine(f func()) {
	// s.grMu.Lock()
	// if s.grRunning {
	// 	s.grWG.Add(1)
	go f()
	// }
	// s.grMu.Unlock()
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
