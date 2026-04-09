// Package warehouse provides this package.
package warehouse

import (
	"embed"

	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
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
	"github.com/iota-uz/iota-sdk/pkg/types"
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
	app := builder.Context().App

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&localeFiles}, nil
	})
	composition.ContributeSchemas(builder, func(*composition.Container) ([]application.GraphSchema, error) {
		return []application.GraphSchema{
			{
				Value: graph.NewExecutableSchema(graph.Config{
					Resolvers: graph.NewResolver(app),
				}),
				BasePath: "/warehouse",
			},
		}, nil
	})
	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})
	composition.ContributeAssets(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&assets.FS}, nil
	})
	composition.ContributeQuickLinks(builder, func(*composition.Container) ([]*spotlight.QuickLink, error) {
		return []*spotlight.QuickLink{
			spotlight.NewQuickLink(ProductsItem.Name, ProductsItem.Href),
			spotlight.NewQuickLink(PositionsItem.Name, PositionsItem.Href),
			spotlight.NewQuickLink(OrdersItem.Name, OrdersItem.Href),
			spotlight.NewQuickLink(UnitsItem.Name, UnitsItem.Href),
			spotlight.NewQuickLink(InventoryItem.Name, InventoryItem.Href),
			spotlight.NewQuickLink("WarehousePositions.List.New", "/warehouse/positions/new"),
			spotlight.NewQuickLink("Products.List.New", "/warehouse/products/new"),
			spotlight.NewQuickLink("WarehouseOrders.List.New", "/warehouse/orders/new"),
			spotlight.NewQuickLink("WarehouseUnits.List.New", "/warehouse/units/new"),
		}, nil
	})

	unitRepo := persistence.NewUnitRepository()
	positionRepo := persistence.NewPositionRepository()
	productRepo := persistence.NewProductRepository()

	unitService := services.NewUnitService(unitRepo, app.EventPublisher())
	productService := productservice.NewProductService(productRepo, app.EventPublisher())
	orderService := orderservice.NewOrderService(
		app.EventPublisher(),
		persistence.NewOrderRepository(productRepo),
		productRepo,
	)
	inventoryService := services.NewInventoryService(app.EventPublisher())

	composition.Provide[*services.UnitService](builder, unitService)
	composition.Provide[*productservice.ProductService](builder, productService)
	composition.Provide[*positionservice.PositionService](builder, func(container *composition.Container) (*positionservice.PositionService, error) {
		uploadService, err := composition.Resolve[*coreservices.UploadService](container)
		if err != nil {
			return nil, err
		}
		return positionservice.NewPositionService(
			positionRepo,
			app.EventPublisher(),
			uploadService,
			unitService,
			productService,
		), nil
	})
	composition.Provide[*orderservice.OrderService](builder, orderService)
	composition.Provide[*services.InventoryService](builder, inventoryService)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
			return []application.Controller{
				controllers.NewProductsController(app),
				controllers.NewPositionsController(app),
				controllers.NewUnitsController(app),
				controllers.NewOrdersController(app),
				controllers.NewInventoryController(app),
			}, nil
		})
	}

	return nil
}
