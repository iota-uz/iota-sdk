// Package graph provides this package.
package graph

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	positionservice "github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	app              application.Application
	orderService     *orderservice.OrderService
	productService   *productservice.ProductService
	positionService  *positionservice.PositionService
	inventoryService *services.InventoryService
}

func NewResolver(app application.Application) *Resolver {
	return &Resolver{
		app:              app,
		orderService:     composition.MustResolveForApp[*orderservice.OrderService](app),
		productService:   composition.MustResolveForApp[*productservice.ProductService](app),
		positionService:  composition.MustResolveForApp[*positionservice.PositionService](app),
		inventoryService: composition.MustResolveForApp[*services.InventoryService](app),
	}
}
