// Package services provides this package.
package services

import (
	"strings"

	"github.com/redis/go-redis/v9"
)

// NewSharedRedisClient returns a single *redis.Client built from REDIS_URL.
// Returns nil, nil when REDIS_URL is unset (caller falls back to in-memory).
// Callers close it via the component's shutdown hook.
func NewSharedRedisClient() (*redis.Client, error) {
	redisURL, ok := envLookup("REDIS_URL")
	if !ok || strings.TrimSpace(redisURL) == "" {
		return nil, nil
	}
	return newRedisClient(redisURL)
}

// newConfiguredRedisComponents builds all Redis-backed infrastructure from a
// single shared client so the process dials only one connection to Redis.
// Returns all nils when Redis is not configured (REDIS_URL unset).
func newConfiguredRedisComponents() (RunEventLog, ActiveRunIndex, *RedisRunJobQueue) {
	client, err := NewSharedRedisClient()
	if err != nil || client == nil {
		return nil, nil, nil
	}

	eventLog, err := NewRedisRunEventLog(RedisRunEventLogConfig{Client: client})
	if err != nil {
		_ = client.Close()
		return nil, nil, nil
	}

	index, err := NewRedisActiveRunIndex(RedisActiveRunIndexConfig{Client: client})
	if err != nil {
		_ = client.Close()
		return nil, nil, nil
	}

	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{Client: client})
	if err != nil {
		_ = client.Close()
		return nil, nil, nil
	}

	return eventLog, index, queue
}
