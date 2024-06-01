package main

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/iota-agency/iota-erp/graph"
	"github.com/iota-agency/iota-erp/pkg/authentication"
	"github.com/iota-agency/iota-erp/pkg/middleware"
	"github.com/iota-agency/iota-erp/pkg/utils"
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
	authService := &authentication.Service{
		Db: db,
	}
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(
		graph.Config{
			Resolvers: &graph.Resolver{
				Db:          db,
				AuthService: authService,
			},
		},
	))

	port := utils.GetEnv("PORT", "3200")
	http.Handle("/", playground.Handler("GraphQL playground", "/graphql"))
	graphHandler := middleware.AuthMiddleware(db, authService)(middleware.LoggingMiddleware(srv))
	http.Handle("/graphql", cors.Default().Handler(graphHandler))

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", port)
	if utils.GetEnv("GO_APP_ENV", "development") == "production" {
		err = http.ListenAndServe(":3200", nil)
	} else {
		err = http.ListenAndServe("localhost:3200", nil)
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
