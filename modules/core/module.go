package core

import (
	"embed"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/validators"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"

	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/core/services/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/application"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
)

//go:generate go run github.com/99designs/gqlgen generate

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/core-schema.sql
var MigrationFiles embed.FS

type ModuleOptions struct {
	PermissionSchema *rbac.PermissionSchema // For UI-only use in RolesController
}

func NewModule(opts *ModuleOptions) application.Module {
	if opts == nil {
		opts = &ModuleOptions{}
	}
	return &Module{
		options: opts,
	}
}

type Module struct {
	options *ModuleOptions
}

func (m *Module) Register(app application.Application) error {
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
	otpRepo := persistence.NewOTPRepository()
	recoveryCodeRepo := persistence.NewRecoveryCodeRepository()

	// Create query repositories
	userQueryRepo := query.NewPgUserQueryRepository()
	groupQueryRepo := query.NewPgGroupQueryRepository()
	roleQueryRepo := query.NewPgRoleQueryRepository()

	// custom validations
	userValidator := validators.NewUserValidator(userRepo)

	// Create services
	tenantService := services.NewTenantService(tenantRepo)
	uploadService := services.NewUploadService(uploadRepo, fsStorage, app.EventPublisher())
	sessionService := services.NewSessionService(persistence.NewSessionRepository(), app.EventPublisher())

	app.RegisterServices(
		uploadService,
		services.NewUserService(userRepo, userValidator, app.EventPublisher(), sessionService),
		services.NewUserQueryService(userQueryRepo),
		services.NewGroupQueryService(groupQueryRepo),
		services.NewRoleQueryService(roleQueryRepo),
		sessionService,
		services.NewExcelExportService(app.DB(), uploadService),
	)
	// Create 2FA service with configuration
	conf := configuration.Use()

	// Create encryptor for TOTP secrets
	var encryptor pkgtwofactor.SecretEncryptor
	if conf.TwoFactorAuth.EncryptionKey != "" {
		// Production: Use AES-256-GCM encryption
		encryptor = pkgtwofactor.NewAESEncryptor(conf.TwoFactorAuth.EncryptionKey)
	} else {
		// Development: Use plaintext (NoopEncryptor)
		// WARNING: Never use in production!
		encryptor = pkgtwofactor.NewNoopEncryptor()
	}

	// Create OTP sender (NoopSender for development)
	// TODO: Replace with real sender (Twilio, SendGrid, etc.) in production
	otpSender := pkgtwofactor.NewNoopSender()

	twoFactorService := twofactor.NewTwoFactorService(
		otpRepo,
		recoveryCodeRepo,
		userRepo,
		twofactor.WithIssuer(conf.TwoFactorAuth.TOTPIssuer),
		twofactor.WithOTPLength(conf.TwoFactorAuth.OTPCodeLength),
		twofactor.WithOTPExpiry(time.Duration(conf.TwoFactorAuth.OTPTTLSeconds)*time.Second),
		twofactor.WithOTPMaxAttempts(conf.TwoFactorAuth.OTPMaxAttempts),
		twofactor.WithSecretEncryptor(encryptor),
		twofactor.WithOTPSender(otpSender),
	)

	app.RegisterServices(
		services.NewAuthService(app),
		services.NewCurrencyService(persistence.NewCurrencyRepository(), app.EventPublisher()),
		services.NewRoleService(roleRepo, app.EventPublisher()),
		tenantService,
		services.NewPermissionService(permRepo, app.EventPublisher()),
		services.NewGroupService(persistence.NewGroupRepository(userRepo, roleRepo), app.EventPublisher()),
		twoFactorService,
	)

	// handlers.RegisterUserHandler(app)

	//controllers.InitCrudShowcase(app)
	app.RegisterControllers(
		controllers.NewHealthController(app),
		controllers.NewDashboardController(app),
		controllers.NewLensEventsController(app),
		controllers.NewLoginController(app),
		controllers.NewTwoFactorSetupController(app),
		controllers.NewTwoFactorVerifyController(app),
		controllers.NewSpotlightController(app),
		controllers.NewAccountController(app),
		controllers.NewLogoutController(app),
		controllers.NewUploadController(app),
		controllers.NewUsersController(app, &controllers.UsersControllerOptions{
			BasePath:         "/users",
			PermissionSchema: m.options.PermissionSchema,
		}),
		controllers.NewRolesController(app, &controllers.RolesControllerOptions{
			BasePath:         "/roles",
			PermissionSchema: m.options.PermissionSchema,
		}),
		controllers.NewGroupsController(app),
		controllers.NewWebSocketController(app),
		controllers.NewSettingsController(app),
	)
	// Register showcase controllers with nil-checks (dev build tag)
	if ctrl := controllers.NewShowcaseController(app); ctrl != nil {
		app.RegisterControllers(ctrl)
	}
	//nolint:staticcheck // SA4023: Always true in dev builds, but false in production (build-tagged stub returns nil)
	if ctrl := controllers.NewCrudShowcaseController(app); ctrl != nil {
		app.RegisterControllers(ctrl)
	}
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
