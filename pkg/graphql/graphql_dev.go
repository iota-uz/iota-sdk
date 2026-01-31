//go:build dev

package graphql

import (
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/executor"
	"github.com/99designs/gqlgen/graphql/handler/extension"
)

func init() {
	registerIntrospection = func(server *Handler, rootExecutor *executor.Executor) {
		server.Use(map[*executor.Executor]graphql.HandlerExtension{
			rootExecutor: extension.Introspection{},
		})
	}
}
