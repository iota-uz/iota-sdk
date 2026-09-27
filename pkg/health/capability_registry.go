// Package health provides this package.
package health

import (
	"context"
	"sync"
)

type CapabilityProbe interface {
	Probe(ctx context.Context) Capability
}

type CapabilityProbeFunc func(ctx context.Context) Capability

func (f CapabilityProbeFunc) Probe(ctx context.Context) Capability {
	return f(ctx)
}

type CapabilityRegistry interface {
	Register(probe CapabilityProbe)
	List() []CapabilityProbe
}

type capabilityRegistryImpl struct {
	mu     sync.RWMutex
	probes []CapabilityProbe
}

func NewCapabilityRegistry() CapabilityRegistry {
	return &capabilityRegistryImpl{
		probes: make([]CapabilityProbe, 0),
	}
}

// Register appends probe to the registry's probe list. If probe produces a
// Capability with a non-empty Key, any earlier-registered probe reporting the
// same Key is superseded at List time (see List for the dedup rules). Nil
// probes are ignored.
func (r *capabilityRegistryImpl) Register(probe CapabilityProbe) {
	if probe == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.probes = append(r.probes, probe)
}

// List returns the currently-registered probes, deduplicated by Capability.Key
// with last-wins semantics: when two probes emit the same Key, only the one
// registered later appears in the returned slice. Probes producing a capability
// with an empty Key are always included (the framework cannot collapse them).
//
// Dedup is evaluated at List call time, not at Register time, because probes
// are free to adjust their reported capability on each Probe call. The probes
// are re-evaluated here by invoking Probe with a background context to read
// only the Key field; the full result is not cached. This is cheap for the
// small N of feature gates and avoids a parallel "key index" that could drift.
//
// The returned slice preserves registration order for the surviving entries,
// so callers get a stable iteration sequence across List calls.
func (r *capabilityRegistryImpl) List() []CapabilityProbe {
	r.mu.RLock()
	probes := make([]CapabilityProbe, len(r.probes))
	copy(probes, r.probes)
	r.mu.RUnlock()

	if len(probes) < 2 {
		return probes
	}

	// Walk in registration order and remember the latest index for each non-empty
	// Key. Then filter out superseded entries.
	latest := make(map[string]int, len(probes))
	for i, p := range probes {
		key := safeProbeKey(p)
		if key == "" {
			continue
		}
		latest[key] = i
	}
	if len(latest) == 0 {
		return probes
	}

	out := make([]CapabilityProbe, 0, len(probes))
	for i, p := range probes {
		key := safeProbeKey(p)
		if key != "" && latest[key] != i {
			continue
		}
		out = append(out, p)
	}
	return out
}

// safeProbeKey calls Probe with a background context and returns Capability.Key
// without panicking on probe failures. Used for dedup only; callers that need
// the full Capability should use CapabilityService.
func safeProbeKey(probe CapabilityProbe) string {
	if probe == nil {
		return ""
	}
	key := ""
	func() {
		defer func() { _ = recover() }()
		key = probe.Probe(context.Background()).Key
	}()
	return key
}
