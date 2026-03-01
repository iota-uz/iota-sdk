package subscription

import (
	"context"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type memoryCacheEntry struct {
	value     []byte
	expiresAt time.Time
}

type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]memoryCacheEntry
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{items: make(map[string]memoryCacheEntry)}
}

func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	now := time.Now()

	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}

	if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
		c.mu.Lock()
		latest, exists := c.items[key]
		if !exists {
			c.mu.Unlock()
			return nil, false, nil
		}
		if !latest.expiresAt.IsZero() && now.After(latest.expiresAt) {
			delete(c.items, key)
			c.mu.Unlock()
			return nil, false, nil
		}
		out := make([]byte, len(latest.value))
		copy(out, latest.value)
		c.mu.Unlock()
		return out, true, nil
	}
	out := make([]byte, len(entry.value))
	copy(out, entry.value)
	return out, true, nil
}

func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	entry := memoryCacheEntry{value: make([]byte, len(value))}
	copy(entry.value, value)
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.mu.Lock()
	c.items[key] = entry
	c.mu.Unlock()
	return nil
}

func (c *MemoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(redisURL string) (*RedisCache, error) {
	const op serrors.Op = "SubscriptionCache.NewRedisCache"

	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return &RedisCache{client: redis.NewClient(opts)}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	const op serrors.Op = "SubscriptionCache.Get"

	value, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, serrors.E(op, err)
	}
	return value, true, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	const op serrors.Op = "SubscriptionCache.Set"

	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	const op serrors.Op = "SubscriptionCache.Delete"

	if err := c.client.Del(ctx, key).Err(); err != nil {
		return serrors.E(op, err)
	}
	return nil
}
