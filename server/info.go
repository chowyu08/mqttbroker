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

	// if info is self.
	if rid == s.ID {
		log.Info("recv self info")
		c.Close()
		return
	}

	if rid == "" {
		log.Error("\tserver/client.go: receive info message error with remoteID is null")
		return
	}

	if !isForward {
		route := &Route{
			remoteUrl: rurl,
			remoteID:  rid,
		}
		c.route = route

		info := fmt.Sprintf(`{"remoteID":"%s","url":"%s","isForward ":true}`, rid, rurl)
		msg.SetPayload([]byte(info))
		s.BroadcastInfoMessage(rid, msg)
	}
	s.ValidAndProcessRemoteInfo(rid, rurl)

}
