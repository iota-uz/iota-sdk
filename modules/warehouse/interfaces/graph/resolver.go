// Package graph provides this package.
package graph

import (
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	positionservice "github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/application"
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

func NewResolver(
	app application.Application,
	orderService *orderservice.OrderService,
	productService *productservice.ProductService,
	positionService *positionservice.PositionService,
	inventoryService *services.InventoryService,
) *Resolver {
	return &Resolver{
		app:              app,
		orderService:     orderService,
		productService:   productService,
		positionService:  positionService,
		inventoryService: inventoryService,
	}
}
