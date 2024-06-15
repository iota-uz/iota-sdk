package main

import (
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/graph"
	"github.com/iota-agency/iota-erp/internal/app"
	"github.com/iota-agency/iota-erp/internal/configuration"
	localMiddleware "github.com/iota-agency/iota-erp/pkg/middleware"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	conf := configuration.Use()
	if err := conf.Load(); err != nil {
		log.Fatal(err)
	}
	log.Println("Connecting to database:", conf.DbOpts())
	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)
	db, err := gorm.Open(postgres.Open(conf.DbOpts()), &gorm.Config{
		Logger:                 newLogger,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(err)
	}
	application := app.New()

	srv := graph.NewDefaultServer(db, application)
	srv.AddTransport(&transport.Websocket{})

	r := mux.NewRouter()
	r.Use(middleware.RequestParams(middleware.DefaultParamsConstructor))
	r.Use(middleware.WithLogger(log.Default()))
	r.Use(middleware.LogRequests)
	r.Use(middleware.Transactions(db))
	r.Use(localMiddleware.Authorization(application.AuthService))
	r.Use(cors.Default().Handler)
	r.Handle("/query", srv)
	r.Handle("/", playground.Handler("GraphQL playground", "/query"))

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", conf.ServerPort())
	log.Fatal(http.ListenAndServe(conf.SocketAddress(), r))
}
