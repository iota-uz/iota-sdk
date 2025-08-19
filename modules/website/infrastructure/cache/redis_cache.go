package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/cache"
	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

func NewRedisCache(client *redis.Client, prefix string, ttl time.Duration) *RedisCache {
	if prefix == "" {
		prefix = "cached_ai_responses"
	}
	return &RedisCache{client: client, prefix: prefix, ttl: ttl}
}

func (c *RedisCache) Get(ctx context.Context, key string) (string, error) {
	result, err := c.client.Get(ctx, c.getKey(key)).Result()
	if err != nil {
		if err == redis.Nil {
			return "", cache.ErrKeyNotFound
		}

		return "", err
	}

	return result, nil
}

func (c *RedisCache) Set(ctx context.Context, key, value string) error {
	return c.client.SetEx(ctx, c.getKey(key), value, c.ttl).Err()
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.getKey(key)).Err()
}

func (c *RedisCache) getKey(key string) string {
	return fmt.Sprintf("%s:%s", c.prefix, key)
}
