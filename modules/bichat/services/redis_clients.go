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
		return nil, nil //nolint:nilnil // REDIS_URL unset means graceful in-memory fallback; nil return is the signal
	}
	return newRedisClient(redisURL)
}

// newConfiguredRedisComponents builds all Redis-backed infrastructure from a
// single shared client so the process dials only one connection to Redis.
//
// When REDIS_URL is unset or empty the function returns all nils and a nil
// error: this is the intentional "no Redis" path (in-memory fallback). When
// REDIS_URL IS set but any construction step fails the error is returned so
// the caller can surface the misconfiguration loudly instead of silently
// degrading to the in-memory fallback with a broken Redis connection.
//
// Ownership invariant: each component's Close() is a no-op because the
// client was supplied externally (ownsClient == false). The caller MUST
// invoke the returned closeFunc exactly once during shutdown to release the
// shared connection. The closeFunc is nil when Redis is not configured.
func newConfiguredRedisComponents() (RunEventLog, ActiveRunIndex, *RedisRunJobQueue, func() error, error) {
	client, err := NewSharedRedisClient()
	if err != nil {
		// REDIS_URL was set but dialling failed — surface the error.
		return nil, nil, nil, nil, err
	}
	if client == nil {
		// REDIS_URL unset — intentional in-memory fallback, not an error.
		return nil, nil, nil, nil, nil
	}

	closeClient := func() error { return client.Close() }

	eventLog, err := NewRedisRunEventLog(RedisRunEventLogConfig{Client: client})
	if err != nil {
		_ = client.Close()
		return nil, nil, nil, nil, err
	}

	index, err := NewRedisActiveRunIndex(RedisActiveRunIndexConfig{Client: client})
	if err != nil {
		_ = client.Close()
		return nil, nil, nil, nil, err
	}

	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{Client: client})
	if err != nil {
		_ = client.Close()
		return nil, nil, nil, nil, err
	}

	return eventLog, index, queue, closeClient, nil
}
