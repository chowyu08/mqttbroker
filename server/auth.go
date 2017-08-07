package server

import "broker/acl"

func (c *client) CheckSubAuth(topic string) bool {
	ip := c.remoteIP
	username := c.username
	clientid := c.clientID
	aclInfo := c.srv.AclConfig
	return acl.CheckSubAuth(aclInfo, ip, username, clientid, topic)
}

func (c *client) CheckPubAuth(topic string) bool {
	ip := c.remoteIP
	username := c.username
	clientid := c.clientID
	aclInfo := c.srv.AclConfig
	return acl.CheckPubAuth(aclInfo, ip, username, clientid, topic)
}
