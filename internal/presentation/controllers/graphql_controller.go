package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/application"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
)

type GraphQLController struct {
	app *application.Application
}

func (g *GraphQLController) Register(r *mux.Router) {
	// TODO: activate when the graph package is implemented
	//r.Handle("/query", graph.NewDefaultServer(g.app))
	//r.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
	//log.Printf("connect to http://localhost:%d/playground for GraphQL playground", configuration.Use().ServerPort)
}

func NewGraphQLController(app *application.Application) shared.Controller {
	return &GraphQLController{
		app: app,
	}
}
