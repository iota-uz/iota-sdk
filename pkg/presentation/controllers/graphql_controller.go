package controllers

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/configuration"
	"github.com/iota-agency/iota-sdk/pkg/graphql"
)

type GraphQLController struct {
	app application.Application
}

func (g *GraphQLController) Register(r *mux.Router) {
	// TODO: activate when the graph package is implemented
	schema := graph.NewExecutableSchema(
		graph.Config{ //nolint:exhaustruct
			Resolvers: graph.NewResolver(g.app),
		},
	)
	srv := graphql.NewBaseServer(schema)

	for _, schema := range g.app.GraphSchemas() {
		srv.AddExecutor(executor.New(schema.Value))
	}
	r.Handle("/query", srv)
	r.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
	for _, schema := range g.app.GraphSchemas() {
		r.Handle(filepath.Join(fmt.Sprintf("/query/%s", schema.BasePath)), graphql.NewHandler(executor.New(schema.Value)))
		r.Handle(filepath.Join(fmt.Sprintf("/playground/%s", schema.BasePath)), playground.Handler(fmt.Sprintf("GraphQL Playground (%s)", schema.BasePath), filepath.Join(fmt.Sprintf("/query/%s", schema.BasePath))))
	}
	log.Printf("connect to http://localhost:%d/playground for GraphQL playground", configuration.Use().ServerPort)
}

func NewGraphQLController(app application.Application) application.Controller {
	return &GraphQLController{
		app: app,
	}
}
