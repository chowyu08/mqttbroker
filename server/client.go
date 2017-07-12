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
)
const (
	// CLIENT is an end user.
	CLIENT = 0
	// ROUTER is another router in the cluster.
	ROUTER = 1
	//REMOTE is the router connect to other cluster
	REMOTE = 2
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
	subs     map[string]*subscription
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
	ipaddr := localIP + ":" + c.srv.info.Cluster.Port
	info := fmt.Sprintf(`{"remoteID":"%s","url":"%s"}`, c.srv.ID, ipaddr)
	log.Info("remoteInfo: ", info)
	infoMsg.SetPayload([]byte(info))
	infoMsg.SetQoS(0)
	infoMsg.SetRetain(false)
	err := c.writeMessage(infoMsg)
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
	err := c.writeMessage(connMsg)
	if err != nil {
		log.Error("\tserver/client.go: send connect message error, ", err)
	}
}

func (c *client) initClient() {
	c.subs = make(map[string]*subscription)
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
	//save remote info and send local subs
	s := c.srv
	s.remotes[c.clientID] = c
	s.startGoRoutine(func() {
		s.SendLocalSubsToRouter(c)
	})
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
	err1 := c.writeMessage(connack)
	if err1 != nil {
		log.Error("\tserver/client.go: send connack error, ", err1)
	}
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
		if c.subs[string(t)] == nil {
			sub := &subscription{
				subject: t,
				qos:     qos[i],
				client:  c,
				queue:   false,
			}
			c.subs[string(t)] = sub
			err := srv.sl.Insert(sub)
			if err != nil {
				log.Error("\tserver/client.go: Insert subscription error: ", err)
				retcodes = append(retcodes, message.QosFailure)
			}
		} else {
			//if exist ,check whether qos change
			c.subs[string(t)].qos = qos[i]
		}
		retcodes = append(retcodes, qos[i])

	}
	if err := suback.AddReturnCodes(retcodes); err != nil {
		log.Error("\tserver/client.go: add return suback code error, ", err)
		return
	}
	if c.typ == CLIENT {
		srv.startGoRoutine(func() {
			srv.BroadcastSubscribeMessage(buf)
		})
	}

subback:
	err1 := c.writeMessage(suback)
	if err1 != nil {
		log.Error("\tserver/client.go: send suback error, ", err1)
	}
}

func (c *client) ProcessUnSubscribe(msg []byte) {
	unsub := message.NewUnsubscribeMessage()
	_, err := unsub.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode UnSubscribe Message error: ", err)
		c.closeConnection()
		return
	}
	topics := unsub.Topics()
	for _, t := range topics {
		//DO UnSub
		log.Info(t)
	}

	resp := message.NewUnsubackMessage()
	resp.SetPacketId(unsub.PacketId())

	err1 := c.writeMessage(resp)
	if err1 != nil {
		log.Error("\tserver/client.go: send ubsuback error, ", err1)
	}
}

func (c *client) ProcessPublish(msg []byte) {
	pubMsg := message.NewPublishMessage()
	_, err := pubMsg.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode Publish Message error: ", err)
		c.closeConnection()
		return
	}
	topic := string(pubMsg.Topic())
	s := c.srv
	//process info message
	if c.typ == ROUTER && topic == BROKER_INFO_TOPIC {
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
	c.ProcessPublishMessage(msg, topic)
	// switch pubMsg.QoS() {
	// case message.QosExactlyOnce:
	// 	resp := message.NewPubrecMessage()
	// 	resp.SetPacketId(pubMsg.PacketId())
	// 	err := c.SendMessage(resp)
	// 	if err != nil {
	// 		log.Error("\tserver/client.go: send pubrec error, ", err)
	// 	}
	// case message.QosAtLeastOnce:
	// 	resp := message.NewPubackMessage()
	// 	resp.SetPacketId(pubMsg.PacketId())

	// 	if err := c.SendMessage(resp); err != nil {
	// 		log.Error("\tserver/client.go: send puback error, ", err)
	// 	}
	// 	c.ProcessPublishMessage(msg, topic)

	// case message.QosAtMostOnce:
	// 	c.ProcessPublishMessage(msg, topic)
	// }
}
func (c *client) ProcessPublishMessage(buf []byte, topic string) {

	s := c.srv
	r := s.sl.Match(topic)
	if len(r.qsubs) == 0 && len(r.psubs) == 0 {
		return
	}

	for _, sub := range r.psubs {
		if sub.client.typ == ROUTER {
			if c.typ == ROUTER {
				continue
			}
		}

		s.startGoRoutine(func() {
			err := sub.client.writeBuffer(buf)
			if err != nil {
				log.Error("\tserver/client.go: process message error, the clientID is ", sub.client.clientID)
			}
		})

	}
	// for _, qsub := range r.qsubs {
	// 	if qsub.client.typ == ROUTER {
	// 		if c.typ == ROUTER {
	// 			continue
	// 		}
	// 		s.startGoRoutine(func() {
	// 			_, err := qsub.client.nc.Write(buf)
	// 			if err != nil {
	// 				log.Error("\tserver/client.go: process message error, the clientID is ", qsub.client.clientID)
	// 			}
	// 		})
	// 	}
	// }
}
func (c *client) writeBuffer(buf []byte) error {
	_, err := c.nc.Write(buf)
	return err
}

func (c *client) writeMessage(msg message.Message) error {
	buf := make([]byte, msg.Len())
	_, err := msg.Encode(buf)
	if err != nil {
		return err
	}
	return c.writeBuffer(buf)
}

func (c *client) ProcessPing() {
	respMsg := message.NewPingrespMessage()
	c.writeMessage(respMsg)
}
