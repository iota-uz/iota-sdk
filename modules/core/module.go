package core

import (
	"embed"

	"github.com/iota-uz/iota-sdk/pkg/spotlight"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

//go:generate go run github.com/99designs/gqlgen generate

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/core-schema.sql
var MigrationFiles embed.FS

func NewModule() application.Module {
	return &Module{}
}

type Module struct {
}

func (m *Module) Register(app application.Application) error {
	app.RBAC().Register(
		permissions.Permissions...,
	)
	app.RegisterMigrationDirs(&MigrationFiles)
	app.RegisterLocaleFiles(&LocaleFiles)
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
		services.NewUploadService(persistence.NewUploadRepository(), fsStorage, app.EventPublisher()),
		services.NewTabService(persistence.NewTabRepository()),
	)
	app.RegisterControllers(
		controllers.NewDashboardController(app),
		controllers.NewLoginController(app),
		controllers.NewSpotlightController(app),
		controllers.NewAccountController(app),
		controllers.NewLogoutController(app),
		controllers.NewUploadController(app),
		controllers.NewUsersController(app),
		controllers.NewRolesController(app),
	)
	app.RegisterHashFsAssets(assets.HashFS)
	app.RegisterGraphSchema(application.GraphSchema{
		Value: graph.NewExecutableSchema(graph.Config{
			Resolvers: graph.NewResolver(app),
		}),
		BasePath: "/",
	})
	app.Spotlight().Register(
		spotlight.NewItem(nil, DashboardLink.Name, DashboardLink.Href),
		spotlight.NewItem(nil, UsersLink.Name, UsersLink.Href),
		spotlight.NewItem(
			icons.Gear(icons.Props{Size: "24"}),
			"NavigationLinks.Navbar.Settings",
			"/account/settings",
		),
		spotlight.NewItem(
			icons.PlusCircle(icons.Props{Size: "24"}),
			"Users.List.New",
			"/users/new",
		),
	)
	return nil
}

func (m *Module) Name() string {
	return "core"
}
