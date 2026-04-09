package bichat

import (
	"context"
	"embed"
	"errors"
	"time"

	bichatperm "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
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

func NewComponent() composition.Component { return &component{} }

type component struct{}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "bichat"}
}

func (c *component) Build(builder *composition.Builder) error {
	ctx := builder.Context()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})

	moduleConfig, servicesContainer, eventBridge, err := loadModule(builder.Context())
	if err != nil {
		return err
	}
	if moduleConfig == nil || servicesContainer == nil {
		return nil
	}

	composition.ContributeApplets(builder, func(*composition.Container) ([]application.Applet, error) {
		return []application.Applet{NewBiChatApplet(moduleConfig, servicesContainer)}, nil
	})

	composition.Provide[bichatservices.SessionCommands](builder, servicesContainer.SessionCommands())
	composition.Provide[bichatservices.SessionQueries](builder, servicesContainer.SessionQueries())
	composition.Provide[bichatservices.TurnCommands](builder, servicesContainer.TurnCommands())
	composition.Provide[bichatservices.TurnQueries](builder, servicesContainer.TurnQueries())
	composition.Provide[bichatservices.StreamCommands](builder, servicesContainer.StreamCommands())
	composition.Provide[bichatservices.HITLCommands](builder, servicesContainer.HITLCommands())
	composition.Provide[bichatservices.AgentService](builder, servicesContainer.AgentService())
	composition.Provide[bichatservices.AttachmentService](builder, servicesContainer.AttachmentService())
	composition.Provide[bichatservices.ArtifactService](builder, servicesContainer.ArtifactService())
	composition.Provide[*services.StreamObservability](builder, servicesContainer.StreamObservability())

	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})
	composition.ContributeQuickLinks(builder, func(*composition.Container) ([]*spotlight.QuickLink, error) {
		return []*spotlight.QuickLink{spotlight.NewQuickLink(BiChatLink.Name, BiChatLink.Href)}, nil
	})

	if moduleConfig.KBSearcher != nil {
		agent := spotlight.NewBIChatAgent(moduleConfig.KBSearcher)
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			return []composition.Hook{{
				Name: "bichat-spotlight-agent",
				Start: func(context.Context, *composition.Container) error {
					app.Spotlight().SetAgent(agent)
					return nil
				},
			}}, nil
		})
	}
	if builder.Context().HasCapability(composition.CapabilityWorker) {
		composition.ContributeHooks(builder, func(*composition.Container) ([]composition.Hook, error) {
			runtime := &runtimeComponent{
				config:      moduleConfig,
				container:   servicesContainer,
				eventBridge: eventBridge,
				pool:        ctx.DB(),
			}
			return []composition.Hook{{
				Name: runtime.Name(),
				Start: func(ctx context.Context, _ *composition.Container) error {
					return runtime.Start(ctx)
				},
				Stop: func(ctx context.Context, _ *composition.Container) error {
					return runtime.Stop(ctx)
				},
			}}, nil
		})
	}

	if builder.Context().HasCapability(composition.CapabilityAPI) {
		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			streamRequirePermission := bichatperm.BiChatAccess
			if moduleConfig.StreamRequireAccessPermission != nil {
				streamRequirePermission = moduleConfig.StreamRequireAccessPermission
			}

			streamOpts := []controllers.ControllerOption{
				controllers.WithRequireAccessPermission(streamRequirePermission),
			}
			if moduleConfig.StreamReadAllPermission != nil {
				streamOpts = append(streamOpts, controllers.WithReadAllPermission(moduleConfig.StreamReadAllPermission))
			}

			if moduleConfig.Logger != nil {
				moduleConfig.Logger.Info("Registered BiChat stream endpoint at /bi-chat/stream")
			}

			return []application.Controller{
				controllers.NewStreamController(
					app,
					servicesContainer.StreamCommands(),
					servicesContainer.SessionQueries(),
					servicesContainer.AttachmentService(),
					streamOpts...,
				),
			}, nil
		})
	}

	return nil
}

type runtimeComponent struct {
	config            *ModuleConfig
	container         *ServiceContainer
	eventBridge       *observability.EventBridge
	pool              *pgxpool.Pool
	titleWorker       *services.TitleJobWorker
	titleWorkerCancel context.CancelFunc
	titleWorkerDone   chan struct{}
}

func (c *runtimeComponent) shutdown(ctx context.Context) error {
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

func (c *runtimeComponent) Name() string {
	return "bichat-runtime"
}

func (c *runtimeComponent) Start(ctx context.Context) error {
	const op serrors.Op = "bichat.runtimeComponent.Start"

	if c.config == nil || c.container == nil {
		return nil
	}
	if c.config.ViewManager != nil {
		if err := c.config.ViewManager.Sync(ctx, c.pool); err != nil {
			return serrors.E(op, err, "failed to sync analytics views")
		}
	}
	if c.titleWorker != nil {
		return nil
	}
	worker, err := c.container.NewTitleJobWorker(c.pool)
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
	c.titleWorker = worker
	c.titleWorkerCancel = workerCancel
	c.titleWorkerDone = make(chan struct{})
	go func() {
		defer close(c.titleWorkerDone)
		if startErr := worker.Start(workerCtx); startErr != nil && c.config.Logger != nil {
			c.config.Logger.WithError(startErr).Warn("bichat title job worker stopped with error")
		}
	}()

	return nil
}

func (c *runtimeComponent) Stop(ctx context.Context) error {
	return c.shutdown(ctx)
}
