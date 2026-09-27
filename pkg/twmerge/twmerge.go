// Package twmerge resolves conflicting Tailwind classes, the way
// github.com/Oudwins/tailwind-merge-go does — every component in this repo
// merges a caller-supplied class string against its own defaults on the render
// path, which means the merge runs on as many goroutines as the server has
// in-flight requests.
//
// The upstream package's ready-made twmerge.Merge is not safe there. Two
// independent races, both reachable from an ordinary page render:
//
//  1. Its default cache (pkg/lru) guards the map and the intrusive list with
//     two separate mutexes. Set removes a node while holding only cacheMutex,
//     while an eviction removes a node holding only listMutex; when both touch
//     the same node the second remove dereferences a nil prev. That is a panic
//     in the middle of writing a response, not a slow render.
//
//  2. CreateTwMerge builds its closures on first call and assigns them —
//     fnToCall, splitModifiers, getClassGroupId, mergeClassList, cache — with
//     no synchronisation at all. Concurrent first calls race on the function
//     value itself.
//
// Passing a cache in settles (1): the nil check in upstream's init is the only
// thing that ever calls lru.Make, so the broken implementation becomes
// unreachable rather than merely unlikely. Warming the function once, from a
// single goroutine, during package initialisation settles (2): every call a
// request can make reads state written before main() began.
//
// Import this instead of the upstream package. The call signature is the same,
// so a call site changes its import and nothing else.
package twmerge

import (
	"sync"

	twm "github.com/Oudwins/tailwind-merge-go/pkg/twmerge"
)

// safeCache is upstream's cache.ICache under one mutex.
//
// It is deliberately not an LRU. The keys are class strings written in source,
// so the live set is bounded by the component tree rather than by traffic, and
// a keyless flush costs a handful of re-merges. An LRU costs a linked list that
// has to stay correct under concurrency — which is the thing upstream got
// wrong, and there is no reason to write it a second time.
type safeCache struct {
	mu       sync.RWMutex
	capacity int
	entries  map[string]string
}

func newSafeCache(capacity int) *safeCache {
	if capacity < 1 {
		capacity = 1
	}
	return &safeCache{
		capacity: capacity,
		entries:  make(map[string]string, capacity),
	}
}

func (c *safeCache) Get(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entries[key]
}

func (c *safeCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.capacity {
		c.entries = make(map[string]string, c.capacity)
	}
	c.entries[key] = value
}

// merge is initialised, and called once, before main() — see the package doc.
// The warm call is load-bearing: without it the first two concurrent renders
// race inside upstream's lazy init.
var merge = func() twm.TwMergeFn {
	config := twm.MakeDefaultConfig()
	fn := twm.CreateTwMerge(config, newSafeCache(config.MaxCacheSize))
	fn("p-0")
	return fn
}()

// Merge joins the given class strings and drops every class a later one
// overrides. The survivors come back in upstream's class-group order, which is
// not the order they were written in.
//
// It is a function rather than a variable so that no importer can reassign the
// merge behaviour of every component in the repo from an init somewhere.
func Merge(classes ...string) string {
	return merge(classes...)
}
