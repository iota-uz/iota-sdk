package graph

import (
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"gorm.io/gorm"
)

//go:generate go run github.com/99designs/gqlgen generate

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	db  *gorm.DB
	app *services.Application
}

func NewDefaultServer(db *gorm.DB, app *services.Application) *handler.Server {
	return handler.NewDefaultServer(NewExecutableSchema(
		Config{
			Resolvers: &Resolver{
				db:  db,
				app: app,
			},
		},
	))
}
