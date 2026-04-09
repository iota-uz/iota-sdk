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
	"github.com/sirupsen/logrus"
)

func InstallComponents(capabilities []composition.Capability, components ...composition.Component) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		engine := composition.NewEngine()
		if err := engine.Register(components...); err != nil {
			return err
		}

		container, err := engine.Compile(rt.BuildContext(), capabilities...)
		if err != nil {
			return err
		}
		return rt.SetComposition(engine, container)
	})
}

func InstallHashFS(fs ...*hashfs.FS) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		if rt.Container() == nil {
			return fmt.Errorf("install components before registering hashfs assets")
		}
		rt.Container().AppendHashFSAssets(fs...)
		return nil
	})
}

func InstallControllers(controllersToRegister ...application.Controller) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		if rt.Container() == nil {
			return fmt.Errorf("install components before registering controllers")
		}
		rt.Container().AppendControllers(controllersToRegister...)
		return nil
	})
}

func InstallStaticFilesController() Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		container := rt.Container()
		if container == nil {
			return fmt.Errorf("install components before core controllers")
		}
		if len(container.HashFSAssets()) == 0 {
			return fmt.Errorf("hashfs assets must be registered before core controllers")
		}
		container.AppendControllers(controllers.NewStaticFilesController(container.HashFSAssets()))
		return nil
	})
}

func InstallCoreControllers() Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		container := rt.Container()
		if container == nil {
			return fmt.Errorf("install components before core controllers")
		}
		if len(container.HashFSAssets()) == 0 {
			return fmt.Errorf("hashfs assets must be registered before core controllers")
		}
		container.AppendControllers(
			controllers.NewStaticFilesController(container.HashFSAssets()),
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
		var result compositionapplet.BuildResult

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

		if opts.WithHTTP || opts.WithRuntime {
			if rt.Container() == nil {
				return fmt.Errorf("install components before installing applets")
			}
			builder := compositionapplet.NewAppletEngineBuilder()
			built, err := builder.Build(compositionapplet.BuildInput{
				Applets:       rt.Container().Applets(),
				Pool:          rt.Pool,
				Bundle:        rt.Bundle,
				Host:          host,
				SessionConfig: opts.SessionConfig,
				Logger:        logger,
				Metrics:       metrics,
				Options:       opts.BuilderOpts,
			})
			if err != nil {
				return fmt.Errorf("build applets: %w", err)
			}
			result = built
		}

		if opts.WithRuntime {
			for _, registration := range result.RuntimeRegistrations {
				runtimeHook := &appletRuntimeHook{
					manager:         registration.Manager,
					pool:            rt.Pool,
					logger:          logger,
					hasPostgresJobs: registration.HasPostgresJobs,
				}
				rt.Container().AppendHooks(composition.Hook{
					Name: runtimeHook.Name(),
					Start: func(ctx context.Context, _ *composition.Container) error {
						return runtimeHook.Start(ctx)
					},
					Stop: func(ctx context.Context, _ *composition.Container) error {
						return runtimeHook.Stop(ctx)
					},
				})
			}
		}

		if opts.WithHTTP {
			appletControllers := make([]application.Controller, 0, len(result.Controllers))
			for _, controller := range result.Controllers {
				appletControllers = append(appletControllers, controller.(application.Controller))
			}
			rt.Container().AppendControllers(appletControllers...)
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
