package main

import (
	"github.com/benbjohnson/hashfs"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/infrastructure/persistence"
	"github.com/iota-agency/iota-erp/internal/modules"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/assets"
	"github.com/iota-agency/iota-erp/internal/presentation/controllers"
	"github.com/iota-agency/iota-erp/internal/server"
	"github.com/iota-agency/iota-erp/internal/services"
	"github.com/iota-agency/iota-erp/pkg/dbutils"
	localMiddleware "github.com/iota-agency/iota-erp/pkg/middleware"
	"github.com/iota-agency/iota-erp/sdk/event"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	_ "github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
)

func constructApp(db *gorm.DB) *application.Application {
	eventPublisher := event.NewEventPublisher()
	app := application.New(db, eventPublisher)
	moneyAccountService := services.NewMoneyAccountService(
		persistence.NewMoneyAccountRepository(),
		eventPublisher,
	)

	app.RegisterService(services.NewUserService(persistence.NewUserRepository(), eventPublisher))
	app.RegisterService(services.NewSessionService(persistence.NewSessionRepository(), eventPublisher))
	app.RegisterService(services.NewAuthService(app))
	app.RegisterService(services.NewRoleService(persistence.NewRoleRepository(), eventPublisher))
	app.RegisterService(services.NewPaymentService(
		persistence.NewPaymentRepository(), eventPublisher, moneyAccountService,
	))
	app.RegisterService(services.NewProjectStageService(persistence.NewProjectStageRepository(), eventPublisher))
	app.RegisterService(services.NewCurrencyService(persistence.NewCurrencyRepository(), eventPublisher))
	app.RegisterService(services.NewExpenseCategoryService(
		persistence.NewExpenseCategoryRepository(),
		eventPublisher,
	))
	app.RegisterService(services.NewPositionService(persistence.NewPositionRepository(), eventPublisher))
	app.RegisterService(services.NewEmployeeService(persistence.NewEmployeeRepository(), eventPublisher))
	app.RegisterService(services.NewAuthLogService(persistence.NewAuthLogRepository(), eventPublisher))
	app.RegisterService(services.NewPromptService(persistence.NewPromptRepository(), eventPublisher))
	app.RegisterService(services.NewExpenseService(
		persistence.NewExpenseRepository(), eventPublisher, moneyAccountService,
	))
	app.RegisterService(services.NewProjectService(persistence.NewProjectRepository(), eventPublisher))

	app.RegisterService(services.NewEmbeddingService(app))
	app.RegisterService(services.NewDialogueService(persistence.NewDialogueRepository(), app))
	app.RegisterService(moneyAccountService)
	return app
}

func main() {
	conf := configuration.Use()
	db, err := dbutils.ConnectDB(conf.DBOpts, logger.Error)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := dbutils.CheckModels(db, server.RegisteredModels); err != nil {
		log.Fatalf("failed to check models: %v", err)
	}

	assetsFs := []*hashfs.FS{
		assets.FS,
	}
	for _, module := range modules.LoadedModules {
		assetsFs = append(assetsFs, module.Assets())
	}
	app := constructApp(db)
	controllerInstances := []shared.Controller{
		controllers.NewAccountController(app),
		controllers.NewEmployeeController(app),
		controllers.NewHomeController(app),
		controllers.NewGraphQLController(app),
		controllers.NewLogoutController(app),
		controllers.NewStaticFilesController(assetsFs),
	}

	for _, module := range modules.LoadedModules {
		for _, c := range module.Controllers() {
			controllerInstances = append(controllerInstances, c(app))
		}
	}

	bundle := modules.LoadBundle()
	authService := app.Service(services.AuthService{}).(*services.AuthService)
	serverInstance := &server.HttpServer{
		Middlewares: []mux.MiddlewareFunc{
			middleware.Cors([]string{"http://localhost:3000", "ws://localhost:3000"}),
			middleware.RequestParams(middleware.DefaultParamsConstructor),
			middleware.WithLogger(log.Default()),
			middleware.LogRequests(),
			middleware.Transactions(db),
			localMiddleware.WithLocalizer(bundle),
			localMiddleware.Authorization(authService),
		},
		Controllers: controllerInstances,
	}
	log.Printf("starting server on %s", conf.SocketAddress)
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
