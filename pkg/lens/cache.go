package lens

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// WithCache wraps a DataSource with an in-memory TTL cache keyed by query string.
// Identical queries within the TTL window return cached results without hitting
// the underlying data source.
func WithCache(ds DataSource, ttl time.Duration) DataSource {
	return &cachedDataSource{
		inner:   ds,
		ttl:     ttl,
		entries: make(map[string]*cacheEntry),
	}
}

type cachedDataSource struct {
	inner   DataSource
	ttl     time.Duration
	mu      sync.RWMutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	result    *QueryResult
	createdAt time.Time
}

func (c *cachedDataSource) Execute(ctx context.Context, query string) (*QueryResult, error) {
	key := cacheKey(query)

	// Check cache.
	c.mu.RLock()
	if e, ok := c.entries[key]; ok && time.Since(e.createdAt) < c.ttl {
		c.mu.RUnlock()
		return e.result, nil
	}
	c.mu.RUnlock()

	// Cache miss — execute.
	result, err := c.inner.Execute(ctx, query)
	if err != nil {
		return nil, err
	}

	// Store.
	c.mu.Lock()
	c.entries[key] = &cacheEntry{result: result, createdAt: time.Now()}
	c.mu.Unlock()

	return result, nil
}

func (c *cachedDataSource) Close() error {
	c.mu.Lock()
	c.entries = make(map[string]*cacheEntry)
	c.mu.Unlock()
	return c.inner.Close()
}

func cacheKey(query string) string {
	h := sha256.Sum256([]byte(query))
	return fmt.Sprintf("%x", h[:16])
}
