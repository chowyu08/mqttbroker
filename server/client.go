package server

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
	"github.com/tidwall/gjson"
)

const (
	outgoing = "out"
	incoming = "in"
)

const (
	startBufSize = 512
	// special pub topic for cluster info BrokerInfoTopic
	BrokerInfoTopic = "broker001info/brokerinfo"
	// DEFAULT_FLUSH_DEADLINE is the write/flush deadlines.

	DEFAULT_FLUSH_DEADLINE = 2 * time.Second
	// CLIENT is an end user.
	CLIENT = 0
	// ROUTER is another router in the cluster.
	ROUTER = 1
	//REMOTE is the router connect to other cluster
	REMOTE = 2
)

type client struct {
	typ      int
	srv      *Server
	nc       net.Conn
	mu       sync.Mutex
	clientID string
	info     *ClientInfo
	subs     map[string]*subscription
	willMsg  *message.PublishMessage
	packets  map[string][]byte
	closeCh  chan bool
}

//clientInfo eg: username and password
type ClientInfo struct {
	username   string
	password   string
	keeplive   uint16
	tlsRequire bool
	remoteID   string
	remoteurl  string
}
type subscription struct {
	client *client
	topic  []byte
	qos    byte
	queue  bool
}

func (c *client) SendInfo() {
	infoMsg := message.NewPublishMessage()
	infoMsg.SetTopic([]byte(BrokerInfoTopic))
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
	c.packets = make(map[string][]byte)
}

func (c *client) readLoop() {
	c.mu.Lock()
	nc := c.nc
	keeplive := c.info.keeplive
	clientID := c.clientID
	c.mu.Unlock()

	if nc == nil {
		return
	}
	first := true
	lastIn := uint16(time.Now().Unix())
	for {
		nowTime := uint16(time.Now().Unix())
		if 0 != keeplive && nowTime-lastIn > keeplive*3/2 {
			log.Error("\tserver/client.go: Client has exceeded timeout, disconnecting. cid:", clientID)
			c.Close()
			break
		}

		buf, err := c.ReadPacket()
		if err != nil {
			if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
				// log.Error("\tserver/client.go: read timeout")
				continue
			}
			log.Error("\tserver/client.go: read buf err: ", err)
			c.Close()
			break
		}
		lastIn = uint16(time.Now().Unix())
		if first {
			c.ProcessConnect(buf)
			first = false
		} else {
			c.parse(buf)
		}

		c.mu.Lock()
		nc := c.nc
		c.mu.Unlock()

		if nc == nil {
			break
		}

	}
}

func (c *client) Close() {
	c.mu.Lock()
	if c == nil || c.nc == nil {
		c.mu.Unlock()
		return
	}
	srv := c.srv
	willMsg := c.willMsg
	c.mu.Unlock()

	if c.nc != nil {
		c.nc.Close()
	}
	c.nc = nil

	log.Info("client closed with cid: ", c.clientID)
	if srv != nil {
		srv.removeClient(c)
		for _, sub := range c.subs {
			// log.Info("remove Sub")
			err := srv.sl.Remove(sub)
			if err != nil {
				log.Error("\tserver/client.go: closed client but remove sublist error, ", err)
			}
			if c.typ == CLIENT {
				srv.BroadcastUnSubscribe(sub)
			}
		}
	}
	if willMsg != nil {
		topic := string(willMsg.Topic())
		r := srv.sl.Match(topic)
		if len(r.qsubs) == 0 && len(r.psubs) == 0 {
			return
		}

		for _, sub := range r.psubs {
			//only CLIENT HAVE WILL MESSAGE
			srv.startGoRoutine(func() {
				err := sub.client.writeMessage(willMsg)
				if err != nil {
					log.Error("\tserver/client.go: process will message for psub error,  ", err)
				}
			})

		}
		for i, sub := range r.qsubs {
			if cnt, exist := srv.queues[string(sub.topic)]; exist && i == cnt {
				if sub != nil {
					err := sub.client.writeMessage(willMsg)
					if err != nil {
						log.Error("\tserver/client.go: process will message for qsub error,  ", err)
					}
				}
				srv.queues[topic] = (srv.queues[topic] + 1) % len(r.qsubs)
				break
			}
		}

	}
	c = nil
}

func (c *client) ProcessConnAck(buf []byte) {
	ackMsg := message.NewConnackMessage()
	_, err := ackMsg.Decode(buf)
	if err != nil {
		log.Error("\tserver/client.go: Decode Connack Message error: ", err)
		if c.typ == CLIENT {
			c.Close()
		}
		return
	}
	rc := ackMsg.ReturnCode()
	if rc != message.ConnectionAccepted {
		log.Error("\tserver/client.go: Connect error with the returnCode is: ", rc)
		if c.typ == CLIENT {
			c.Close()
		}
		return
	}
	//save remote info and send local subs
	c.mu.Lock()
	s := c.srv
	c.mu.Unlock()
	if s == nil {
		return
	}
	s.mu.Lock()
	s.remotes[c.clientID] = c
	s.mu.Unlock()

	s.startGoRoutine(func() {
		s.SendLocalSubsToRouter(c)
	})
}

func (c *client) ProcessConnect(msg []byte) {
	connMsg := message.NewConnectMessage()
	_, err := connMsg.Decode(msg)
	if err != nil {
		if !message.ValidConnackError(err) {
			log.Error("\tserver/client.go: Decode Connection Message error: ", err)
			if c.typ == CLIENT {
				c.Close()
			}
			return
		}
	}

	c.mu.Lock()
	srv := c.srv
	typ := c.typ
	c.mu.Unlock()

	connack := message.NewConnackMessage()
	var keeplive uint16
	if version := connMsg.Version(); version != 0x04 && version != 0x03 {
		connack.SetReturnCode(message.ErrInvalidProtocolVersion)
		goto connback
	}

	c.info.username = string(connMsg.Username())
	c.info.password = string(connMsg.Password())

	keeplive = connMsg.KeepAlive()
	if keeplive < 10 {
		c.info.keeplive = 60
	} else {
		c.info.keeplive = keeplive
	}

	c.clientID = string(connMsg.ClientId())

	if connMsg.WillFlag() {
		msg := message.NewPublishMessage()
		msg.SetQoS(connMsg.WillQos())
		msg.SetPayload(connMsg.WillMessage())
		msg.SetRetain(connMsg.WillRetain())
		msg.SetTopic(connMsg.WillTopic())
		msg.SetDup(false)
		c.willMsg = msg
	} else {
		c.willMsg = nil
	}

	srv.mu.Lock()
	if typ == CLIENT {
		srv.clients[c.clientID] = c
	} else if typ == ROUTER {
		srv.routers[c.clientID] = c
	}
	srv.mu.Unlock()

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

	suback := message.NewSubackMessage()
	var retcodes []byte

	msg := message.NewSubscribeMessage()
	_, err := msg.Decode(buf)

	if err != nil {
		log.Error("\tserver/client.go: Decode Subscribe Message error: ", err)
		if c.typ == CLIENT {
			c.Close()
		}
		return
	}
	topics = msg.Topics()
	qos = msg.Qos()

	suback.SetPacketId(msg.PacketId())

	c.mu.Lock()
	srv := c.srv
	c.mu.Unlock()

	for i, t := range topics {
		if _, exist := c.subs[string(t)]; !exist {
			queue := false
			if strings.HasPrefix(string(t), "$queue/") {
				if len(t) > 6 {
					t = t[7:]
					queue = true
					if _, exists := srv.queues[string(t)]; !exists {
						srv.queues[string(t)] = 0
					}
				} else {
					retcodes = append(retcodes, message.QosFailure)
					continue
				}
			}
			sub := &subscription{
				topic:  t,
				qos:    qos[i],
				client: c,
				queue:  queue,
			}
			c.subs[string(t)] = sub
			err := srv.sl.Insert(sub)
			if err != nil {
				log.Error("\tserver/client.go: Insert subscription error: ", err)
				retcodes = append(retcodes, message.QosFailure)
			}
			retcodes = append(retcodes, qos[i])
		} else {
			//if exist ,check whether qos change
			c.subs[string(t)].qos = qos[i]
			retcodes = append(retcodes, qos[i])
		}

	}

	if err := suback.AddReturnCodes(retcodes); err != nil {
		log.Error("\tserver/client.go: add return suback code error, ", err)
		if c.typ == CLIENT {
			c.Close()
		}
		return
	}
	if c.typ == CLIENT {
		srv.startGoRoutine(func() {
			srv.BroadcastSubscribeMessage(buf)
		})
	}

	err1 := c.writeMessage(suback)
	if err1 != nil {
		log.Error("\tserver/client.go: send suback error, ", err1)
	}
	for _, t := range topics {
		srv.startGoRoutine(func() {
			bufs := srv.rl.Match(t)
			for _, buf := range bufs {
				log.Info("process retain  message: ", string(buf))
				if buf != nil && string(buf) != "" {
					c.writeBuffer(buf)
				}
			}
		})
	}
}

func (c *client) ProcessUnSubscribe(msg []byte) {
	unsub := message.NewUnsubscribeMessage()
	_, err := unsub.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode UnSubscribe Message error: ", err)
		if c.typ == CLIENT {
			c.Close()
		}
		return
	}
	topics := unsub.Topics()

	for _, t := range topics {
		var sub *subscription
		ok := false

		if sub, ok = c.subs[string(t)]; ok {
			c.unsubscribe(sub)
		}

	}
	if c.typ == CLIENT {
		c.srv.BroadcastUnSubscribeMessage(msg)
	}

	resp := message.NewUnsubackMessage()
	resp.SetPacketId(unsub.PacketId())

	err1 := c.writeMessage(resp)
	if err1 != nil {
		log.Error("\tserver/client.go: send ubsuback error, ", err1)
	}
}

func (c *client) unsubscribe(sub *subscription) {

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subs, string(sub.topic))

	if c.srv != nil {
		c.srv.sl.Remove(sub)
	}
}

func (c *client) ProcessPing() {
	respMsg := message.NewPingrespMessage()
	err := c.writeMessage(respMsg)
	if err != nil {
		log.Error("\tserver/client.go: send pingresp error, ", err)
	}
}

func (c *client) ProcessPublish(msg []byte) {
	pubMsg := message.NewPublishMessage()
	_, err := pubMsg.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode Publish Message error: ", err)
		if c.typ == CLIENT {
			c.Close()
		}
		return
	}
	topic := string(pubMsg.Topic())

	c.mu.Lock()
	s := c.srv
	c.mu.Unlock()
	//process info message
	if c.typ == ROUTER && topic == BrokerInfoTopic {
		remoteID := gjson.GetBytes(pubMsg.Payload(), "remoteID").String()
		url := gjson.GetBytes(pubMsg.Payload(), "url").String()
		if remoteID == "" {
			log.Error("\tserver/client.go: receive info message error with remoteID is null")
			return
		}
		s.ValidAndProcessRemoteInfo(remoteID, url)
		return
	}

	if pubMsg.Retain() {
		s.startGoRoutine(func() {
			err := s.rl.Insert(pubMsg.Topic(), msg)
			if err != nil {
				log.Error("\tserver/client.go: Insert Retain Message error: ", err)
			}
		})
	}
	//process normal publish message
	// c.ProcessPublishMessage(msg, topic)
	switch pubMsg.QoS() {
	case message.QosExactlyOnce:
		c.mu.Lock()
		key := incoming + string(pubMsg.PacketId())
		c.packets[key] = msg
		c.mu.Unlock()

		resp := message.NewPubrecMessage()
		resp.SetPacketId(pubMsg.PacketId())
		err := c.writeMessage(resp)
		if err != nil {
			log.Error("\tserver/client.go: send pubrec error, ", err)
		}

	case message.QosAtLeastOnce:
		resp := message.NewPubackMessage()
		resp.SetPacketId(pubMsg.PacketId())

		if err := c.writeMessage(resp); err != nil {
			log.Error("\tserver/client.go: send puback error, ", err)
		}
		c.ProcessPublishMessage(msg, topic)

	case message.QosAtMostOnce:
		c.ProcessPublishMessage(msg, topic)
	}
}
func (c *client) ProcessPublishMessage(buf []byte, topic string) {

	c.mu.Lock()
	s := c.srv
	c.mu.Unlock()

	r := s.sl.Match(topic)
	// log.Info("psubs num: ", len(r.psubs))
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
			if sub != nil {
				err := sub.client.writeBuffer(buf)
				if err != nil {
					log.Error("\tserver/client.go: process message for psub error,  ", err)
				}
			}
		})

	}

	for i, sub := range r.qsubs {
		if sub.client.typ == ROUTER {
			if c.typ == ROUTER {
				continue
			}
		}
		if cnt, exist := s.queues[string(sub.topic)]; exist && i == cnt {
			if sub != nil {
				err := sub.client.writeBuffer(buf)
				if err != nil {
					log.Error("\tserver/client.go: process will message for qsub error,  ", err)
				}
			}
			s.queues[topic] = (s.queues[topic] + 1) % len(r.qsubs)
			break
		}
	}
}

func (c *client) ProcessPubREL(msg []byte) {
	pubrel := message.NewPubrelMessage()
	_, err := pubrel.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode Pubrel Message error: ", err)
		return
	}
	c.mu.Lock()
	var buf []byte
	exist := false
	key := incoming + string(pubrel.PacketId())
	if buf, exist = c.packets[key]; !exist {
		log.Error("\tserver/client.go: search qos 2 Message error ")
		return
	}

	resp := message.NewPubcompMessage()
	resp.SetPacketId(pubrel.PacketId())
	c.writeMessage(resp)
	delete(c.packets, key)
	c.mu.Unlock()

	pubmsg := message.NewPublishMessage()
	pubmsg.Decode(buf)

	c.ProcessPublishMessage(buf, string(pubmsg.Topic()))

}

func (c *client) ProcessPubREC(msg []byte) {
	pubRec := message.NewPubrecMessage()
	_, err := pubRec.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode Pubrec Message error: ", err)
		return
	}
	c.mu.Lock()
	c.packets[outgoing+string(pubRec.PacketId())] = msg
	c.mu.Unlock()

	resp := message.NewPubrelMessage()
	resp.SetPacketId(pubRec.PacketId())
	c.writeMessage(resp)
}

func (c *client) ProcessPubAck(msg []byte) {
	puback := message.NewPubackMessage()
	_, err := puback.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode PubAck Message error: ", err)
		return
	}
	key := outgoing + string(puback.PacketId())

	c.mu.Lock()
	delete(c.packets, key)
	c.mu.Unlock()
}

func (c *client) ProcessPubComp(msg []byte) {
	pubcomp := message.NewPubcompMessage()
	_, err := pubcomp.Decode(msg)
	if err != nil {
		log.Error("\tserver/client.go: Decode PubAck Message error: ", err)
		return
	}
	key := outgoing + string(pubcomp.PacketId())

	c.mu.Lock()
	delete(c.packets, key)
	c.mu.Unlock()
}

func (c *client) writeBuffer(buf []byte) error {
	c.mu.Lock()
	nc := c.nc
	if nc == nil {
		c.mu.Unlock()
		return errors.New("conn is nul")
	}
	// nc.SetWriteDeadline(time.Now().Add(DEFAULT_WRITE_TIMEOUT))
	_, err := nc.Write(buf)
	// nc.SetWriteDeadline(time.Time{})
	c.mu.Unlock()
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
