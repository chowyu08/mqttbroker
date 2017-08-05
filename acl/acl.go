package acl

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
)

const (
	PUB      = 1
	SUB      = 2
	PUBSUB   = 3
	CLIENTID = "clientid"
	USERNAME = "username"
	IP       = "ip"
	ALLOW    = "allow"
	DENY     = "deny"
)

type AuthInfo struct {
	Auth   string
	Typ    string
	Val    string
	Topic  string
	PubSub int
}

type ACLConfig struct {
	File string
	Info []*AuthInfo
}

var ACLInfo *ACLConfig

func AclConfigLoad(file string) error {
	if file == "" {
		file = "./conf/acl.conf"
	}
	ACLInfo := &ACLConfig{
		File: file,
		Info: make([]*AuthInfo, 0, 4),
	}
	err := ACLInfo.Prase()
	return err
}

func (c *ACLConfig) Prase() error {
	f, err := os.Open(c.File)
	defer f.Close()
	if err != nil {
		return err
	}
	buf := bufio.NewReader(f)
	var parseErr error
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		if isCommentOut(line) {
			continue
		}
		if line == "" {
			return parseErr
		}
		// fmt.Println(line)
		tmpArr := strings.Fields(line)
		if len(tmpArr) != 5 {
			parseErr = errors.New("\"" + line + "\" format is error")
			break
		}
		if tmpArr[0] != ALLOW && tmpArr[0] != DENY {
			parseErr = errors.New("\"" + line + "\" format is error")
			break
		}
		if tmpArr[1] != CLIENTID && tmpArr[1] != USERNAME && tmpArr[1] != IP {
			parseErr = errors.New("\"" + line + "\" format is error")
			break
		}
		var pubsub int
		pubsub, err = strconv.Atoi(tmpArr[4])
		if err != nil {
			parseErr = errors.New("\"" + line + "\" format is error")
			break
		}

		tmpAuth := &AuthInfo{
			Auth:   tmpArr[0],
			Typ:    tmpArr[1],
			Val:    tmpArr[2],
			Topic:  tmpArr[3],
			PubSub: pubsub,
		}
		c.Info = append(c.Info, tmpAuth)
		if err != nil {
			if err != io.EOF {
				parseErr = err
			}
			break
		}
	}
	return parseErr
}
func isCommentOut(line string) bool {
	if strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "*") {
		return true
	} else {
		return false
	}
}
