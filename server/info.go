package server

import (
	"fmt"

	simplejson "github.com/bitly/go-simplejson"
	log "github.com/cihub/seelog"
	"github.com/surgemq/message"
)

func (c *client) ProcessInfo(msg *message.PublishMessage) {
	nc := c.nc
	s := c.srv

	if nc == nil {
		return
	}

	log.Info("recv remoteInfo: ", string(msg.Payload()))

	js, e := simplejson.NewJson(msg.Payload())
	if e != nil {
		log.Warn("parse info message err", e)
		return
	}

	rid := js.Get("remoteID").MustString()
	rurl := js.Get("url").MustString()
	isForward := js.Get("isForward").MustBool()

	if rid == "" {
		log.Error("receive info message error with remoteID is null")
		return
	}

	if rid == s.ID {
		log.Info("recv self info")
		if !isForward {
			c.Close() //close connet self
		}
		return
	}

	exist := s.CheckRemoteExist(rid, rurl)
	if !exist {
		s.startGoRoutine(func() {
			s.connectRouter(rurl, rid)
		})
	}
	// log.Info("isforword: ", isForward)
	if !isForward {
		route := &Route{
			remoteUrl: rurl,
			remoteID:  rid,
		}
		c.route = route
		s.startGoRoutine(func() {
			s.SendLocalSubsToRouter(c)
		})
		// log.Info("BroadcastInfoMessage starting... ")
		infoMsg := NewInfo(rid, rurl, true)
		s.BroadcastInfoMessage(rid, infoMsg)
	}

	return
}

func NewInfo(sid, url string, isforword bool) *message.PublishMessage {
	infoMsg := message.NewPublishMessage()
	infoMsg.SetTopic([]byte(BrokerInfoTopic))
	info := fmt.Sprintf(`{"remoteID":"%s","url":"%s","isForward":%t}`, sid, url, isforword)
	// log.Info("new info", string(info))
	infoMsg.SetPayload([]byte(info))
	infoMsg.SetQoS(0)
	infoMsg.SetRetain(false)
	return infoMsg
}
