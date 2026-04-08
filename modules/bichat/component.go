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
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

//go:embed infrastructure/persistence/schema/bichat-schema.sql
var MigrationFiles embed.FS

func NewComponent(cfg *ModuleConfig) composition.Component {
	component := &component{
		config: cfg,
	}
	if cfg != nil && len(cfg.ObservabilityProviders) > 0 {
		component.observabilityProviders = cfg.ObservabilityProviders
		component.eventBridge = observability.NewEventBridge(cfg.EventBus, cfg.ObservabilityProviders)
	}
	return component
}

type component struct {
	config                 *ModuleConfig
	container              *ServiceContainer
	observabilityProviders []observability.Provider
	eventBridge            *observability.EventBridge
	titleWorker            *services.TitleJobWorker
	titleWorkerCancel      context.CancelFunc
	titleWorkerDone        chan struct{}
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "bichat"}
}

func (c *component) Build(builder *composition.Builder) error {
	app := builder.Context().App

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})
	composition.ContributeApplets(builder, func(*composition.Container) ([]application.Applet, error) {
		return []application.Applet{NewBiChatApplet(c.config, c.container)}, nil
	})

	if c.config == nil {
		return nil
	}

	container, err := c.config.BuildServices()
	if err != nil {
		return fmt.Errorf("failed to build BiChat services: %w", err)
	}
	c.container = container

	app.RegisterServices(
		container.SessionCommands(),
		container.SessionQueries(),
		container.TurnCommands(),
		container.TurnQueries(),
		container.StreamCommands(),
		container.HITLCommands(),
		container.AgentService(),
		container.AttachmentService(),
		container.ArtifactService(),
		container.StreamObservability(),
	)

	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})

	app.QuickLinks().Add(spotlight.NewQuickLink(BiChatLink.Name, BiChatLink.Href))
	if c.config.KBSearcher != nil {
		app.Spotlight().SetAgent(spotlight.NewBIChatAgent(c.config.KBSearcher))
	}
	app.RegisterRuntime(application.RuntimeRegistration{
		Component: &runtimeComponent{component: c, pool: app.DB()},
		Tags: []application.RuntimeTag{
			application.RuntimeTagWorker,
		},
	})

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(*composition.Container) ([]application.Controller, error) {
			streamRequirePermission := bichatperm.BiChatAccess
			if c.config.StreamRequireAccessPermission != nil {
				streamRequirePermission = c.config.StreamRequireAccessPermission
			}

			streamOpts := []controllers.ControllerOption{
				controllers.WithRequireAccessPermission(streamRequirePermission),
			}
			if c.config.StreamReadAllPermission != nil {
				streamOpts = append(streamOpts, controllers.WithReadAllPermission(c.config.StreamReadAllPermission))
			}

			if c.config.Logger != nil {
				c.config.Logger.Info("Registered BiChat stream endpoint at /bi-chat/stream")
			}

			return []application.Controller{
				controllers.NewStreamController(
					app,
					container.StreamCommands(),
					container.SessionQueries(),
					container.AttachmentService(),
					streamOpts...,
				),
			}, nil
		})
	}

	return nil
}

func (c *component) Shutdown(ctx context.Context) error {
	var shutdownErr error

	if c.titleWorkerCancel != nil {
		c.titleWorkerCancel()
		c.titleWorkerCancel = nil
	}
	if c.titleWorkerDone != nil {
		select {
		case <-c.titleWorkerDone:
		case <-ctx.Done():
			shutdownErr = errors.Join(shutdownErr, ctx.Err())
		}
		c.titleWorkerDone = nil
	}
	c.titleWorker = nil

	if c.container != nil {
		if err := c.container.CloseTitleQueue(); err != nil {
			shutdownErr = errors.Join(shutdownErr, err)
		}
	}

	if c.eventBridge == nil {
		return shutdownErr
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := c.eventBridge.Shutdown(shutdownCtx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}

	return shutdownErr
}

type runtimeComponent struct {
	component *component
	pool      *pgxpool.Pool
}

func (c *runtimeComponent) Name() string {
	return "bichat-runtime"
}

func (c *runtimeComponent) Start(ctx context.Context) error {
	const op serrors.Op = "bichat.runtimeComponent.Start"

	if c.component == nil || c.component.config == nil || c.component.container == nil {
		return nil
	}
	if c.component.config.ViewManager != nil {
		if err := c.component.config.ViewManager.Sync(ctx, c.pool); err != nil {
			return serrors.E(op, err, "failed to sync analytics views")
		}
	}
	if c.component.titleWorker != nil {
		return nil
	}
	worker, err := c.component.container.NewTitleJobWorker(c.pool)
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
	c.component.titleWorker = worker
	c.component.titleWorkerCancel = workerCancel
	c.component.titleWorkerDone = make(chan struct{})
	go func() {
		defer close(c.component.titleWorkerDone)
		if startErr := worker.Start(workerCtx); startErr != nil && c.component.config.Logger != nil {
			c.component.config.Logger.WithError(startErr).Warn("bichat title job worker stopped with error")
		}
	}()

	return nil
}

func (c *runtimeComponent) Stop(ctx context.Context) error {
	if c.component == nil {
		return nil
	}
	return c.component.Shutdown(ctx)
}
