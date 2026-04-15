package dbconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// Pure field-for-field mapping — no validation, no derivation.
func FromLegacy(c *configuration.Configuration) Config {
	db := c.Database
	return Config{
		Name:          db.Name,
		Host:          db.Host,
		Port:          db.Port,
		User:          db.User,
		Password:      db.Password,
		MigrationsDir: c.MigrationsDir,
		Pool: PoolTuning{
			MaxConns:              db.MaxConns,
			MinConns:              db.MinConns,
			MaxConnLifetime:       db.MaxConnLifetime,
			MaxConnLifetimeJitter: db.MaxConnLifetimeJitter,
			MaxConnIdleTime:       db.MaxConnIdleTime,
			HealthCheckPeriod:     db.HealthCheckPeriod,
			ConnectTimeout:        db.ConnectTimeout,
		},
	}
}
