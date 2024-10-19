package server

import (
	"encoding/json"
	"github.com/NYTimes/gziphandler"
	"github.com/iota-agency/iota-erp/pkg/dbutils"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/presentation/controllers"
	localMiddleware "github.com/iota-agency/iota-erp/pkg/middleware"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
	"gorm.io/gorm/logger"
)

type Server struct {
	conf        *configuration.Configuration
	controllers []controllers.Controller
	middlewares []mux.MiddlewareFunc
}

func (s *Server) init() error {
	return nil
}

func loadBundle() *i18n.Bundle {
	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustLoadMessageFile("pkg/locales/en.json")
	bundle.MustLoadMessageFile("pkg/locales/ru.json")
	return bundle
}

func (s *Server) Start() error {
	r := mux.NewRouter()
	r.Use(s.middlewares...)
	for _, controller := range s.controllers {
		controller.Register(r)
	}
	var notFoundHandler http.Handler = controllers.NotFound()
	var notAllowedHandler http.Handler = controllers.MethodNotAllowed()
	for i := len(s.middlewares) - 1; i >= 0; i-- {
		notFoundHandler = s.middlewares[i](notFoundHandler)
		notAllowedHandler = s.middlewares[i](notAllowedHandler)
	}
	r.NotFoundHandler = notFoundHandler
	r.MethodNotAllowedHandler = notAllowedHandler
	return http.ListenAndServe(s.conf.SocketAddress, gziphandler.GzipHandler(r))
}

func DefaultServer() (*Server, error) {
	conf := configuration.Use()
	db, err := dbutils.ConnectDB(conf.DBOpts, logger.Info)
	if err != nil {
		return nil, err
	}
	if err := dbutils.CheckModels(db, RegisteredModels); err != nil {
		return nil, err
	}
	application := app.New(db)
	bundle := loadBundle()
	return &Server{
		conf: configuration.Use(),
		middlewares: []mux.MiddlewareFunc{
			middleware.Cors([]string{"http://localhost:3000", "ws://localhost:3000"}),
			middleware.RequestParams(middleware.DefaultParamsConstructor),
			middleware.WithLogger(log.Default()),
			middleware.LogRequests(),
			middleware.Transactions(db),
			localMiddleware.WithLocalizer(bundle),
			localMiddleware.Authorization(application.AuthService),
		},
		controllers: []controllers.Controller{
			controllers.NewAccountController(application),
			controllers.NewHomeController(application),
			controllers.NewLoginController(application),
			controllers.NewUsersController(application),
			controllers.NewExpenseCategoriesController(application),
			controllers.NewProjectsController(application),
			controllers.NewPaymentsController(application),
			controllers.NewExpensesController(application),
			controllers.NewGraphQLController(application),
			controllers.NewLogoutController(application),
			controllers.NewStaticFilesController(),
		},
	}, nil
}
