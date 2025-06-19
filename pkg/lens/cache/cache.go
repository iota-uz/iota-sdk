package cache

import (
	"context"
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/executor"
)

// Cache provides caching functionality for query results
type Cache interface {
	// Get retrieves a cached result by key
	Get(ctx context.Context, key string) (*executor.ExecutionResult, bool)

	// Set stores a result in the cache with TTL
	Set(ctx context.Context, key string, result *executor.ExecutionResult, ttl time.Duration) error

	// Delete removes a cached result
	Delete(ctx context.Context, key string) error

	// Clear removes all cached results
	Clear(ctx context.Context) error

	// Stats returns cache statistics
	Stats() CacheStats
}

// CacheStats provides statistics about cache usage
type CacheStats struct {
	Hits        int64     // Number of cache hits
	Misses      int64     // Number of cache misses
	Entries     int       // Number of cached entries
	HitRate     float64   // Hit rate percentage
	LastCleanup time.Time // Last cleanup time
}

// MemoryCache is an in-memory cache implementation
type MemoryCache struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	stats      CacheStats
	maxEntries int
	cleanupTTL time.Duration
	stopChan   chan struct{}
}

// cacheEntry represents a cached query result
type cacheEntry struct {
	result    *executor.ExecutionResult
	createdAt time.Time
	ttl       time.Duration
	lastUsed  time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxEntries int, cleanupInterval time.Duration) *MemoryCache {
	if maxEntries == 0 {
		maxEntries = 1000 // Default max entries
	}
	if cleanupInterval == 0 {
		cleanupInterval = 5 * time.Minute // Default cleanup interval
	}

	cache := &MemoryCache{
		entries:    make(map[string]*cacheEntry),
		maxEntries: maxEntries,
		cleanupTTL: cleanupInterval,
		stopChan:   make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanupRoutine()

	return cache
}

// Get retrieves a cached result by key
func (mc *MemoryCache) Get(ctx context.Context, key string) (*executor.ExecutionResult, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	entry, exists := mc.entries[key]
	if !exists {
		mc.stats.Misses++
		return nil, false
	}

	// Check if expired
	if mc.isExpired(entry) {
		mc.stats.Misses++
		return nil, false
	}

	// Update last used time
	entry.lastUsed = time.Now()

	// Mark as cache hit
	result := *entry.result
	result.CacheHit = true

	mc.stats.Hits++
	return &result, true
}

// Set stores a result in the cache with TTL
func (mc *MemoryCache) Set(ctx context.Context, key string, result *executor.ExecutionResult, ttl time.Duration) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Check if we need to evict entries
	if len(mc.entries) >= mc.maxEntries {
		mc.evictOldest()
	}

	// Create cache entry
	entry := &cacheEntry{
		result:    result,
		createdAt: time.Now(),
		ttl:       ttl,
		lastUsed:  time.Now(),
	}

	mc.entries[key] = entry
	mc.stats.Entries = len(mc.entries)

	return nil
}

// Delete removes a cached result
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.entries, key)
	mc.stats.Entries = len(mc.entries)

	return nil
}

// Clear removes all cached results
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.entries = make(map[string]*cacheEntry)
	mc.stats.Entries = 0

	return nil
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats() CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	stats := mc.stats
	stats.Entries = len(mc.entries)

	// Calculate hit rate
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total) * 100
	}

	return stats
}

// Close stops the cache cleanup routine
func (mc *MemoryCache) Close() error {
	close(mc.stopChan)
	return nil
}

// Helper methods

// isExpired checks if a cache entry has expired
func (mc *MemoryCache) isExpired(entry *cacheEntry) bool {
	if entry.ttl == 0 {
		return false // No expiration
	}
	return time.Since(entry.createdAt) > entry.ttl
}

// evictOldest removes the oldest cache entry
func (mc *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range mc.entries {
		if oldestKey == "" || entry.lastUsed.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.lastUsed
		}
	}

	if oldestKey != "" {
		delete(mc.entries, oldestKey)
	}
}

// cleanupRoutine periodically removes expired entries
func (mc *MemoryCache) cleanupRoutine() {
	ticker := time.NewTicker(mc.cleanupTTL)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.cleanup()
		case <-mc.stopChan:
			return
		}
	}
}

// cleanup removes expired entries
func (mc *MemoryCache) cleanup() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	now := time.Now()
	for key, entry := range mc.entries {
		if mc.isExpired(entry) {
			delete(mc.entries, key)
		}
	}

	mc.stats.Entries = len(mc.entries)
	mc.stats.LastCleanup = now
}

// CachingExecutor wraps an executor with caching functionality
type CachingExecutor struct {
	executor executor.Executor
	cache    Cache
	ttl      time.Duration
}

// NewCachingExecutor creates a new caching executor
func NewCachingExecutor(exec executor.Executor, cache Cache, defaultTTL time.Duration) *CachingExecutor {
	if defaultTTL == 0 {
		defaultTTL = 5 * time.Minute // Default TTL
	}

	return &CachingExecutor{
		executor: exec,
		cache:    cache,
		ttl:      defaultTTL,
	}
}

// Execute executes a query with caching
func (ce *CachingExecutor) Execute(ctx context.Context, query executor.ExecutionQuery) (*executor.ExecutionResult, error) {
	// Generate cache key
	cacheKey := ce.generateCacheKey(query)

	// Try to get from cache first
	if result, found := ce.cache.Get(ctx, cacheKey); found {
		return result, nil
	}

	// Execute query
	result, err := ce.executor.Execute(ctx, query)
	if err != nil {
		return result, err
	}

	// Cache the result
	ttl := ce.ttl
	if query.TimeRange.End.Sub(query.TimeRange.Start) < time.Hour {
		// Shorter TTL for recent data
		ttl = 1 * time.Minute
	}

	ce.cache.Set(ctx, cacheKey, result, ttl)

	return result, nil
}

// ExecutePanel executes a panel query with caching
func (ce *CachingExecutor) ExecutePanel(ctx context.Context, panel lens.PanelConfig, variables map[string]interface{}) (*executor.ExecutionResult, error) {
	return ce.executor.ExecutePanel(ctx, panel, variables)
}

// ExecuteDashboard executes dashboard queries with caching
func (ce *CachingExecutor) ExecuteDashboard(ctx context.Context, dashboard lens.DashboardConfig) (*executor.DashboardResult, error) {
	return ce.executor.ExecuteDashboard(ctx, dashboard)
}

// RegisterDataSource registers a data source
func (ce *CachingExecutor) RegisterDataSource(id string, ds datasource.DataSource) error {
	return ce.executor.RegisterDataSource(id, ds)
}

// Close closes the caching executor
func (ce *CachingExecutor) Close() error {
	return ce.executor.Close()
}

// generateCacheKey generates a cache key for a query
func (ce *CachingExecutor) generateCacheKey(query executor.ExecutionQuery) string {
	// Create a hash of the query parameters
	data := fmt.Sprintf("%s:%s:%v:%v:%d:%s",
		query.DataSourceID,
		query.Query,
		query.Variables,
		query.TimeRange,
		query.MaxRows,
		query.Format,
	)

	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("query:%x", hash)
}

// Global cache instance
var DefaultCache Cache

// Initialize default cache
func init() {
	DefaultCache = NewMemoryCache(1000, 5*time.Minute)
}

// Convenience functions for global cache

// Get retrieves a result from the default cache
func Get(ctx context.Context, key string) (*executor.ExecutionResult, bool) {
	return DefaultCache.Get(ctx, key)
}

// Set stores a result in the default cache
func Set(ctx context.Context, key string, result *executor.ExecutionResult, ttl time.Duration) error {
	return DefaultCache.Set(ctx, key, result, ttl)
}

// Delete removes a result from the default cache
func Delete(ctx context.Context, key string) error {
	return DefaultCache.Delete(ctx, key)
}

// Clear removes all results from the default cache
func Clear(ctx context.Context) error {
	return DefaultCache.Clear(ctx)
}

// Stats returns statistics from the default cache
func Stats() CacheStats {
	return DefaultCache.Stats()
}
