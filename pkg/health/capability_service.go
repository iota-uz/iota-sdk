package health

import (
	"context"
	"fmt"
)

type CapabilityService interface {
	GetCapabilities(ctx context.Context) []Capability
}

type capabilityServiceImpl struct {
	registry CapabilityRegistry
}

func NewCapabilityService(registry CapabilityRegistry) CapabilityService {
	if registry == nil {
		registry = NewCapabilityRegistry()
	}

	return &capabilityServiceImpl{registry: registry}
}

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
