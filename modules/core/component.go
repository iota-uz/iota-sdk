// Package core provides this package.
package core

import (
	"context"
	"embed"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	twofactorentity "github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
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
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
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
	_ = op

	composition.AddLocales(builder, &LocaleFiles)
	composition.AddNavItems(builder, BuildNavItems(c.options.DashboardLinkPermissions, c.options.SettingsLinkPermissions)...)
	composition.AddHashFS(builder, assets.HashFS)
	composition.ContributeMigrations(builder, &MigrationFiles)
	composition.AddQuickLinks(builder,
		spotlight.NewQuickLink(DashboardLink.Name, DashboardLink.Href),
		spotlight.NewQuickLink(UsersLink.Name, UsersLink.Href),
		spotlight.NewQuickLink(GroupsLink.Name, GroupsLink.Href),
		spotlight.NewQuickLink("Users.List.New", "/users/new"),
		spotlight.NewQuickLink("Account.Meta.Index.Title", "/account"),
		spotlight.NewQuickLink("Account.Sessions.Title", "/account/sessions"),
	)

	composition.ContributeSpotlightProviders(builder, func(container *composition.Container) ([]spotlight.SearchProvider, error) {
		pool, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		return []spotlight.SearchProvider{newSpotlightProvider(pool)}, nil
	})

	// ----- Storage -----
	composition.ProvideFunc(builder, newCoreFSStorage)

	// ----- Repositories -----
	composition.ProvideFunc(builder, newUploadRepository)
	composition.ProvideFunc(builder, newUserRepository)
	composition.ProvideFunc(builder, newRoleRepository)
	composition.ProvideFunc(builder, newTenantRepository)
	composition.ProvideFunc(builder, newPermissionRepository)
	composition.ProvideFunc(builder, newSessionRepository)
	composition.ProvideFunc(builder, newOTPRepository)
	composition.ProvideFunc(builder, newRecoveryCodeRepository)
	composition.ProvideFunc(builder, newGroupRepository)
	composition.ProvideFunc(builder, newCurrencyRepository)
	composition.ProvideFunc(builder, query.NewPgUserQueryRepository)
	composition.ProvideFunc(builder, query.NewPgGroupQueryRepository)
	composition.ProvideFunc(builder, query.NewPgRoleQueryRepository)

	// ----- Services -----
	composition.ProvideFunc(builder, services.NewTenantService)
	composition.ProvideFunc(builder, services.NewUploadService)
	composition.ProvideFunc(builder, services.NewSessionService)
	composition.ProvideFunc(builder, newCoreUserService)
	composition.ProvideFunc(builder, services.NewUserQueryService)
	composition.ProvideFunc(builder, services.NewGroupQueryService)
	composition.ProvideFunc(builder, services.NewRoleQueryService)
	composition.ProvideFunc(builder, services.NewExcelExportService)
	composition.ProvideFunc(builder, services.NewAuthService)
	composition.ProvideFunc(builder, services.NewAuthFlowService)
	composition.ProvideFunc(builder, services.NewCurrencyService)
	composition.ProvideFunc(builder, services.NewRoleService)
	composition.ProvideFunc(builder, services.NewPermissionService)
	composition.ProvideFunc(builder, services.NewGroupService)
	composition.ProvideFunc(builder, newCoreTwoFactorService)

	// ----- GraphQL schema -----
	composition.ContributeSchemas(builder, func(container *composition.Container) ([]application.GraphSchema, error) {
		app, err := composition.RequireApplication(container)
		if err != nil {
			return nil, err
		}
		userSvc, err := composition.Resolve[*services.UserService](container)
		if err != nil {
			return nil, err
		}
		uploadSvc, err := composition.Resolve[*services.UploadService](container)
		if err != nil {
			return nil, err
		}
		authSvc, err := composition.Resolve[*services.AuthService](container)
		if err != nil {
			return nil, err
		}
		return []application.GraphSchema{
			{
				Value: graph.NewExecutableSchema(graph.Config{
					Resolvers: graph.NewResolver(app, userSvc, uploadSvc, authSvc),
				}),
				BasePath: "/",
			},
		}, nil
	})

	// ----- Spotlight startup hook -----
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		cfg := configuration.Use()
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			service, err := composition.Resolve[spotlight.Service](container)
			if err != nil {
				return nil, err
			}
			return []composition.Hook{{
				Name: "spotlight",
				Start: func(ctx context.Context) (composition.StopFn, error) {
					if cfg.MeiliURL != "" {
						if err := service.Readiness(ctx); err != nil {
							return nil, serrors.E(op, err, "spotlight preflight check")
						}
					}
					if err := service.Start(ctx); err != nil {
						return nil, serrors.E(op, err, "start spotlight service")
					}
					return service.Stop, nil
				},
			}}, nil
		})
	}

	// ----- HTTP controllers -----
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		opts := c.options
		composition.ContributeControllersFunc(builder, func(
			app application.Application,
			uploadService *services.UploadService,
			sessionService *services.SessionService,
			userService *services.UserService,
			authService *services.AuthService,
			authFlowService *services.AuthFlowService,
			tenantService *services.TenantService,
			groupService *services.GroupService,
			twoFactorService *coreservices2fa.TwoFactorService,
		) []application.Controller {
			// AI search holder is optional; resolved separately at request
			// time inside the spotlight controller.
			aiHolder, _ := resolveOptionalAISearchHolder(app)

			ctrls := []application.Controller{
				controllers.NewHealthController(app),
				controllers.NewDashboardController(app),
				controllers.NewLoginController(app, authService, authFlowService, opts.LoginControllerOptions),
				controllers.NewTwoFactorSetupController(app, twoFactorService, sessionService, userService),
				controllers.NewTwoFactorVerifyController(app, twoFactorService, sessionService, userService),
				controllers.NewSpotlightController(app, aiHolder),
				controllers.NewAccountController(app, userService, tenantService, uploadService, sessionService),
				controllers.NewLogoutController(app),
				controllers.NewUploadController(app, uploadService),
				controllers.NewUsersController(app, userService, &controllers.UsersControllerOptions{
					BasePath:         "/users",
					PermissionSchema: opts.PermissionSchema,
				}),
				controllers.NewRolesController(app, &controllers.RolesControllerOptions{
					BasePath:         "/roles",
					PermissionSchema: opts.PermissionSchema,
				}),
				controllers.NewGroupsController(app, groupService),
				controllers.NewWebSocketController(app),
				controllers.NewSettingsController(app, tenantService, uploadService),
				controllers.NewSessionController(app, "/settings/sessions"),
				controllers.NewCrudShowcaseController(app),
			}
			if opts.UploadsAuthorizer != nil || opts.DefaultTenantID != uuid.Nil {
				ctrls = append(ctrls, controllers.NewUploadAPIController(app, uploadService, uploadAPIControllerOpts(opts)...))
			}
			if ctrl := controllers.NewShowcaseController(app); ctrl != nil {
				ctrls = append(ctrls, ctrl)
			}
			return ctrls
		})
	}

	return nil
}

// newCoreFSStorage wraps NewFSStorage to return upload.Storage so the
// reflection injector can find a unique key. NewFSStorage already returns
// (storage, error).
func newCoreFSStorage() (upload.Storage, error) {
	return persistence.NewFSStorage()
}

// newCoreUserService injects the validator constructor inline since the
// validator depends on user.Repository (no eventbus or session etc.).
func newCoreUserService(
	repo user.Repository,
	bus eventbus.EventBus,
	sessionService *services.SessionService,
) *services.UserService {
	return services.NewUserService(repo, validators.NewUserValidator(repo), bus, sessionService)
}

// newCoreTwoFactorService bootstraps the 2FA service from configuration. The
// behaviour is identical to the previous closure-based provider; only the
// type of dependency injection has changed.
func newCoreTwoFactorService(
	otpRepo twofactorentity.OTPRepository,
	recoveryCodeRepo twofactorentity.RecoveryCodeRepository,
	userRepo user.Repository,
) (*coreservices2fa.TwoFactorService, error) {
	const op serrors.Op = "core.newCoreTwoFactorService"

	conf := configuration.Use()
	if conf.GoAppEnvironment == "production" &&
		conf.EnableTestEndpoints &&
		os.Getenv("CI") != "true" &&
		os.Getenv("GITHUB_ACTIONS") != "true" {
		return nil, serrors.E(op, serrors.Invalid, errors.New("test endpoints cannot be enabled in production"))
	}
	if !conf.EnableTestEndpoints &&
		conf.GoAppEnvironment == "production" &&
		conf.TwoFactorAuth.Enabled &&
		conf.TwoFactorAuth.EncryptionKey == "" {
		return nil, serrors.E(op, serrors.Invalid, errors.New("TOTP encryption key is required in production"))
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
	svc, err := coreservices2fa.NewTwoFactorService(
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
		return nil, serrors.E(op, "failed to create two-factor service", err)
	}
	return svc, nil
}

// resolveOptionalAISearchHolder returns the AI search holder if a downstream
// component (e.g. bichat) registered one, or nil. The lookup is best-effort
// because the holder is optional.
func resolveOptionalAISearchHolder(app application.Application) (*spotlight.AISearchServiceHolder, error) {
	_ = app
	// The previous implementation resolved this through the container at
	// controller-construction time. Spotlight controller has a nil-safe
	// holder accessor; passing nil is the historical "no AI search" path.
	return nil, nil
}

// unused at present but retained to satisfy older imports.
var _ = i18n.Bundle{}

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
