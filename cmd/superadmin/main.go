package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	internalassets "github.com/iota-uz/iota-sdk/internal/assets"
	"github.com/iota-uz/iota-sdk/internal/server"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/core/validators"
	"github.com/iota-uz/iota-sdk/modules/superadmin"
	superadminMiddleware "github.com/iota-uz/iota-sdk/modules/superadmin/middleware"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/logging"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
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
	logger := conf.Logger()

	// Set up OpenTelemetry if enabled
	var tracingCleanup func()
	if conf.OpenTelemetry.Enabled {
		tracingCleanup = logging.SetupTracing(
			context.Background(),
			conf.OpenTelemetry.ServiceName+"-superadmin",
			conf.OpenTelemetry.TempoURL,
		)
		defer tracingCleanup()
		logger.Info("OpenTelemetry tracing enabled for Super Admin, exporting to Tempo at " + conf.OpenTelemetry.TempoURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	pool, err := pgxpool.New(ctx, conf.Database.Opts)
	if err != nil {
		panic(err)
	}
	bundle := application.LoadBundle()
	app, err := application.New(&application.ApplicationOptions{
		Pool:     pool,
		Bundle:   bundle,
		EventBus: eventbus.NewEventPublisher(logger),
		Logger:   logger,
		Huber: application.NewHub(&application.HuberOptions{
			Pool:           pool,
			Logger:         logger,
			Bundle:         bundle,
			UserRepository: persistence.NewUserRepository(persistence.NewUploadRepository()),
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		}),
	})
	if err != nil {
		log.Fatalf("failed to initialize application: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		if stopErr := app.Spotlight().Stop(stopCtx); stopErr != nil {
			logger.WithError(stopErr).Warn("failed to stop spotlight service")
		}
	}()

	// Manually register only necessary parts from core module (without its controllers)
	// This avoids exposing core module's admin pages (/users, /roles, etc.) in superadmin

	// Register core locales
	app.RegisterLocaleFiles(&core.LocaleFiles)

	// Register core repositories and services (needed for authentication)
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

	// Register first batch of services (without AuthService)
	app.RegisterServices(
		uploadService,
		services.NewUserService(userRepo, userValidator, app.EventPublisher(), sessionService),
		services.NewUserQueryService(userQueryRepo),
		services.NewGroupQueryService(groupQueryRepo),
		sessionService,
		services.NewExcelExportService(app.DB(), uploadService),
	)
	// Register second batch (including AuthService which depends on UserService)
	app.RegisterServices(
		services.NewAuthService(app),
		services.NewCurrencyService(persistence.NewCurrencyRepository(), app.EventPublisher()),
		services.NewRoleService(roleRepo, app.EventPublisher()),
		tenantService,
		services.NewPermissionService(permRepo, app.EventPublisher()),
		services.NewGroupService(persistence.NewGroupRepository(userRepo, roleRepo), app.EventPublisher()),
	)

	// Register only auth-related controllers from core (Login, Logout, Account)
	app.RegisterControllers(
		controllers.NewLoginController(app),
		controllers.NewLogoutController(app),
		controllers.NewAccountController(app),
		controllers.NewUploadController(app),
	)

	// Register core assets
	app.RegisterHashFsAssets(assets.HashFS)

	// Load superadmin module
	superadminModule := superadmin.NewModule(&superadmin.ModuleOptions{})
	if err := superadminModule.Register(app); err != nil {
		log.Fatalf("failed to load superadmin module: %v", err)
	}

	// Register navigation items only from superadmin
	app.RegisterNavItems(superadmin.NavItems...)

	// Register internal assets and static files controller
	app.RegisterHashFsAssets(internalassets.HashFS)
	app.RegisterControllers(
		controllers.NewStaticFilesController(app.HashFsAssets()),
	)

	options := &server.DefaultOptions{
		Logger:        logger,
		Configuration: conf,
		Application:   app,
		Pool:          pool,
	}

	// Create server first - this sets up core middleware including RequestParams
	serverInstance, err := server.Default(options)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	// Apply authentication middleware chain globally AFTER core middleware is set up
	// Execution order will be:
	// 1. Core middleware (Logger, RequestParams, etc.) - from server.Default()
	// 2. Authorize() - reads cookie/token, populates session
	// 3. ProvideUser() - reads session, populates user in context
	// 4. RedirectNotAuthenticated() - redirects to /login if not authenticated
	// 5. RequireSuperAdmin() - checks superadmin status
	app.RegisterMiddleware(
		middleware.Authorize(),
		middleware.ProvideUser(),
		middleware.RedirectNotAuthenticated(),
		superadminMiddleware.RequireSuperAdmin(),
	)

	logger.Info("Super Admin Server starting...")
	logger.Info("Listening on: " + conf.Origin)
	logger.Info("Only superadmin module loaded (core services only, no core controllers)")
	logger.Info("SuperAdmin authentication required for all routes")

	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
