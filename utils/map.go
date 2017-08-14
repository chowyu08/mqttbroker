package utils

import "sync"

type Map interface {
	Set(key string, val interface{})
	Get(key string) (interface{}, bool)
	Items() interface{}
	Exist(key string) bool
	Count() int
	Remove(key string)
}

type SMap struct {
	items map[string]interface{}
	mu    *sync.RWMutex
}

func NewMap() Map {
	smap := &SMap{
		items: make(map[string]interface{}),
		mu:    new(sync.RWMutex),
	}
	return smap
}

func (s *SMap) Set(key string, val interface{}) {
	s.mu.Lock()
	s.items[key] = val
	s.mu.Unlock()
}

func (s *SMap) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	val, ok := s.items[key]
	s.mu.RUnlock()
	return val, ok
}

func (s *SMap) Exist(key string) bool {
	s.mu.RLock()
	_, ok := s.items[key]
	s.mu.RUnlock()
	return ok
}

func (s *SMap) Count() int {
	s.mu.RLock()
	len := len(s.items)
	s.mu.RUnlock()
	return len
}

func (s *SMap) Remove(key string) {
	s.mu.Lock()
	delete(s.items, key)
	s.mu.RUnlock()
}

func (s *SMap) Items() interface{} {
	s.mu.RLock()
	items := s.items
	s.mu.RUnlock()
	return items
}
