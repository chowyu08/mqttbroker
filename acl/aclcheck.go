package acl

import "strings"

func CheckSubAuth(ip, username, clientid, topic string) bool {
	for _, info := range ACLInfo.Info {
		typ := info.Typ
		switch typ {
		case CLIENTID:
			if info.checkSubWithClientID(clientid, topic) {
				return true
			}
		case USERNAME:
			if info.checkSubWithUsername(username, topic) {
				return true
			}
		case IP:
			if info.checkSubWithip(ip, topic) {
				return true
			}
		}
	}
	return false
}

func (a *AuthInfo) checkSubWithClientID(clientid, topic string) bool {
	auth := false
	des := strings.Replace(a.Topic, "%c", clientid, -1)
	if a.Typ == "*" {
		if subTopicMatch(topic, des) && (a.PubSub == SUB || a.PubSub == PUBSUB) {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	} else {
		if subTopicMatch(topic, des) && (a.PubSub == SUB || a.PubSub == PUBSUB) && a.Val == clientid {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	}
	return auth
}

func (a *AuthInfo) checkSubWithUsername(username, topic string) bool {
	auth := false
	des := strings.Replace(a.Topic, "%u", username, -1)
	if a.Typ == "*" {
		if subTopicMatch(topic, des) && (a.PubSub == SUB || a.PubSub == PUBSUB) {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	} else {
		if subTopicMatch(topic, des) && (a.PubSub == SUB || a.PubSub == PUBSUB) && a.Val == username {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	}
	return auth
}

func (a *AuthInfo) checkSubWithip(ip, topic string) bool {
	auth := false
	des := a.Topic
	if a.Typ == "*" {
		if subTopicMatch(topic, des) && (a.PubSub == SUB || a.PubSub == PUBSUB) {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	} else {
		if subTopicMatch(topic, des) && (a.PubSub == SUB || a.PubSub == PUBSUB) && a.Val == ip {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	}
	return auth
}

func CheckPubAuth(ip, username, clientid, topic string) bool {
	for _, info := range ACLInfo.Info {
		typ := info.Typ
		switch typ {
		case CLIENTID:
			if info.checkPubWithClientID(clientid, topic) {
				return true
			}
		case USERNAME:
			if info.checkPubWithUsername(username, topic) {
				return true
			}
		case IP:
			if info.checkPubWithip(ip, topic) {
				return true
			}
		}
	}
	return false
}

func (a *AuthInfo) checkPubWithClientID(clientid, topic string) bool {
	auth := false
	des := strings.Replace(a.Topic, "%c", clientid, -1)
	if a.Typ == "*" {
		if pubTopicMatch(topic, des) && (a.PubSub == PUB || a.PubSub == PUBSUB) {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	} else {
		if pubTopicMatch(topic, des) && (a.PubSub == PUB || a.PubSub == PUBSUB) && a.Val == clientid {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	}
	return auth
}

func (a *AuthInfo) checkPubWithUsername(username, topic string) bool {
	auth := false
	des := strings.Replace(a.Topic, "%u", username, -1)
	if a.Typ == "*" {
		if pubTopicMatch(topic, des) && (a.PubSub == PUB || a.PubSub == PUBSUB) {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	} else {
		if pubTopicMatch(topic, des) && (a.PubSub == PUB || a.PubSub == PUBSUB) && a.Val == username {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	}
	return auth
}

func (a *AuthInfo) checkPubWithip(ip, topic string) bool {
	auth := false
	des := a.Topic
	if a.Typ == "*" {
		if pubTopicMatch(topic, des) && (a.PubSub == PUB || a.PubSub == PUBSUB) {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	} else {
		if pubTopicMatch(topic, des) && (a.PubSub == PUB || a.PubSub == PUBSUB) && a.Val == ip {
			if a.Auth == ALLOW {
				auth = true
			}
		}
	}
	return auth
}

func pubTopicMatch(pub, des string) bool {
	dest, _ := SubscribeTopicSpilt(des)
	topic, _ := PublishTopicSpilt(pub)
	for i, t := range dest {
		if i > len(topic)-1 {
			return false
		}
		if t == "#" {
			return true
		}
		if t == "+" || t == topic[i] {
			continue
		}
		if t != topic[i] {
			return false
		}
	}
	return true
}

func subTopicMatch(pub, des string) bool {
	dest, _ := SubscribeTopicSpilt(des)
	topic, _ := SubscribeTopicSpilt(pub)
	for i, t := range dest {
		if i > len(topic)-1 {
			return false
		}
		if t == "#" {
			return true
		}
		if t == "+" || "+" == topic[i] || t == topic[i] {
			continue
		}
		if t != topic[i] {
			return false
		}
	}
	return true
}
