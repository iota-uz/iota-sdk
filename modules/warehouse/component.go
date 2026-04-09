// Package warehouse provides this package.
package warehouse

import (
	"embed"

	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
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
	ctx := builder.Context()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&localeFiles}, nil
	})
	composition.ContributeSchemas(builder, func(container *composition.Container) ([]application.GraphSchema, error) {
		app, err := composition.RequireApplication(container)
		if err != nil {
			return nil, err
		}
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

	userRepo := composition.Use[coreuser.Repository]()
	unitRepo := composition.Use[unit.Repository]()
	positionRepo := composition.Use[position.Repository]()
	productRepo := composition.Use[product.Repository]()
	orderRepo := composition.Use[order.Repository]()
	inventoryRepo := composition.Use[inventory.Repository]()
	unitService := composition.Use[*services.UnitService]()
	productService := composition.Use[*productservice.ProductService]()

	composition.Provide[unit.Repository](builder, func() unit.Repository {
		return persistence.NewUnitRepository()
	})
	composition.Provide[position.Repository](builder, func() position.Repository {
		return persistence.NewPositionRepository()
	})
	composition.Provide[product.Repository](builder, func() product.Repository {
		return persistence.NewProductRepository()
	})
	composition.Provide[order.Repository](builder, func(container *composition.Container) (order.Repository, error) {
		resolvedProductRepo, err := productRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return persistence.NewOrderRepository(resolvedProductRepo), nil
	})
	composition.Provide[inventory.Repository](builder, func(container *composition.Container) (inventory.Repository, error) {
		resolvedUserRepo, err := userRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedPositionRepo, err := positionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return persistence.NewInventoryRepository(resolvedUserRepo, resolvedPositionRepo), nil
	})
	composition.Provide[*services.UnitService](builder, func(container *composition.Container) (*services.UnitService, error) {
		resolvedUnitRepo, err := unitRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewUnitService(resolvedUnitRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*productservice.ProductService](builder, func(container *composition.Container) (*productservice.ProductService, error) {
		resolvedProductRepo, err := productRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return productservice.NewProductService(resolvedProductRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*positionservice.PositionService](builder, func(container *composition.Container) (*positionservice.PositionService, error) {
		uploadService, err := composition.Resolve[*coreservices.UploadService](container)
		if err != nil {
			return nil, err
		}
		resolvedPositionRepo, err := positionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedUnitService, err := unitService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedProductService, err := productService.Resolve(container)
		if err != nil {
			return nil, err
		}
		return positionservice.NewPositionService(
			resolvedPositionRepo,
			ctx.EventPublisher(),
			uploadService,
			resolvedUnitService,
			resolvedProductService,
		), nil
	})
	composition.Provide[*orderservice.OrderService](builder, func(container *composition.Container) (*orderservice.OrderService, error) {
		resolvedOrderRepo, err := orderRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedProductRepo, err := productRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return orderservice.NewOrderService(ctx.EventPublisher(), resolvedOrderRepo, resolvedProductRepo), nil
	})
	composition.Provide[*services.InventoryService](builder, func(container *composition.Container) (*services.InventoryService, error) {
		resolvedInventoryRepo, err := inventoryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedPositionRepo, err := positionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedProductRepo, err := productRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewInventoryService(resolvedInventoryRepo, resolvedPositionRepo, resolvedProductRepo, ctx.EventPublisher()), nil
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
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
