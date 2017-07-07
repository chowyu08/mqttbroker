package server

import (
	"net"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
)

// Type of client connection.
const (
	// CLIENT is an end user.
	CLIENT = iota
	// ROUTER is another router in the cluster.
	ROUTER
	//REMOTE is the router connect to other cluster
	REMOTE
)

type MQInfo *message.ConnectMessage

type client struct {
	typ      int
	srv      *Server
	nc       net.Conn
	mu       sync.Mutex
	clientID string
	mqInfo   MQInfo
	route    *route
}
type route struct {
	remoteID string
	url      string
}
type subscription struct {
	client  *client
	subject []byte
	qos     byte
	queue   bool
}

func (c *client) initClient() {
}
func (c *client) readLoop() {
	if c.nc == nil {
		return
	}
	for {
		buf, err := getMessageBuffer(c.nc)
		if err != nil {
			c.closeConnection()
			break
		}
		c.parse(buf)
	}
}
func (c *client) closeConnection() {
	if c.nc != nil {
		c.nc.Close()
	}
}
func (c *client) ProcessSubscribe(buf []byte) {

	var topics [][]byte
	var qos []byte
	srv := c.srv

	suback := message.NewSubackMessage()
	msg := message.NewSubscribeMessage()
	_, err := msg.Decode(buf)

	if err != nil {
		log.Error("\tserver/client.go: Decode Subscribe Message error: ", err)
		suback.AddReturnCode(message.QosFailure)
		goto subback
	}
	topics = msg.Topics()
	qos = msg.Qos()

	suback.SetPacketId(msg.PacketId())

	for i, t := range topics {
		sub := &subscription{
			subject: t,
			qos:     qos[i],
			client:  c,
		}
		err := srv.sl.Insert(sub)
		if err != nil {
			log.Error("\tserver/client.go: Insert subscription error: ", err)
			suback.AddReturnCode(message.QosFailure)
			goto subback
		}
	}

subback:
	b := make([]byte, suback.Len())
	_, err1 := suback.Encode(b)
	if err1 != nil {
		log.Error("\tserver/client.go: subscribe error,", err1)
	}
	c.nc.Write(buf)
}

func (c *client) ProcessConnect(msg []byte) {
	connMsg := message.NewConnectMessage()
	_, err := connMsg.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode Connection Message error: ", err)
		c.closeConnection()
		return
	}
	srv := c.srv
	connack := message.NewConnackMessage()

	if version := connMsg.Version(); version != 0x04 && version != 0x03 {
		connack.SetReturnCode(message.ErrInvalidProtocolVersion)
		goto connback
	}
	if connMsg.WillFlag() {
		//do will topic
	}
	c.mqInfo = connMsg
	c.clientID = string(connMsg.ClientId())

	if c.typ == CLIENT {
		srv.clients[c.clientID] = c
	}
	if c.typ == ROUTER {
		srv.startGoRoutine(func() {
			srv.routers[c.clientID] = c
			remoteID := string(connMsg.Username())
			remoteURL := string(connMsg.Password())
		})

	}
	connack.SetReturnCode(message.ConnectionAccepted)
connback:
	buf := make([]byte, connack.Len())
	_, err1 := connack.Encode(buf)
	if err1 != nil {
		//glog.Debugf("Write error: %v", err)
		log.Error("\tserver/client.go: connect error,", err1)
	}
	c.nc.Write(buf)
}
