package common

import (
	"context"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetDatabasePool creates a database connection pool with the specified database name
func GetDatabasePool(ctx context.Context, dbName string) (*pgxpool.Pool, error) {
	conf := configuration.Use()

	// Override database name if specified
	if dbName != "" {
		originalName := conf.Database.Name
		conf.Database.Name = dbName
		conf.Database.Opts = conf.Database.ConnectionString()
		defer func() {
			conf.Database.Name = originalName
		}()
	}

	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database %s: %w", dbName, err)
	}

	return pool, nil
}

// GetDefaultDatabasePool creates a database connection pool using default configuration
func GetDefaultDatabasePool() (*pgxpool.Pool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return GetDatabasePool(ctx, "")
}

// NewApplication creates a new application with consistent setup patterns
func NewApplication(pool *pgxpool.Pool, mods ...application.Module) (application.Application, error) {
	conf := configuration.Use()
	bundle := application.LoadBundle()

	app, err := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize application: %w", err)
	}

	if err := modules.Load(app, mods...); err != nil {
		return nil, fmt.Errorf("failed to load modules: %w", err)
	}

	return app, nil
}

// NewApplicationWithDefaults creates an application with default database and built-in modules
func NewApplicationWithDefaults(mods ...application.Module) (application.Application, *pgxpool.Pool, error) {
	pool, err := GetDefaultDatabasePool()
	if err != nil {
		return nil, nil, err
	}

	app, err := NewApplication(pool, mods...)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}

	return app, pool, nil
}
