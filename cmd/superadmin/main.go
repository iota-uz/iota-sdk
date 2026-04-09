package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	superadminMiddleware "github.com/iota-uz/iota-sdk/modules/superadmin/middleware"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/server"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			configuration.Use().Unload()
			log.Println(r)
			debug.PrintStack()
			os.Exit(1)
		}
	}()

	if err := run(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}

func run() error {
	conf := configuration.Use()
	serviceName := conf.OpenTelemetry.ServiceName
	if serviceName != "" {
		serviceName += "-superadmin"
	}

	rt, cleanup, err := bootstrap.NewRuntime(
		context.Background(),
		bootstrap.IotaConfigWithServiceName(conf, serviceName),
	)
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
			core.NewComponent(&core.ModuleOptions{}),
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
