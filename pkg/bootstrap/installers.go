package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/benbjohnson/hashfs"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	compositionapplet "github.com/iota-uz/iota-sdk/pkg/composition/applet"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
)

func InstallComponents(capabilities []composition.Capability, components ...composition.Component) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		engine := composition.NewEngine()
		if err := engine.Register(components...); err != nil {
			return err
		}

		container, err := engine.Compile(composition.BuildContext{App: rt.App}, capabilities...)
		if err != nil {
			return err
		}

		if err := composition.Apply(rt.App, container, composition.ApplyOptions{IncludeControllers: true}); err != nil {
			return err
		}

		return rt.SetComposition(engine, container)
	})
}

func InstallNavItems(items ...types.NavigationItem) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		rt.App.RegisterNavItems(items...)
		return nil
	})
}

func InstallHashFS(fs ...*hashfs.FS) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		rt.App.RegisterHashFsAssets(fs...)
		return nil
	})
}

func InstallControllers(controllersToRegister ...application.Controller) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		rt.App.RegisterControllers(controllersToRegister...)
		return nil
	})
}

func InstallCoreControllers() Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		if len(rt.App.HashFsAssets()) == 0 {
			return fmt.Errorf("hashfs assets must be registered before core controllers")
		}
		rt.App.RegisterControllers(
			controllers.NewStaticFilesController(rt.App.HashFsAssets()),
			controllers.NewGraphQLController(rt.App),
		)
		return nil
	})
}

type AppletsOptions struct {
	HostServices  applets.HostServices
	SessionConfig applets.SessionConfig
	Logger        *logrus.Logger
	Metrics       applets.MetricsRecorder
	BuilderOpts   []applets.BuilderOption
	WithRuntime   bool
	WithHTTP      bool
}

func InstallApplets(opts AppletsOptions) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		var appletControllers []application.Controller

		host := opts.HostServices
		if host == nil {
			host = NewSDKHostServices(rt.Pool)
		}

		logger := opts.Logger
		if logger == nil {
			logger = rt.Logger
		}

		metrics := opts.Metrics
		if metrics == nil {
			metrics = applet.NewNoopMetricsRecorder()
		}

		if opts.WithHTTP {
			var err error
			appletControllers, err = rt.App.CreateAppletControllers(
				host,
				opts.SessionConfig,
				logger,
				metrics,
				opts.BuilderOpts...,
			)
			if err != nil {
				return fmt.Errorf("create applet controllers: %w", err)
			}
		}

		if opts.WithRuntime {
			if rt.Container() == nil {
				return fmt.Errorf("install components before installing applet runtime")
			}
			builder := compositionapplet.NewAppletEngineBuilder()
			result, err := builder.Build(compositionapplet.BuildInput{
				Applets:       rt.App.AppletRegistry().All(),
				Pool:          rt.Pool,
				Bundle:        rt.App.Bundle(),
				Host:          host,
				SessionConfig: opts.SessionConfig,
				Logger:        logger,
				Metrics:       metrics,
				Options:       opts.BuilderOpts,
			})
			if err != nil {
				return fmt.Errorf("register applet runtime: %w", err)
			}
			for _, registration := range result.RuntimeRegistrations {
				component := application.NewAppletRuntimeComponent(
					registration.Manager,
					rt.Pool,
					logger,
					registration.HasPostgresJobs,
				)
				rt.Container().AppendHooks(composition.Hook{
					Name: component.Name(),
					Start: func(ctx context.Context, _ *composition.Container) error {
						return component.Start(ctx)
					},
					Stop: func(ctx context.Context, _ *composition.Container) error {
						return component.Stop(ctx)
					},
				})
			}
		}

		if len(appletControllers) > 0 {
			rt.App.RegisterControllers(appletControllers...)
		}

		return nil
	})
}

func StartComposition() Installer {
	return InstallerFunc(func(ctx context.Context, rt *Runtime) error {
		startCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		return rt.Start(startCtx)
	})
}
