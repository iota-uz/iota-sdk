package main

import (
	"context"
	"log"
	"os"
	"runtime/debug"

	"github.com/iota-uz/applets"
	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules"
	bichatbootstrap "github.com/iota-uz/iota-sdk/modules/bichat/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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

	conf := configuration.Use()
	rt, cleanup, err := bootstrap.NewRuntime(context.Background(), bootstrap.IotaConfig(conf))
	if err != nil {
		log.Fatalf("failed to initialize runtime: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			rt.Logger.WithError(err).Warn("failed to clean up runtime")
		}
	}()

	if err := rt.Install(
		context.Background(),
		bootstrap.InstallModules(modules.BuiltInModules...),
		bootstrap.InstallNavItems(modules.NavLinks...),
		bootstrap.InstallHashFS(internalassets.HashFS),
		bichatbootstrap.New(bichatbootstrap.WithTransports()),
		bootstrap.InstallApplets(bootstrap.AppletsOptions{
			SessionConfig: applets.DefaultSessionConfig,
			WithHTTP:      true,
			WithRuntime:   true,
		}),
		bootstrap.InstallModuleTransports(modules.BuiltInModules...),
		bootstrap.InstallCoreControllers(),
		bootstrap.StartRuntime(application.RuntimeTagAPI, application.RuntimeTagWorker),
	); err != nil {
		log.Fatalf("failed to compose server runtime: %v", err)
	}

	serverInstance, err := server.New(rt)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	log.Printf("Listening on: %s\n", conf.Origin)
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
