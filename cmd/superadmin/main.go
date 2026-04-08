package main

import (
	"context"
	"log"
	"os"
	"runtime/debug"

	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	coreassets "github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/core/validators"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	superadminMiddleware "github.com/iota-uz/iota-sdk/modules/superadmin/middleware"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bootstrap"
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
		log.Fatalf("failed to initialize runtime: %v", err)
	}
	defer func() {
		if err := cleanup(); err != nil {
			rt.Logger.WithError(err).Warn("failed to clean up runtime")
		}
	}()

	app := rt.App

	app.RegisterLocaleFiles(&core.LocaleFiles)

	fsStorage, err := persistence.NewFSStorage()
	if err != nil {
		log.Fatalf("failed to create file storage: %v", err)
	}
	uploadRepo := persistence.NewUploadRepository()
	userRepo := persistence.NewUserRepository(uploadRepo)
	roleRepo := persistence.NewRoleRepository()
	tenantRepo := persistence.NewTenantRepository()
	permRepo := persistence.NewPermissionRepository()
	userQueryRepo := query.NewPgUserQueryRepository()
	groupQueryRepo := query.NewPgGroupQueryRepository()
	userValidator := validators.NewUserValidator(userRepo)

	tenantService := services.NewTenantService(tenantRepo)
	uploadService := services.NewUploadService(uploadRepo, fsStorage, app.EventPublisher())
	sessionService := services.NewSessionService(persistence.NewSessionRepository(), app.EventPublisher())

	app.RegisterServices(
		uploadService,
		services.NewUserService(userRepo, userValidator, app.EventPublisher(), sessionService),
		services.NewUserQueryService(userQueryRepo),
		services.NewGroupQueryService(groupQueryRepo),
		sessionService,
		services.NewExcelExportService(app.DB(), uploadService),
	)
	app.RegisterServices(
		services.NewAuthService(app),
		services.NewCurrencyService(persistence.NewCurrencyRepository(), app.EventPublisher()),
		services.NewRoleService(roleRepo, app.EventPublisher()),
		tenantService,
		services.NewPermissionService(permRepo, app.EventPublisher()),
		services.NewGroupService(persistence.NewGroupRepository(userRepo, roleRepo), app.EventPublisher()),
	)

	app.RegisterControllers(
		controllers.NewLoginController(app),
		controllers.NewLogoutController(app),
		controllers.NewAccountController(app),
		controllers.NewUploadController(app),
	)
	app.RegisterHashFsAssets(coreassets.HashFS)

	superadminModule := superadmin.NewModule(&superadmin.ModuleOptions{})
	if err := superadminModule.RegisterWiring(app); err != nil {
		log.Fatalf("failed to wire superadmin module: %v", err)
	}
	if err := superadminModule.RegisterTransports(app); err != nil {
		log.Fatalf("failed to register superadmin transports: %v", err)
	}

	app.RegisterNavItems(superadmin.NavItems...)
	app.RegisterHashFsAssets(internalassets.HashFS)
	app.RegisterControllers(controllers.NewStaticFilesController(app.HashFsAssets()))

	if err := app.StartRuntime(context.Background(), application.RuntimeTagAPI); err != nil {
		log.Fatalf("failed to start runtime: %v", err)
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
		log.Fatalf("failed to create server: %v", err)
	}

	rt.Logger.Info("Super Admin Server starting...")
	rt.Logger.Info("Listening on: " + conf.Origin)
	rt.Logger.Info("Only superadmin module loaded (core services only, no core controllers)")
	rt.Logger.Info("SuperAdmin authentication required for all routes")

	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
