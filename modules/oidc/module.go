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
	options *ModuleOptions
}

func (m *Module) Name() string {
	return "oidc"
}

func (m *Module) Register(app application.Application) error {
	// Register locales
	app.RegisterLocaleFiles(&LocaleFiles)

	// Get configuration
	config := configuration.Use()

	// Only register OIDC when required settings are configured
	if !config.OIDC.IsConfigured() {
		return nil
	}

	// Create repositories
	clientRepo := persistence.NewClientRepository()
	authRequestRepo := persistence.NewAuthRequestRepository()
	tokenRepo := persistence.NewTokenRepository()

	// Create user repository from core persistence.
	// This avoids tight coupling to service-registration order and concrete service types.
	userRepo := corepersistence.NewUserRepository(corepersistence.NewUploadRepository())

	// Create OIDC storage adapter (bridge to zitadel/oidc library)
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

	// Bootstrap signing keys on startup (if not already present)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := oidc.BootstrapKeys(ctx, app.DB(), config.OIDC.CryptoKey); err != nil {
		const op serrors.Op = "Module.Register"
		return serrors.E(op, "failed to bootstrap OIDC signing keys", err)
	}

	// Create service
	oidcService := services.NewOIDCService(clientRepo, authRequestRepo)

	// Register services
	app.RegisterServices(oidcService)

	// Register controller
	app.RegisterControllers(
		controllers.NewOIDCController(app, storage, &config.OIDC, oidcService),
	)

	return nil
}
