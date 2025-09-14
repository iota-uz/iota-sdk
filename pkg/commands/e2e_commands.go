package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	coreseed "github.com/iota-uz/iota-sdk/modules/core/seed"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/aichatconfig"
	websiteseed "github.com/iota-uz/iota-sdk/modules/website/seed"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	E2E_DB_NAME = "iota_erp_e2e"
)

func pgxPoolE2E() (*pgxpool.Pool, error) {
	conf := configuration.Use()

	// Override database name for e2e tests
	originalName := conf.Database.Name
	conf.Database.Name = E2E_DB_NAME
	conf.Database.Opts = conf.Database.ConnectionString()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		// If connection to e2e database fails, try connecting to postgres database to create it
		conf.Database.Name = "postgres"
		conf.Database.Opts = conf.Database.ConnectionString()
		postgresPool, postgresErr := pgxpool.New(ctx, conf.Database.Opts)
		if postgresErr != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", postgresErr)
		}
		defer postgresPool.Close()

		// Create the e2e database
		_, createErr := postgresPool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", E2E_DB_NAME))
		if createErr != nil {
			// Database might already exist, try to connect again
			conf.Database.Name = originalName
		}

		// Try connecting to e2e database again
		conf.Database.Name = E2E_DB_NAME
		conf.Database.Opts = conf.Database.ConnectionString()
		pool, err = pgxpool.New(ctx, conf.Database.Opts)
		if err != nil {
			return nil, err
		}
	}
	return pool, nil
}

func E2ECreate() error {
	ctx := context.Background()
	conf := configuration.Use()

	// Connect directly to postgres database
	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		conf.Database.Host, conf.Database.Port, conf.Database.User, conf.Database.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	// Drop existing e2e database if exists
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2E_DB_NAME))
	if err != nil {
		return fmt.Errorf("failed to drop existing e2e database: %w", err)
	}

	// Create new e2e database
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", E2E_DB_NAME))
	if err != nil {
		return fmt.Errorf("failed to create e2e database: %w", err)
	}

	conf.Logger().Info(" Created e2e database: %s\n", E2E_DB_NAME)
	return nil
}

func E2EDrop() error {
	ctx := context.Background()
	conf := configuration.Use()

	// Connect directly to postgres database
	connString := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=disable",
		conf.Database.Host, conf.Database.Port, conf.Database.User, conf.Database.Password)

	conn, err := pgx.Connect(ctx, connString)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres database: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	// Drop e2e database
	_, err = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", E2E_DB_NAME))
	if err != nil {
		return fmt.Errorf("failed to drop e2e database: %w", err)
	}

	conf.Logger().Info(" Dropped e2e database: %s\n", E2E_DB_NAME)
	return nil
}

func E2EMigrate() error {
	// Get current directory and find project root (where go.mod is)
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Go up directories until we find go.mod (project root)
	projectRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return fmt.Errorf("could not find project root with go.mod")
		}
		projectRoot = parent
	}

	if err := os.Chdir(projectRoot); err != nil {
		return fmt.Errorf("failed to change to project root: %w", err)
	}

	// Set environment variable for e2e database
	_ = os.Setenv("DB_NAME", E2E_DB_NAME)

	// Run migrations using the existing migrate command
	conf := configuration.Use()
	conf.Database.Name = E2E_DB_NAME
	conf.Database.Opts = conf.Database.ConnectionString()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		return fmt.Errorf("failed to connect to e2e database for migrations: %w", err)
	}
	defer pool.Close()

	bundle := application.LoadBundle()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	})
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		return err
	}

	// Apply migrations
	migrations := app.Migrations()
	if err := migrations.Run(); err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	conf.Logger().Info(" Applied migrations to e2e database\n")
	return nil
}

func E2ESeed() error {
	// Set environment variable for e2e database
	_ = os.Setenv("DB_NAME", E2E_DB_NAME)

	conf := configuration.Use()
	ctx := context.Background()
	pool, err := pgxPoolE2E()
	if err != nil {
		return err
	}
	defer pool.Close()

	bundle := application.LoadBundle()
	app := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(conf.Logger()),
		Logger:   conf.Logger(),
	})
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		return err
	}
	app.RegisterNavItems(modules.NavLinks...)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}

	seeder := application.NewSeeder()

	// Create test user
	usr, err := user.New(
		"Test",
		"User",
		internet.MustParseEmail("test@gmail.com"),
		user.UILanguageEN,
	).SetPassword("TestPass123!")
	if err != nil {
		return err
	}

	// Add default tenant to context
	defaultTenant := &composables.Tenant{
		ID:     uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		Name:   "Default",
		Domain: "default.localhost",
	}

	allPermissions := defaults.AllPermissions()
	seeder.Register(
		coreseed.CreateDefaultTenant,
		coreseed.CreateCurrencies,
		func(ctx context.Context, app application.Application) error {
			return coreseed.CreatePermissions(ctx, app, allPermissions)
		},
		coreseed.UserSeedFunc(usr, allPermissions),
		coreseed.UserSeedFunc(user.New(
			"AI",
			"User",
			internet.MustParseEmail("ai@llm.com"),
			user.UILanguageEN,
			user.WithTenantID(defaultTenant.ID),
		), allPermissions),
		websiteseed.AIChatConfigSeedFunc(aichatconfig.MustNew(
			"gemma-12b-it",
			aichatconfig.AIModelTypeOpenAI,
			"https://llm2.eai.uz/v1",
			aichatconfig.WithTenantID(defaultTenant.ID),
		)),
	)

	ctxWithTenant := composables.WithTenantID(
		composables.WithTx(ctx, tx),
		defaultTenant.ID,
	)

	if err := seeder.Seed(ctxWithTenant, app); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("rollback failed: %w (original error: %w)", rollbackErr, err)
		}
		return err
	}

	if err := tx.Commit(ctxWithTenant); err != nil {
		return err
	}

	conf.Logger().Info(" Seeded e2e database with test data\n")
	return nil
}

func E2ESetup() error {
	fmt.Println("🚀 Setting up e2e database...")
	if err := E2ECreate(); err != nil {
		return err
	}
	if err := E2EMigrate(); err != nil {
		return err
	}
	if err := E2ESeed(); err != nil {
		return err
	}
	fmt.Println("✅ E2E database setup complete!")
	return nil
}

func E2EReset() error {
	fmt.Println("🔄 Resetting e2e database...")
	if err := E2ECreate(); err != nil { // This drops and recreates
		return err
	}
	if err := E2EMigrate(); err != nil {
		return err
	}
	if err := E2ESeed(); err != nil {
		return err
	}
	fmt.Println("✅ E2E database reset complete!")
	return nil
}
