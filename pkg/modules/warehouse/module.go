package warehouse

import (
	"context"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/permission"
	"github.com/iota-agency/iota-sdk/pkg/modules/shared"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/assets"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/controllers"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/permissions"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/persistence"
	"github.com/iota-agency/iota-sdk/pkg/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app *application.Application) error {
	unitService := services.NewUnitService(persistence.NewUnitRepository(), app.EventPublisher)
	positionService := services.NewPositionService(persistence.NewPositionRepository(), app.EventPublisher)
	productService := services.NewProductService(persistence.NewProductRepository(), app.EventPublisher, positionService)
	app.RegisterService(unitService)
	app.RegisterService(positionService)
	app.RegisterService(productService)
	app.Rbac.Register(
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
	)
	return nil
}

func (m *Module) MigrationDirs() []string {
	return []string{
		"pkg/modules/warehouse/migrations",
	}
}

func (m *Module) Assets() *hashfs.FS {
	return assets.FS
}

func (m *Module) Seed(ctx context.Context, app *application.Application) error {
	return nil
}

func (m *Module) Name() string {
	return "warehouse"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Warehouse"}),
			Icon: icons.Warehouse(icons.Props{Size: "20"}),
			Href: "#",
			Children: []types.NavigationItem{
				{
					Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Products"}),
					Href: "/warehouse/products",
					Permissions: []permission.Permission{
						permissions.ProductRead,
					},
				},
				{
					Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.WarehousePositions"}),
					Href: "/warehouse/positions",
					Permissions: []permission.Permission{
						permissions.PositionRead,
					},
				},
				{
					Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.WarehouseUnits"}),
					Href: "/warehouse/units",
					Permissions: []permission.Permission{
						permissions.UnitRead,
					},
				},
				{
					Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.WarehouseOrders"}),
					Href: "/warehouse/orders",
					Permissions: []permission.Permission{
						permissions.OrderRead,
					},
				},
			},
		},
	}
}

func (m *Module) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{
		controllers.NewProductsController,
		controllers.NewPositionsController,
		controllers.NewUnitsController,
	}
}

func (m *Module) LocaleFiles() []string {
	return []string{
		"pkg/modules/warehouse/locales/en.json",
		"pkg/modules/warehouse/locales/ru.json",
		"pkg/modules/warehouse/locales/uz.json",
	}
}
