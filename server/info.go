package server

import (
	"fmt"

	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
	"github.com/tidwall/gjson"
)

func (c *client) ProcessInfo(msg *message.PublishMessage) {
	nc := c.nc
	s := c.srv

	if nc == nil {
		return
	}

	log.Info("recv remoteInfo: ", string(msg.Payload()))

	rid := gjson.GetBytes(msg.Payload(), "remoteID").String()
	rurl := gjson.GetBytes(msg.Payload(), "url").String()
	isForward := gjson.GetBytes(msg.Payload(), "isForward ").Bool()

	if rid == "" {
		log.Error("\tserver/client.go: receive info message error with remoteID is null")
		return
	}

	if rid == s.ID {
		log.Info("recv self info")
		if !isForward {
			c.Close() //close connet self
		}
		return
	}

	if !isForward {
		route := &Route{
			remoteUrl: rurl,
			remoteID:  rid,
		}
		c.route = route

		infoMsg := NewInfo(rid, rurl, true)
		s.BroadcastInfoMessage(rid, infoMsg)
	}
	s.ValidAndProcessRemoteInfo(rid, rurl)

}

func NewInfo(sid, url string, isforword bool) *message.PublishMessage {
	infoMsg := message.NewPublishMessage()
	infoMsg.SetTopic([]byte(BrokerInfoTopic))
	info := fmt.Sprintf(`{"remoteID":"%s","url":"%s","isForward":%t}`, sid, url, isforword)
	// log.Info(string(info))
	infoMsg.SetPayload([]byte(info))
	infoMsg.SetQoS(0)
	infoMsg.SetRetain(false)
	return infoMsg
}
