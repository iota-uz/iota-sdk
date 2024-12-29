package warehouse

import (
	"embed"
	"github.com/iota-uz/iota-sdk/modules/warehouse/assets"
	"github.com/iota-uz/iota-sdk/modules/warehouse/controllers"
	"github.com/iota-uz/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-uz/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-uz/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	orderservice "github.com/iota-uz/iota-sdk/modules/warehouse/services/order_service"
	positionservice "github.com/iota-uz/iota-sdk/modules/warehouse/services/position_service"
	productservice "github.com/iota-uz/iota-sdk/modules/warehouse/services/product_service"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:generate go run github.com/99designs/gqlgen generate

//go:embed locales/*.json
var localeFiles embed.FS

//go:embed migrations/*.sql
var migrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	app.RegisterServices(
		services.NewUnitService(persistence.NewUnitRepository(), app.EventPublisher()),
		productservice.NewProductService(persistence.NewProductRepository(), app.EventPublisher()),
	)

	app.RegisterServices(
		positionservice.NewPositionService(
			persistence.NewPositionRepository(),
			app.EventPublisher(),
			app,
		),
		orderservice.NewOrderService(
			app.EventPublisher(),
			persistence.NewOrderRepository(),
			persistence.NewProductRepository(),
		),
		services.NewInventoryService(app.EventPublisher()),
	)

	app.RegisterPermissions(
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
	app.RegisterMigrationDirs(&migrationFiles)
	app.RegisterAssets(&assets.FS)
	sl := app.Spotlight()
	for _, l := range NavItems {
		sl.Register(spotlight.NewItem(l.Icon, l.Name, l.Href))
	}
	app.Spotlight().Register(
		spotlight.NewItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"WarehousePositions.List.New",
			"/warehouse/positions/new",
		),
		spotlight.NewItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Products.List.New",
			"/warehouse/products/new",
		),
		spotlight.NewItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"WarehouseOrders.List.New",
			"/warehouse/orders/new",
		),
		spotlight.NewItem(
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
