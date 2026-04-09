// Package warehouse provides this package.
package warehouse

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:generate go run github.com/99designs/gqlgen generate

//go:embed presentation/locales/*.json
var localeFiles embed.FS

func NewComponent() composition.Component {
	return &component{}
}

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{
		Name:     "warehouse",
		Requires: []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.AddLocales(builder, &localeFiles)
	composition.AddNavItems(builder, NavItems...)
	composition.AddAssets(builder, &assets.FS)
	composition.AddQuickLinks(builder,
		spotlight.NewQuickLink(ProductsItem.Name, ProductsItem.Href),
		spotlight.NewQuickLink(PositionsItem.Name, PositionsItem.Href),
		spotlight.NewQuickLink(OrdersItem.Name, OrdersItem.Href),
		spotlight.NewQuickLink(UnitsItem.Name, UnitsItem.Href),
		spotlight.NewQuickLink(InventoryItem.Name, InventoryItem.Href),
		spotlight.NewQuickLink("WarehousePositions.List.New", "/warehouse/positions/new"),
		spotlight.NewQuickLink("Products.List.New", "/warehouse/products/new"),
		spotlight.NewQuickLink("WarehouseOrders.List.New", "/warehouse/orders/new"),
		spotlight.NewQuickLink("WarehouseUnits.List.New", "/warehouse/units/new"),
	)

	composition.ProvideFunc(builder, persistence.NewUnitRepository)
	composition.ProvideFunc(builder, persistence.NewPositionRepository)
	composition.ProvideFunc(builder, persistence.NewProductRepository)
	composition.ProvideFunc(builder, persistence.NewOrderRepository)
	composition.ProvideFunc(builder, persistence.NewInventoryRepository)
	composition.ProvideFunc(builder, services.NewUnitService)
	composition.ProvideFunc(builder, productservice.NewProductService)
	composition.ProvideFunc(builder, positionservice.NewPositionService)
	composition.ProvideFunc(builder, orderservice.NewOrderService)
	composition.ProvideFunc(builder, services.NewInventoryService)

	composition.ContributeControllersFunc(builder, func(
		app application.Application,
		unitService *services.UnitService,
		productService *productservice.ProductService,
		positionService *positionservice.PositionService,
		orderService *orderservice.OrderService,
		inventoryService *services.InventoryService,
	) []application.Controller {
		return []application.Controller{
			controllers.NewProductsController(app, productService, positionService),
			controllers.NewPositionsController(app),
			controllers.NewUnitsController(app, unitService),
			controllers.NewOrdersController(app, orderService, positionService, productService),
			controllers.NewInventoryController(app, inventoryService, positionService),
		}
	})

	composition.ContributeSchemas(builder, func(container *composition.Container) ([]application.GraphSchema, error) {
		app, err := composition.RequireApplication(container)
		if err != nil {
			return nil, err
		}
		orderSvc, err := composition.Resolve[*orderservice.OrderService](container)
		if err != nil {
			return nil, err
		}
		productSvc, err := composition.Resolve[*productservice.ProductService](container)
		if err != nil {
			return nil, err
		}
		positionSvc, err := composition.Resolve[*positionservice.PositionService](container)
		if err != nil {
			return nil, err
		}
		inventorySvc, err := composition.Resolve[*services.InventoryService](container)
		if err != nil {
			return nil, err
		}
		return []application.GraphSchema{
			{
				Value: graph.NewExecutableSchema(graph.Config{
					Resolvers: graph.NewResolver(app, orderSvc, productSvc, positionSvc, inventorySvc),
				}),
				BasePath: "/warehouse",
			},
		}, nil
	})

	return nil
}
