package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisStaleTTLBuffer = 24 * time.Hour
const redisStaleKeyPrefix = "stale:"

// RedisCache implements the Cache interface using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisClient creates a new Redis client
func NewRedisClient(ctx context.Context, addr string) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Test the connection
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCache{client: client}, nil
}

// Get retrieves a value from Redis
func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, err
	}
	return val, nil
}

// GetStale retrieves a value from Redis even if the main key has expired,
// by using a separate stale key with extended TTL.
func (c *RedisCache) GetStale(ctx context.Context, key string) ([]byte, error) {
	// First try the main key (faster path)
	val, err := c.client.Get(ctx, key).Bytes()
	if err == nil {
		return val, nil
	}

	// Try the stale key
	val, err = c.client.Get(ctx, redisStaleKeyPrefix+key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrCacheMiss
		}
		return nil, err
	}

	return val, nil
}

// Set stores a value in Redis with the specified TTL.
// Also stores a copy with extended TTL under a "stale:" prefix for GetStale.
// If ttl is 0, the value will not be cached.
func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl == 0 {
		return nil // Skip caching if TTL is 0
	}

	// Use pipeline to set both keys atomically
	pipe := c.client.Pipeline()
	pipe.Set(ctx, key, value, ttl)
	pipe.Set(ctx, redisStaleKeyPrefix+key, value, ttl+redisStaleTTLBuffer)

	_, err := pipe.Exec(ctx)

	return err
}

// Close releases the Redis client
func (c *RedisCache) Close() error {
	return c.client.Close()
}
