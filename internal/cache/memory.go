package cache

import (
	"context"
	"maps"
	"sync"
	"time"
)

// MemoryCache implements the Cache interface using RAM
type MemoryCache struct {
	cache map[string][]byte
	mu    sync.RWMutex
}

// NewMemoryClient creates a new cache client
func NewMemoryClient() *MemoryCache {
	cache := make(map[string][]byte, 100)

	return &MemoryCache{cache: cache}
}

// Get retrieves a value from memory
func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	val, exists := c.cache[key]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrCacheMiss
	}

	return val, nil
}

// Set stores a value in memory with the specified TTL
// If ttl is 0, the value will not be cached
func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		return nil // Skip caching if TTL is 0
	}

	c.mu.Lock()
	c.cache[key] = value
	c.mu.Unlock()

	go func() {
		time.Sleep(ttl)
		c.mu.Lock()
		delete(c.cache, key)
		c.mu.Unlock()
	}()

	return nil
}

// Close releases the memory
func (c *MemoryCache) Close() error {
	c.mu.Lock()
	clear(c.cache)
	c.mu.Unlock()

	return nil
}

// snapshot returns a copy of the cache for testing purposes
func (c *MemoryCache) snapshot() map[string][]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string][]byte, len(c.cache))

	maps.Copy(result, c.cache)

	return result
}
