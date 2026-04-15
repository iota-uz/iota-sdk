package dbconfig_test

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// buildLegacyConfiguration returns a *configuration.Configuration with
// representative values for dbconfig mapping tests. It does NOT call
// configuration.Use() or load any .env files.
func buildLegacyConfiguration() *configuration.Configuration {
	return &configuration.Configuration{
		MigrationsDir: "migrations",
		Database: configuration.DatabaseOptions{
			Name:                  "legacydb",
			Host:                  "db.legacy.example.com",
			Port:                  "5432",
			User:                  "legacyuser",
			Password:              "legacypass",
			MaxConns:              64,
			MinConns:              16,
			MaxConnLifetime:       2 * time.Hour,
			MaxConnLifetimeJitter: 10 * time.Minute,
			MaxConnIdleTime:       30 * time.Minute,
			HealthCheckPeriod:     2 * time.Minute,
			ConnectTimeout:        15 * time.Second,
		},
	}
}
