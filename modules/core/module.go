package core

import (
	"embed"
	"github.com/iota-agency/iota-sdk/modules/core/seed"
	"github.com/iota-agency/iota-sdk/pkg/presentation/assets"
	"github.com/iota-agency/iota-sdk/pkg/presentation/controllers"

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
	app.RegisterSeedFuncs(
		seed.CreatePermissions,
		seed.CreateCurrencies,
		seed.CreateUser,
	)
	app.RegisterControllers(
		controllers.NewLoginController(app),
		controllers.NewSpotlightController(app),
		controllers.NewAccountController(app),
		controllers.NewEmployeeController(app),
		controllers.NewGraphQLController(app),
		controllers.NewLogoutController(app),
		controllers.NewUploadController(app),
	)
	app.RegisterHashFsAssets(assets.HashFS)
	app.RegisterModule(m)
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
