package server

import (
	"encoding/json"
	"gorm.io/gorm/logger"
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
	return http.ListenAndServe(s.conf.SocketAddress, r)
}

func DefaultServer() (*Server, error) {
	conf := configuration.Use()
	log.Println("Connecting to database:", conf.DbOpts)
	db, err := ConnectDB(conf.DbOpts, logger.Error)
	if err != nil {
		return nil, err
	}
	CheckModels(db)
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
			controllers.NewHomeController(application),
			controllers.NewLoginController(application),
			controllers.NewUsersController(application),
			controllers.NewExpenseCategoriesController(application),
			controllers.NewPaymentsController(application),
			controllers.NewGraphQLController(application),
			controllers.NewStaticFilesController(),
		},
	}, nil
}
