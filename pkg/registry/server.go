package registry

import (
	"embed"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/presentation/assets"
	"github.com/iota-agency/iota-sdk/pkg/presentation/controllers"
	"github.com/iota-agency/iota-sdk/pkg/server"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/shared"
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

	registry := modules.Load()
	app := ConstructApp(db)

	assetsFs := append([]*embed.FS{&assets.FS}, registry.Assets()...)
	controllerInstances := []shared.Controller{
		controllers.NewLoginController(app),
		controllers.NewAccountController(app),
		controllers.NewEmployeeController(app),
		controllers.NewGraphQLController(app),
		controllers.NewLogoutController(app),
		controllers.NewStaticFilesController(assetsFs),
	}

	for _, module := range registry.Modules() {
		if err := module.Register(app); err != nil {
			return nil, errors.Wrapf(err, "failed to register module %s", module.Name())
		}
	}

	for _, c := range registry.Controllers() {
		controllerInstances = append(controllerInstances, c(app))
	}

	bundle := modules.LoadBundle(registry)
	authService := app.Service(services.AuthService{}).(*services.AuthService)
	serverInstance := &server.HttpServer{
		Middlewares: []mux.MiddlewareFunc{
			middleware.Cors([]string{"http://localhost:3000", "ws://localhost:3000"}),
			middleware.RequestParams(middleware.DefaultParamsConstructor),
			middleware.WithLogger(log.Default()),
			middleware.LogRequests(),
			middleware.Transactions(db),
			middleware.Authorization(authService),
			middleware.WithLocalizer(bundle),
			middleware.NavItems(),
		},
		Controllers: controllerInstances,
	}
	return serverInstance, nil
}
