package main

import (
	"context"
	"log"

	"github.com/benbjohnson/hashfs"
	internalassets "github.com/iota-agency/iota-sdk/internal/assets"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/event"
	"github.com/iota-agency/iota-sdk/pkg/logging"
	"github.com/iota-agency/iota-sdk/pkg/server"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.DBOpts)
	if err != nil {
		panic(err)
	}
	app := application.New(db, pool, event.NewEventPublisher())
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to load modules: %v", err)
	}
	assetsFs := append(
		[]*hashfs.FS{
			internalassets.HashFS,
		},
		app.HashFsAssets()...,
	)
	app.RegisterControllers(
		controllers.NewStaticFilesController(assetsFs),
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
