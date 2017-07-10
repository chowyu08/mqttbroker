package server

import (
	"fmt"
	"net"
	"strings"
	"sync"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
	"github.com/tidwall/gjson"
)

const (
	BROKER_INFO_TOPIC = "broker001info/brokerinfo"
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
	remote   remoteInfo
}
type remoteInfo struct {
	remoteID string
	url      string
}
type subscription struct {
	client  *client
	subject []byte
	qos     byte
	queue   bool
}

func (c *client) SendInfo() {
	infoMsg := message.NewPublishMessage()
	infoMsg.SetTopic([]byte(BROKER_INFO_TOPIC))
	localIP := strings.Split(c.nc.LocalAddr().String(), ":")[0]
	info := fmt.Sprintf(`{"remoteID":"%s","url":"%s"}`, c.srv.ID, localIP)
	log.Info("remoteInfo: ", info)
	infoMsg.SetPayload([]byte(info))
	infoMsg.SetQoS(0)
	infoMsg.SetRetain(false)
	err := c.SendMessage(infoMsg)
	if err != nil {
		log.Error("\tserver/client.go: send info message error, ", err)
	}

}
func (c *client) SendConnect() {
	clientID := GenUniqueId()
	c.clientID = clientID
	connMsg := message.NewConnectMessage()
	connMsg.SetClientId([]byte(clientID))
	connMsg.SetVersion(0x04)
	err := c.SendMessage(connMsg)
	if err != nil {
		log.Error("\tserver/client.go: send connect message error, ", err)
	}
}
func (c *client) SendMessage(msg message.Message) error {
	buf := make([]byte, msg.Len())
	_, err := msg.Encode(buf)
	if err != nil {
		return err
	}
	_, err = c.nc.Write(buf)
	return err
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

func (c *client) ProcessConnAck(buf []byte) {
	ackMsg := message.NewConnackMessage()
	_, err := ackMsg.Decode(buf)
	if err != nil {
		log.Error("\tserver/client.go: Decode Connack Message error: ", err)
		c.closeConnection()
		return
	}
	rc := ackMsg.ReturnCode()
	if rc != message.ConnectionAccepted {
		log.Error("\tserver/client.go: Connect error with the returnCode is: ", rc)
		c.closeConnection()
		return
	}
	//save remote info
	c.srv.remotes[c.clientID] = c
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
		srv.routers[c.clientID] = c
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

func (c *client) ProcessSubscribe(buf []byte) {

	var topics [][]byte
	var qos []byte
	srv := c.srv
	suback := message.NewSubackMessage()
	var retcodes []byte

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
		if IsValidSubject(string(t)) {
			sub := &subscription{
				subject: t,
				qos:     qos[i],
				client:  c,
			}
			err := srv.sl.Insert(sub)
			if err != nil {
				log.Error("\tserver/client.go: Insert subscription error: ", err)
				retcodes = append(retcodes, message.QosFailure)
			}
			retcodes = append(retcodes, qos[i])
		} else {
			retcodes = append(retcodes, message.QosFailure)
		}

	}
	if err := suback.AddReturnCodes(retcodes); err != nil {
		log.Error("\tserver/client.go: return suback error, ", err)
		return
	}
	if c.typ == CLIENT {
		srv.startGoRoutine(func() {
			srv.BroadcastSubscribeMessage(buf)
		})
	}

subback:
	b := make([]byte, suback.Len())
	_, err1 := suback.Encode(b)
	if err1 != nil {
		log.Error("\tserver/client.go: subscribe error,", err1)
	}
	c.nc.Write(buf)
}

func (c *client) ProcessPublish(msg []byte) {
	pubMsg := message.NewPublishMessage()
	_, err := pubMsg.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode Publish Message error: ", err)
		c.closeConnection()
		return
	}
	s := c.srv
	//process info message
	if c.typ == ROUTER && string(pubMsg.Topic()) == BROKER_INFO_TOPIC {
		remoteID := gjson.GetBytes(pubMsg.Payload(), "remoteID").String()
		url := gjson.GetBytes(pubMsg.Payload(), "url").String()
		if remoteID == "" {
			log.Error("\tserver/client.go: receive info message error with remoteID is null")
			return
		}
		s.ValidAndProcessRemoteInfo(remoteID, url)
		return
	}
	//process normal publish message

}
