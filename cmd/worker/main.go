package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/iota-uz/applets"
	"github.com/iota-uz/iota-sdk/modules"
	bichatbootstrap "github.com/iota-uz/iota-sdk/modules/bichat/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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

	conf := configuration.Use()
	serviceName := conf.OpenTelemetry.ServiceName
	if serviceName != "" {
		serviceName += "-worker"
	}

	rt, cleanup, err := bootstrap.NewRuntime(
		context.Background(),
		bootstrap.IotaConfigWithServiceName(conf, serviceName),
	)
	if err != nil {
		log.Fatalf("failed to initialize worker runtime: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			rt.Logger.WithError(err).Warn("failed to clean up worker runtime")
		}
	}()

	if err := rt.Install(
		context.Background(),
		bootstrap.InstallModules(modules.BuiltInModules...),
		bichatbootstrap.New(),
		bootstrap.InstallApplets(bootstrap.AppletsOptions{
			SessionConfig: applets.DefaultSessionConfig,
			WithRuntime:   true,
		}),
		bootstrap.StartRuntime(application.RuntimeTagWorker),
	); err != nil {
		log.Fatalf("failed to compose worker runtime: %v", err)
	}

	rt.Logger.Info("worker runtime started")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	sig := <-sigCh
	rt.Logger.Infof("received signal %v, shutting down worker", sig)
}
