package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/sirupsen/logrus"
	"github.com/ulule/limiter/v3"
)

type Option func(*options)

type options struct {
	logger                  *logrus.Logger
	before                  []mux.MiddlewareFunc
	after                   []mux.MiddlewareFunc
	notFoundHandler         http.Handler
	methodNotAllowedHandler http.Handler
	corsOrigins             []string
	rateLimit               *RateLimitOptions
}

type RateLimitOptions struct {
	Enabled   bool
	GlobalRPS int
	Storage   string
	RedisURL  string
}

func WithBeforeMiddleware(middlewareFns ...mux.MiddlewareFunc) Option {
	return func(o *options) {
		o.before = append(o.before, middlewareFns...)
	}
}

func WithAfterMiddleware(middlewareFns ...mux.MiddlewareFunc) Option {
	return func(o *options) {
		o.after = append(o.after, middlewareFns...)
	}
}

func WithNotFoundHandler(handler http.Handler) Option {
	return func(o *options) {
		o.notFoundHandler = handler
	}
}

func WithMethodNotAllowedHandler(handler http.Handler) Option {
	return func(o *options) {
		o.methodNotAllowedHandler = handler
	}
}

func WithCORS(origins ...string) Option {
	return func(o *options) {
		o.corsOrigins = append([]string(nil), origins...)
	}
}

func WithRateLimit(cfg RateLimitOptions) Option {
	return func(o *options) {
		rateLimitCfg := cfg
		o.rateLimit = &rateLimitCfg
	}
}

func New(rt *bootstrap.Runtime, opts ...Option) (*HTTPServer, error) {
	if rt == nil || rt.App == nil {
		return nil, fmt.Errorf("bootstrap runtime with application is required")
	}
	if rt.Container() == nil {
		return nil, fmt.Errorf("bootstrap runtime with composition container is required")
	}

	cfg := options{
		logger:                  rt.Logger,
		notFoundHandler:         controllers.NotFound(),
		methodNotAllowedHandler: controllers.MethodNotAllowed(),
		corsOrigins:             []string{"http://localhost:3000"},
	}
	if appConfig, ok := rt.Config.(*configuration.Configuration); ok && appConfig != nil {
		cfg.rateLimit = &RateLimitOptions{
			Enabled:   appConfig.RateLimit.Enabled,
			GlobalRPS: appConfig.RateLimit.GlobalRPS,
			Storage:   appConfig.RateLimit.Storage,
			RedisURL:  appConfig.RateLimit.RedisURL,
		}
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	stack := make([]mux.MiddlewareFunc, 0, len(cfg.before)+len(cfg.after)+10)
	stack = append(stack, cfg.before...)
	stack = append(stack,
		middleware.WithLogger(cfg.logger, middleware.DefaultLoggerOptions()),
		middleware.TracedMiddleware("database"),
		middleware.Provide(constants.AppKey, rt.App),
		middleware.Provide(constants.ContainerKey, rt.Container()),
		middleware.Provide(constants.HeadKey, layouts.DefaultHead()),
		middleware.Provide(constants.LogoKey, assets.DefaultLogo()),
		middleware.Provide(constants.PoolKey, rt.Pool),
		middleware.ProvideLocalizer(rt.App.Bundle(), rt.App.GetSupportedLanguages()),
		middleware.TracedMiddleware("cors"),
		middleware.Cors(cfg.corsOrigins...),
	)

	if cfg.rateLimit != nil && cfg.rateLimit.Enabled {
		var store limiter.Store
		switch cfg.rateLimit.Storage {
		case "redis":
			redisStore, err := middleware.NewRedisStore(cfg.rateLimit.RedisURL)
			if err != nil {
				return nil, fmt.Errorf("create redis rate limit store: %w", err)
			} else {
				store = redisStore
			}
		default:
			store = middleware.NewMemoryStore()
		}
		stack = append(stack,
			middleware.TracedMiddleware("rateLimit"),
			middleware.RateLimit(middleware.RateLimitConfig{
				RequestsPerPeriod: cfg.rateLimit.GlobalRPS,
				Store:             store,
			}),
		)
	}

	stack = append(stack,
		middleware.TracedMiddleware("requestParams"),
		middleware.RequestParams(),
	)
	stack = append(stack, cfg.after...)
	rt.Container().AppendMiddleware(stack...)

	return NewHTTPServer(
		rt.App,
		cfg.notFoundHandler,
		cfg.methodNotAllowedHandler,
	), nil
}

func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

func (s *HTTPServer) Handler() http.Handler {
	return s.handler()
}
