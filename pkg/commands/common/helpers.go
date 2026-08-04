// Package common provides this package.
package common

import (
	"context"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/dbconfig"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// GetDatabasePool creates a database connection pool using the provided dbconfig.Config.
// If dbName is non-empty it overrides cfg.Name for this call only.
func GetDatabasePool(ctx context.Context, cfg *dbconfig.Config, dbName string) (*pgxpool.Pool, error) {
	connStr := cfg.ConnectionString()
	if dbName != "" {
		override := *cfg
		override.Name = dbName
		connStr = override.ConnectionString()
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database %s: %w", dbName, err)
	}

	return pool, nil
}

// GetDefaultDatabasePool creates a database connection pool using the config's default database name.
func GetDefaultDatabasePool(cfg *dbconfig.Config) (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return GetDatabasePool(ctx, cfg, "")
}

// NewApplication creates a new application with consistent setup patterns.
// src is the config.Source attached to the composition BuildContext so that
// stdconfig values (dbconfig, httpconfig, uploadsconfig, …) are auto-registered
// in the DI container and constructor injection works end-to-end. Pass nil
// only when the caller is certain no component will resolve a stdconfig.
func NewApplication(pool *pgxpool.Pool, logger *logrus.Logger, src config.Source, components ...composition.Component) (application.Application, error) {
	bundle := application.LoadBundle()

	app, err := application.New(&application.ApplicationOptions{
		Pool:               pool,
		Bundle:             bundle,
		EventBus:           eventbus.NewEventPublisher(logger),
		Logger:             logger,
		SupportedLanguages: application.DefaultSupportedLanguages(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	engine := composition.NewEngine()
	if err := engine.Register(components...); err != nil {
		return nil, serrors.E(serrors.Op("commands.common.NewApplication"), err)
	}
	_, err = engine.Compile(
		composition.NewBuildContext(app, src, composition.WithLogger(logger)),
		composition.CapabilityAPI,
		composition.CapabilityWorker,
	)
	if err != nil {
		return nil, serrors.E(serrors.Op("commands.common.NewApplication"), err)
	}

	return app, nil
}

// NewApplicationWithDefaults creates an application with default database and built-in modules.
// src is threaded through to NewApplication; see NewApplication for its role.
func NewApplicationWithDefaults(cfg *dbconfig.Config, logger *logrus.Logger, src config.Source, components ...composition.Component) (application.Application, *pgxpool.Pool, error) {
	pool, err := GetDefaultDatabasePool(cfg)
	if err != nil {
		return nil, nil, err
	}

	app, err := NewApplication(pool, logger, src, components...)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}

	return app, pool, nil
}
