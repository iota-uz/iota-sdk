package graph

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/pkg/application"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app          application.Application
	orderService *services.OrderService
}

func NewResolver(app application.Application) *Resolver {
	return &Resolver{
		app:          app,
		orderService: app.Service(services.OrderService{}).(*services.OrderService),
	}
}
