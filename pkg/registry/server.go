package registry

import (
	"github.com/benbjohnson/hashfs"
	"github.com/go-faster/errors"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/presentation/assets"
	"github.com/iota-agency/iota-sdk/pkg/presentation/controllers"
	"github.com/iota-agency/iota-sdk/pkg/server"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"gorm.io/gorm/logger"
	"log"
)

func NewServer(conf *configuration.Configuration) (*server.HttpServer, error) {
	db, err := dbutils.ConnectDB(conf.DBOpts, logger.Error)
	if err != nil {
		return nil, err
	}
	if err := dbutils.CheckModels(db, server.RegisteredModels); err != nil {
		return nil, err
	}

	loadedModules := modules.Load()
	app := ConstructApp(db)

	for _, module := range loadedModules {
		if err := module.Register(app); err != nil {
			return nil, errors.Wrapf(err, "failed to register module %s", module.Name())
		} else {
			log.Printf("Module %s registered", module.Name())
		}
	}
	assetsFs := append([]*hashfs.FS{assets.HashFS}, app.HashFsAssets()...)
	app.RegisterControllers(
		controllers.NewLoginController(app),
		controllers.NewAccountController(app),
		controllers.NewEmployeeController(app),
		controllers.NewGraphQLController(app),
		controllers.NewLogoutController(app),
		controllers.NewStaticFilesController(assetsFs),
	)
	authService := app.Service(services.AuthService{}).(*services.AuthService)
	bundle, err := app.Bundle()
	if err != nil {
		return nil, err
	}
	app.RegisterMiddleware(
		middleware.Cors([]string{"http://localhost:3000", "ws://localhost:3000"}),
		middleware.RequestParams(middleware.DefaultParamsConstructor),
		middleware.WithLogger(log.Default()),
		middleware.LogRequests(),
		middleware.Transactions(db),
		middleware.Authorization(authService),
		middleware.WithLocalizer(bundle),
		middleware.NavItems(app),
	)
	serverInstance := &server.HttpServer{
		Middlewares: app.Middleware(),
		Controllers: app.Controllers(),
	}
	return serverInstance, nil
}
