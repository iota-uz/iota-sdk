package core

import (
	"context"
	"embed"
	"github.com/iota-agency/iota-sdk/modules/core/seed"

	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
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
	app.RegisterMigrationDirs(&migrationFiles)
	app.RegisterLocaleFiles(&localeFiles)
	app.RegisterModule(m)
	return nil
}

func (m *Module) Seed(ctx context.Context, app application.Application) error {
	seedFuncs := []application.SeedFunc{
		seed.CreatePermissions,
		seed.CreateCurrencies,
		seed.CreateUser,
	}
	for _, seedFunc := range seedFuncs {
		if err := seedFunc(ctx, app); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) Name() string {
	return "core"
}

func (m *Module) NavigationItems(localizer *i18n.Localizer) []types.NavigationItem {
	return []types.NavigationItem{
		{
			Name:     localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Dashboard"}),
			Icon:     icons.Warehouse(icons.Props{Size: "20"}),
			Href:     "/",
			Children: nil,
		},
	}
}