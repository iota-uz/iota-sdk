package server

import (
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type DefaultOptions struct {
	Logger        *logrus.Logger
	Configuration *configuration.Configuration
	Application   application.Application
	Pool          *pgxpool.Pool
}

func Default(options *DefaultOptions) (*server.HTTPServer, error) {
	app := options.Application

	// Core middleware stack with tracing capabilities
	app.RegisterMiddleware(
		middleware.WithLogger(options.Logger, middleware.DefaultLoggerOptions()), // This now creates the root span for each request

		// Add traced middleware for each of your key middleware functions
		middleware.TracedMiddleware("database"),
		middleware.Provide(constants.AppKey, app),
		middleware.Provide(constants.HeadKey, layouts.DefaultHead()),
		middleware.Provide(constants.LogoKey, layouts.DefaultLogo()),
		middleware.Provide(constants.SidebarHeaderKey, layouts.DefaultSidebarHeader()),
		middleware.Provide(constants.PoolKey, options.Pool),

		middleware.TracedMiddleware("cors"),
		middleware.Cors("http://localhost:3000", "ws://localhost:3000"),

		middleware.TracedMiddleware("requestParams"),
		middleware.RequestParams(),
	)

	serverInstance := server.NewHTTPServer(
		app,
		controllers.NotFound(options.Application),
		controllers.MethodNotAllowed(),
	)
	return serverInstance, nil
}
