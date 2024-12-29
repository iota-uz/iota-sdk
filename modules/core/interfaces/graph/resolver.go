package graph

import (
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app         application.Application
	userService *services.UserService
}

func NewResolver(app application.Application) *Resolver {
	return &Resolver{
		app:         app,
		userService: app.Service(services.UserService{}).(*services.UserService),
	}
}
