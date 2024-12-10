package server

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/presentation/controllers"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/layouts"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type DefaultOptions struct {
	Logger        *logrus.Logger
	Configuration *configuration.Configuration
	Application   application.Application
	Db            *gorm.DB
}

func Default(options *DefaultOptions) (*HttpServer, error) {
	db := options.Db
	app := options.Application

	if err := dbutils.CheckModels(db, RegisteredModels); err != nil {
		return nil, err
	}
	app.RegisterMiddleware(
		middleware.WithLogger(options.Logger),
		middleware.Provide(constants.HeadKey, layouts.Head()),
		middleware.Provide(constants.LogoKey, layouts.DefaultLogo()),
		middleware.Provide(constants.DBKey, db),
		middleware.Provide(constants.TxKey, db),
		middleware.Cors("http://localhost:3000", "ws://localhost:3000"),
		middleware.RequestParams(),
		middleware.LogRequests(),
	)
	serverInstance := &HttpServer{
		Middlewares:             app.Middleware(),
		Controllers:             app.Controllers(),
		NotFoundHandler:         controllers.NotFound(options.Application),
		MethodNotAllowedHandler: controllers.MethodNotAllowed(),
	}
	return serverInstance, nil
}
