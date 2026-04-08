// Package core provides this package.
package core

import (
	"context"
	"embed"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/query"
	"github.com/iota-uz/iota-sdk/modules/core/interfaces/graph"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	coreservices2fa "github.com/iota-uz/iota-sdk/modules/core/services/twofactor"
	"github.com/iota-uz/iota-sdk/modules/core/validators"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

//go:generate go run github.com/99designs/gqlgen generate

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/core-schema.sql
var MigrationFiles embed.FS

type ModuleOptions struct {
	PermissionSchema         *rbac.PermissionSchema
	UploadsAuthorizer        types.UploadsAuthorizer
	DefaultTenantID          uuid.UUID
	LoginControllerOptions   *controllers.LoginControllerOptions
	DashboardLinkPermissions []permission.Permission
	SettingsLinkPermissions  []permission.Permission
}

func NewComponent(opts *ModuleOptions) composition.Component {
	if opts == nil {
		opts = &ModuleOptions{}
	}
	return &component{options: opts}
}

type component struct {
	options *ModuleOptions
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "core"}
}

func (c *component) Build(builder *composition.Builder) error {
	const op serrors.Op = "core.component.Build"

	app := builder.Context().App

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})
	composition.ContributeSchemas(builder, func(*composition.Container) ([]application.GraphSchema, error) {
		return []application.GraphSchema{
			{
				Value: graph.NewExecutableSchema(graph.Config{
					Resolvers: graph.NewResolver(app),
				}),
				BasePath: "/",
			},
		}, nil
	})
	composition.ContributeSpotlightProviders(builder, func(*composition.Container) ([]spotlight.SearchProvider, error) {
		return []spotlight.SearchProvider{newSpotlightProvider(app.DB())}, nil
	})
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		cfg := configuration.Use()
		composition.ContributeHooks(builder, func(*composition.Container) ([]composition.Hook, error) {
			service := app.Spotlight()
			return []composition.Hook{{
				Name: "spotlight",
				Start: func(ctx context.Context, _ *composition.Container) error {
					if cfg.MeiliURL != "" {
						if err := service.Readiness(ctx); err != nil {
							return serrors.E(op, err, "spotlight preflight check")
						}
					}
					if err := service.Start(ctx); err != nil {
						return serrors.E(op, err, "start spotlight service")
					}
					return nil
				},
				Stop: func(ctx context.Context, _ *composition.Container) error {
					return service.Stop(ctx)
				},
			}}, nil
		})
	}

	fsStorage, err := persistence.NewFSStorage()
	if err != nil {
		return serrors.E(op, err)
	}

	uploadRepo := persistence.NewUploadRepository()
	userRepo := persistence.NewUserRepository(uploadRepo)
	roleRepo := persistence.NewRoleRepository()
	tenantRepo := persistence.NewTenantRepository()
	permRepo := persistence.NewPermissionRepository()
	otpRepo := persistence.NewOTPRepository()
	recoveryCodeRepo := persistence.NewRecoveryCodeRepository()

	userQueryRepo := query.NewPgUserQueryRepository()
	groupQueryRepo := query.NewPgGroupQueryRepository()
	roleQueryRepo := query.NewPgRoleQueryRepository()
	userValidator := validators.NewUserValidator(userRepo)

	tenantService := services.NewTenantService(tenantRepo)
	uploadService := services.NewUploadService(uploadRepo, fsStorage, app.EventPublisher())
	sessionService := services.NewSessionService(persistence.NewSessionRepository(), app.EventPublisher())
	userService := services.NewUserService(userRepo, userValidator, app.EventPublisher(), sessionService)
	userQueryService := services.NewUserQueryService(userQueryRepo)
	groupQueryService := services.NewGroupQueryService(groupQueryRepo)
	roleQueryService := services.NewRoleQueryService(roleQueryRepo)
	excelExportService := services.NewExcelExportService(app.DB(), uploadService)
	authService := services.NewAuthService(userService, sessionService)
	authFlowService := services.NewAuthFlowService(authService, sessionService)

	conf := configuration.Use()
	if conf.GoAppEnvironment == "production" &&
		conf.EnableTestEndpoints &&
		os.Getenv("CI") != "true" &&
		os.Getenv("GITHUB_ACTIONS") != "true" {
		return serrors.E(op, serrors.Invalid, errors.New("test endpoints cannot be enabled in production"))
	}
	if !conf.EnableTestEndpoints &&
		conf.GoAppEnvironment == "production" &&
		conf.TwoFactorAuth.Enabled &&
		conf.TwoFactorAuth.EncryptionKey == "" {
		return serrors.E(op, serrors.Invalid, errors.New("TOTP encryption key is required in production"))
	}

	var encryptor pkgtwofactor.SecretEncryptor
	if conf.EnableTestEndpoints {
		encryptor = pkgtwofactor.NewNoopEncryptor()
	} else if conf.TwoFactorAuth.EncryptionKey != "" {
		encryptor = pkgtwofactor.NewAESEncryptor(conf.TwoFactorAuth.EncryptionKey)
	} else {
		encryptor = pkgtwofactor.NewNoopEncryptor()
	}

	var otpSender pkgtwofactor.OTPSender
	if conf.EnableTestEndpoints {
		otpSender = pkgtwofactor.NewNoopSender()
	} else if conf.GoAppEnvironment == "production" || conf.GoAppEnvironment == "staging" {
		composite := pkgtwofactor.NewCompositeSender(nil)
		if conf.OTPDelivery.EnableEmail && conf.SMTP.Host != "" {
			composite.Register(
				pkgtwofactor.ChannelEmail,
				coreservices2fa.NewEmailOTPSender(
					conf.SMTP.Host,
					conf.SMTP.Port,
					conf.SMTP.Username,
					conf.SMTP.Password,
					conf.SMTP.From,
				),
			)
		}
		if conf.OTPDelivery.EnableSMS && conf.Twilio.AccountSID != "" && conf.Twilio.AuthToken != "" {
			composite.Register(
				pkgtwofactor.ChannelSMS,
				coreservices2fa.NewSMSOTPSender(
					conf.Twilio.AccountSID,
					conf.Twilio.AuthToken,
					conf.Twilio.PhoneNumber,
				),
			)
		}
		otpSender = composite
	} else {
		otpSender = pkgtwofactor.NewNoopSender()
	}

	twoFactorService, err := coreservices2fa.NewTwoFactorService(
		otpRepo,
		recoveryCodeRepo,
		userRepo,
		coreservices2fa.WithIssuer(conf.TwoFactorAuth.TOTPIssuer),
		coreservices2fa.WithOTPLength(conf.TwoFactorAuth.OTPCodeLength),
		coreservices2fa.WithOTPExpiry(time.Duration(conf.TwoFactorAuth.OTPTTLSeconds)*time.Second),
		coreservices2fa.WithOTPMaxAttempts(conf.TwoFactorAuth.OTPMaxAttempts),
		coreservices2fa.WithSecretEncryptor(encryptor),
		coreservices2fa.WithOTPSender(otpSender),
	)
	if err != nil {
		return serrors.E(op, "failed to create two-factor service", err)
	}

	currencyService := services.NewCurrencyService(persistence.NewCurrencyRepository(), app.EventPublisher())
	roleService := services.NewRoleService(roleRepo, app.EventPublisher())
	permissionService := services.NewPermissionService(permRepo, app.EventPublisher())
	groupService := services.NewGroupService(persistence.NewGroupRepository(userRepo, roleRepo), app.EventPublisher())

	composition.Provide[*services.UploadService](builder, uploadService)
	composition.Provide[*services.UserService](builder, userService)
	composition.Provide[*services.UserQueryService](builder, userQueryService)
	composition.Provide[*services.GroupQueryService](builder, groupQueryService)
	composition.Provide[*services.RoleQueryService](builder, roleQueryService)
	composition.Provide[*services.SessionService](builder, sessionService)
	composition.Provide[*services.ExcelExportService](builder, excelExportService)
	composition.Provide[*services.AuthService](builder, authService)
	composition.Provide[*services.AuthFlowService](builder, authFlowService)
	composition.Provide[*services.CurrencyService](builder, currencyService)
	composition.Provide[*services.RoleService](builder, roleService)
	composition.Provide[*services.TenantService](builder, tenantService)
	composition.Provide[*services.PermissionService](builder, permissionService)
	composition.Provide[*services.GroupService](builder, groupService)
	composition.Provide[*coreservices2fa.TwoFactorService](builder, twoFactorService)

	DashboardLinkPermissions = c.options.DashboardLinkPermissions
	SettingsLinkPermissions = c.options.SettingsLinkPermissions
	NavItems = ResolvedNavItems()

	app.RegisterHashFsAssets(assets.HashFS)
	app.QuickLinks().Add(
		spotlight.NewQuickLink(DashboardLink.Name, DashboardLink.Href),
		spotlight.NewQuickLink(UsersLink.Name, UsersLink.Href),
		spotlight.NewQuickLink(GroupsLink.Name, GroupsLink.Href),
		spotlight.NewQuickLink("Users.List.New", "/users/new"),
		spotlight.NewQuickLink("Account.Meta.Index.Title", "/account"),
		spotlight.NewQuickLink("Account.Sessions.Title", "/account/sessions"),
	)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
			controllersToRegister := []application.Controller{
				controllers.NewHealthController(app),
				controllers.NewDashboardController(app),
				controllers.NewLoginController(app, c.options.LoginControllerOptions),
				controllers.NewTwoFactorSetupController(app),
				controllers.NewTwoFactorVerifyController(app),
				controllers.NewSpotlightController(app),
				controllers.NewAccountController(app),
				controllers.NewLogoutController(app),
				controllers.NewUploadController(app),
				controllers.NewUsersController(app, &controllers.UsersControllerOptions{
					BasePath:         "/users",
					PermissionSchema: c.options.PermissionSchema,
				}),
				controllers.NewRolesController(app, &controllers.RolesControllerOptions{
					BasePath:         "/roles",
					PermissionSchema: c.options.PermissionSchema,
				}),
				controllers.NewGroupsController(app),
				controllers.NewWebSocketController(app),
				controllers.NewSettingsController(app),
				controllers.NewSessionController(app, "/settings/sessions"),
				controllers.NewCrudShowcaseController(app),
			}
			if c.options.UploadsAuthorizer != nil || c.options.DefaultTenantID != uuid.Nil {
				controllersToRegister = append(
					controllersToRegister,
					controllers.NewUploadAPIController(app, uploadAPIControllerOpts(c.options)...),
				)
			}
			if ctrl := controllers.NewShowcaseController(app); ctrl != nil {
				controllersToRegister = append(controllersToRegister, ctrl)
			}
			return controllersToRegister, nil
		})
	}

	return nil
}

func uploadAPIControllerOpts(opts *ModuleOptions) []controllers.UploadAPIControllerOption {
	var result []controllers.UploadAPIControllerOption
	if opts.UploadsAuthorizer != nil {
		result = append(result, controllers.WithAPIUploadsAuthorizer(opts.UploadsAuthorizer))
	}
	if opts.DefaultTenantID != uuid.Nil {
		result = append(result, controllers.WithDefaultTenantID(opts.DefaultTenantID))
	}
	return result
}
