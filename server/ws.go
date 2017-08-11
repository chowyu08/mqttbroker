package server

import (
	"net"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	log "github.com/cihub/seelog"
)

func (s *Server) AcceptWSLoop() {
	r := mux.NewRouter()
	r.HandleFunc(s.info.WsPath, s.HandleWS).Methods("GET")
	log.Info("\tListen websocket on port: ", s.info.WsPort)
	if s.info.WsTLS {
		http.ListenAndServeTLS(":"+s.info.WsPort, s.info.TlsInfo.CertFile, s.info.TlsInfo.KeyFile, r)
	} else {
		http.ListenAndServe(":"+s.info.WsPort, r)
	}

}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, e := websocket.Upgrade(w, r, nil, 1024, 1024)
	if _, ok := e.(websocket.HandshakeError); ok {
		log.Error("not a websocket Handshake")
		return
	} else if e != nil {
		log.Error("setup ws connection error: ", e)
		return
	}
	s.startGoRoutine(func() {
		s.createWsClient(conn)
	})
}

func (s *Server) createWsClient(conn *websocket.Conn) *client {
	c := &client{srv: s, wsConn: conn, typ: CLIENT, tlsRequired: s.info.WsTLS, isWs: true}
	c.initClient()

	s.startGoRoutine(func() { c.wsreadLoop() })
	return c
}

func (c *client) wsreadLoop() {
	ws := c.wsConn
	keeplive := c.keepAlive
	if ws == nil {
		return
	}
	first := true
	lastIn := uint16(time.Now().Unix())
	for {
		nowTime := uint16(time.Now().Unix())
		if 0 != keeplive && nowTime-lastIn > keeplive*3/2 {
			log.Error("Client has exceeded timeout, disconnecting.")
			c.Close()
			return
		}
		var buf []byte
		typ, playload, err := ws.ReadMessage()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				// log.Error("read timeout")
				continue
			}
			log.Error("read buf err: ", err)
			c.Close()
			return
		}
		switch typ {
		case websocket.BinaryMessage:
			buf = playload
		case websocket.TextMessage:
			buf = playload
		case websocket.CloseMessage:
			c.Close()
			return
		default:
			continue
		}

		lastIn = uint16(time.Now().Unix())
		if first {
			if c.typ == CLIENT {
				c.ProcessConnect(buf)
				first = false
				continue
			}
		}
		c.parse(buf)

		ws := c.wsConn

		if ws == nil {
			return
		}
	}
}
