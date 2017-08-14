package server

import (
	"broker/acl"
	"strings"

	"github.com/prometheus/common/log"
)

const (
	PUB = 1
	SUB = 2
)

func (c *client) CheckTopicAuth(topic string, typ int) bool {
	if c.typ != CLIENT || !c.srv.info.Acl {
		return true
	}
	log.Info("check auth topic : ", topic)
	if strings.HasPrefix(topic, "$queue/") {
		topic = string([]byte(topic)[7:])
		if topic == "" {
			return false
		}
	}
	ip := c.remoteIP
	username := c.username
	clientid := c.clientID
	aclInfo := c.srv.AclConfig
	return acl.CheckTopicAuth(aclInfo, typ, ip, username, clientid, topic)

}
