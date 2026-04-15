package main

import (
	"context"
	"fmt"

	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	superadminMiddleware "github.com/iota-uz/iota-sdk/modules/superadmin/middleware"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/server"
)

func main() {
	bootstrap.Main(run)
}

func run() error {
	conf := configuration.Use()
	rt, cleanup, err := bootstrap.NewIotaRuntime(context.Background(), conf, "superadmin")
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			rt.Logger.WithError(err).Warn("failed to clean up runtime")
		}
	}()

	if err := rt.Install(
		context.Background(),
		bootstrap.InstallComponents(
			[]composition.Capability{composition.CapabilityAPI},
			core.NewComponent(&core.ModuleOptions{
				PermissionSchema:     defaults.PermissionSchema(),
				SkipAdminControllers: true,
			}),
			superadmin.NewComponent(&superadmin.ModuleOptions{}),
		),
		bootstrap.InstallHashFS(internalassets.HashFS),
		bootstrap.InstallStaticFilesController(),
		bootstrap.StartComposition(),
	); err != nil {
		return fmt.Errorf("failed to compose superadmin runtime: %w", err)
	}

	serverInstance, err := server.New(
		rt,
		server.WithAfterMiddleware(
			middleware.Authorize(),
			middleware.ProvideUser(),
			middleware.RedirectNotAuthenticated(),
			superadminMiddleware.RequireSuperAdmin(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	rt.Logger.Info("Super Admin Server starting...")
	rt.Logger.Info("Listening on: " + conf.SocketAddress)
	rt.Logger.Info("Core auth/upload controllers and superadmin controllers loaded")
	rt.Logger.Info("SuperAdmin authentication required for all routes")

	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}
