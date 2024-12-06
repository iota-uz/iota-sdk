package testutils

import (
	"context"
	"database/sql"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/logging"
	"github.com/iota-agency/iota-sdk/pkg/server"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	"strings"
)

type TestContext struct {
	SQLDB   *sql.DB
	GormDB  *gorm.DB
	Context context.Context
	Tx      *gorm.DB
}

func DBSetup(app application.Application) error {
	db, err := app.DB().DB()
	if err != nil {
		return err
	}
	if err := DropPublicSchema(db); err != nil {
		return err
	}
	if err := app.RunMigrations(); err != nil {
		return err
	}
	return nil
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
	conf := configuration.Use()
	db, err := dbutils.ConnectDB(
		conf.DBOpts,
		gormlogger.New(
			logging.ConsoleLogger(logrus.InfoLevel),
			gormlogger.Config{
				SlowThreshold:             0,
				LogLevel:                  gormlogger.Info,
				IgnoreRecordNotFoundError: false,
				Colorful:                  true,
				ParameterizedQueries:      true,
			},
		),
	)
	if err != nil {
		panic(err)
	}
	app := server.ConstructApp(db)
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		panic(err)
	}
	tx := db.Begin()
	sqlDB, err := tx.DB()
	if err != nil {
		panic(err)
	}
	if err := DBSetup(app); err != nil {
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
