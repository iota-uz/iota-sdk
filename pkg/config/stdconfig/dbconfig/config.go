// Package dbconfig provides typed configuration for the PostgreSQL database
// connection and connection-pool tuning. It is a stdconfig package intended
// to be registered via config.Register[dbconfig.Config].
package dbconfig

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolTuning groups all pgxpool tuning parameters under the "pool" sub-key.
type PoolTuning struct {
	MaxConns              int32         `koanf:"maxconns"`
	MinConns              int32         `koanf:"minconns"`
	MaxConnLifetime       time.Duration `koanf:"maxconnlifetime"`
	MaxConnLifetimeJitter time.Duration `koanf:"maxconnlifetimejitter"`
	MaxConnIdleTime       time.Duration `koanf:"maxconnidletime"`
	HealthCheckPeriod     time.Duration `koanf:"healthcheckperiod"`
	ConnectTimeout        time.Duration `koanf:"connecttimeout"`
}

// Config holds all database connection and pool-tuning settings.
// Env prefix: "db" (e.g. DB_HOST → db.host, DB_POOL_MAX_CONNS → db.pool.maxconns).
type Config struct {
	Name          string     `koanf:"name"`
	Host          string     `koanf:"host"`
	Port          string     `koanf:"port"`
	User          string     `koanf:"user"`
	Password      string     `koanf:"password" secret:"true"`
	MigrationsDir string     `koanf:"migrationsdir"`
	Pool          PoolTuning `koanf:"pool"`
}

// ConnectionString returns a libpq-style connection string.
func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Name, c.Password,
	)
}

// PoolConfig returns a fully configured *pgxpool.Config derived from the
// connection string and pool-tuning fields.
//
// Validation: MaxConns > 0, MinConns <= MaxConns, ConnectTimeout > 0.
// The AfterConnect hook sets idle_in_transaction_session_timeout to 120s.
func (c *Config) PoolConfig() (*pgxpool.Config, error) {
	if c.Pool.MaxConns <= 0 {
		return nil, fmt.Errorf("dbconfig: pool.maxconns must be positive, got %d", c.Pool.MaxConns)
	}
	if c.Pool.MinConns > c.Pool.MaxConns {
		return nil, fmt.Errorf(
			"dbconfig: pool.minconns (%d) must not exceed pool.maxconns (%d)",
			c.Pool.MinConns, c.Pool.MaxConns,
		)
	}
	if c.Pool.ConnectTimeout <= 0 {
		return nil, fmt.Errorf(
			"dbconfig: pool.connecttimeout must be positive, got %s",
			c.Pool.ConnectTimeout,
		)
	}

	cfg, err := pgxpool.ParseConfig(c.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("dbconfig: parse connection string: %w", err)
	}

	cfg.MaxConns = c.Pool.MaxConns
	cfg.MinConns = c.Pool.MinConns
	cfg.MaxConnLifetime = c.Pool.MaxConnLifetime
	cfg.MaxConnLifetimeJitter = c.Pool.MaxConnLifetimeJitter
	cfg.MaxConnIdleTime = c.Pool.MaxConnIdleTime
	cfg.HealthCheckPeriod = c.Pool.HealthCheckPeriod
	cfg.ConnConfig.ConnectTimeout = c.Pool.ConnectTimeout

	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET idle_in_transaction_session_timeout = '120s'")
		return err
	}

	return cfg, nil
}
