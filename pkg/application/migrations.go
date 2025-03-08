package application

import (
	"context"
	"embed"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-gorp/gorp/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	migrate "github.com/rubenv/sql-migrate"
	"github.com/sirupsen/logrus"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/schema/collector"
)

// MigrationManager is an interface for handling database migrations
type MigrationManager interface {
	// CollectSchema collects schema changes from embedded modules and generates SQL migration
	CollectSchema(ctx context.Context) error
	// Run applies pending migrations to the database
	Run() error
	// Rollback rolls back the last applied migration
	Rollback() error
	// RegisterSchema registers an embedded filesystem containing schema definitions
	RegisterSchema(fs ...*embed.FS)
	// SchemaFSs returns all registered schema embedded filesystems
	SchemaFSs() []*embed.FS
}

func NewMigrationManager(pool *pgxpool.Pool) MigrationManager {
	conf := configuration.Use()
	return &migrationManager{
		migrationsDir:  conf.MigrationsDir,
		logger:         conf.Logger(),
		pool:           pool,
		schemaEmbedFSs: make([]*embed.FS, 0),
	}
}

// migrationManager implements the MigrationManager interface
type migrationManager struct {
	migrationsDir  string
	logger         logrus.FieldLogger
	pool           *pgxpool.Pool
	schemaEmbedFSs []*embed.FS // For schema definitions in embed.FS
}

func (m *migrationManager) SchemaFSs() []*embed.FS {
	return m.schemaEmbedFSs
}

func (m *migrationManager) RegisterSchema(fs ...*embed.FS) {
	m.schemaEmbedFSs = append(m.schemaEmbedFSs, fs...)
}

// CollectSchema collects schema changes from embedded module.FS and generates SQL migration
func (m *migrationManager) CollectSchema(ctx context.Context) error {
	if err := os.MkdirAll(m.migrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	schemaCollector := collector.New(collector.Config{
		MigrationsPath: m.migrationsDir,
		LogLevel:       logrus.InfoLevel,
		EmbedFSs:       m.schemaEmbedFSs,
	})

	// Collect migrations
	upChanges, downChanges, err := schemaCollector.CollectMigrations(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect migrations: %w", err)
	}

	// Store migrations to file
	return schemaCollector.StoreMigrations(upChanges, downChanges)
}

func newTxError(migration *migrate.PlannedMigration, err error) *migrate.TxError {
	return &migrate.TxError{
		Migration: migration.Migration,
		Err:       err,
	}
}

func (m *migrationManager) applyMigrations(
	ctx context.Context,
	dir migrate.MigrationDirection,
	migrations []*migrate.PlannedMigration,
	dbMap *gorp.DbMap,
) (int, error) {
	applied := 0
	for _, migration := range migrations {
		e, err := dbMap.Begin()
		if err != nil {
			return applied, newTxError(migration, err)
		}
		executor := e.WithContext(ctx)

		for _, stmt := range migration.Queries {
			// remove the semicolon from stmt, fix ORA-00922 issue in database oracle
			stmt = strings.TrimSuffix(stmt, "\n")
			stmt = strings.TrimSuffix(stmt, " ")
			stmt = strings.TrimSuffix(stmt, ";")
			if _, err := executor.Exec(stmt); err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		}

		switch dir {
		case migrate.Up:
			err = executor.Insert(&migrate.MigrationRecord{
				Id:        migration.Id,
				AppliedAt: time.Now(),
			})
			if err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		case migrate.Down:
			_, err := executor.Delete(&migrate.MigrationRecord{
				Id: migration.Id,
			})
			if err != nil {
				if trans, ok := executor.(*gorp.Transaction); ok {
					_ = trans.Rollback()
				}

				return applied, newTxError(migration, err)
			}
		default:
			panic("Not possible")
		}

		if trans, ok := executor.(*gorp.Transaction); ok {
			if err := trans.Commit(); err != nil {
				return applied, newTxError(migration, err)
			}
		}

		applied++
	}

	return applied, nil
}

func (m *migrationManager) Run() error {
	db := stdlib.OpenDB(*m.pool.Config().ConnConfig)
	migrationSource := &migrate.FileMigrationSource{
		Dir: m.migrationsDir,
	}
	ms := migrate.MigrationSet{}
	plannedMigrations, dbMap, err := ms.PlanMigration(db, "postgres", migrationSource, migrate.Up, 0)
	if err != nil {
		return err
	}

	applied, err := m.applyMigrations(context.Background(), migrate.Up, plannedMigrations, dbMap)
	if err != nil {
		return err
	}
	m.logger.Infof("Applied %d migrations", applied)
	return nil
}

func (m *migrationManager) Rollback() error {
	db := stdlib.OpenDB(*m.pool.Config().ConnConfig)
	migrationSource := &migrate.FileMigrationSource{
		Dir: m.migrationsDir,
	}
	ms := migrate.MigrationSet{}
	plannedMigrations, dbMap, err := ms.PlanMigration(db, "postgres", migrationSource, migrate.Down, 0)
	if err != nil {
		return err
	}

	applied, err := m.applyMigrations(context.Background(), migrate.Down, plannedMigrations, dbMap)
	if err != nil {
		return err
	}
	m.logger.Infof("Rolled back %d migrations", applied)
	return nil
}
