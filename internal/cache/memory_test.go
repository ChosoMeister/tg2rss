package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCacheWrite(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		prepare func() *MemoryCache
		want    map[string][]byte
	}{
		{
			name: "should write a new entry in empty cache",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test", []byte{42}, 100*time.Millisecond); err != nil {
					t.Error(err)
				}

				return c
			},
			want: map[string][]byte{"test": {42}},
		},
		{
			name: "should write a new entry in non-empty cache",
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
			want: map[string][]byte{"test1": {42}, "test2": {42}},
		},
		{
			name: "should skip caching if TTL is 0",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test", []byte{42}, 0); err != nil {
					t.Error(err)
				}

				return c
			},
			want: map[string][]byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.prepare()
			snapshot := c.snapshot()

			assert.Equal(t, len(tt.want), len(snapshot))
			assert.Equal(t, tt.want, snapshot)
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
			c := tt.prepare()
			v, err := c.Get(ctx, tt.key)

			assert.Equal(t, tt.want, v)
			assert.Equal(t, tt.err, err)
		})
	}
}

func TestMemoryCacheClose(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		prepare func() *MemoryCache
		want    map[string][]byte
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
			want: map[string][]byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.prepare()
			err := c.Close()
			require.NoError(t, err)
			snapshot := c.snapshot()

			assert.Equal(t, len(tt.want), len(snapshot))
			assert.Equal(t, tt.want, snapshot)
		})
	}
}

func TestMemoryCacheTTL(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		prepare func() *MemoryCache
		want    map[string][]byte
	}{
		{
			name: "should remove a cache entry after TTL",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test", []byte{42}, 5*time.Millisecond); err != nil {
					t.Error(err)
				}

				return c
			},
			want: map[string][]byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.prepare()

			time.Sleep(10 * time.Millisecond)

			snapshot := c.snapshot()
			assert.Equal(t, len(tt.want), len(snapshot))
			assert.Equal(t, tt.want, snapshot)
		})
	}
}

func TestMemoryContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tests := []struct {
		name    string
		prepare func() *MemoryCache
		want    map[string][]byte
	}{
		{
			name: "should stop TTL-goroutine on context cancel",
			prepare: func() *MemoryCache {
				c := NewMemoryClient()

				if err := c.Set(ctx, "test", []byte{42}, 5*time.Millisecond); err != nil {
					t.Error(err)
				}

				return c
			},
			want: map[string][]byte{"test": {42}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.prepare()

			cancel()
			time.Sleep(10 * time.Millisecond)

			snapshot := c.snapshot()
			assert.Equal(t, len(tt.want), len(snapshot))
			assert.Equal(t, tt.want, snapshot)
		})
	}
}
