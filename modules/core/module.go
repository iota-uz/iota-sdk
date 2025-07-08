package core

import (
	"embed"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/pkg/crud"

	"github.com/iota-uz/iota-sdk/modules/core/validators"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"

	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/handlers"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
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
	tenantRepo := persistence.NewTenantRepository()
	permRepo := persistence.NewPermissionRepository()

	// Create query repositories
	userQueryRepo := query.NewPgUserQueryRepository()
	groupQueryRepo := query.NewPgGroupQueryRepository()

	// custom validations
	userValidator := validators.NewUserValidator(userRepo)

	// Create services
	tabService := services.NewTabService(persistence.NewTabRepository())
	tenantService := services.NewTenantService(tenantRepo)
	uploadService := services.NewUploadService(uploadRepo, fsStorage, app.EventPublisher())

	app.RegisterServices(
		uploadService,
		services.NewUserService(userRepo, userValidator, app.EventPublisher()),
		services.NewUserQueryService(userQueryRepo),
		services.NewGroupQueryService(groupQueryRepo),
		services.NewSessionService(persistence.NewSessionRepository(), app.EventPublisher()),
		services.NewExcelExportService(app.DB(), uploadService),
	)
	app.RegisterServices(
		services.NewAuthService(app),
		services.NewCurrencyService(persistence.NewCurrencyRepository(), app.EventPublisher()),
		services.NewRoleService(roleRepo, app.EventPublisher()),
		tabService,
		tenantService,
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

	fields := crud.NewFields([]crud.Field{
		crud.NewStringField(
			"code",
			crud.WithKey(),
			crud.WithMaxLen(3),
			crud.WithSearchable(),
		),
		crud.NewStringField(
			"name",
			crud.WithMaxLen(255),
			crud.WithSearchable(),
		),
		crud.NewStringField(
			"symbol",
			crud.WithMaxLen(3),
			crud.WithSearchable(),
		),
		crud.NewDateTimeField("created_at",
			crud.WithHidden(),
			crud.WithInitialValue(func() any {
				return time.Now()
			}),
		),
		crud.NewDateTimeField(
			"updated_at",
			crud.WithHidden(),
			crud.WithInitialValue(func() any {
				return time.Now()
			}),
		),
	})

	schema := crud.NewSchema[currency.Currency](
		"currencies",
		fields,
		currency.NewMapper(fields),
	)

	builder := crud.NewBuilder[currency.Currency](
		schema,
		app.EventPublisher(),
	)

	app.RegisterControllers(
		controllers.NewHealthController(app),
		controllers.NewDashboardController(app),
		controllers.NewLensEventsController(app),
		controllers.NewLoginController(app),
		controllers.NewSpotlightController(app),
		controllers.NewAccountController(app),
		controllers.NewLogoutController(app),
		controllers.NewUploadController(app),
		controllers.NewUsersController(app),
		controllers.NewRolesController(app),
		controllers.NewGroupsController(app),
		controllers.NewShowcaseController(app),
		controllers.NewWebSocketController(app),
		controllers.NewSettingsController(app),
		controllers.NewCrudController[currency.Currency](
			"/currencies",
			app,
			builder,
		),
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
