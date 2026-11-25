package automations

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// DefaultEngineUsage is the usage recorder the module's engine runs with by default.
func DefaultEngineUsage(db *pgxpool.Pool) catalog.UsageRecorder {
	return defaultEngineConfig(db, nil, nil, nil, nil).Usage
}

// StartPlatformConnectors is the start hook of the platform credentials of the connectors.
var StartPlatformConnectors = startPlatformConnectors

// RenewPlatformConnectors is one renewal of the platform reservations' leases.
var RenewPlatformConnectors = renewPlatformConnectors
