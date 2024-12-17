package graph

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
)

//go:generate go run github.com/99designs/gqlgen generate

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app application.Application
}

func NewResolver(app application.Application) *Resolver {
	return &Resolver{
		app: app,
	}
}
