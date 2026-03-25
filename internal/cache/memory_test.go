package cache

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCacheWrite(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		value    []byte
		duration time.Duration
		wantLen  int
	}{
		{
			name:     "should write a new entry in empty cache",
			value:    []byte{42},
			duration: 100 * time.Millisecond,
			wantLen:  1,
		},
		{
			name:     "should skip caching if TTL is 0",
			value:    []byte{42},
			duration: 0,
			wantLen:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := NewMemoryClient()
				defer c.Close()

				if err := c.Set(ctx, "test", tt.value, tt.duration); err != nil {
					t.Error(err)
				}

				snapshot := c.snapshot()

				assert.Len(t, snapshot, tt.wantLen)
			})
		})
	}
}

func TestMemoryCacheRead(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		key     string
		prepare func() *MemoryCache
		want    []byte
		err     error
	}{
		{
			name: "should read from cache",
			key:  "test",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test", []byte{42}, 100*time.Millisecond); err != nil {
					t.Error(err)
				}

				return c
			},
			want: []byte{42},
			err:  nil,
		},
		{
			name: "should respond with ErrCacheMiss when no such entry exists",
			key:  "test3",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test1", []byte{42}, 100*time.Millisecond); err != nil {
					t.Error(err)
				}

				if err := c.Set(ctx, "test2", []byte{42}, 100*time.Millisecond); err != nil {
					t.Error(err)
				}

				return c
			},
			want: nil,
			err:  ErrCacheMiss,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := tt.prepare()
				defer c.Close()

				v, err := c.Get(ctx, tt.key)

				assert.Equal(t, tt.want, v)
				assert.Equal(t, tt.err, err)
			})
		})
	}
}

func TestMemoryCacheClose(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		prepare func() *MemoryCache
		want    map[string]cacheItem
	}{
		{
			name: "should clear cache on close",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test", []byte{42}, 100*time.Millisecond); err != nil {
					t.Error(err)
				}

				return c
			},
			want: map[string]cacheItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := tt.prepare()
				err := c.Close()
				require.NoError(t, err)
				snapshot := c.snapshot()

				assert.Equal(t, len(tt.want), len(snapshot))
				assert.Equal(t, tt.want, snapshot)
			})
		})
	}
}

func TestMemoryCacheTTL(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		prepare func() *MemoryCache
		want    map[string]cacheItem
	}{
		{
			name: "should remove a cache entry after TTL via cleanup",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test", []byte{42}, 30*time.Second); err != nil {
					t.Error(err)
				}

				return c
			},
			want: map[string]cacheItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				c := tt.prepare()
				defer c.Close()

				// Items are only cleaned up after staleBufferDuration (24h) past expiry
				time.Sleep(staleBufferDuration + 2*time.Minute)
				synctest.Wait()

				snapshot := c.snapshot()
				assert.Equal(t, len(tt.want), len(snapshot))
				assert.Equal(t, tt.want, snapshot)
			})
		})
	}

	t.Run("should return ErrCacheMiss on Get for expired entry but keep for stale reads", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			c := NewMemoryClient()
			defer c.Close()

			// Set with TTL less than cleanup interval (1 minute)
			err := c.Set(ctx, "test", []byte{42}, 30*time.Second)
			require.NoError(t, err)

			// Verify entry exists via Get
			val, err := c.Get(ctx, "test")
			require.NoError(t, err)
			assert.Equal(t, []byte{42}, val)

			// Wait for TTL to expire (but less than cleanup interval)
			time.Sleep(31 * time.Second)

			// Get should detect expiration
			val, err = c.Get(ctx, "test")
			assert.Equal(t, ErrCacheMiss, err)
			assert.Nil(t, val)

			// But GetStale should still return the value
			val, err = c.GetStale(ctx, "test")
			require.NoError(t, err)
			assert.Equal(t, []byte{42}, val)

			// Entry should still be in snapshot (kept for stale reads)
			snapshot := c.snapshot()
			assert.Len(t, snapshot, 1)
		})
	})
}
