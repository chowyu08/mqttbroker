package server

import "broker/acl"

func (c *client) CheckSubAuth(topic string) bool {
	ip := c.remoteIP
	username := c.username
	clientid := c.clientID
	return acl.CheckSubAuth(ip, username, clientid, topic)
}

func (c *client) CheckPubAuth(topic string) bool {
	ip := c.remoteIP
	username := c.username
	clientid := c.clientID
	return acl.CheckPubAuth(ip, username, clientid, topic)
}
