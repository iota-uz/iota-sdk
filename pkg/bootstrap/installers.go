package bootstrap

import (
	"context"
	"fmt"

	"github.com/benbjohnson/hashfs"
	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/sirupsen/logrus"
)

func InstallModules(modules ...application.Module) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		return application.Wire(rt.App, modules...)
	})
}

func InstallModuleTransports(modules ...application.Module) Installer {
	return InstallerFunc(func(_ context.Context, rt *Runtime) error {
		return application.RegisterTransports(rt.App, modules...)
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
			appletControllers, err := rt.App.CreateAppletControllers(
				host,
				opts.SessionConfig,
				logger,
				metrics,
				opts.BuilderOpts...,
			)
			if err != nil {
				return fmt.Errorf("create applet controllers: %w", err)
			}
			rt.App.RegisterControllers(appletControllers...)
		}

		if opts.WithRuntime {
			if err := rt.App.RegisterAppletRuntime(
				host,
				opts.SessionConfig,
				logger,
				metrics,
				opts.BuilderOpts...,
			); err != nil {
				return fmt.Errorf("register applet runtime: %w", err)
			}
		}

		return nil
	})
}

func StartRuntime(tags ...application.RuntimeTag) Installer {
	return InstallerFunc(func(ctx context.Context, rt *Runtime) error {
		return rt.App.StartRuntime(ctx, tags...)
	})
}
