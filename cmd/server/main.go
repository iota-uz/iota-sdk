package main

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/iota-agency/iota-erp/graph"
	"github.com/iota-agency/iota-erp/pkg/utils"
	_ "github.com/lib/pq"
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
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: &graph.Resolver{
		Db: db,
	}}))

	port := utils.GetEnv("PORT", "3200")

	http.Handle("/", playground.Handler("GraphQL playground", "/graphql"))
	http.Handle("/graphql", srv)

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
