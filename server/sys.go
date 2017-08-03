package server

import (
	"fmt"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
)

const (
	ONCONENCTED    = "$SYS/brokers/clients/connected/"
	ONDISCONENCTED = "$SYS/brokers/clients/disconnected/"
)

func NewSYSMessage(topic, playload []byte) *message.PublishMessage {
	infoMsg := message.NewPublishMessage()
	infoMsg.SetTopic(topic)
	infoMsg.SetPayload(playload)
	infoMsg.SetQoS(0)
	infoMsg.SetRetain(false)
	return infoMsg
}

func (s *Server) PublishOnConnectedMessage(ip, username, clientID string) {
	onConnect := []byte(fmt.Sprintf(`{ipaddress:"%s",username:"%s",clientID:%s}`, ip, username, clientID))
	topic := []byte(ONCONENCTED + clientID)
	msg := NewSYSMessage(topic, onConnect)

	s.PublishMessage(msg)
}

func (s *Server) PublishOnDisconnectedMessage(username, clientID string) {
	onConnect := []byte(fmt.Sprintf(`{username:"%s",clientID:%s}`, username, clientID))
	topic := []byte(ONDISCONENCTED + clientID)
	msg := NewSYSMessage(topic, onConnect)
	s.PublishMessage(msg)
}

func (s *Server) PublishMessage(msg *message.PublishMessage) {

	topic := string(msg.Topic())
	r := s.sl.Match(topic)
	// log.Info("psubs num: ", len(r.psubs))
	if len(r.qsubs) == 0 && len(r.psubs) == 0 {
		return
	}

	for _, sub := range r.psubs {
		s.startGoRoutine(func() {
			if sub != nil {
				err := sub.client.writeMessage(msg)
				if err != nil {
					log.Error("process message for psub error,  ", err)
				}
			}
		})

	}

	for i, sub := range r.qsubs {
		s.mu.Lock()
		if cnt, exist := s.queues[string(sub.topic)]; exist && i == cnt {
			if sub != nil {
				err := sub.client.writeMessage(msg)
				if err != nil {
					log.Error("process will message for qsub error,  ", err)
				}
			}
			s.queues[topic] = (s.queues[topic] + 1) % len(r.qsubs)
			break
		}
		s.mu.Unlock()
	}
}
