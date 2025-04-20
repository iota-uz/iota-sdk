package warehouse

import (
	"embed"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/orderservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/productservice"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:generate go run github.com/99designs/gqlgen generate

//go:embed presentation/locales/*.json
var localeFiles embed.FS

//go:embed infrastructure/persistence/schema/warehouse-schema.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	unitRepo := persistence.NewUnitRepository()
	positionRepo := persistence.NewPositionRepository()
	productRepo := persistence.NewProductRepository()

	unitService := services.NewUnitService(unitRepo, app.EventPublisher())
	app.RegisterServices(unitService)

	productService := productservice.NewProductService(productRepo, app.EventPublisher())
	app.RegisterServices(productService)
	app.RegisterServices(
		services.NewUnitService(unitRepo, app.EventPublisher()),
		productservice.NewProductService(productRepo, app.EventPublisher()),
	)

	app.RegisterServices(
		positionservice.NewPositionService(
			positionRepo,
			app.EventPublisher(),
			app,
		),
		orderservice.NewOrderService(
			app.EventPublisher(),
			persistence.NewOrderRepository(productRepo),
			productRepo,
		),
		services.NewInventoryService(app.EventPublisher()),
	)

	app.RBAC().Register(
		permissions.ProductCreate,
		permissions.ProductRead,
		permissions.ProductUpdate,
		permissions.ProductDelete,
		permissions.PositionCreate,
		permissions.PositionRead,
		permissions.PositionUpdate,
		permissions.PositionDelete,
		permissions.OrderCreate,
		permissions.OrderRead,
		permissions.OrderUpdate,
		permissions.OrderDelete,
		permissions.UnitCreate,
		permissions.UnitRead,
		permissions.UnitUpdate,
		permissions.UnitDelete,
		permissions.InventoryCreate,
		permissions.InventoryRead,
		permissions.InventoryUpdate,
		permissions.InventoryDelete,
	)
	app.RegisterControllers(
		controllers.NewProductsController(app),
		controllers.NewPositionsController(app),
		controllers.NewUnitsController(app),
		controllers.NewOrdersController(app),
		controllers.NewInventoryController(app),
	)
	app.RegisterLocaleFiles(&localeFiles)
	app.Migrations().RegisterSchema(&migrationFiles)
	app.RegisterAssets(&assets.FS)
	app.Spotlight().Register(
		spotlight.NewLocalizedItem(nil, ProductsItem.Name, ProductsItem.Href),
		spotlight.NewLocalizedItem(nil, PositionsItem.Name, PositionsItem.Href),
		spotlight.NewLocalizedItem(nil, OrdersItem.Name, OrdersItem.Href),
		spotlight.NewLocalizedItem(nil, UnitsItem.Name, UnitsItem.Href),
		spotlight.NewLocalizedItem(nil, InventoryItem.Name, InventoryItem.Href),
		spotlight.NewLocalizedItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"WarehousePositions.List.New",
			"/warehouse/positions/new",
		),
		spotlight.NewLocalizedItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Products.List.New",
			"/warehouse/products/new",
		),
		spotlight.NewLocalizedItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"WarehouseOrders.List.New",
			"/warehouse/orders/new",
		),
		spotlight.NewLocalizedItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"WarehouseUnits.List.New",
			"/warehouse/units/new",
		),
	)

	app.RegisterGraphSchema(application.GraphSchema{
		Value: graph.NewExecutableSchema(graph.Config{
			Resolvers: graph.NewResolver(app),
		}),
		BasePath: "/warehouse",
	})
	return nil
}

func (m *Module) Name() string {
	return "warehouse"
}
