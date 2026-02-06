package bichat

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/bichat-schema.sql
var MigrationFiles embed.FS

// NewModule creates a BiChat module for schema and locale registration.
//
// For full BiChat functionality, you must:
// 1. Create a ModuleConfig using NewModuleConfig() in config.go
// 2. Implement required dependencies (Model, ChatRepository, etc.)
// 3. Register controllers and services in your application setup
//
// See CLAUDE.md for complete configuration examples.
func NewModule() application.Module {
	return &Module{}
}

// NewModuleWithConfig creates a BiChat module with observability integration.
// This constructor initializes the EventBridge and registers observability providers.
//
// Usage:
//
//	cfg := bichat.NewModuleConfig(...)
//	module := bichat.NewModuleWithConfig(cfg)
//	app.RegisterModule(module)
//	defer module.Shutdown(context.Background())
func NewModuleWithConfig(cfg *ModuleConfig) *Module {
	m := &Module{
		config:                 cfg,
		observabilityProviders: cfg.ObservabilityProviders,
	}

	// Initialize EventBridge if providers are configured
	if len(cfg.ObservabilityProviders) > 0 {
		m.eventBridge = observability.NewEventBridge(cfg.EventBus, cfg.ObservabilityProviders)
	}

	return m
}

type Module struct {
	config                 *ModuleConfig
	observabilityProviders []observability.Provider
	eventBridge            *observability.EventBridge
}

func (m *Module) Register(app application.Application) error {
	// Register database schema
	app.Migrations().RegisterSchema(&MigrationFiles)

	// Register translation files
	app.RegisterLocaleFiles(&LocaleFiles)

	// Register BiChat applet (unified applet system)
	bichatApplet := NewBiChatApplet(m.config)
	if err := app.RegisterApplet(bichatApplet); err != nil {
		return fmt.Errorf("failed to register BiChat applet: %w", err)
	}

	controllersToRegister := []application.Controller{}

	// Register controllers if config is available
	if m.config != nil {
		// Sync analytics views to database (fail-fast on startup)
		if m.config.ViewManager != nil {
			if err := m.config.ViewManager.Sync(context.Background(), app.DB()); err != nil {
				return fmt.Errorf("failed to sync analytics views: %w", err)
			}
		}

		// Build services (fail fast - no try/continue)
		if err := m.config.BuildServices(); err != nil {
			return fmt.Errorf("failed to build BiChat services: %w", err)
		}

		chatService := m.config.ChatService()
		agentService := m.config.AgentService()
		attachmentService := m.config.AttachmentService()
		artifactService := m.config.ArtifactService()
		app.RegisterServices(chatService, agentService, attachmentService, artifactService)
		app.QuickLinks().Add(spotlight.NewQuickLink(BiChatLink.Icon, BiChatLink.Name, BiChatLink.Href))

		// Create and register controllers.
		// Applet request/response APIs should go through applet RPC.
		// Streaming is exposed via StreamController.
		streamController := controllers.NewStreamController(
			app,
			chatService,
		)
		controllersToRegister = append(controllersToRegister, streamController)

		if m.config.Logger != nil {
			m.config.Logger.Info("Registered BiChat stream endpoint at /bi-chat/stream")
		}
	}

	app.RegisterControllers(controllersToRegister...)

	return nil
}

func (m *Module) Name() string {
	return "bichat"
}

// Shutdown gracefully shuts down the module, ensuring observability providers flush pending data.
// This method should be called during application shutdown (e.g., via defer or shutdown hooks).
//
// It performs the following operations:
// 1. Calls Flush() on all providers to send pending observations
// 2. Calls Shutdown() on all providers to release resources
// 3. Uses a timeout context to prevent hanging during shutdown
//
// Usage:
//
//	module := bichat.NewModuleWithConfig(cfg)
//	defer module.Shutdown(context.Background())
func (m *Module) Shutdown(ctx context.Context) error {
	if m.eventBridge == nil {
		return nil // No observability configured
	}

	// Create timeout context for shutdown (30 seconds)
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return m.eventBridge.Shutdown(shutdownCtx)
}
