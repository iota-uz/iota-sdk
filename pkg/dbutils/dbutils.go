package dbutils

import (
	"bytes"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/graphql/helpers"
	migrate "github.com/rubenv/sql-migrate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

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
	registry := modules.Load()
	migrationDirs := append([]*embed.FS{&embeddedMigrations}, registry.MigrationDirs()...)

	var migrations []*migrate.Migration
	for _, fs := range migrationDirs {
		files, err := fs.ReadDir("migrations")
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.IsDir() {
				continue
			}
			content, err := fs.ReadFile("migrations/" + file.Name())
			if err != nil {
				return nil, err
			}
			migration, err := migrate.ParseMigration(file.Name(), bytes.NewReader(content))
			if err != nil {
				return nil, err
			}
			migrations = append(migrations, migration)
		}
	}
	if len(migrations) == 0 {
		return nil, errors.New("no migrations found")
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
		return errors.New("no migrations applied")
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
