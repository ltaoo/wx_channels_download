package cache

import (
	"hash/fnv"
	"sync"
	"time"
)

const (
	// Default number of shards
	shardCount = 256
)

type item struct {
	object     interface{}
	expiration int64
}

// Cache is a thread-safe cache with expiration support.
type Cache struct {
	shards []*shard
}

type shard struct {
	sync.RWMutex
	items map[string]item
}

// New creates a new Cache instance.
func New() *Cache {
	c := &Cache{
		shards: make([]*shard, shardCount),
	}
	for i := 0; i < shardCount; i++ {
		c.shards[i] = &shard{
			items: make(map[string]item),
		}
	}
	// Start cleanup goroutine
	go c.cleanupLoop()
	return c
}

func (c *Cache) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return c.shards[h.Sum32()%shardCount]
}

// Set adds an item to the cache with the specified key, value, and duration.
func (c *Cache) Set(key string, value interface{}, duration time.Duration) {
	shard := c.getShard(key)
	shard.Lock()
	defer shard.Unlock()

	var expiration int64
	if duration > 0 {
		expiration = time.Now().Add(duration).UnixNano()
	}

	shard.items[key] = item{
		object:     value,
		expiration: expiration,
	}
}

// Get retrieves an item from the cache.
// It returns the value and a boolean indicating whether the key was found.
func (c *Cache) Get(key string) (interface{}, bool) {
	shard := c.getShard(key)
	shard.RLock()
	item, found := shard.items[key]
	shard.RUnlock()

	if !found {
		return nil, false
	}

	if item.expiration > 0 && time.Now().UnixNano() > item.expiration {
		return nil, false
	}

	return item.object, true
}

// Delete removes an item from the cache.
func (c *Cache) Delete(key string) {
	shard := c.getShard(key)
	shard.Lock()
	delete(shard.items, key)
	shard.Unlock()
}

// cleanupLoop periodically removes expired items.
func (c *Cache) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().UnixNano()
		for _, shard := range c.shards {
			shard.Lock()
			for key, item := range shard.items {
				if item.expiration > 0 && now > item.expiration {
					delete(shard.items, key)
				}
			}
			shard.Unlock()
		}
	}
}
