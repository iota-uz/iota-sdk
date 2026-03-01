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

func (r *capabilityRegistryImpl) Register(probe CapabilityProbe) {
	if probe == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.probes = append(r.probes, probe)
}

func (r *capabilityRegistryImpl) List() []CapabilityProbe {
	r.mu.RLock()
	defer r.mu.RUnlock()

	probes := make([]CapabilityProbe, len(r.probes))
	copy(probes, r.probes)

	return probes
}
