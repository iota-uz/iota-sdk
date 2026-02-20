package bichat

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"

	bichatperm "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks"
	"github.com/iota-uz/iota-sdk/pkg/bichat/hooks/handlers"
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
	artifactUnsubscribe    func()
	titleWorker            *services.TitleJobWorker
	titleWorkerCancel      context.CancelFunc
	titleWorkerDone        chan struct{}
}

func (m *Module) Register(app application.Application) error {
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

		if m.artifactUnsubscribe == nil && m.config.EventBus != nil && m.config.ChatRepo != nil {
			artifactHandler := handlers.NewArtifactHandler(m.config.ChatRepo)
			m.artifactUnsubscribe = m.config.EventBus.Subscribe(
				artifactHandler,
				string(hooks.EventToolComplete),
			)
		}

		if m.titleWorker == nil {
			worker, err := m.config.NewTitleJobWorker(app.DB())
			if err != nil && !errors.Is(err, ErrTitleJobWorkerDisabled) {
				return fmt.Errorf("failed to create title job worker: %w", err)
			}
			if err == nil && worker != nil {
				workerCtx, workerCancel := context.WithCancel(context.Background())
				m.titleWorker = worker
				m.titleWorkerCancel = workerCancel
				m.titleWorkerDone = make(chan struct{})
				go func() {
					defer close(m.titleWorkerDone)
					if startErr := worker.Start(workerCtx); startErr != nil && m.config.Logger != nil {
						m.config.Logger.WithError(startErr).Warn("bichat title job worker stopped with error")
					}
				}()
			}
		}

		app.QuickLinks().Add(spotlight.NewQuickLink(BiChatLink.Icon, BiChatLink.Name, BiChatLink.Href))

		// Create and register controllers.
		// Applet request/response APIs should go through applet RPC.
		// Streaming is exposed via StreamController.
		streamRequirePermission := bichatperm.BiChatAccess
		if m.config.StreamRequireAccessPermission != nil {
			streamRequirePermission = m.config.StreamRequireAccessPermission
		}

		streamOpts := []controllers.ControllerOption{
			controllers.WithRequireAccessPermission(streamRequirePermission),
		}
		if m.config.StreamReadAllPermission != nil {
			streamOpts = append(
				streamOpts,
				controllers.WithReadAllPermission(m.config.StreamReadAllPermission),
			)
		}
		streamController := controllers.NewStreamController(
			app,
			chatService,
			attachmentService,
			streamOpts...,
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
	var shutdownErr error

	if m.artifactUnsubscribe != nil {
		m.artifactUnsubscribe()
		m.artifactUnsubscribe = nil
	}

	if m.titleWorkerCancel != nil {
		m.titleWorkerCancel()
		m.titleWorkerCancel = nil
	}
	if m.titleWorkerDone != nil {
		select {
		case <-m.titleWorkerDone:
		case <-ctx.Done():
			shutdownErr = errors.Join(shutdownErr, ctx.Err())
		}
		m.titleWorkerDone = nil
	}
	m.titleWorker = nil

	if m.config != nil {
		if err := m.config.CloseTitleQueue(); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	if m.eventBridge == nil {
		return shutdownErr
	}

	// Create timeout context for shutdown (30 seconds)
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := m.eventBridge.Shutdown(shutdownCtx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	return shutdownErr
}
