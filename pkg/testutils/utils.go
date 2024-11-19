package testutils

import (
	"context"
	"database/sql"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/dbutils"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
)

type TestContext struct {
	SQLDB   *sql.DB
	GormDB  *gorm.DB
	Context context.Context
	Tx      *gorm.DB
}

func DBSetup(db *sql.DB) error {
	if err := DropPublicSchema(db); err != nil {
		return err
	}
	if err := dbutils.RunMigrations(db); err != nil {
		return err
	}
	return nil
}

func DBTeardown(db *sql.DB) error {
	return dbutils.RollbackMigrations(db)
}

func DropPublicSchema(db *sql.DB) error {
	q := strings.Join(
		[]string{
			"DROP SCHEMA IF EXISTS public CASCADE",
			"CREATE SCHEMA public",
			"GRANT ALL ON SCHEMA public TO postgres",
			"GRANT ALL ON SCHEMA public TO public;",
		}, ";",
	)
	_, err := db.Exec(q)
	return err
}

func GetTestContext() *TestContext {
	db, err := dbutils.ConnectDB(configuration.Use().DBOpts, logger.Warn)
	if err != nil {
		panic(err)
	}
	tx := db.Begin()
	sqlDB, err := tx.DB()
	if err != nil {
		panic(err)
	}
	if err := DBSetup(sqlDB); err != nil {
		panic(err)
	}
	ctx := composables.WithTx(context.Background(), tx)
	ctx = composables.WithParams(
		ctx,
		&composables.Params{
			IP:            "",
			UserAgent:     "",
			Authenticated: true,
			Request:       nil,
			Writer:        nil,
			Meta:          nil,
		},
	)

	return &TestContext{
		SQLDB:   sqlDB,
		GormDB:  db,
		Tx:      tx,
		Context: ctx,
	}
}
