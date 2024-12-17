package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/graphql"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
)

type GraphQLController struct {
	app application.Application
}

func (c *GraphQLController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.WithTransaction(),
	}
	schema := graph.NewExecutableSchema(
		graph.Config{
			Resolvers: graph.NewResolver(c.app),
		},
	)

	subRouter := r.PathPrefix("/graphql").Subrouter()
	subRouter.Use(commonMiddleware...)
	subRouter.Handle("/warehouse", graphql.NewDefaultGraphServer(schema))
}

func NewGraphQLController(app application.Application) application.Controller {
	return &GraphQLController{
		app: app,
	}
}
