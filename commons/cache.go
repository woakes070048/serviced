package commons

import (
	"fmt"
	"reflect"
	"sync"
)

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
	// Make sure that we dereference in case val is a pointer
	v := reflect.Indirect(reflect.ValueOf(val)).Interface()
	c.data[key] = v
}

func (c *CopyCache) Get(key string) (interface{}, bool) {
	val, ok := c.data[key]
	return val, ok
}

func (c *CopyCache) GetInto(key string, target interface{}) (bool, error) {
	val, ok := c.Get(key)
	if !ok {
		return false, nil
	}
	t := reflect.ValueOf(target)
	if t.Kind() != reflect.Ptr {
		return false, fmt.Errorf("GetInto() requires a pointer")
	}
	v := reflect.ValueOf(val)
	t.Elem().Set(v)
	return true, nil
}