package jscache

import (
	"container/list"
	"sync"
)

// Entry is a cached compiled action payload.
type Entry struct {
	Name    string
	Group   string
	Etag    string
	Timeout int
	JsURL   string
	JS      string
}

type item struct {
	key   string
	entry Entry
}

// LRU is a tiny thread-safe LRU of compiled JS keyed by "group/name".
type LRU struct {
	mu       sync.Mutex
	capacity int
	ll       *list.List
	table    map[string]*list.Element
}

func New(capacity int) *LRU {
	if capacity < 1 {
		capacity = 128
	}
	return &LRU{
		capacity: capacity,
		ll:       list.New(),
		table:    make(map[string]*list.Element, capacity),
	}
}

func Key(group, name string) string {
	return group + "\x00" + name
}

func (c *LRU) Get(key string) (Entry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.table[key]
	if !ok {
		return Entry{}, false
	}
	c.ll.MoveToFront(el)
	return el.Value.(*item).entry, true
}

func (c *LRU) Put(key string, e Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.table[key]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*item).entry = e
		return
	}
	el := c.ll.PushFront(&item{key: key, entry: e})
	c.table[key] = el
	for c.ll.Len() > c.capacity {
		back := c.ll.Back()
		if back == nil {
			break
		}
		c.ll.Remove(back)
		delete(c.table, back.Value.(*item).key)
	}
}

func (c *LRU) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ll.Len()
}
