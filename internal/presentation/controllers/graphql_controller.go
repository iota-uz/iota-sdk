package controllers

import (
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/configuration"
	"github.com/iota-agency/iota-erp/internal/interfaces/graph"
	"log"
)

type GraphQLController struct {
	app *services.Application
}

func (g *GraphQLController) Register(r *mux.Router) {
	r.Handle("/query", graph.NewDefaultServer(g.app))
	r.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
	log.Printf("connect to http://localhost:%d/playground for GraphQL playground", configuration.Use().ServerPort)
}

func NewGraphQLController(app *services.Application) Controller {
	return &GraphQLController{
		app: app,
	}
}
