package main

import (
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/graph"
	"github.com/iota-agency/iota-erp/internal/app"
	"github.com/iota-agency/iota-erp/internal/domain/auth"
	"github.com/iota-agency/iota-erp/internal/domain/user"
	infrastructure "github.com/iota-agency/iota-erp/internal/infrastracture"
	"github.com/iota-agency/iota-erp/internal/infrastracture/event"
	"github.com/iota-agency/iota-erp/internal/infrastracture/persistence"
	"github.com/iota-agency/iota-erp/models"
	"github.com/iota-agency/iota-erp/sdk/middleware"
	"github.com/iota-agency/iota-erp/sdk/utils"
	_ "github.com/lib/pq"
	"github.com/rs/cors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"log"
	"net/http"
)

func main() {
	utils.LoadEnv()
	log.Println("Connecting to database:", utils.DbOpts())
	db, err := gorm.Open(postgres.Open(utils.DbOpts()), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	authService := auth.NewService()
	eventPublisher := event.NewEventPublisher()
	registry := infrastructure.NewRepositoryRegistry()
	registry.RegisterUserRepository(persistence.NewUserRepository())
	registry.RegisterUploadRepository(persistence.NewUploadRepository())
	//registry.RegisterSessionRepository(persistence.NewSessionRepository())
	application := app.New(registry, eventPublisher)

	srv := graph.NewDefaultServer(db, application)
	srv.AddTransport(&transport.Websocket{})

	r := mux.NewRouter()
	r.Use(middleware.RequestParams(middleware.DefaultParamsConstructor))
	r.Use(middleware.WithLogger(log.Default()))
	r.Use(middleware.LogRequests)
	r.Use(middleware.Transactions(db))
	r.Use(middleware.Authorization[user.User, models.Session](authService))
	r.Use(cors.Default().Handler)
	r.Handle("/query", srv)
	r.Handle("/", playground.Handler("GraphQL playground", "/query"))

	port := utils.GetEnv("PORT", "3200")
	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	if utils.GetEnv("GO_APP_ENV", "development") == "production" {
		err = http.ListenAndServe(":3200", r)
	} else {
		err = http.ListenAndServe("localhost:3200", r)
	}
	log.Fatal(err)

	//telegramServer := tgServer.New(db)
	//wg := sync.WaitGroup{}
	//wg.Add(2)
	//go func() {
	//	httpServer.Start()
	//	wg.Done()
	//}()
	//go func() {
	//	telegramServer.Start()
	//	wg.Done()
	//}()
	//wg.Wait()
}
