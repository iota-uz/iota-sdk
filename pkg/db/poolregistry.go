// Package db provides shared database utilities used across SDK modules.
//
// The PoolRegistry lets consumers hold multiple role-scoped *pgxpool.Pool
// instances under distinct names and resolve them by name at tool
// construction time. The SDK ships no default pools: consumers dial and
// register their own.
package db

import (
	"fmt"
	"sort"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolRegistry is a thread-safe name→pool registry.
//
// Typical usage: an application wires two pools at boot — one under the
// default role for writes, one under a read-only role for the LLM — and
// hands the registry to services that need to pick between them.
//
//	reg := db.NewPoolRegistry()
//	_ = reg.Register("default", mainPool)
//	_ = reg.Register("ai_readonly", readonlyPool)
//	pool, err := reg.Get("ai_readonly")
type PoolRegistry struct {
	mu    sync.RWMutex
	pools map[string]*pgxpool.Pool
}

// NewPoolRegistry returns an empty registry.
func NewPoolRegistry() *PoolRegistry {
	return &PoolRegistry{pools: make(map[string]*pgxpool.Pool)}
}

// Register binds name to pool. Returns an error when name is empty, pool is
// nil, or name is already registered. Re-registration is a bug (would lose
// the previous pool); callers that want replacement must Unregister first.
func (r *PoolRegistry) Register(name string, pool *pgxpool.Pool) error {
	if name == "" {
		return fmt.Errorf("db.PoolRegistry.Register: name is required")
	}
	if pool == nil {
		return fmt.Errorf("db.PoolRegistry.Register: pool is nil (name=%q)", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.pools[name]; exists {
		return fmt.Errorf("db.PoolRegistry.Register: pool %q already registered", name)
	}
	r.pools[name] = pool
	return nil
}

// Unregister removes name from the registry. Returns true if the entry
// existed. Does not close the pool; the caller still owns its lifecycle.
func (r *PoolRegistry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.pools[name]; !ok {
		return false
	}
	delete(r.pools, name)
	return true
}

// Get returns the pool registered under name. Errors when no entry exists.
func (r *PoolRegistry) Get(name string) (*pgxpool.Pool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pool, ok := r.pools[name]
	if !ok {
		return nil, fmt.Errorf("db.PoolRegistry.Get: pool %q not registered", name)
	}
	return pool, nil
}

// Has reports whether name is registered.
func (r *PoolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.pools[name]
	return ok
}

// Names returns registered names in sorted order. Intended for diagnostics
// and tests; do not rely on the slice for lifecycle decisions (entries may
// be added or removed concurrently).
func (r *PoolRegistry) Names() []string {
	r.mu.RLock()
	names := make([]string, 0, len(r.pools))
	for name := range r.pools {
		names = append(names, name)
	}
	r.mu.RUnlock()
	sort.Strings(names)
	return names
}
