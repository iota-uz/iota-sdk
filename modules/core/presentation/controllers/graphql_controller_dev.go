//go:build dev

package controllers

import (
	"log"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func init() {
	registerPlaygroundHandler = func(router *mux.Router) {
		router.Handle("/playground", playground.Handler("GraphQL playground", "/query"))
		log.Printf("See %s/playground for GraphQL playground", configuration.Use().Origin)
	}
}
