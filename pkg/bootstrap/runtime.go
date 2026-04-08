package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type Runtime struct {
	Config any
	Logger *logrus.Logger
	Pool   *pgxpool.Pool
	Bundle *i18n.Bundle
	App    application.Application

	values map[reflect.Type]any
}

type Installer interface {
	Install(ctx context.Context, rt *Runtime) error
}

type InstallerFunc func(ctx context.Context, rt *Runtime) error

func (f InstallerFunc) Install(ctx context.Context, rt *Runtime) error {
	return f(ctx, rt)
}

func (rt *Runtime) Install(ctx context.Context, installers ...Installer) error {
	for _, installer := range installers {
		if installer == nil {
			continue
		}
		if err := installer.Install(ctx, rt); err != nil {
			return err
		}
	}
	return nil
}

func (rt *Runtime) Provide(values ...any) {
	if rt.values == nil {
		rt.values = make(map[reflect.Type]any, len(values))
	}
	for _, value := range values {
		if value == nil {
			continue
		}
		rt.values[reflect.TypeOf(value)] = value
	}
}

func (rt *Runtime) Use(target any) bool {
	if rt == nil || target == nil {
		return false
	}
	ptr := reflect.ValueOf(target)
	if ptr.Kind() != reflect.Ptr || ptr.IsNil() {
		return false
	}
	value, ok := rt.values[ptr.Elem().Type()]
	if !ok {
		return false
	}
	ptr.Elem().Set(reflect.ValueOf(value))
	return true
}

type Option func(*options)

type options struct {
	config        any
	loggerFactory func(context.Context, any) (*logrus.Logger, func() error, error)
	poolFactory   func(context.Context, any, *logrus.Logger) (*pgxpool.Pool, func() error, error)
	bundleFactory func(context.Context, any) (*i18n.Bundle, error)
	appFactory    func(context.Context, *Runtime) (application.Application, error)
}

func WithConfig(config any) Option {
	return func(o *options) {
		o.config = config
	}
}

func WithLoggerFactory(factory func(context.Context, any) (*logrus.Logger, func() error, error)) Option {
	return func(o *options) {
		o.loggerFactory = factory
	}
}

func WithPoolFactory(factory func(context.Context, any, *logrus.Logger) (*pgxpool.Pool, func() error, error)) Option {
	return func(o *options) {
		o.poolFactory = factory
	}
}

func WithBundleFactory(factory func(context.Context, any) (*i18n.Bundle, error)) Option {
	return func(o *options) {
		o.bundleFactory = factory
	}
}

func WithApplicationFactory(factory func(context.Context, *Runtime) (application.Application, error)) Option {
	return func(o *options) {
		o.appFactory = factory
	}
}

func NewRuntime(ctx context.Context, opts ...Option) (*Runtime, func() error, error) {
	cfg := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	if cfg.loggerFactory == nil {
		return nil, nil, errors.New("bootstrap runtime requires a logger factory")
	}
	if cfg.poolFactory == nil {
		return nil, nil, errors.New("bootstrap runtime requires a pool factory")
	}
	if cfg.bundleFactory == nil {
		return nil, nil, errors.New("bootstrap runtime requires a bundle factory")
	}
	if cfg.appFactory == nil {
		return nil, nil, errors.New("bootstrap runtime requires an application factory")
	}

	rt := &Runtime{
		Config: cfg.config,
		values: make(map[reflect.Type]any),
	}

	var cleanup []func() error

	logger, loggerCleanup, err := cfg.loggerFactory(ctx, cfg.config)
	if err != nil {
		return nil, nil, fmt.Errorf("build logger: %w", err)
	}
	rt.Logger = logger
	if loggerCleanup != nil {
		cleanup = append(cleanup, loggerCleanup)
	}

	pool, poolCleanup, err := cfg.poolFactory(ctx, cfg.config, logger)
	if err != nil {
		runCleanup(cleanup)
		return nil, nil, fmt.Errorf("build pool: %w", err)
	}
	rt.Pool = pool
	if poolCleanup != nil {
		cleanup = append(cleanup, poolCleanup)
	}

	bundle, err := cfg.bundleFactory(ctx, cfg.config)
	if err != nil {
		runCleanup(cleanup)
		return nil, nil, fmt.Errorf("build bundle: %w", err)
	}
	rt.Bundle = bundle

	app, err := cfg.appFactory(ctx, rt)
	if err != nil {
		runCleanup(cleanup)
		return nil, nil, fmt.Errorf("build application: %w", err)
	}
	rt.App = app
	rt.Provide(logger, pool, bundle, app)

	return rt, func() error {
		var cleanupErr error
		if rt.App != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cleanupErr = errors.Join(cleanupErr, rt.App.StopRuntime(stopCtx))
		}
		cleanupErr = errors.Join(cleanupErr, runCleanup(cleanup))
		return cleanupErr
	}, nil
}

func runCleanup(cleanups []func() error) error {
	var err error
	for i := len(cleanups) - 1; i >= 0; i-- {
		err = errors.Join(err, cleanups[i]())
	}
	return err
}
