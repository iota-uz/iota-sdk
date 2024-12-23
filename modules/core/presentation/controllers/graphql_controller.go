package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/graphql"
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
	r.Handle("/graphql/core", graphql.NewDefaultGraphServer(schema))
}

func NewGraphQLController(app application.Application) application.Controller {
	return &GraphQLController{
		app: app,
	}
}
