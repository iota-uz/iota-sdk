package warehouse

import (
	"embed"

	"github.com/iota-agency/iota-sdk/modules/warehouse/assets"
	"github.com/iota-agency/iota-sdk/modules/warehouse/controllers"
	"github.com/iota-agency/iota-sdk/modules/warehouse/interfaces/graph"
	"github.com/iota-agency/iota-sdk/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/modules/warehouse/presentation/templates"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	orderservice "github.com/iota-agency/iota-sdk/modules/warehouse/services/order_service"
	positionservice "github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	productservice "github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	"github.com/iota-agency/iota-sdk/pkg/application"
)

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
	app.RegisterTemplates(&templates.FS)

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
