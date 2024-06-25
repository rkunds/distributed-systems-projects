package main

import (
	"sync"
)

type CMap struct {
	m  map[interface{}]interface{}
	mu sync.RWMutex
}

func NewCMap() *CMap {
	return &CMap{
		m:  make(map[interface{}]interface{}),
		mu: sync.RWMutex{},
	}
}

func (c *CMap) Load(key interface{}) (value interface{}, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, ok = c.m[key]
	if !ok {
		return nil, false
	}

	if txnPtr, ok := value.(**Transaction); ok {
		return *txnPtr, true
	}

	if txn, ok := value.(*Transaction); ok {
		return txn, true
	}

	return nil, false
}

func (c *CMap) Store(key interface{}, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.m[key] = value
}
