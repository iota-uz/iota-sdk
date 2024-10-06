package test_utils

import (
	"context"
	"database/sql"
	"github.com/iota-agency/iota-erp/sdk/composables"
	_ "github.com/lib/pq"
	"github.com/rubenv/sql-migrate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
)

func SqlOpen(dbOpts string) (*sql.DB, error) {
	log.Println("dbOpts: ", dbOpts)
	return sql.Open("postgres", dbOpts)
}

func RunMigrations(db *sql.DB) error {
	log.Println("Run migrations")
	migrations := &migrate.FileMigrationSource{
		Dir: "migrations",
	}
	_, err := migrate.Exec(db, "postgres", migrations, migrate.Up)
	return err
}

func RollbackMigrations(db *sql.DB) error {
	migrations := &migrate.FileMigrationSource{
		Dir: "migrations",
	}
	_, err := migrate.Exec(db, "postgres", migrations, migrate.Down)
	return err
}

func GetTestContext(dbOpts string) (context.Context, *gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dbOpts), &gorm.Config{})
	if err != nil {
		return nil, nil, err
	}
	tx := db.Begin()
	return composables.WithTx(context.Background(), tx), tx, nil
}
