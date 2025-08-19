package warehouse

import (
	"embed"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/warehouse/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph"
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
	app.QuickLinks().Add(
		spotlight.NewQuickLink(nil, ProductsItem.Name, ProductsItem.Href),
		spotlight.NewQuickLink(nil, PositionsItem.Name, PositionsItem.Href),
		spotlight.NewQuickLink(nil, OrdersItem.Name, OrdersItem.Href),
		spotlight.NewQuickLink(nil, UnitsItem.Name, UnitsItem.Href),
		spotlight.NewQuickLink(nil, InventoryItem.Name, InventoryItem.Href),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"WarehousePositions.List.New",
			"/warehouse/positions/new",
		),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Products.List.New",
			"/warehouse/products/new",
		),
		spotlight.NewQuickLink(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"WarehouseOrders.List.New",
			"/warehouse/orders/new",
		),
		spotlight.NewQuickLink(
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
