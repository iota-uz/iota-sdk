package controllers

import (
	"net/http"
	"path"

	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/graphql"
)

// registerPlaygroundHandler is a package-level variable that can be overridden by build tags.
// In development builds, this will register the GraphQL playground handler.
// In production builds, this will be a no-op.
var registerPlaygroundHandler = func(r *mux.Router) {}

type GraphQLController struct {
	app             application.Application
	resolverOptions []graph.ResolverOption
}

// GraphQLControllerOption is a functional option for configuring the GraphQLController.
type GraphQLControllerOption func(*GraphQLController)

// WithResolverOptions sets custom resolver options (e.g., authorizers).
func WithResolverOptions(opts ...graph.ResolverOption) GraphQLControllerOption {
	return func(c *GraphQLController) {
		c.resolverOptions = append(c.resolverOptions, opts...)
	}
}

func (g *GraphQLController) Key() string {
	return "/graphql/core"
}

func (g *GraphQLController) Register(r *mux.Router) {
	schema := graph.NewExecutableSchema(
		graph.Config{
			Resolvers: graph.NewResolver(g.app, g.resolverOptions...),
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
		middleware.ProvideLocalizer(g.app, middleware.LocaleOptions{AcceptLanguageHighPriority: true}),
	)

	router.Handle("/query", srv)
	registerPlaygroundHandler(router)
	for _, schema := range g.app.GraphSchemas() {
		exec := executor.New(schema.Value)
		if schema.ExecutorCb != nil {
			schema.ExecutorCb(exec)
		}
		router.Handle(path.Join("/query", schema.BasePath), graphql.NewHandler(exec))
	}
}

// NewGraphQLController creates a new GraphQL controller with optional configuration.
// Use WithResolverOptions to provide custom authorizers.
//
// Example:
//
//	NewGraphQLController(app,
//	    WithResolverOptions(
//	        graph.WithUsersAuthorizer(customUsersAuthorizer),
//	        graph.WithUploadsAuthorizer(customUploadsAuthorizer),
//	    ),
//	)
func NewGraphQLController(app application.Application, opts ...GraphQLControllerOption) application.Controller {
	c := &GraphQLController{
		app: app,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
