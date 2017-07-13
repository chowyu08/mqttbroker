package server

import (
	"errors"
	"sync"
)

type RetainList struct {
	sync.RWMutex
	root *rlevel
}
type rlevel struct {
	nodes map[string]*rnode
}
type rnode struct {
	next *rlevel
	msg  []byte
}

func newRNode() *rnode {
	return &rnode{msg: make([]byte, 0, 4)}
}

func newRLevel() *rlevel {
	return &rlevel{nodes: make(map[string]*rnode)}
}

func NewRetainList() *RetainList {
	return &RetainList{root: newRLevel()}
}

func (r *RetainList) Insert(topic, buf []byte) error {

	tokens, err := PublishTopicCheckAndSpilt(topic)
	if err != nil {
		return err
	}
	r.Lock()

	l := r.root
	var n *rnode
	for _, t := range tokens {
		if len(t) == 0 {
			return errors.New("Invalid Publish Topic")
		}
		n = l.nodes[t]
		if n == nil {
			n = newRNode()
			l.nodes[t] = n
		}
		if n.next == nil {
			n.next = newRLevel()
		}
		l = n.next
	}
	n.msg = buf

	r.Unlock()
	return nil
}

func (r *RetainList) Match(topic []byte) [][]byte {

	tokens, err := SubscribeTopicCheckAndSpilt(topic)
	if err != nil {
		return nil
	}
	var results [][]byte

	r.Lock()
	l := r.root
	matchRLevel(l, tokens, results)
	r.Unlock()

	return results

}
func matchRLevel(l *rlevel, toks []string, results [][]byte) {
	var n *rnode
	for i, t := range toks {
		if l == nil {
			return
		}
		if t == "#" {
			for _, n := range l.nodes {
				n.GetAll(results)
			}
		}
		if t == "+" {
			for _, n := range l.nodes {
				matchRLevel(n.next, toks[i+1:], results)
			}
		}

		// if t == "/+" {
		// 	for tp, n := range l.nodes {
		// 		if strings.Index(tp, "/") == 0 {
		// 			matchRLevel(n.next, toks[i+1:], results)
		// 		}
		// 	}
		// }

		// if t == "/+/" {
		// 	for tp, n := range l.nodes {
		// 		r, _ := regexp.Compile("/([a-z]+)/")
		// 		if r.MatchString(tp) {
		// 			results = append(results, n.msg)
		// 		}
		// 	}
		// }
		// if t == "/#" {
		// 	for tp, n := range l.nodes {
		// 		if strings.Index(tp, "/") == 0 {
		// 			n.GetAll(results)
		// 		}
		// 	}
		// }

		n = l.nodes[t]
		if n != nil {
			l = n.next
		} else {
			l = nil
		}
	}
	if n != nil {
		results = append(results, n.msg)
	}
}

func (r *rnode) GetAll(results [][]byte) {
	if r.msg != nil && string(r.msg) != "" {
		results = append(results, r.msg)
	}
	l := r.next
	for _, n := range l.nodes {
		n.GetAll(results)
	}
}
