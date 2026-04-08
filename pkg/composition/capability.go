package composition

import (
	"slices"
	"strings"
)

type Capability string

const (
	CapabilityAPI     Capability = "api"
	CapabilityWorker  Capability = "worker"
	CapabilityMigrate Capability = "migrate"
	CapabilitySeed    Capability = "seed"
	CapabilityCLI     Capability = "cli"
)

func normalizeCapabilities(capabilities []Capability) []Capability {
	if len(capabilities) == 0 {
		return nil
	}

	seen := make(map[Capability]struct{}, len(capabilities))
	normalized := make([]Capability, 0, len(capabilities))
	for _, capability := range capabilities {
		capability = Capability(strings.TrimSpace(string(capability)))
		if capability == "" {
			continue
		}
		if _, ok := seen[capability]; ok {
			continue
		}
		seen[capability] = struct{}{}
		normalized = append(normalized, capability)
	}
	slices.Sort(normalized)
	return normalized
}
