package cache

import (
	"context"
	"maps"
	"sync"
	"time"
)

const staleBufferDuration = 24 * time.Hour

type cacheItem struct {
	value     []byte
	expiresAt time.Time
}

// MemoryCache implements the Cache interface using RAM
type MemoryCache struct {
	cache map[string]cacheItem
	mu    sync.RWMutex
	stop  chan struct{}
}

// NewMemoryClient creates a new cache client
func NewMemoryClient() *MemoryCache {
	mc := &MemoryCache{
		cache: make(map[string]cacheItem, 100),
		stop:  make(chan struct{}),
	}

	go func() {
		for {
			select {
			case <-mc.stop:
				return
			case <-time.After(1 * time.Minute):
				mc.cleanup(time.Now())
			}
		}
	}()

	return mc
}

// Get retrieves a value from memory. Returns ErrCacheMiss if expired or not found.
func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	item, exists := c.cache[key]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrCacheMiss
	}

	if item.expiresAt.Before(time.Now()) {
		return nil, ErrCacheMiss
	}

	return item.value, nil
}

// GetStale retrieves a value from memory even if it has expired.
// Returns ErrCacheMiss only if the key was never stored.
func (c *MemoryCache) GetStale(_ context.Context, key string) ([]byte, error) {
	c.mu.RLock()
	item, exists := c.cache[key]
	c.mu.RUnlock()

	if !exists {
		return nil, ErrCacheMiss
	}

	return item.value, nil
}

// Set stores a value in memory with the specified TTL
// If ttl is 0, the value will not be cached
func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		return nil // Skip caching if TTL is 0
	}

	c.mu.Lock()
	c.cache[key] = cacheItem{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()

	return nil
}

// Close releases the memory
func (c *MemoryCache) Close() error {
	close(c.stop)

	c.mu.Lock()
	clear(c.cache)
	c.mu.Unlock()

	return nil
}

// cleanup removes items that have been expired for longer than staleBufferDuration.
// Items within the stale buffer are kept for GetStale to use.
func (c *MemoryCache) cleanup(now time.Time) {
	c.mu.RLock()

	var expiredKeys []string

	for key, item := range c.cache {
		// Only fully remove items that expired more than staleBufferDuration ago
		if item.expiresAt.Add(staleBufferDuration).Before(now) {
			expiredKeys = append(expiredKeys, key)
		}
	}

	c.mu.RUnlock()

	if len(expiredKeys) > 0 {
		c.mu.Lock()
		for _, key := range expiredKeys {
			delete(c.cache, key)
		}
		c.mu.Unlock()
	}
}

// snapshot returns a copy of the cache for testing purposes
func (c *MemoryCache) snapshot() map[string]cacheItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]cacheItem, len(c.cache))

	maps.Copy(result, c.cache)

	return result
}
