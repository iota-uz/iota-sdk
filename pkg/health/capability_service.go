package health

import "context"

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
		capabilities = append(capabilities, probe.Probe(ctx))
	}

	return capabilities
}
