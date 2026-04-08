// Package oidc provides this package.
package oidc

import (
	"context"
	"embed"
	"time"

	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/authrequest"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/client"
	"github.com/iota-uz/iota-sdk/modules/oidc/domain/entities/token"
	oidcinfra "github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/oidc/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed presentation/locales/*.toml
var LocaleFiles embed.FS

type ModuleOptions struct{}

type OIDCConfig = configuration.OIDCOptions

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
	return composition.Descriptor{
		Name:     "oidc",
		Requires: []string{"core"},
	}
}

func (c *component) Build(builder *composition.Builder) error {
	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})

	config := configuration.Use().OIDC
	if !config.IsConfigured() {
		return nil
	}

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeHooks(builder, func(*composition.Container) ([]composition.Hook, error) {
			component := &oidcBootstrapComponent{
				pool:      builder.Context().App.DB(),
				cryptoKey: config.CryptoKey,
			}
			return []composition.Hook{{
				Name: component.Name(),
				Start: func(ctx context.Context, _ *composition.Container) error {
					return component.Start(ctx)
				},
				Stop: func(ctx context.Context, _ *composition.Container) error {
					return component.Stop(ctx)
				},
			}}, nil
		})
	}

	composition.Provide[OIDCConfig](builder, config)
	composition.Provide[client.Repository](builder, persistence.NewClientRepository())
	composition.Provide[authrequest.Repository](builder, persistence.NewAuthRequestRepository())
	composition.Provide[token.Repository](builder, persistence.NewTokenRepository())
	composition.Provide[coreuser.Repository](builder, func() coreuser.Repository {
		return corepersistence.NewUserRepository(corepersistence.NewUploadRepository())
	})
	composition.Provide[*oidcinfra.Storage](builder, func(container *composition.Container) (*oidcinfra.Storage, error) {
		cfg, err := composition.Resolve[OIDCConfig](container)
		if err != nil {
			return nil, err
		}
		clientRepo, err := composition.Resolve[client.Repository](container)
		if err != nil {
			return nil, err
		}
		authRequestRepo, err := composition.Resolve[authrequest.Repository](container)
		if err != nil {
			return nil, err
		}
		tokenRepo, err := composition.Resolve[token.Repository](container)
		if err != nil {
			return nil, err
		}
		userRepo, err := composition.Resolve[coreuser.Repository](container)
		if err != nil {
			return nil, err
		}
		return oidcinfra.NewStorage(
			clientRepo,
			authRequestRepo,
			tokenRepo,
			userRepo,
			builder.Context().App.DB(),
			cfg.CryptoKey,
			cfg.IssuerURL,
			cfg.AccessTokenLifetime,
			cfg.RefreshTokenLifetime,
		), nil
	})
	composition.Provide[*services.OIDCService](builder, func(container *composition.Container) (*services.OIDCService, error) {
		clientRepo, err := composition.Resolve[client.Repository](container)
		if err != nil {
			return nil, err
		}
		authRequestRepo, err := composition.Resolve[authrequest.Repository](container)
		if err != nil {
			return nil, err
		}
		return services.NewOIDCService(clientRepo, authRequestRepo), nil
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		optionalConfig := composition.UseOptional[OIDCConfig]()
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			cfg, ok, err := optionalConfig.Resolve(container)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, nil
			}
			storage, err := composition.Resolve[*oidcinfra.Storage](container)
			if err != nil {
				return nil, err
			}
			oidcService, err := composition.Resolve[*services.OIDCService](container)
			if err != nil {
				return nil, err
			}
			sessionService, err := composition.Resolve[*coreservices.SessionService](container)
			if err != nil {
				return nil, err
			}
			cfgCopy := cfg
			return []application.Controller{
				controllers.NewOIDCController(storage, &cfgCopy, oidcService, sessionService),
			}, nil
		})
	}

	return nil
}

type oidcBootstrapComponent struct {
	pool      *pgxpool.Pool
	cryptoKey string
}

func (c *oidcBootstrapComponent) Name() string {
	return "oidc-bootstrap-keys"
}

func (c *oidcBootstrapComponent) Start(ctx context.Context) error {
	const op serrors.Op = "oidcBootstrapComponent.Start"

	if c.pool == nil {
		return serrors.E(op, serrors.Invalid, "database pool is nil")
	}

	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := oidcinfra.BootstrapKeys(startCtx, c.pool, c.cryptoKey); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func (c *oidcBootstrapComponent) Stop(ctx context.Context) error {
	_ = ctx
	return nil
}
