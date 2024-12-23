package core

import (
	"embed"
	"github.com/iota-agency/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-agency/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-agency/iota-sdk/modules/core/seed"
	"github.com/iota-agency/iota-sdk/modules/core/services"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/presentation/assets"
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
	fsStorage, err := persistence.NewFSStorage()
	if err != nil {
		return err
	}
	app.RegisterServices(
		services.NewUserService(persistence.NewUserRepository(), app.EventPublisher()),
		services.NewSessionService(persistence.NewSessionRepository(), app.EventPublisher()),
	)
	app.RegisterServices(
		services.NewAuthService(app),
		services.NewCurrencyService(persistence.NewCurrencyRepository(), app.EventPublisher()),
		services.NewRoleService(persistence.NewRoleRepository(), app.EventPublisher()),
		services.NewPositionService(persistence.NewPositionRepository(), app.EventPublisher()),
		services.NewEmployeeService(persistence.NewEmployeeRepository(), app.EventPublisher()),
		services.NewUploadService(persistence.NewUploadRepository(), fsStorage, app.EventPublisher()),
		services.NewTabService(persistence.NewTabRepository()),
	)
	app.RegisterControllers(
		controllers.NewDashboardController(app),
		controllers.NewLoginController(app),
		controllers.NewSpotlightController(app),
		controllers.NewAccountController(app),
		controllers.NewEmployeeController(app),
		controllers.NewGraphQLController(app),
		controllers.NewLogoutController(app),
		controllers.NewUploadController(app),
	)
	app.RegisterHashFsAssets(assets.HashFS)
	return nil
}

func (m *Module) Name() string {
	return "core"
}
