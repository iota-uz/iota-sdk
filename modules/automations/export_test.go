package automations

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
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
