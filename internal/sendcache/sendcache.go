package sendcache

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// Cache deduplicates outgoing messages within a TTL window.
// Reserve returns true if the (dst, msg) pair is new; false if it was
// seen within the TTL.
type Cache struct {
	lru      *expirable.LRU[string, struct{}]
	disabled bool
}

// New creates a Cache with the given TTL. TTL == 0 disables dedup (Reserve
// always returns true).
func New(ttl time.Duration) *Cache {
	if ttl == 0 {
		return &Cache{disabled: true}
	}
	return &Cache{lru: expirable.NewLRU[string, struct{}](1024, nil, ttl)}
}

func (c *Cache) Reserve(dst, msg string) bool {
	if c.disabled {
		return true
	}
	key := dst + "\x00" + msg
	if _, ok := c.lru.Get(key); ok {
		return false
	}
	c.lru.Add(key, struct{}{})
	return true
}
