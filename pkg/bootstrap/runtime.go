package bootstrap

import (
	"context"
	"errors"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type Runtime struct {
	Config any
	Logger *logrus.Logger
	Pool   *pgxpool.Pool
	Bundle *i18n.Bundle
	App    application.Application
	Engine *composition.Engine

	container *composition.Container
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

func (rt *Runtime) Container() *composition.Container {
	if rt == nil {
		return nil
	}
	return rt.container
}

func (rt *Runtime) BuildContext() composition.BuildContext {
	if rt == nil {
		return composition.BuildContext{}
	}
	cfg, _ := rt.Config.(*configuration.Configuration)
	return composition.NewBuildContext(rt.App, cfg)
}

func (rt *Runtime) SetComposition(engine *composition.Engine, container *composition.Container) error {
	if rt == nil {
		return nil
	}
	rt.Engine = engine
	rt.container = container
	if rt.App != nil && container != nil {
		if binder, ok := rt.App.(application.RuntimeBinder); ok {
			if err := binder.AttachRuntimeSource(container); err != nil {
				return err
			}
		}
	}
	return nil
}

func (rt *Runtime) Start(ctx context.Context) error {
	if rt == nil || rt.container == nil {
		return nil
	}
	return composition.Start(ctx, rt.container)
}

func (rt *Runtime) Stop(ctx context.Context) error {
	if rt == nil || rt.container == nil {
		return nil
	}
	return composition.Stop(ctx, rt.container)
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
	const op serrors.Op = "bootstrap.NewRuntime"

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
	}

	var cleanup []func() error

	logger, loggerCleanup, err := cfg.loggerFactory(ctx, cfg.config)
	if err != nil {
		return nil, nil, serrors.E(op, err, "build logger")
	}
	rt.Logger = logger
	if loggerCleanup != nil {
		cleanup = append(cleanup, loggerCleanup)
	}

	pool, poolCleanup, err := cfg.poolFactory(ctx, cfg.config, logger)
	if err != nil {
		return nil, nil, errors.Join(serrors.E(op, err, "build pool"), runCleanup(cleanup))
	}
	rt.Pool = pool
	if poolCleanup != nil {
		cleanup = append(cleanup, poolCleanup)
	}

	bundle, err := cfg.bundleFactory(ctx, cfg.config)
	if err != nil {
		return nil, nil, errors.Join(serrors.E(op, err, "build bundle"), runCleanup(cleanup))
	}
	rt.Bundle = bundle

	app, err := cfg.appFactory(ctx, rt)
	if err != nil {
		return nil, nil, errors.Join(serrors.E(op, err, "build application"), runCleanup(cleanup))
	}
	rt.App = app

	return rt, func() error {
		var cleanupErr error
		if rt.container != nil {
			stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cleanupErr = errors.Join(cleanupErr, rt.Stop(stopCtx))
		}
		if rt.App != nil {
			if binder, ok := rt.App.(application.RuntimeBinder); ok {
				binder.DetachRuntimeSource()
			}
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
