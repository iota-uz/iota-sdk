package graph

import (
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/order_service"
	positionservice "github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	"github.com/iota-agency/iota-sdk/pkg/application"
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
		orderService:     app.Service(orderservice.OrderService{}).(*orderservice.OrderService),
		productService:   app.Service(productservice.ProductService{}).(*productservice.ProductService),
		positionService:  app.Service(positionservice.PositionService{}).(*positionservice.PositionService),
		inventoryService: app.Service(services.InventoryService{}).(*services.InventoryService),
	}
}
