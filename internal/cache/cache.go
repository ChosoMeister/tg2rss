package cache

import (
	"context"
	"errors"
	"time"
)

// ErrCacheMiss is returned when a key is not found in the cache
var ErrCacheMiss = errors.New("cache miss")

// Cache defines the interface for caching data
type Cache interface {
	// Get retrieves a value from the cache. Returns ErrCacheMiss if the key
	// is not found or has expired.
	Get(ctx context.Context, key string) ([]byte, error)

	// GetStale retrieves a value from the cache even if it has expired.
	// This is used for stale-while-revalidate and graceful degradation.
	// Returns ErrCacheMiss only if the key was never stored.
	GetStale(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with optional expiration.
	// If ttl is 0, the value will not be cached.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Close releases any resources used by the cache
	Close() error
}
