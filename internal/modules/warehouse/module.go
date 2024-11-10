package warehouse

import (
	"context"
	"github.com/benbjohnson/hashfs"
	"github.com/iota-agency/iota-erp/internal/modules/shared"
	"github.com/iota-agency/iota-erp/internal/modules/warehouse/assets"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
	"github.com/iota-agency/iota-erp/internal/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func NewModule() shared.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) MigrationDirs() []string {
	return []string{
		"internal/modules/warehouse/migrations",
	}
}

func (m *Module) Migrations() []string {
	return []string{
		"internal/modules/warehouse/migrations",
	}
}

func (m *Module) Assets() *hashfs.FS {
	return assets.FS
}

func (m *Module) Seed(ctx context.Context) error {
	return nil
}

func (m *Module) Name() string {
	return "warehouse"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Warehouse"}),
			Icon: icons.Book(icons.Props{Size: "20"}),
			Href: "#",
			Children: []types.NavigationItem{
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Products"}),
					Href:        "/warehouse/products",
					Permissions: nil,
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.WarehousePositions"}),
					Href:        "/warehouse/positions",
					Permissions: nil,
				},
			},
		},
	}
}

func (m *Module) Controllers() []shared.ControllerConstructor {
	return []shared.ControllerConstructor{}
}

func (m *Module) LocaleFiles() []string {
	return []string{
		"internal/modules/warehouse/locales/en.json",
		"internal/modules/warehouse/locales/ru.json",
		"internal/modules/warehouse/locales/uz.json",
	}
}
