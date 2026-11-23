package letterboxd

import (
	"sync"
	"time"
)

// Cache is a small in-memory TTL cache with in-flight deduplication: when
// several goroutines ask for the same missing key, the loader runs once and
// everybody gets its result.
type Cache[T any] struct {
	ttl time.Duration

	mu       sync.Mutex
	items    map[string]cacheItem[T]
	inflight map[string]*call[T]
	sets     int
}

type cacheItem[T any] struct {
	val     T
	expires time.Time
}

type call[T any] struct {
	done chan struct{}
	val  T
	err  error
}

func NewCache[T any](ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		ttl:      ttl,
		items:    make(map[string]cacheItem[T]),
		inflight: make(map[string]*call[T]),
	}
}

// GetOrLoad returns the cached value for key, or calls load to produce it.
// Only successful results are cached.
func (c *Cache[T]) GetOrLoad(key string, load func() (T, error)) (T, error) {
	c.mu.Lock()
	if it, ok := c.items[key]; ok && time.Now().Before(it.expires) {
		c.mu.Unlock()
		return it.val, nil
	}
	if cl, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-cl.done
		return cl.val, cl.err
	}
	cl := &call[T]{done: make(chan struct{})}
	c.inflight[key] = cl
	c.mu.Unlock()

	cl.val, cl.err = load()

	c.mu.Lock()
	delete(c.inflight, key)
	if cl.err == nil {
		c.items[key] = cacheItem[T]{val: cl.val, expires: time.Now().Add(c.ttl)}
		c.sets++
		if c.sets%64 == 0 {
			c.sweepLocked()
		}
	}
	c.mu.Unlock()
	close(cl.done)
	return cl.val, cl.err
}

func (c *Cache[T]) sweepLocked() {
	now := time.Now()
	for k, it := range c.items {
		if now.After(it.expires) {
			delete(c.items, k)
		}
	}
}
