package controllers

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	"github.com/iota-uz/iota-sdk/pkg/middleware"

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
		exec := executor.New(schema.Value)
		if schema.ExecutorCb != nil {
			schema.ExecutorCb(exec)
		}
		srv.AddExecutor(exec)
	}
	router := r.Methods(http.MethodGet, http.MethodPost).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.ProvideUser(),
		middleware.ProvideLocalizer(g.app.Bundle()),
	)

	router.Handle("/query", srv)
	router.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
	for _, schema := range g.app.GraphSchemas() {
		exec := executor.New(schema.Value)
		if schema.ExecutorCb != nil {
			schema.ExecutorCb(exec)
		}
		router.Handle(filepath.Join(fmt.Sprintf("/query/%s", schema.BasePath)), graphql.NewHandler(exec))
	}
	log.Printf("See %s/playground for GraphQL playground", configuration.Use().Origin)
}

func NewGraphQLController(app application.Application) application.Controller {
	return &GraphQLController{
		app: app,
	}
}
