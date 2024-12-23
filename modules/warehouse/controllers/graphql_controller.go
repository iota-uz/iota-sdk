package controllers

import (
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/pkg/application"
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

	subRouter := r.PathPrefix("/graphql").Subrouter()
	subRouter.Use(commonMiddleware...)
}

func NewGraphQLController(app application.Application) application.Controller {
	return &GraphQLController{
		app: app,
	}
}
