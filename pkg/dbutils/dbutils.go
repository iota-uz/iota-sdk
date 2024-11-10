package dbutils

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/iota-agency/iota-erp/internal/modules"
	"github.com/iota-agency/iota-erp/sdk/graphql/helpers"
	migrate "github.com/rubenv/sql-migrate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

func NewLogger(level logger.LogLevel) logger.Interface {
	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)
}

func ConnectDB(dbOpts string, level logger.LogLevel) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dbOpts), &gorm.Config{
		//nolint:exhaustruct
		Logger:                 NewLogger(level),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CheckModels(db *gorm.DB, modelsToTest []interface{}) error {
	var errs []error
	for _, model := range modelsToTest {
		if err := helpers.CheckModelIsInSync(db, model); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("models are out of sync: %w", errors.Join(errs...))
}

func collectMigrations() ([]*migrate.Migration, error) {
	migrationDirs := []string{"migrations/postgres"}
	for _, m := range modules.LoadedModules {
		migrationDirs = append(migrationDirs, m.MigrationDirs()...)
	}
	var migrations []*migrate.Migration
	for _, dir := range migrationDirs {
		migrationSource := &migrate.FileMigrationSource{
			Dir: dir,
		}
		foundMigrations, err := migrationSource.FindMigrations()
		if err != nil {
			return nil, err
		}
		if len(foundMigrations) == 0 {
			return nil, errors.New("no migrations found")
		}
		migrations = append(migrations, foundMigrations...)
	}
	return migrations, nil
}

func RunMigrations(db *sql.DB) error {
	migrations, err := collectMigrations()
	if err != nil {
		return err
	}
	migrationSource := &migrate.MemoryMigrationSource{
		Migrations: migrations,
	}
	n, err := migrate.Exec(db, "postgres", migrationSource, migrate.Up)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no migrations found")
	}
	return nil
}

func RollbackMigrations(db *sql.DB) error {
	migrations, err := collectMigrations()
	if err != nil {
		return err
	}
	migrationSource := &migrate.MemoryMigrationSource{
		Migrations: migrations,
	}
	n, err := migrate.Exec(db, "postgres", migrationSource, migrate.Down)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no migrations found")
	}
	return nil
}
