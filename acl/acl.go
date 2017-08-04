package acl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type ACLInterface interface {
	Key(key string) AuthInfo
}

const (
	PUB    = 1
	SUBS   = 2
	PUBSUB = 3
)

type AuthInfo struct {
	User  string
	Topic string
	Auth  int
}

type ACLConfig struct {
	File string
	Keys []AuthInfo
}

var ACLInfo ACLInterface

func AclConfigLoad(file string) {
	var aclConf string
	if file == "" {
		aclConf = "./conf/acl.conf"
	}
	config := &ACLConfig{aclConf, make(map[string]AuthInfo)}
	if err := config.Prase(); err != nil {
		fmt.Println("WARN:load conf file failure,", err)
		//don't panic in init func!!!
		//panic(err)
	}
	ACLInfo = config
}
func (c *ACLConfig) Prase() error {
	f, err := os.Open(c.File)
	defer f.Close()
	if err != nil {
		return errors.New("Open file" + c.File + " failed")
	}
	buf := bufio.NewReader(f)
	var parseErr error
	for {
		line, err := buf.ReadString('\n')
		// line = strings.TrimSpace(line)
		if isCommentOut(line) {
			continue
		}
		if line == "" {
			return parseErr
		}
		// fmt.Println(line)
		tmpArr := strings.Fields(line)
		if len(tmpArr) != 3 {
			parseErr = errors.New("\"" + line + "\" format is error")
			break
		}
		var auth int
		auth, err = strconv.Atoi(tmpArr[2])
		if err != nil {
			parseErr = errors.New("\"" + line + "\" format is error")
			break
		}
		tmpAuth := AuthInfo{
			Topic: tmpArr[1],
			Auth:  auth,
		}
		c.Keys[tmpArr[0]] = tmpAuth
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

func (c *ACLConfig) Key(key string) AuthInfo {
	return c.Keys[key]
}
