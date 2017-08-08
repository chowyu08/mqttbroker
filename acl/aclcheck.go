package acl

import "strings"

func CheckTopicAuth(ACLInfo *ACLConfig, typ int, ip, username, clientid, topic string) bool {
	for _, info := range ACLInfo.Info {
		ctyp := info.Typ
		switch ctyp {
		case CLIENTID:
			if info.checkWithClientID(typ, clientid, topic) {
				return true
			}
		case USERNAME:
			if info.checkWithUsername(typ, username, topic) {
				return true
			}
		case IP:
			if info.checkWithIP(typ, ip, topic) {
				return true
			}
		}
	}
	return false
}

func (a *AuthInfo) checkWithClientID(typ int, clientid, topic string) bool {
	auth := false
	if a.Val == "*" || a.Val == clientid {
		for _, tp := range a.Topics {
			des := strings.Replace(tp, "%c", clientid, -1)
			if typ == PUB {
				if pubTopicMatch(topic, des) {
					auth = a.checkAuth(PUB)
				}
			} else if typ == SUB {
				if subTopicMatch(topic, des) {
					auth = a.checkAuth(SUB)
				}
			}
		}
	}
	return auth
}

func (a *AuthInfo) checkWithUsername(typ int, username, topic string) bool {
	auth := false
	if a.Val == "*" || a.Val == username {
		for _, tp := range a.Topics {
			des := strings.Replace(tp, "%u", username, -1)
			if typ == PUB {
				if pubTopicMatch(topic, des) {
					auth = a.checkAuth(PUB)
				}
			} else if typ == SUB {
				if subTopicMatch(topic, des) {
					auth = a.checkAuth(SUB)
				}
			}
		}
	}
	return auth
}

func (a *AuthInfo) checkWithIP(typ int, ip, topic string) bool {
	auth := false
	if a.Val == "*" || a.Val == ip {
		for _, tp := range a.Topics {
			des := tp
			if typ == PUB {
				if pubTopicMatch(topic, des) {
					auth = a.checkAuth(PUB)
				}
			} else if typ == SUB {
				if subTopicMatch(topic, des) {
					auth = a.checkAuth(SUB)
				}
			}
		}
	}
	return auth
}

func (a *AuthInfo) checkAuth(typ int) bool {
	auth := false
	if typ == PUB {
		if a.Auth == ALLOW && (a.PubSub == PUB || a.PubSub == PUBSUB) {
			auth = true
		} else if a.Auth == DENY && a.PubSub == SUB {
			auth = true
		}
	} else if typ == SUB {
		if a.Auth == ALLOW && (a.PubSub == SUB || a.PubSub == PUBSUB) {
			auth = true
		} else if a.Auth == DENY && a.PubSub == PUB {
			auth = true
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
