package server

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
	"gorm.io/gorm"
	"log"

	"github.com/go-faster/errors"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/services"
)

type DefaultOptions struct {
	Configuration *configuration.Configuration
	LoadedModules []application.Module
	Application   application.Application
	Db            *gorm.DB
}

func Default(options *DefaultOptions) (*HttpServer, error) {
	db := options.Db
	app := options.Application

	if err := dbutils.CheckModels(db, RegisteredModels); err != nil {
		return nil, err
	}
	for _, module := range options.LoadedModules {
		if err := module.Register(app); err != nil {
			return nil, errors.Wrapf(err, "failed to register module %s", module.Name())
		} else {
			log.Printf("Module %s registered", module.Name())
		}
	}
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
	serverInstance := &HttpServer{
		Middlewares: app.Middleware(),
		Controllers: app.Controllers(),
	}
	return serverInstance, nil
}
