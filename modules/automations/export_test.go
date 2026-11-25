package automations

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// DefaultEngineUsage is the usage recorder the module's engine runs with by default.
func DefaultEngineUsage(db *pgxpool.Pool) catalog.UsageRecorder {
	return defaultEngineConfig(db, nil, nil, nil, nil, nil).Usage
}

// EngineConnectors is the connector directory the module's engine masks call output by.
func EngineConnectors(container *composition.Container) connectors.Directory {
	return containerEngineConfig(container, nil, nil, nil, nil).Connectors
}

// EngineLinks are the links the composition's engine runs with; nil without an engine.
func EngineLinks(container *composition.Container) (engine.Links, error) {
	if !composition.Has[*engineHolder](container) {
		return nil, nil
	}
	h, err := composition.Resolve[*engineHolder](container)
	if err != nil {
		return nil, err
	}
	return h.links, nil
}

// StartPlatformConnectors is the start hook of the platform credentials of the connectors.
var StartPlatformConnectors = startPlatformConnectors

// RenewPlatformConnectors is one renewal of the platform reservations' leases.
var RenewPlatformConnectors = renewPlatformConnectors
