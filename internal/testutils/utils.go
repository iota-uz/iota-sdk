package testutils

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

type TestContext struct {
	SqlDB   *sql.DB
	GormDB  *gorm.DB
	Context context.Context
	Tx      *gorm.DB
}

func DBSetup(db *sql.DB) error {
	if err := DropPublicSchema(db); err != nil {
		return err
	}
	if err := RunMigrations(db); err != nil {
		return err
	}
	return nil
}

func DBTeardown(db *sql.DB) error {
	return RollbackMigrations(db)
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

func GormOpen() *gorm.DB {
	dbOpts := configuration.Use().DBOpts
	db, err := gorm.Open(postgres.Open(dbOpts), &gorm.Config{}) //nolint:exhaustruct
	if err != nil {
		panic(err)
	}
	return db
}

func GetTestContext() *TestContext {
	db := GormOpen()
	tx := db.Begin()
	sqlDB, err := tx.DB()
	if err != nil {
		panic(err)
	}
	if err := DBSetup(sqlDB); err != nil {
		panic(err)
	}
	return &TestContext{
		SqlDB:   sqlDB,
		GormDB:  db,
		Tx:      tx,
		Context: composables.WithTx(context.Background(), tx),
	}
}
