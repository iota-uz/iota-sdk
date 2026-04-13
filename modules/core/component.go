// Package core provides this package.
package core

import (
	"context"
	"embed"
	"errors"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	twofactorentity "github.com/iota-uz/iota-sdk/modules/core/domain/entities/twofactor"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/handlers"
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

type ModuleOptions struct {
	PermissionSchema         *rbac.PermissionSchema
	UploadsAuthorizer        types.UploadsAuthorizer
	DefaultTenantID          uuid.UUID
	LoginControllerOptions   *controllers.LoginControllerOptions
	DashboardLinkPermissions []permission.Permission
	SettingsLinkPermissions  []permission.Permission
	UserControllerOptions    []controllers.UserControllerOption

	// SkipAdminControllers suppresses registration of the admin-facing
	// controllers (dashboard, users, roles, groups, settings, sessions,
	// spotlight, websocket). Auth controllers (login, logout, two-factor,
	// account) and infrastructure controllers (health, upload) are still
	// registered. Use this for specialized binaries like superadmin that
	// provide their own admin UI.
	SkipAdminControllers bool
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

	composition.AddLocales(builder, &LocaleFiles)
	composition.AddHashFS(builder, assets.HashFS)
	// Self-service quick links are always available (AccountController
	// is registered regardless of SkipAdminControllers).
	composition.AddQuickLinks(builder,
		spotlight.NewQuickLink("Account.Meta.Index.Title", "/account"),
		spotlight.NewQuickLink("Account.Sessions.Title", "/account/sessions"),
	)
	if !c.options.SkipAdminControllers {
		composition.AddNavItems(builder, BuildNavItems(c.options.DashboardLinkPermissions, c.options.SettingsLinkPermissions)...)
		composition.AddQuickLinks(builder,
			spotlight.NewQuickLink(DashboardLink.Name, DashboardLink.Href),
			spotlight.NewQuickLink(UsersLink.Name, UsersLink.Href),
			spotlight.NewQuickLink(GroupsLink.Name, GroupsLink.Href),
			spotlight.NewQuickLink("Users.List.New", "/users/new"),
		)
	}

	composition.ContributeSpotlightProviders(builder, func(container *composition.Container) ([]spotlight.SearchProvider, error) {
		pool, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		return []spotlight.SearchProvider{newSpotlightProvider(pool)}, nil
	})

	// ----- Storage -----
	composition.ProvideFuncAs[upload.Storage](builder, persistence.NewFSStorage)

	// ----- Repositories -----
	composition.ProvideFunc(builder, persistence.NewUploadRepository)
	composition.ProvideFunc(builder, persistence.NewUserRepository)
	composition.ProvideFunc(builder, persistence.NewRoleRepository)
	composition.ProvideFunc(builder, persistence.NewTenantRepository)
	composition.ProvideFunc(builder, persistence.NewPermissionRepository)
	composition.ProvideFunc(builder, persistence.NewSessionRepository)
	composition.ProvideFunc(builder, persistence.NewOTPRepository)
	composition.ProvideFunc(builder, persistence.NewRecoveryCodeRepository)
	composition.ProvideFunc(builder, persistence.NewGroupRepository)
	composition.ProvideFunc(builder, persistence.NewCurrencyRepository)
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
	composition.ProvideFunc(builder, newCoreAuthService)
	composition.ProvideFunc(builder, services.NewAuthFlowService)
	composition.ProvideFunc(builder, services.NewCurrencyService)
	composition.ProvideFunc(builder, services.NewRoleService)
	composition.ProvideFunc(builder, services.NewPermissionService)
	composition.ProvideFunc(builder, services.NewGroupService)
	composition.ProvideFunc(builder, newCoreTwoFactorService)

	// ----- Event handlers -----
	// Revoke active sessions whenever a user's password changes so that
	// leaked credentials stop being honoured after a reset.
	composition.ProvideFunc(builder, handlers.NewUserHandler)
	composition.ContributeEventHandlerFunc(builder, func(h *handlers.UserHandler) any {
		return h.OnPasswordUpdated
	})
	// Realtime websocket broadcasts for user CRUD events — one subscription
	// per event kind, torn down cleanly via the hook lifecycle.
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ProvideFunc(builder, controllers.NewUserRealtimeUpdates)
		composition.ContributeEventHandlerFunc(builder, func(ru *controllers.UserRealtimeUpdates) any {
			return ru.OnUserCreated
		})
		composition.ContributeEventHandlerFunc(builder, func(ru *controllers.UserRealtimeUpdates) any {
			return ru.OnUserUpdated
		})
		composition.ContributeEventHandlerFunc(builder, func(ru *controllers.UserRealtimeUpdates) any {
			return ru.OnUserDeleted
		})
	}

	// ----- GraphQL schema -----
	composition.ContributeSchemas(builder, func(container *composition.Container) ([]application.GraphSchema, error) {
		app, err := composition.Resolve[application.Application](container)
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
			bus eventbus.EventBus,
			uploadService *services.UploadService,
			sessionService *services.SessionService,
			userService *services.UserService,
			authService *services.AuthService,
			authFlowService *services.AuthFlowService,
			tenantService *services.TenantService,
			groupService *services.GroupService,
			twoFactorService *coreservices2fa.TwoFactorService,
		) []application.Controller {
			// Auth and infrastructure controllers — always registered.
			ctrls := []application.Controller{
				controllers.NewHealthController(app),
				controllers.NewLoginController(authService, authFlowService, opts.LoginControllerOptions),
				controllers.NewTwoFactorSetupController(twoFactorService, sessionService, userService),
				controllers.NewTwoFactorVerifyController(twoFactorService, sessionService, userService),
				controllers.NewAccountController(app, userService, tenantService, uploadService, sessionService),
				controllers.NewLogoutController(),
				controllers.NewUploadController(uploadService),
			}
			if opts.UploadsAuthorizer != nil || opts.DefaultTenantID != uuid.Nil {
				ctrls = append(ctrls, controllers.NewUploadAPIController(uploadService, uploadAPIControllerOpts(opts)...))
			}
			userControllerOpts := []controllers.UserControllerOption{
				controllers.WithUserControllerBasePath("/users"),
				controllers.WithUserControllerPermissionSchema(c.options.PermissionSchema),
			}
			userControllerOpts = append(userControllerOpts, c.options.UserControllerOptions...)

			// Admin UI controllers — skipped for specialized binaries
			// (e.g. superadmin) that provide their own admin interface.
			if !opts.SkipAdminControllers {
				ctrls = append(ctrls,
					controllers.NewDashboardController(),
					// Spotlight controller accepts a nil AI search holder; downstream
					// components that need AI-assisted search install one explicitly.
					controllers.NewSpotlightController(app, nil),
					controllers.NewUsersController(app, userControllerOpts...),
					controllers.NewRolesController(&controllers.RolesControllerOptions{
						BasePath:         "/roles",
						PermissionSchema: opts.PermissionSchema,
					}),
					controllers.NewGroupsController(app, groupService),
					controllers.NewWebSocketController(app),
					controllers.NewSettingsHubController(),
					controllers.NewSettingsLogoController(tenantService, uploadService),
					controllers.NewSessionController("/settings/sessions"),
				)
				// NewCrudShowcaseController returns nil in the `!dev` build so
				// we must nil-guard the append rather than splatting it into
				// the literal above.
				if ctrl := controllers.NewCrudShowcaseController(bus); ctrl != nil {
					ctrls = append(ctrls, ctrl)
				}
				if ctrl := controllers.NewShowcaseController(); ctrl != nil {
					ctrls = append(ctrls, ctrl)
				}
			}

			return ctrls
		})
	}

	return nil
}

// newCoreAuthService adapts services.NewAuthService (which takes a variadic
// options slice) to a non-variadic constructor that the reflection injector
// can call. The injector refuses variadic constructors because silently
// dropping options is a footgun; call NewAuthService explicitly here and
// return the result.
func newCoreAuthService(
	usersService *services.UserService,
	sessionService *services.SessionService,
) *services.AuthService {
	return services.NewAuthService(usersService, sessionService)
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
