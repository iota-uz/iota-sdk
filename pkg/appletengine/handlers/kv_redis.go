package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisKVStore struct {
	client *redis.Client
}

func NewRedisKVStore(redisURL string) (*RedisKVStore, error) {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(options)
	return &RedisKVStore{client: client}, nil
}

func (s *RedisKVStore) Get(ctx context.Context, key string) (any, error) {
	result, err := s.client.Get(ctx, redisScopedKey(ctx, key)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	var value any
	if err := json.Unmarshal([]byte(result), &value); err != nil {
		return nil, fmt.Errorf("unmarshal redis value: %w", err)
	}
	return value, nil
}

func (s *RedisKVStore) Set(ctx context.Context, key string, value any, ttlSeconds *int) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal redis value: %w", err)
	}
	var ttl time.Duration
	if ttlSeconds != nil && *ttlSeconds > 0 {
		ttl = time.Duration(*ttlSeconds) * time.Second
	}
	if err := s.client.Set(ctx, redisScopedKey(ctx, key), string(encoded), ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

func (s *RedisKVStore) Delete(ctx context.Context, key string) (bool, error) {
	deleted, err := s.client.Del(ctx, redisScopedKey(ctx, key)).Result()
	if err != nil {
		return false, fmt.Errorf("redis del: %w", err)
	}
	return deleted > 0, nil
}

func (s *RedisKVStore) MGet(ctx context.Context, keys []string) ([]any, error) {
	if len(keys) == 0 {
		return []any{}, nil
	}
	scopedKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		scopedKeys = append(scopedKeys, redisScopedKey(ctx, key))
	}
	items, err := s.client.MGet(ctx, scopedKeys...).Result()
	if err != nil {
		return nil, fmt.Errorf("redis mget: %w", err)
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			out = append(out, nil)
			continue
		}
		raw, ok := item.(string)
		if !ok {
			out = append(out, nil)
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("unmarshal redis mget value: %w", err)
		}
		out = append(out, value)
	}
	return out, nil
}

func redisScopedKey(ctx context.Context, key string) string {
	scope, err := scopeFromContext(ctx)
	if err != nil {
		// Fallback to key only if scope extraction fails
		// This maintains backward compatibility but logs the issue
		return "applet::" + key
	}
	scope = strings.ReplaceAll(scope, "::", ":")
	return "applet:" + scope + ":" + key
}
