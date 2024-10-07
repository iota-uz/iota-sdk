package test_utils

import (
	"context"
	"database/sql"
	"errors"

	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/sdk/composables"
	_ "github.com/lib/pq"
	migrate "github.com/rubenv/sql-migrate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func DBSetup() (*sql.DB, error) {
	db, err := SQLOpen(configuration.Use().DBOpts)
	if err != nil {
		return nil, err
	}
	if err := DropPublicSchema(db); err != nil {
		return nil, err
	}
	if err := RunMigrations(db); err != nil {
		return nil, err
	}
	return db, nil
}

func DBTeardown(db *sql.DB) error {
	return RollbackMigrations(db)
}

func SQLOpen(dbOpts string) (*sql.DB, error) {
	return sql.Open("postgres", dbOpts)
}

func DropPublicSchema(db *sql.DB) error {
	_, err := db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	return err
}

func RunMigrations(db *sql.DB) error {
	migrations := &migrate.FileMigrationSource{
		Dir: "migrations/postgres",
	}
	n, err := migrate.Exec(db, "postgres", migrations, migrate.Up)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no migrations found")
	}
	return nil
}

func RollbackMigrations(db *sql.DB) error {
	migrations := &migrate.FileMigrationSource{
		Dir: "migrations/postgres",
	}
	n, err := migrate.Exec(db, "postgres", migrations, migrate.Down)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("no migrations found")
	}
	return nil
}

func GormOpen(dbOpts string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(dbOpts), &gorm.Config{})
}

func GetTestContext(dbOpts string) (context.Context, *gorm.DB, error) {
	db, err := GormOpen(dbOpts)
	if err != nil {
		return nil, nil, err
	}
	tx := db.Begin()
	return composables.WithTx(context.Background(), tx), tx, nil
}
