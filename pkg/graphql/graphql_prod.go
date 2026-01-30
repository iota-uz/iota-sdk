//go:build !dev

package graphql

import "github.com/99designs/gqlgen/graphql/executor"

func init() {
	registerIntrospection = func(server *Handler, rootExecutor *executor.Executor) {
		// No introspection in production
	}
}
