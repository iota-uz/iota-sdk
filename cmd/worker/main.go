package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/bichat"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
)

func main() {
	bootstrap.Main(run)
}

func run() error {
	src, err := config.Build(envprov.New(".env", ".env.local"))
	if err != nil {
		return fmt.Errorf("failed to build config source: %w", err)
	}

	rt, cleanup, err := bootstrap.NewRuntime(
		context.Background(),
		bootstrap.IotaSourceWithServiceName(src, resolveWorkerServiceName(src)),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize worker runtime: %w", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			rt.Logger.WithError(err).Warn("failed to clean up worker runtime")
		}
	}()

	if err := rt.Install(
		context.Background(),
		bootstrap.InstallComponents(
			[]composition.Capability{composition.CapabilityWorker},
			append(modules.Components(), bichat.NewComponent())...,
		),
		bootstrap.InstallApplets(bootstrap.AppletsOptions{
			SessionConfig: applets.DefaultSessionConfig,
			WithRuntime:   true,
		}),
		bootstrap.StartComposition(),
	); err != nil {
		return fmt.Errorf("failed to compose worker runtime: %w", err)
	}

	rt.Logger.Info("worker runtime started")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	sig := <-sigCh
	rt.Logger.Infof("received signal %v, shutting down worker", sig)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := rt.Stop(shutdownCtx); err != nil {
		rt.Logger.WithError(err).Warn("failed to stop worker runtime gracefully")
	}
	return nil
}

// resolveWorkerServiceName reads the telemetry service name from the source and
// appends "-worker". Falls back to empty string when not configured.
func resolveWorkerServiceName(src config.Source) string {
	type telOnly struct {
		OTEL struct {
			ServiceName string `koanf:"servicename"`
		} `koanf:"otel"`
	}
	var t telOnly
	if err := src.Unmarshal("telemetry", &t); err != nil || t.OTEL.ServiceName == "" {
		return ""
	}
	return t.OTEL.ServiceName + "-worker"
}
