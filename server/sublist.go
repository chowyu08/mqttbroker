// Copyright 2016 Apcera Inc. All rights reserved.

// Package sublist is a routing mechanism to handle subject distribution
// and provides a facility to match subjects from published messages to
// interested subscribers. Subscribers can have wildcard subjects to match
// multiple published subjects.
package server

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"sync"

	"github.com/smallnest/rpcx/log"
)

// Sublist related errors
var (
	ErrInvalidSubject = errors.New("sublist: Invalid Subject")
	ErrNotFound       = errors.New("sublist: No Matches Found")
)

// A result structure better optimized for queue subs.
type SublistResult struct {
	psubs []*subscription
	qsubs []*subscription // don't make this a map, too expensive to iterate
}

// A Sublist stores and efficiently retrieves subscriptions.
type Sublist struct {
	sync.RWMutex
	cache map[string]*SublistResult
	root  *level
}

// A node contains subscriptions and a pointer to the next level.
type node struct {
	next  *level
	psubs []*subscription
	qsubs []*subscription
}

// A level represents a group of nodes and special pointers to
// wildcard nodes.
type level struct {
	nodes map[string]*node
}

// Create a new default node.
func newNode() *node {
	return &node{psubs: make([]*subscription, 0, 4), qsubs: make([]*subscription, 0, 4)}
}

// Create a new default level. We use FNV1A as the hash
// algortihm for the tokens, which should be short.
func newLevel() *level {
	return &level{nodes: make(map[string]*node)}
}

// New will create a default sublist
func NewSublist() *Sublist {
	return &Sublist{root: newLevel(), cache: make(map[string]*SublistResult)}
}

// Insert adds a subscription into the sublist
func (s *Sublist) Insert(sub *subscription) error {

	tokens, err := validAndSpiltTopic(sub.subject)
	if err != nil {
		return err
	}
	s.Lock()

	l := s.root
	var n *node
	for _, t := range tokens {
		if len(t) == 0 {
			return errors.New("Invalid Subject")
		}
		n = l.nodes[t]
		if n == nil {
			n = newNode()
			l.nodes[t] = n
		}
		if n.next == nil {
			n.next = newLevel()
		}
		l = n.next
	}
	if sub.queue {
		//check qsub is already exist
		for i := range n.qsubs {
			if equal(n.qsubs[i], sub) {
				n.qsubs[i].qos = sub.qos
				return nil
			}
		}
		n.qsubs = append(n.qsubs, sub)
	} else {
		//check psub is already exist
		for i := range n.psubs {
			if equal(n.psubs[i], sub) {
				n.psubs[i].qos = sub.qos
				return nil
			}
		}
		n.psubs = append(n.psubs, sub)
	}
	s.Unlock()
	return nil
}

func (s *Sublist) Match(subject string) *SublistResult {

	tokens, err := validAndSpiltTopic([]byte(subject))
	if err != nil {
		log.Error("\tserver/sublist.go: ", err)
		return nil
	}

	result := &SublistResult{}

	s.Lock()
	matchLevel(s.root, tokens, result)
	s.Unlock()
	log.Info("SublistResult: ", result)
	return result
}

func matchLevel(l *level, toks []string, results *SublistResult) {
	var n *node
	for i, t := range toks {
		if l == nil {
			return
		}
		if _, exist := l.nodes["#"]; exist {
			addNodeToResults(l.nodes["#"], results)
		}
		if _, exist := l.nodes["+"]; exist {
			matchLevel(l.nodes["+"].next, toks[i+1:], results)
		}
		n = l.nodes[t]
		if n != nil {
			l = n.next
		} else {
			l = nil
		}
	}
	if n != nil {
		addNodeToResults(n, results)
	}
	if _, exist := l.nodes["+"]; exist {
		addNodeToResults(l.nodes["+"], results)
	}
}

// This will add in a node's results to the total results.
func addNodeToResults(n *node, results *SublistResult) {
	results.psubs = append(results.psubs, n.psubs...)
	results.qsubs = append(results.qsubs, n.qsubs...)

}

func validAndSpiltTopic(subject []byte) ([]string, error) {
	if bytes.IndexByte(subject, '#') != -1 {
		if bytes.IndexByte(subject, '#') != len(subject)-1 {
			return nil, errors.New("Topic format error with index of #")
		}
	}
	topic := string(subject)
	re := strings.Split(topic, "/")
	if re[0] == "" {
		re[1] = "/" + re[1]
		re = re[1:]
	}
	if re[len(re)-1] == "" {
		if strings.Contains(re[len(re)-2], "+") {
			if re[len(re)-2] != "+" && re[len(re)-2] != "/+" {
				return nil, errors.New("Topic format error with index of +")
			}
		}
		re[len(re)-2] = re[len(re)-2] + "/"
		re = re[:len(re)-1]
	}
	for i, v := range re {
		if i == 0 || i == len(re)-1 {
			continue
		}
		if strings.Contains(v, "+") && v != "+" {
			return nil, errors.New("Topic format error with index of +")
		}
	}
	return re, nil
}

func equal(k1, k2 interface{}) bool {
	if reflect.TypeOf(k1) != reflect.TypeOf(k2) {
		return false
	}

	if reflect.ValueOf(k1).Kind() == reflect.Func {
		return &k1 == &k2
	}

	if k1 == k2 {
		return true
	}

	switch k1 := k1.(type) {
	case string:
		return k1 == k2.(string)

	case int64:
		return k1 == k2.(int64)

	case int32:
		return k1 == k2.(int32)

	case int16:
		return k1 == k2.(int16)

	case int8:
		return k1 == k2.(int8)

	case int:
		return k1 == k2.(int)

	case float32:
		return k1 == k2.(float32)

	case float64:
		return k1 == k2.(float64)

	case uint:
		return k1 == k2.(uint)

	case uint8:
		return k1 == k2.(uint8)

	case uint16:
		return k1 == k2.(uint16)

	case uint32:
		return k1 == k2.(uint32)

	case uint64:
		return k1 == k2.(uint64)

	case uintptr:
		return k1 == k2.(uintptr)
	}

	return false
}
