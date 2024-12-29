package main

import (
	"context"
	"log"
	"time"

	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/internal/server"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/finance"
	"github.com/iota-uz/iota-sdk/modules/warehouse"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/application/dbutils"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/event"
	"github.com/iota-uz/iota-sdk/pkg/logging"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	gormlogger "gorm.io/gorm/logger"
)

func main() {
	conf := configuration.Use()
	logFile, logger, err := logging.FileLogger(conf.LogrusLogLevel())
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logFile.Close()

	db, err := dbutils.ConnectDB(
		conf.DBOpts,
		gormlogger.New(
			logger,
			gormlogger.Config{
				SlowThreshold:             0,
				LogLevel:                  conf.GormLogLevel(),
				IgnoreRecordNotFoundError: false,
				Colorful:                  true,
				ParameterizedQueries:      true,
			},
		),
	)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.DBOpts)
	if err != nil {
		panic(err)
	}
	app := application.New(db, pool, event.NewEventPublisher())
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to load modules: %v", err)
	}
	app.RegisterNavItems(core.NavItems...)
	app.RegisterNavItems(warehouse.NavItems...)
	app.RegisterNavItems(finance.NavItems...)
	app.RegisterHashFsAssets(internalassets.HashFS)
	app.RegisterControllers(
		controllers.NewStaticFilesController(app.HashFsAssets()),
		controllers.NewGraphQLController(app),
	)

	if err := dbutils.CheckModels(db, server.RegisteredModels); err != nil {
		log.Fatal(err)
	}

	options := &server.DefaultOptions{
		Logger:        logger,
		Configuration: conf,
		Db:            db,
		Application:   app,
		Pool:          pool,
	}
	serverInstance, err := server.Default(options)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
