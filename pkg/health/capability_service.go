package health

import (
	"context"
	"fmt"
)

// CapabilityService returns health-capability snapshots from registered probes.
type CapabilityService interface {
	GetCapabilities(ctx context.Context) []Capability
}

type capabilityServiceImpl struct {
	registry CapabilityRegistry
}

// NewCapabilityService creates a capability service backed by a registry.
func NewCapabilityService(registry CapabilityRegistry) CapabilityService {
	if registry == nil {
		registry = NewCapabilityRegistry()
	}

	return &capabilityServiceImpl{registry: registry}
}

// GetCapabilities evaluates registered probes and returns safe capabilities data.
func (s *capabilityServiceImpl) GetCapabilities(ctx context.Context) []Capability {
	probes := s.registry.List()
	capabilities := make([]Capability, 0, len(probes))

	for _, probe := range probes {
		capabilities = append(capabilities, safeProbeCapability(probe, ctx))
	}

	return capabilities
}

func safeProbeCapability(probe CapabilityProbe, ctx context.Context) Capability {
	capability := Capability{
		Status: StatusDown,
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			capability = Capability{
				Status:  StatusDown,
				Message: fmt.Sprintf("failed to execute health probe: %v", recovered),
			}
		}
	}()

	if probe != nil {
		capability = probe.Probe(ctx)
	}

	return capability
}
