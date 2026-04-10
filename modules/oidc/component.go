// Package oidc provides this package.
package oidc

import (
	"context"
	"embed"
	"time"

	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
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
	composition.AddLocales(builder, &LocaleFiles)

	config := configuration.Use().OIDC
	if !config.IsConfigured() {
		return nil
	}

	composition.Provide[OIDCConfig](builder, config)
	composition.ProvideFunc(builder, persistence.NewClientRepository)
	composition.ProvideFunc(builder, persistence.NewAuthRequestRepository)
	composition.ProvideFunc(builder, persistence.NewTokenRepository)
	composition.ProvideFunc(builder, services.NewOIDCService)
	composition.ProvideFunc(builder, newOIDCStorage)

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		pool := builder.Context().DB()
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			cfg, err := composition.Resolve[OIDCConfig](container)
			if err != nil {
				return nil, err
			}
			cryptoKey := cfg.CryptoKey
			return []composition.Hook{{
				Name: "oidc-bootstrap-keys",
				Start: func(ctx context.Context) (composition.StopFn, error) {
					const op serrors.Op = "oidc.bootstrap.Start"
					if pool == nil {
						return nil, serrors.E(op, serrors.Invalid, "database pool is nil")
					}
					startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()
					if err := oidcinfra.BootstrapKeys(startCtx, pool, cryptoKey); err != nil {
						return nil, serrors.E(op, err)
					}
					return func(context.Context) error { return nil }, nil
				},
			}}, nil
		})

		composition.ContributeControllersFunc(builder, func(
			cfg OIDCConfig,
			storage *oidcinfra.Storage,
			oidcService *services.OIDCService,
			sessionService *coreservices.SessionService,
		) []application.Controller {
			return []application.Controller{
				controllers.NewOIDCController(storage, &cfg, oidcService, sessionService),
			}
		})
	}

	return nil
}

// newOIDCStorage adapts the OIDC storage constructor (which mixes typed deps
// with config fields) into a function shape ProvideFunc can call.
func newOIDCStorage(
	cfg OIDCConfig,
	clientRepo client.Repository,
	authRequestRepo authrequest.Repository,
	tokenRepo token.Repository,
	userRepo coreuser.Repository,
	pool *pgxpool.Pool,
) *oidcinfra.Storage {
	return oidcinfra.NewStorage(
		clientRepo,
		authRequestRepo,
		tokenRepo,
		userRepo,
		pool,
		cfg.CryptoKey,
		cfg.IssuerURL,
		cfg.AccessTokenLifetime,
		cfg.RefreshTokenLifetime,
	)
}
