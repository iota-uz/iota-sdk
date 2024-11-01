package main

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/modules/elxolding"
	"github.com/iota-agency/iota-erp/internal/modules/iota"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/controllers"
	"github.com/iota-agency/iota-erp/pkg/dbutils"
	"github.com/iota-agency/iota-erp/pkg/intl"
	localMiddleware "github.com/iota-agency/iota-erp/pkg/middleware"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	"gorm.io/gorm/logger"
	"log"
	"slices"

	"github.com/iota-agency/iota-erp/internal/server"
	_ "github.com/lib/pq"
)

var AllModules = []shared.Module{
	iota.NewUserModule(),
	elxolding.NewUserModule(),
}

func loadModules() []shared.Module {
	jsonConf := configuration.UseJsonConfig()
	modules := make([]shared.Module, 0, len(AllModules))
	for _, module := range AllModules {
		if slices.Contains(jsonConf.Modules, module.Name()) {
			modules = append(modules, module)
		}
	}
	return modules
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
	modules := loadModules()
	application := app.New(db)
	bundle := intl.LoadBundle()
	serverInstance := &server.HttpServer{
		Middlewares: []mux.MiddlewareFunc{
			middleware.Cors([]string{"http://localhost:3000", "ws://localhost:3000"}),
			middleware.RequestParams(middleware.DefaultParamsConstructor),
			middleware.WithLogger(log.Default()),
			middleware.LogRequests(),
			middleware.Transactions(db),
			localMiddleware.WithLocalizer(bundle),
			localMiddleware.Authorization(application.AuthService),
		},
		Modules: modules,
		Controllers: []shared.Controller{
			controllers.NewAccountController(application),
			controllers.NewEmployeeController(application),
			controllers.NewMoneyAccountController(application),
			controllers.NewHomeController(application),
			controllers.NewLoginController(application),
			controllers.NewExpenseCategoriesController(application),
			controllers.NewProjectsController(application),
			controllers.NewPaymentsController(application),
			controllers.NewExpensesController(application),
			controllers.NewGraphQLController(application),
			controllers.NewLogoutController(application),
			controllers.NewStaticFilesController(),
		},
	}
	if err := serverInstance.Start(application, conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
