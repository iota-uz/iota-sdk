// Package core provides this package.
package core

import (
	"context"
	"embed"
	"errors"
	"os"
	"time"

	"github.com/benbjohnson/hashfs"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tenant"
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

	ctx := builder.Context()

	storage := composition.Use[upload.Storage]()
	uploadRepo := composition.Use[upload.Repository]()
	userRepo := composition.Use[user.Repository]()
	roleRepo := composition.Use[role.Repository]()
	tenantRepo := composition.Use[tenant.Repository]()
	permissionRepo := composition.Use[permission.Repository]()
	sessionRepo := composition.Use[session.Repository]()
	otpRepo := composition.Use[twofactorentity.OTPRepository]()
	recoveryCodeRepo := composition.Use[twofactorentity.RecoveryCodeRepository]()
	groupRepo := composition.Use[group.Repository]()
	currencyRepo := composition.Use[currency.Repository]()
	userQueryRepo := composition.Use[query.UserQueryRepository]()
	groupQueryRepo := composition.Use[query.GroupQueryRepository]()
	roleQueryRepo := composition.Use[query.RoleQueryRepository]()
	uploadService := composition.Use[*services.UploadService]()
	sessionService := composition.Use[*services.SessionService]()
	userService := composition.Use[*services.UserService]()
	authService := composition.Use[*services.AuthService]()
	authFlowService := composition.Use[*services.AuthFlowService]()
	tenantService := composition.Use[*services.TenantService]()
	groupService := composition.Use[*services.GroupService]()
	twoFactorService := composition.Use[*coreservices2fa.TwoFactorService]()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})
	composition.ContributeSchemas(builder, func(container *composition.Container) ([]application.GraphSchema, error) {
		app, err := composition.RequireApplication(container)
		if err != nil {
			return nil, err
		}
		resolvedUserService, err := userService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedUploadService, err := uploadService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedAuthService, err := authService.Resolve(container)
		if err != nil {
			return nil, err
		}
		return []application.GraphSchema{
			{
				Value: graph.NewExecutableSchema(graph.Config{
					Resolvers: graph.NewResolver(app, resolvedUserService, resolvedUploadService, resolvedAuthService),
				}),
				BasePath: "/",
			},
		}, nil
	})
	composition.ContributeSpotlightProviders(builder, func(*composition.Container) ([]spotlight.SearchProvider, error) {
		return []spotlight.SearchProvider{newSpotlightProvider(ctx.DB())}, nil
	})
	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return BuildNavItems(c.options.DashboardLinkPermissions, c.options.SettingsLinkPermissions), nil
	})
	composition.ContributeHashFS(builder, func(*composition.Container) ([]*hashfs.FS, error) {
		return []*hashfs.FS{assets.HashFS}, nil
	})
	composition.ContributeQuickLinks(builder, func(*composition.Container) ([]*spotlight.QuickLink, error) {
		return []*spotlight.QuickLink{
			spotlight.NewQuickLink(DashboardLink.Name, DashboardLink.Href),
			spotlight.NewQuickLink(UsersLink.Name, UsersLink.Href),
			spotlight.NewQuickLink(GroupsLink.Name, GroupsLink.Href),
			spotlight.NewQuickLink("Users.List.New", "/users/new"),
			spotlight.NewQuickLink("Account.Meta.Index.Title", "/account"),
			spotlight.NewQuickLink("Account.Sessions.Title", "/account/sessions"),
		}, nil
	})
	if builder.Context().HasCapability(composition.CapabilityAPI) {
		cfg := configuration.Use()
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
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

	composition.Provide[upload.Storage](builder, func() (upload.Storage, error) {
		fsStorage, err := persistence.NewFSStorage()
		if err != nil {
			return nil, serrors.E(op, err)
		}
		return fsStorage, nil
	})
	composition.Provide[upload.Repository](builder, func() upload.Repository {
		return newUploadRepository()
	})
	composition.Provide[user.Repository](builder, func(container *composition.Container) (user.Repository, error) {
		resolvedUploadRepo, err := uploadRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return newUserRepository(resolvedUploadRepo), nil
	})
	composition.Provide[role.Repository](builder, func() role.Repository {
		return newRoleRepository()
	})
	composition.Provide[tenant.Repository](builder, func() tenant.Repository {
		return newTenantRepository()
	})
	composition.Provide[permission.Repository](builder, func() permission.Repository {
		return newPermissionRepository()
	})
	composition.Provide[session.Repository](builder, func() session.Repository {
		return newSessionRepository()
	})
	composition.Provide[twofactorentity.OTPRepository](builder, func() twofactorentity.OTPRepository {
		return newOTPRepository()
	})
	composition.Provide[twofactorentity.RecoveryCodeRepository](builder, func() twofactorentity.RecoveryCodeRepository {
		return newRecoveryCodeRepository()
	})
	composition.Provide[group.Repository](builder, func(container *composition.Container) (group.Repository, error) {
		resolvedUserRepo, err := userRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedRoleRepo, err := roleRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return newGroupRepository(resolvedUserRepo, resolvedRoleRepo), nil
	})
	composition.Provide[currency.Repository](builder, func() currency.Repository {
		return newCurrencyRepository()
	})
	composition.Provide[query.UserQueryRepository](builder, func() query.UserQueryRepository {
		return query.NewPgUserQueryRepository()
	})
	composition.Provide[query.GroupQueryRepository](builder, func() query.GroupQueryRepository {
		return query.NewPgGroupQueryRepository()
	})
	composition.Provide[query.RoleQueryRepository](builder, func() query.RoleQueryRepository {
		return query.NewPgRoleQueryRepository()
	})
	composition.Provide[*services.TenantService](builder, func(container *composition.Container) (*services.TenantService, error) {
		resolvedTenantRepo, err := tenantRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewTenantService(resolvedTenantRepo), nil
	})
	composition.Provide[*services.UploadService](builder, func(container *composition.Container) (*services.UploadService, error) {
		resolvedUploadRepo, err := uploadRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedStorage, err := storage.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewUploadService(resolvedUploadRepo, resolvedStorage, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.SessionService](builder, func(container *composition.Container) (*services.SessionService, error) {
		resolvedSessionRepo, err := sessionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewSessionService(resolvedSessionRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.UserService](builder, func(container *composition.Container) (*services.UserService, error) {
		resolvedUserRepo, err := userRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedSessionService, err := sessionService.Resolve(container)
		if err != nil {
			return nil, err
		}
		userValidator := validators.NewUserValidator(resolvedUserRepo)
		return services.NewUserService(resolvedUserRepo, userValidator, ctx.EventPublisher(), resolvedSessionService), nil
	})
	composition.Provide[*services.UserQueryService](builder, func(container *composition.Container) (*services.UserQueryService, error) {
		resolvedUserQueryRepo, err := userQueryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewUserQueryService(resolvedUserQueryRepo), nil
	})
	composition.Provide[*services.GroupQueryService](builder, func(container *composition.Container) (*services.GroupQueryService, error) {
		resolvedGroupQueryRepo, err := groupQueryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewGroupQueryService(resolvedGroupQueryRepo), nil
	})
	composition.Provide[*services.RoleQueryService](builder, func(container *composition.Container) (*services.RoleQueryService, error) {
		resolvedRoleQueryRepo, err := roleQueryRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewRoleQueryService(resolvedRoleQueryRepo), nil
	})
	composition.Provide[*services.ExcelExportService](builder, func(container *composition.Container) (*services.ExcelExportService, error) {
		resolvedUploadService, err := uploadService.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewExcelExportService(ctx.DB(), resolvedUploadService), nil
	})
	composition.Provide[*services.AuthService](builder, func(container *composition.Container) (*services.AuthService, error) {
		resolvedUserService, err := userService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedSessionService, err := sessionService.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewAuthService(resolvedUserService, resolvedSessionService), nil
	})
	composition.Provide[*services.AuthFlowService](builder, func(container *composition.Container) (*services.AuthFlowService, error) {
		resolvedAuthService, err := authService.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedSessionService, err := sessionService.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewAuthFlowService(resolvedAuthService, resolvedSessionService), nil
	})
	composition.Provide[*services.CurrencyService](builder, func(container *composition.Container) (*services.CurrencyService, error) {
		resolvedCurrencyRepo, err := currencyRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewCurrencyService(resolvedCurrencyRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.RoleService](builder, func(container *composition.Container) (*services.RoleService, error) {
		resolvedRoleRepo, err := roleRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewRoleService(resolvedRoleRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.PermissionService](builder, func(container *composition.Container) (*services.PermissionService, error) {
		resolvedPermissionRepo, err := permissionRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewPermissionService(resolvedPermissionRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*services.GroupService](builder, func(container *composition.Container) (*services.GroupService, error) {
		resolvedGroupRepo, err := groupRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		return services.NewGroupService(resolvedGroupRepo, ctx.EventPublisher()), nil
	})
	composition.Provide[*coreservices2fa.TwoFactorService](builder, func(container *composition.Container) (*coreservices2fa.TwoFactorService, error) {
		conf := ctx.Config()
		if conf == nil {
			conf = configuration.Use()
		}
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
		resolvedOTPRepo, err := otpRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedRecoveryCodeRepo, err := recoveryCodeRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		resolvedUserRepo, err := userRepo.Resolve(container)
		if err != nil {
			return nil, err
		}
		twoFactorService, err := coreservices2fa.NewTwoFactorService(
			resolvedOTPRepo,
			resolvedRecoveryCodeRepo,
			resolvedUserRepo,
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
		return twoFactorService, nil
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			resolvedUploadService, err := uploadService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedSessionService, err := sessionService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedUserService, err := userService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedAuthService, err := authService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedAuthFlowService, err := authFlowService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedTenantService, err := tenantService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedGroupService, err := groupService.Resolve(container)
			if err != nil {
				return nil, err
			}
			resolvedTwoFactorService, err := twoFactorService.Resolve(container)
			if err != nil {
				return nil, err
			}
			// AI search is optional; the holder is only provided when a
			// downstream component (e.g. bichat) registers it.
			resolvedAISearchHolder, _, err := composition.ResolveOptional[*spotlight.AISearchServiceHolder](container)
			if err != nil {
				return nil, err
			}
			controllersToRegister := []application.Controller{
				controllers.NewHealthController(app),
				controllers.NewDashboardController(app),
				controllers.NewLoginController(app, resolvedAuthService, resolvedAuthFlowService, c.options.LoginControllerOptions),
				controllers.NewTwoFactorSetupController(app, resolvedTwoFactorService, resolvedSessionService, resolvedUserService),
				controllers.NewTwoFactorVerifyController(app, resolvedTwoFactorService, resolvedSessionService, resolvedUserService),
				controllers.NewSpotlightController(app, resolvedAISearchHolder),
				controllers.NewAccountController(app, resolvedUserService, resolvedTenantService, resolvedUploadService, resolvedSessionService),
				controllers.NewLogoutController(app),
				controllers.NewUploadController(app, resolvedUploadService),
				controllers.NewUsersController(app, resolvedUserService, &controllers.UsersControllerOptions{
					BasePath:         "/users",
					PermissionSchema: c.options.PermissionSchema,
				}),
				controllers.NewRolesController(app, &controllers.RolesControllerOptions{
					BasePath:         "/roles",
					PermissionSchema: c.options.PermissionSchema,
				}),
				controllers.NewGroupsController(app, resolvedGroupService),
				controllers.NewWebSocketController(app),
				controllers.NewSettingsController(app, resolvedTenantService, resolvedUploadService),
				controllers.NewSessionController(app, "/settings/sessions"),
			}
			if ctrl := controllers.NewCrudShowcaseController(app); ctrl != nil {
				controllersToRegister = append(controllersToRegister, ctrl)
			}
			if c.options.UploadsAuthorizer != nil || c.options.DefaultTenantID != uuid.Nil {
				controllersToRegister = append(
					controllersToRegister,
					controllers.NewUploadAPIController(app, resolvedUploadService, uploadAPIControllerOpts(c.options)...),
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
