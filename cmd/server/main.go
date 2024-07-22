package main

import (
	"encoding/json"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/graph"
	"github.com/iota-agency/iota-erp/internal/app"
	"github.com/iota-agency/iota-erp/internal/configuration"
	localMiddleware "github.com/iota-agency/iota-erp/pkg/middleware"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	_ "github.com/lib/pq"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/rs/cors"
	"golang.org/x/text/language"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"net/http"
	"os"
	"time"
)

func newLogger() logger.Interface {
	return logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)
}

func main() {
	conf := configuration.Use()
	if err := conf.Load(); err != nil {
		log.Fatal(err)
	}
	log.Println("Connecting to database:", conf.DbOpts)
	db, err := gorm.Open(postgres.Open(conf.DbOpts), &gorm.Config{
		Logger:                 newLogger(),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(err)
	}

	bundle := i18n.NewBundle(language.Russian)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	bundle.MustLoadMessageFile("pkg/locales/en.json")
	bundle.MustLoadMessageFile("pkg/locales/ru.json")

	application := app.New(db)

	r := mux.NewRouter()
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "ws://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
	}).Handler)
	r.Use(middleware.RequestParams(middleware.DefaultParamsConstructor))
	r.Use(middleware.WithLogger(log.Default()))
	r.Use(middleware.LogRequests)
	r.Use(middleware.Transactions(db))
	r.Use(localMiddleware.WithLocalizer(bundle))
	r.Use(localMiddleware.Authorization(application.AuthService))
	r.Handle("/query", graph.NewDefaultServer(db, application))
	r.Handle("/", playground.Handler("GraphQL playground", "/query"))
	r.HandleFunc("/oauth/google/callback", application.AuthService.OauthGoogleCallback)

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", conf.ServerPort)
	log.Fatal(http.ListenAndServe(conf.SocketAddress, r))
}
