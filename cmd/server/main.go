package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/iota-uz/applets"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/modules/testkit"
	"github.com/iota-uz/iota-sdk/modules/testkit/services"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/iota-uz/iota-sdk/pkg/testenv"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	bootstrap.Main(run)
}

func run() error {
	src, err := config.Build(envprov.New(".env", ".env.local"))
	if err != nil {
		return fmt.Errorf("failed to build config source: %w", err)
	}

	rt, cleanup, err := bootstrap.NewRuntime(context.Background(), bootstrap.IotaSource(src))
	if err != nil {
		return fmt.Errorf("failed to initialize runtime: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			rt.Logger.WithError(err).Warn("failed to clean up runtime")
		}
	}()
	components := append(modules.Components(), bichat.NewComponent())
	if scopeID := os.Getenv("TESTENV_ENVIRONMENT_ID"); scopeID != "" {
		components = append(components, testkit.NewScenarioComponent(func(pool *pgxpool.Pool) (*testenv.Registry, string, error) {
			registry := testenv.NewEnvironmentRegistry([]string{"postgres"}, true, scopeID)
			service := services.NewTestDataService(pool)
			if err := service.RegisterScenarios(registry, func(ctx context.Context, _ string) error { return service.ResetDatabase(ctx, true) }); err != nil {
				return nil, "", err
			}
			if err := registry.AllowScope(scopeID); err != nil {
				return nil, "", err
			}
			return registry, os.Getenv("TESTENV_CONTROL_TOKEN"), nil
		}))
	}

	if err := rt.Install(
		context.Background(),
		bootstrap.InstallComponents(
			[]composition.Capability{composition.CapabilityAPI, composition.CapabilityWorker},
			components...,
		),
		bootstrap.InstallHashFS(internalassets.HashFS),
		bootstrap.InstallApplets(bootstrap.AppletsOptions{
			SessionConfig: applets.DefaultSessionConfig,
			WithHTTP:      true,
			WithRuntime:   true,
		}),
		bootstrap.InstallCoreControllers(),
		bootstrap.StartComposition(),
	); err != nil {
		return fmt.Errorf("failed to compose server runtime: %w", err)
	}

	serverInstance, err := server.New(rt)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	httpCfg, err := composition.Resolve[*httpconfig.Config](rt.Container())
	if err != nil {
		return fmt.Errorf("failed to resolve httpconfig: %w", err)
	}
	appCfg, err := composition.Resolve[*appconfig.Config](rt.Container())
	if err != nil {
		return fmt.Errorf("failed to resolve appconfig: %w", err)
	}

	socketAddr := appCfg.SocketAddress(httpCfg.Port)
	log.Printf("Listening on: %s\n", httpCfg.Origin(appCfg))
	if err := serverInstance.Start(socketAddr); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}
