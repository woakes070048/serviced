package commons

import "sync"

type CopyCache struct {
	data map[string]interface{}
	sync.Mutex
}

func NewCache() *CopyCache {
	return &CopyCache{data: make(map[string]interface{})}
}

func (c *CopyCache) Invalidate(key string) {
	c.Lock()
	defer c.Unlock()
	delete(c.data, key)
}

func (c *CopyCache) Add(key string, val interface{}) {
	c.Lock()
	defer c.Unlock()
	c.data[key] = val
}

func (c *CopyCache) Get(key string) (interface{}, bool) {
	val, ok := c.data[key]
	return val, ok
}