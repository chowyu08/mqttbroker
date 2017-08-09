package server

import (
	"broker/acl"
	"strings"
)

const (
	PUB = 1
	SUB = 2
)

func (c *client) CheckTopicAuth(topic string, typ int) bool {
	if typ != CLIENT || !c.srv.info.Acl {
		return true
	}
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
