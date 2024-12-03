package main

import (
	"github.com/benbjohnson/hashfs"
	internalassets "github.com/iota-agency/iota-sdk/internal/assets"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/presentation/assets"
	"github.com/iota-agency/iota-sdk/pkg/presentation/controllers"
	"github.com/iota-agency/iota-sdk/pkg/server"
	_ "github.com/lib/pq"
	"gorm.io/gorm/logger"
	"log"
)

func main() {
	conf := configuration.Use()
	db, err := dbutils.ConnectDB(conf.DBOpts, logger.Error)
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}

	app := server.ConstructApp(db)
	if err := modules.Load(app, modules.BuiltInModules...); err != nil {
		log.Fatalf("failed to load modules: %v", err)
	}
	assetsFs := append(
		[]*hashfs.FS{
			assets.HashFS,
			internalassets.HashFS,
		},
		app.HashFsAssets()...,
	)
	app.RegisterControllers(
		controllers.NewLoginController(app),
		controllers.NewAccountController(app),
		controllers.NewEmployeeController(app),
		controllers.NewGraphQLController(app),
		controllers.NewLogoutController(app),
		controllers.NewStaticFilesController(assetsFs),
		controllers.NewUploadController(app),
	)
	options := &server.DefaultOptions{
		Configuration: conf,
		Db:            db,
		Application:   app,
	}
	serverInstance, err := server.Default(options)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	log.Printf("starting server on %s", conf.SocketAddress)
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
