package server

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/application/dbutils"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/constants"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/layouts"
	"github.com/iota-agency/iota-sdk/pkg/services"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type DefaultOptions struct {
	Logger        *logrus.Logger
	Configuration *configuration.Configuration
	Application   application.Application
	Db            *gorm.DB
}

func head() types.HeadComponent {
	return layouts.Head
}

func Default(options *DefaultOptions) (*HttpServer, error) {
	db := options.Db
	app := options.Application

	if err := dbutils.CheckModels(db, RegisteredModels); err != nil {
		return nil, err
	}
	authService := app.Service(services.AuthService{}).(*services.AuthService)
	tabService := app.Service(services.TabService{}).(*services.TabService)
	bundle, err := app.Bundle()
	if err != nil {
		return nil, err
	}

	app.RegisterMiddleware(
		middleware.WithLogger(options.Logger),
		middleware.Provide(constants.HeadKey, head()),
		middleware.Provide(constants.LogoKey, layouts.DefaultLogo()),
		middleware.Provide(constants.DBKey, db),
		middleware.Cors("http://localhost:3000", "ws://localhost:3000"),
		middleware.RequestParams(),
		middleware.LogRequests(),
		middleware.Transactions(),
		middleware.Authorization(authService),
		middleware.WithLocalizer(bundle),
		middleware.Tabs(tabService),
		middleware.NavItems(app),
	)
	serverInstance := &HttpServer{
		Middlewares: app.Middleware(),
		Controllers: app.Controllers(),
	}
	return serverInstance, nil
}
