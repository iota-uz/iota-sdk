package main

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/modules"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/presentation/controllers"
	"github.com/iota-agency/iota-erp/internal/server"
	"github.com/iota-agency/iota-erp/pkg/dbutils"
	"github.com/iota-agency/iota-erp/pkg/intl"
	localMiddleware "github.com/iota-agency/iota-erp/pkg/middleware"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	_ "github.com/lib/pq"
	"gorm.io/gorm/logger"
	"log"
)

func main() {
	conf := configuration.Use()
	db, err := dbutils.ConnectDB(conf.DBOpts, logger.Error)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	if err := dbutils.CheckModels(db, server.RegisteredModels); err != nil {
		log.Fatalf("failed to check models: %v", err)
	}

	application := app.New(db)
	controllerInstances := []shared.Controller{
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
	}

	for _, module := range modules.Load() {
		for _, c := range module.Controllers() {
			controllerInstances = append(controllerInstances, c(application))
		}
	}

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
		Controllers: controllerInstances,
	}
	if err := serverInstance.Start(conf.SocketAddress); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
