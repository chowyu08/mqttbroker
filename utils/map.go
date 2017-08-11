package utils

// import (
// 	"encoding/json"
// 	"sync"
// )

// var DEFAULT_COUNT = 32

// type Map []*MapShared

// type MapShared struct {
// 	items        map[string]interface{}
// 	sync.RWMutex // Read Write mutex, guards access to internal map.
// }

// // create a new  map.
// func New() Map {
// 	m := make(Map, DEFAULT_COUNT)
// 	for i := 0; i < DEFAULT_COUNT; i++ {
// 		m[i] = &MapShared{items: make(map[string]interface{})}
// 	}
// 	return m
// }

// // return shardmap under given key
// func (m Map) getMapShard(key string) *MapShared {
// 	return m[uint(hash64(key))%uint(DEFAULT_COUNT)]
// }

// func (m Map) MSet(data map[string]interface{}) {
// 	for key, value := range data {
// 		shard := m.getMapShard(key)
// 		shard.Lock()
// 		shard.items[key] = value
// 		shard.Unlock()
// 	}
// }

// // Sets the given value under the specified key.
// func (m Map) Set(key string, value interface{}) {
// 	// Get map shard.
// 	shard := m.getMapShard(key)
// 	shard.Lock()
// 	shard.items[key] = value
// 	shard.Unlock()
// }

// // Callback to return new element to be inserted into the map
// // It is called while lock is held, therefore it MUST NOT
// // try to access other keys in same map, as it can lead to deadlock since
// // Go sync.RWLock is not reentrant
// type UpsertCb func(exist bool, valueInMap interface{}, newValue interface{}) interface{}

// // Insert or Update - updates existing element or inserts a new one using UpsertCb
// func (m Map) Upsert(key string, value interface{}, cb UpsertCb) (res interface{}) {
// 	shard := m.getMapShard(key)
// 	shard.Lock()
// 	v, ok := shard.items[key]
// 	res = cb(ok, v, value)
// 	shard.items[key] = res
// 	shard.Unlock()
// 	return res
// }

// // Sets the given value under the specified key if no value was associated with it.
// func (m Map) SetIfAbsent(key string, value interface{}) bool {
// 	// Get map shard.
// 	shard := m.getMapShard(key)
// 	shard.Lock()
// 	_, ok := shard.items[key]
// 	if !ok {
// 		shard.items[key] = value
// 	}
// 	shard.Unlock()
// 	return !ok
// }

// // Retrieves an element from map under given key.
// func (m Map) Get(key string) (interface{}, bool) {
// 	// Get shard
// 	shard := m.getMapShard(key)
// 	shard.RLock()
// 	// Get item from shard.
// 	val, ok := shard.items[key]
// 	shard.RUnlock()
// 	return val, ok
// }

// // Returns the number of elements within the map.
// func (m Map) Count() int {
// 	count := 0
// 	for i := 0; i < DEFAULT_COUNT; i++ {
// 		shard := m[i]
// 		shard.RLock()
// 		count += len(shard.items)
// 		shard.RUnlock()
// 	}
// 	return count
// }

// // Looks up an item under specified key
// func (m Map) Has(key string) bool {
// 	// Get shard
// 	shard := m.getMapShard(key)
// 	shard.RLock()
// 	// See if element is within shard.
// 	_, ok := shard.items[key]
// 	shard.RUnlock()
// 	return ok
// }

// // Removes an element from the map.
// func (m Map) Remove(key string) {
// 	// Try to get shard.
// 	shard := m.getMapShard(key)
// 	shard.Lock()
// 	delete(shard.items, key)
// 	shard.Unlock()
// }

// // Removes an element from the map and returns it
// func (m Map) Pop(key string) (v interface{}, exists bool) {
// 	// Try to get shard.
// 	shard := m.getMapShard(key)
// 	shard.Lock()
// 	v, exists = shard.items[key]
// 	delete(shard.items, key)
// 	shard.Unlock()
// 	return v, exists
// }

// // Checks if map is empty.
// func (m Map) IsEmpty() bool {
// 	return m.Count() == 0
// }

// // Used by the Iter & IterBuffered functions to wrap two variables together over a channel,
// type Tuple struct {
// 	Key string
// 	Val interface{}
// }

// // Returns an iterator which could be used in a for range loop.
// //
// // Deprecated: using IterBuffered() will get a better performence
// func (m Map) Iter() <-chan Tuple {
// 	chans := snapshot(m)
// 	ch := make(chan Tuple)
// 	go fanIn(chans, ch)
// 	return ch
// }

// // Returns a buffered iterator which could be used in a for range loop.
// func (m Map) IterBuffered() <-chan Tuple {
// 	chans := snapshot(m)
// 	total := 0
// 	for _, c := range chans {
// 		total += cap(c)
// 	}
// 	ch := make(chan Tuple, total)
// 	go fanIn(chans, ch)
// 	return ch
// }

// // Returns a array of channels that contains elements in each shard,
// // which likely takes a snapshot of `m`.
// // It returns once the size of each buffered channel is determined,
// // before all the channels are populated using goroutines.
// func snapshot(m Map) (chans []chan Tuple) {
// 	chans = make([]chan Tuple, DEFAULT_COUNT)
// 	wg := sync.WaitGroup{}
// 	wg.Add(DEFAULT_COUNT)
// 	// Foreach shard.
// 	for index, shard := range m {
// 		go func(index int, shard *MapShared) {
// 			// Foreach key, value pair.
// 			shard.RLock()
// 			chans[index] = make(chan Tuple, len(shard.items))
// 			wg.Done()
// 			for key, val := range shard.items {
// 				chans[index] <- Tuple{key, val}
// 			}
// 			shard.RUnlock()
// 			close(chans[index])
// 		}(index, shard)
// 	}
// 	wg.Wait()
// 	return chans
// }

// // fanIn reads elements from channels `chans` into channel `out`
// func fanIn(chans []chan Tuple, out chan Tuple) {
// 	wg := sync.WaitGroup{}
// 	wg.Add(len(chans))
// 	for _, ch := range chans {
// 		go func(ch chan Tuple) {
// 			for t := range ch {
// 				out <- t
// 			}
// 			wg.Done()
// 		}(ch)
// 	}
// 	wg.Wait()
// 	close(out)
// }

// // Returns all items as map[string]interface{}
// func (m Map) Items() map[string]interface{} {
// 	tmp := make(map[string]interface{})

// 	// Insert items to temporary map.
// 	for item := range m.IterBuffered() {
// 		tmp[item.Key] = item.Val
// 	}

// 	return tmp
// }

// // Iterator callback,called for every key,value found in
// // maps. RLock is held for all calls for a given shard
// // therefore callback sess consistent view of a shard,
// // but not across the shards
// type IterCb func(key string, v interface{})

// // Callback based iterator, cheapest way to read
// // all elements in a map.
// func (m Map) IterCb(fn IterCb) {
// 	for idx := range m {
// 		shard := (m)[idx]
// 		shard.RLock()
// 		for key, value := range shard.items {
// 			fn(key, value)
// 		}
// 		shard.RUnlock()
// 	}
// }

// // Return all keys as []string
// func (m Map) Keys() []string {
// 	count := m.Count()
// 	ch := make(chan string, count)
// 	go func() {
// 		// Foreach shard.
// 		wg := sync.WaitGroup{}
// 		wg.Add(DEFAULT_COUNT)
// 		for _, shard := range m {
// 			go func(shard *MapShared) {
// 				// Foreach key, value pair.
// 				shard.RLock()
// 				for key := range shard.items {
// 					ch <- key
// 				}
// 				shard.RUnlock()
// 				wg.Done()
// 			}(shard)
// 		}
// 		wg.Wait()
// 		close(ch)
// 	}()

// 	// Generate keys
// 	keys := make([]string, 0, count)
// 	for k := range ch {
// 		keys = append(keys, k)
// 	}
// 	return keys
// }

// //Reviles Map "private" variables to json marshal.
// func (m Map) MarshalJSON() ([]byte, error) {
// 	// Create a temporary map, which will hold all item spread across shards.
// 	tmp := make(map[string]interface{})

// 	// Insert items to temporary map.
// 	for item := range m.IterBuffered() {
// 		tmp[item.Key] = item.Val
// 	}
// 	return json.Marshal(tmp)
// }

// func hash64(key string) uint64 {
// 	hash := uint64(1756193281)
// 	const prime32 = uint64(19635842)
// 	for i := 0; i < len(key); i++ {
// 		hash *= prime32
// 		hash ^= uint64(key[i])
// 	}
// 	return hash
// }
