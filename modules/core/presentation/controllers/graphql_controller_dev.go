//go:build dev

package controllers

import (
	"log"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
)

func init() {
	// registerPlaygroundHandler is overridden per-controller-instance in
	// initDevPlayground (called from NewGraphQLController in dev builds).
	// The package-level default still registers the route so that
	// controllers constructed without httpCfg still expose /playground.
	registerPlaygroundHandler = func(router *mux.Router) {
		router.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
	}
}

// initDevPlayground wires a per-instance playground handler that logs the
// full URL using the controller's injected configs. It replaces the
// package-level registerPlaygroundHandler with a closure over the controller.
func initDevPlayground(c *GraphQLController) {
	if c.httpCfg == nil {
		return
	}
	httpCfg := c.httpCfg
	appCfg := c.appCfg
	registerPlaygroundHandler = func(router *mux.Router) {
		router.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
		if appCfg != nil {
			log.Printf("See %s/playground for GraphQL playground", httpCfg.Origin(appCfg))
		}
	}
}
