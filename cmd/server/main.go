package main

import (
	"context"
	"fmt"
	"log"

	"github.com/iota-uz/applets"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/server"
)

func main() {
	bootstrap.Main(run)
}

func run() error {
	conf := configuration.Use()
	rt, cleanup, err := bootstrap.NewIotaRuntime(context.Background(), conf, "")
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
			[]composition.Capability{composition.CapabilityAPI, composition.CapabilityWorker},
			append(modules.Components(), bichat.NewComponent())...,
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

	log.Printf("Listening on: %s\n", conf.Origin)
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	return nil
}
