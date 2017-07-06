package server

import (
	"net"
	"sync"
	"sync/atomic"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
)

// Type of client connection.
const (
	// CLIENT is an end user.
	CLIENT = iota
	// ROUTER is another router in the cluster.
	ROUTER
)

type MQInfo *message.ConnectMessage

type client struct {
	typ      int
	cid      uint64
	srv      *Server
	nc       net.Conn
	mu       sync.Mutex
	clientID string
	mqInfo   MQInfo
}
type subscription struct {
	client  *client
	subject []byte
	qos     byte
	queue   bool
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
		srv.clients[c.cid] = c
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
