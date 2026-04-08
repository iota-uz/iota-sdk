// Package oidc provides this package.
package oidc

import (
	"context"
	"embed"
	"time"

	corepersistence "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/oidc/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/oidc/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed presentation/locales/*.toml
var LocaleFiles embed.FS

type ModuleOptions struct{}

func NewModule(opts *ModuleOptions) application.Module {
	if opts == nil {
		opts = &ModuleOptions{}
	}
	return &Module{
		options: opts,
	}
}

type Module struct {
	options     *ModuleOptions
	enabled     bool
	storage     *oidc.Storage
	config      *configuration.OIDCOptions
	oidcService *services.OIDCService
}

func (m *Module) Name() string {
	return "oidc"
}

func (m *Module) RegisterWiring(app application.Application) error {
	app.RegisterLocaleFiles(&LocaleFiles)

	config := configuration.Use()

	if !config.OIDC.IsConfigured() {
		m.enabled = false
		return nil
	}
	m.enabled = true
	m.config = &config.OIDC

	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()
	tokenRepo := persistence.NewTokenRepository()

	// Create user repository from core persistence.
	// This avoids tight coupling to service-registration order and concrete service types.
	userRepo := corepersistence.NewUserRepository(corepersistence.NewUploadRepository())

	storage := oidc.NewStorage(
		clientRepo,
		authRequestRepo,
		tokenRepo,
		userRepo,
		app.DB(),
		config.OIDC.CryptoKey,
		config.OIDC.IssuerURL,
		config.OIDC.AccessTokenLifetime,
		config.OIDC.RefreshTokenLifetime,
	)
	m.storage = storage

	oidcService := services.NewOIDCService(clientRepo, authRequestRepo)
	m.oidcService = oidcService

	app.RegisterServices(oidcService)
	app.RegisterRuntime(application.RuntimeRegistration{
		Component: &oidcBootstrapComponent{
			pool:      app.DB(),
			cryptoKey: config.OIDC.CryptoKey,
		},
		Tags: []application.RuntimeTag{
			application.RuntimeTagAPI,
		},
	})

	return nil
}

func (m *Module) RegisterTransports(app application.Application) error {
	if !m.enabled {
		return nil
	}
	app.RegisterControllers(
		controllers.NewOIDCController(app, m.storage, m.config, m.oidcService),
	)
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
	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := oidc.BootstrapKeys(startCtx, c.pool, c.cryptoKey); err != nil {
		const op serrors.Op = "oidcBootstrapComponent.Start"
		return serrors.E(op, "failed to bootstrap OIDC signing keys", err)
	}
	return nil
}

func (c *oidcBootstrapComponent) Stop(ctx context.Context) error {
	_ = ctx
	return nil
}
