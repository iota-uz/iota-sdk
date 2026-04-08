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
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5/pgxpool"
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
	container              *ServiceContainer
	observabilityProviders []observability.Provider
	eventBridge            *observability.EventBridge
	titleWorker            *services.TitleJobWorker
	titleWorkerCancel      context.CancelFunc
	titleWorkerDone        chan struct{}
}

func (m *Module) RegisterWiring(app application.Application) error {
	app.RegisterLocaleFiles(&LocaleFiles)

	if m.config != nil {
		container, err := m.config.BuildServices()
		if err != nil {
			return fmt.Errorf("failed to build BiChat services: %w", err)
		}
		m.container = container

		sessionCommands := container.SessionCommands()
		sessionQueries := container.SessionQueries()
		turnCommands := container.TurnCommands()
		turnQueries := container.TurnQueries()
		streamCommands := container.StreamCommands()
		hitlCommands := container.HITLCommands()
		agentService := container.AgentService()
		attachmentService := container.AttachmentService()
		artifactService := container.ArtifactService()
		streamObservability := container.StreamObservability()
		app.RegisterServices(
			sessionCommands,
			sessionQueries,
			turnCommands,
			turnQueries,
			streamCommands,
			hitlCommands,
			agentService,
			attachmentService,
			artifactService,
			streamObservability,
		)

		app.QuickLinks().Add(spotlight.NewQuickLink(BiChatLink.Name, BiChatLink.Href))
		if m.config.KBSearcher != nil {
			app.Spotlight().SetAgent(spotlight.NewBIChatAgent(m.config.KBSearcher))
		}
		app.RegisterRuntime(application.RuntimeRegistration{
			Component: &runtimeComponent{module: m, pool: app.DB()},
			Tags: []application.RuntimeTag{
				application.RuntimeTagWorker,
			},
		})
	}

	bichatApplet := NewBiChatApplet(m.config, m.container)
	if err := app.RegisterApplet(bichatApplet); err != nil {
		return fmt.Errorf("failed to register BiChat applet: %w", err)
	}

	return nil
}

func (m *Module) RegisterTransports(app application.Application) error {
	if m.config == nil || m.container == nil {
		return nil
	}

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
	app.RegisterControllers(
		controllers.NewStreamController(
			app,
			m.container.StreamCommands(),
			m.container.SessionQueries(),
			m.container.AttachmentService(),
			streamOpts...,
		),
	)
	if m.config.Logger != nil {
		m.config.Logger.Info("Registered BiChat stream endpoint at /bi-chat/stream")
	}
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

	if m.container != nil {
		if err := m.container.CloseTitleQueue(); err != nil {
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

type runtimeComponent struct {
	module *Module
	pool   *pgxpool.Pool
}

func (c *runtimeComponent) Name() string {
	return "bichat-runtime"
}

func (c *runtimeComponent) Start(ctx context.Context) error {
	const op serrors.Op = "bichat.runtimeComponent.Start"

	if c.module == nil || c.module.config == nil || c.module.container == nil {
		return nil
	}
	if c.module.config.ViewManager != nil {
		if err := c.module.config.ViewManager.Sync(ctx, c.pool); err != nil {
			return serrors.E(op, err, "failed to sync analytics views")
		}
	}
	if c.module.titleWorker != nil {
		return nil
	}
	worker, err := c.module.container.NewTitleJobWorker(c.pool)
	if err != nil {
		if errors.Is(err, ErrTitleJobWorkerDisabled) {
			return nil
		}
		return serrors.E(op, err, "failed to create title job worker")
	}
	if worker == nil {
		return nil
	}
	workerCtx, workerCancel := context.WithCancel(context.Background())
	c.module.titleWorker = worker
	c.module.titleWorkerCancel = workerCancel
	c.module.titleWorkerDone = make(chan struct{})
	go func() {
		defer close(c.module.titleWorkerDone)
		if startErr := worker.Start(workerCtx); startErr != nil && c.module.config.Logger != nil {
			c.module.config.Logger.WithError(startErr).Warn("bichat title job worker stopped with error")
		}
	}()
	return nil
}

func (c *runtimeComponent) Stop(ctx context.Context) error {
	if c.module == nil {
		return nil
	}
	return c.module.Shutdown(ctx)
}
