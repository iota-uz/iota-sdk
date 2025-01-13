package controllers

import (
	"fmt"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"log"
	"net/http"
	"path/filepath"

	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/graphql"
)

type GraphQLController struct {
	app application.Application
}

func (g *GraphQLController) Key() string {
	return "/graphql/core"
}

func (g *GraphQLController) Register(r *mux.Router) {
	schema := graph.NewExecutableSchema(
		graph.Config{
			Resolvers: graph.NewResolver(g.app),
		},
	)
	srv := graphql.NewBaseServer(schema)

	for _, schema := range g.app.GraphSchemas() {
		srv.AddExecutor(executor.New(schema.Value))
	}
	router := r.Methods(http.MethodGet, http.MethodPost).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.ProvideUser(),
	)

	router.Handle("/query", srv)
	router.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
	for _, schema := range g.app.GraphSchemas() {
		router.Handle(filepath.Join(fmt.Sprintf("/query/%s", schema.BasePath)), graphql.NewHandler(executor.New(schema.Value)))
	}
	log.Printf("connect to http://localhost:%d/playground for GraphQL playground", configuration.Use().ServerPort)
}

func NewGraphQLController(app application.Application) application.Controller {
	return &GraphQLController{
		app: app,
	}
}
