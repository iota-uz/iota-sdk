package server

import (
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type DefaultOptions struct {
	Logger        *logrus.Logger
	Configuration *configuration.Configuration
	Application   application.Application
	Db            *gorm.DB
}

func Default(options *DefaultOptions) (*server.HttpServer, error) {
	app := options.Application
	app.RegisterMiddleware(
		middleware.WithLogger(options.Logger),
		middleware.Provide(constants.AppKey, app),
		middleware.Provide(constants.HeadKey, layouts.Head()),
		middleware.Provide(constants.LogoKey, layouts.DefaultLogo()),
		middleware.Provide(constants.DBKey, options.Db),
		middleware.Provide(constants.TxKey, options.Db),
		middleware.Cors("http://localhost:3000", "ws://localhost:3000"),
		middleware.RequestParams(),
		middleware.LogRequests(),
	)
	serverInstance := server.NewHttpServer(
		app,
		controllers.NotFound(options.Application),
		controllers.MethodNotAllowed(),
	)
	return serverInstance, nil
}
