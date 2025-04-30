package core

import (
	"embed"

	"github.com/iota-uz/iota-sdk/modules/core/validators"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/handlers"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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
	app.Migrations().RegisterSchema(&MigrationFiles)
	app.RegisterLocaleFiles(&LocaleFiles)
	fsStorage, err := persistence.NewFSStorage()
	if err != nil {
		return err
	}
	// Register upload repository first since user repository needs it
	uploadRepo := persistence.NewUploadRepository()

	// Create repositories
	userRepo := persistence.NewUserRepository(uploadRepo)
	roleRepo := persistence.NewRoleRepository()
	permRepo := persistence.NewPermissionRepository()

	// custom validations
	userValidator := validators.NewUserValidator(userRepo)

	app.RegisterServices(
		services.NewUploadService(uploadRepo, fsStorage, app.EventPublisher()),
		services.NewUserService(userRepo, userValidator, app.EventPublisher()),
		services.NewSessionService(persistence.NewSessionRepository(), app.EventPublisher()),
	)
	app.RegisterServices(
		services.NewAuthService(app),
		services.NewCurrencyService(persistence.NewCurrencyRepository(), app.EventPublisher()),
		services.NewRoleService(roleRepo, app.EventPublisher()),
		services.NewPermissionService(permRepo, app.EventPublisher()),
		services.NewTabService(persistence.NewTabRepository()),
		services.NewGroupService(persistence.NewGroupRepository(userRepo, roleRepo), app.EventPublisher()),
	)

	tabHandler := handlers.NewTabHandler(
		app,
		configuration.Use().Logger(),
	)
	tabHandler.Register(app.EventPublisher())

	handlers.RegisterUserHandler(app)

	app.RegisterControllers(
		controllers.NewDashboardController(app),
		controllers.NewLoginController(app),
		controllers.NewSpotlightController(app),
		controllers.NewAccountController(app),
		controllers.NewLogoutController(app),
		controllers.NewUploadController(app),
		controllers.NewUsersController(app),
		controllers.NewRolesController(app),
		controllers.NewGroupsController(app),
		controllers.NewDIExampleController(app),
		controllers.NewShowcaseController(app),
	)
	app.RegisterHashFsAssets(assets.HashFS)
	app.RegisterGraphSchema(application.GraphSchema{
		Value: graph.NewExecutableSchema(graph.Config{
			Resolvers: graph.NewResolver(app),
		}),
		BasePath: "/",
	})
	app.Spotlight().Register(&dataSource{})
	app.QuickLinks().Add(
		spotlight.NewQuickLink(DashboardLink.Icon, DashboardLink.Name, DashboardLink.Href),
		spotlight.NewQuickLink(UsersLink.Icon, UsersLink.Name, UsersLink.Href),
		spotlight.NewQuickLink(GroupsLink.Icon, GroupsLink.Name, GroupsLink.Href),
		spotlight.NewQuickLink(
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
