package server

import (
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"github.com/ulule/limiter/v3"
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
	middlewares := []mux.MiddlewareFunc{
		middleware.WithLogger(options.Logger, middleware.DefaultLoggerOptions()), // This now creates the root span for each request

		// Add traced middleware for each of your key middleware functions
		middleware.TracedMiddleware("database"),
		middleware.Provide(constants.AppKey, app),
		middleware.Provide(constants.HeadKey, layouts.DefaultHead()),
		middleware.Provide(constants.LogoKey, assets.DefaultLogo()),
		middleware.Provide(constants.PoolKey, options.Pool),

		middleware.TracedMiddleware("cors"),
		middleware.Cors("http://localhost:3000", "ws://localhost:3000"),
	}

	// Add rate limiting middleware if enabled
	if options.Configuration.RateLimit.Enabled {
		var store limiter.Store
		var err error

		// Choose storage backend
		switch options.Configuration.RateLimit.Storage {
		case "redis":
			store, err = middleware.NewRedisStore(options.Configuration.RateLimit.RedisURL)
			if err != nil {
				options.Logger.WithError(err).Warn("Failed to create Redis store for rate limiting, falling back to memory")
				store = middleware.NewMemoryStore()
			}
		default:
			store = middleware.NewMemoryStore()
		}

		// Add global rate limiting middleware
		middlewares = append(middlewares,
			middleware.TracedMiddleware("rateLimit"),
			middleware.RateLimit(middleware.RateLimitConfig{
				RequestsPerPeriod: options.Configuration.RateLimit.GlobalRPS,
				Store:             store,
			}),
		)
	}

	middlewares = append(middlewares,
		middleware.TracedMiddleware("requestParams"),
		middleware.RequestParams(),
	)

	app.RegisterMiddleware(middlewares...)

	serverInstance := server.NewHTTPServer(
		app,
		controllers.NotFound(options.Application),
		controllers.MethodNotAllowed(),
	)
	return serverInstance, nil
}
