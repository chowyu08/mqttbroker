package server

import "broker/acl"

const (
	PUB = 1
	SUB = 2
)

func (c *client) CheckTopicAuth(topic string, typ int) bool {
	ip := c.remoteIP
	username := c.username
	clientid := c.clientID
	aclInfo := c.srv.AclConfig
	if typ == PUB {
		return acl.CheckPubAuth(aclInfo, ip, username, clientid, topic)
	} else if typ == SUB {
		return acl.CheckSubAuth(aclInfo, ip, username, clientid, topic)
	}
	return false
}
