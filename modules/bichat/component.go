package bichat

import (
	"context"
	"embed"
	"errors"
	"os"
	"strings"
	"time"

	bichatperm "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
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

// ErrBiChatDisabled is returned by providers when BiChat is not configured.
// The sentinel is swallowed by capability-gated contribute closures so that
// cmd/server and cmd/worker continue to boot without the OPENAI_API_KEY set.
var ErrBiChatDisabled = errors.New("bichat: not configured (OPENAI_API_KEY not set)")

// Option configures the BiChat composition component before it is built.
//
// The default component reads OPENAI_API_KEY from the environment, builds a
// LangFuse observability provider if LANGFUSE_PUBLIC_KEY is set, and uses
// the global SDK configuration. Options let downstream consumers (the eai
// `ali` module is the canonical example) inject their own knowledge-base
// searchers, prompt extensions, additional agent options, and to override
// the controller mount path or required permissions.
type Option func(*componentOptions)

type componentOptions struct {
	// extraConfigOptions are appended to the ones produced by buildModuleConfig.
	extraConfigOptions []ConfigOption
	// streamControllerOptions overrides the BiChat stream endpoint wiring.
	streamBasePath          string
	streamRequirePermission permission.Permission
	streamReadAllPermission permission.Permission
}

// WithExtraConfigOptions appends ConfigOption values to the BiChat module
// configuration. Use this to register custom KB searchers, prompt extensions,
// observability providers, etc., from a downstream component without forking
// the bichat package.
func WithExtraConfigOptions(opts ...ConfigOption) Option {
	return func(o *componentOptions) {
		for _, opt := range opts {
			if opt != nil {
				o.extraConfigOptions = append(o.extraConfigOptions, opt)
			}
		}
	}
}

// WithComponentStreamBasePath overrides the path the BiChat stream controller
// mounts at. The default is the BiChatLink href.
func WithComponentStreamBasePath(path string) Option {
	return func(o *componentOptions) {
		o.streamBasePath = path
	}
}

// WithComponentStreamRequirePermission overrides the permission required to
// access the BiChat stream endpoint. The default is bichatperm.BiChatAccess.
func WithComponentStreamRequirePermission(p permission.Permission) Option {
	return func(o *componentOptions) {
		o.streamRequirePermission = p
	}
}

// WithComponentStreamReadAllPermission sets an optional read-all permission
// for the BiChat stream endpoint.
func WithComponentStreamReadAllPermission(p permission.Permission) Option {
	return func(o *componentOptions) {
		o.streamReadAllPermission = p
	}
}

func NewComponent(opts ...Option) composition.Component {
	c := &component{}
	for _, opt := range opts {
		if opt != nil {
			opt(&c.options)
		}
	}
	return c
}

type component struct {
	options componentOptions
}

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "bichat"}
}

// bichatBundle is the lazily-built BiChat runtime graph. Held as a single
// provider so buildModuleConfig is invoked at most once per container.
type bichatBundle struct {
	config      *ModuleConfig
	services    *ServiceContainer
	eventBridge *observability.EventBridge
}

func (c *component) Build(builder *composition.Builder) error {
	buildCtx := builder.Context()

	composition.ContributeLocales(builder, func(*composition.Container) ([]*embed.FS, error) {
		return []*embed.FS{&LocaleFiles}, nil
	})

	// Gate: env var check is cheap and deterministic. When unset, BiChat
	// registers no providers and no hooks, and the component compiles to a
	// no-op. Downstream consumers get NOT_PROVIDED at compile time rather
	// than silent nil behavior at runtime.
	openAIKey := strings.TrimSpace(os.Getenv(openAIAPIKeyEnv))
	if openAIKey == "" {
		if logger := buildCtx.Logger(); logger != nil {
			logger.Info("OPENAI_API_KEY not set - BiChat module disabled")
		}
		return nil
	}

	// Single lazy provider backing the entire BiChat graph. Resolved once per
	// container instantiation; downstream providers read individual services
	// off the bundle.
	extraOpts := append([]ConfigOption(nil), c.options.extraConfigOptions...)
	composition.Provide[*bichatBundle](builder, func(*composition.Container) (*bichatBundle, error) {
		moduleConfig, servicesContainer, eventBridge, err := loadModule(buildCtx, extraOpts...)
		if err != nil {
			return nil, err
		}
		if moduleConfig == nil || servicesContainer == nil {
			return nil, ErrBiChatDisabled
		}
		return &bichatBundle{
			config:      moduleConfig,
			services:    servicesContainer,
			eventBridge: eventBridge,
		}, nil
	})

	composition.Provide[bichatservices.SessionCommands](builder, func(container *composition.Container) (bichatservices.SessionCommands, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.SessionCommands(), nil
	})
	composition.Provide[bichatservices.SessionQueries](builder, func(container *composition.Container) (bichatservices.SessionQueries, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.SessionQueries(), nil
	})
	composition.Provide[bichatservices.TurnCommands](builder, func(container *composition.Container) (bichatservices.TurnCommands, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.TurnCommands(), nil
	})
	composition.Provide[bichatservices.TurnQueries](builder, func(container *composition.Container) (bichatservices.TurnQueries, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.TurnQueries(), nil
	})
	composition.Provide[bichatservices.StreamCommands](builder, func(container *composition.Container) (bichatservices.StreamCommands, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.StreamCommands(), nil
	})
	composition.Provide[bichatservices.HITLCommands](builder, func(container *composition.Container) (bichatservices.HITLCommands, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.HITLCommands(), nil
	})
	composition.Provide[bichatservices.AgentService](builder, func(container *composition.Container) (bichatservices.AgentService, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.AgentService(), nil
	})
	composition.Provide[bichatservices.AttachmentService](builder, func(container *composition.Container) (bichatservices.AttachmentService, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.AttachmentService(), nil
	})
	composition.Provide[bichatservices.ArtifactService](builder, func(container *composition.Container) (bichatservices.ArtifactService, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.ArtifactService(), nil
	})
	composition.Provide[*services.StreamObservability](builder, func(container *composition.Container) (*services.StreamObservability, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return b.services.StreamObservability(), nil
	})

	composition.ContributeApplets(builder, func(container *composition.Container) ([]application.Applet, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return []application.Applet{NewBiChatApplet(b.config, b.services)}, nil
	})

	composition.ContributeNavItems(builder, func(*composition.Container) ([]types.NavigationItem, error) {
		return NavItems, nil
	})
	composition.ContributeQuickLinks(builder, func(*composition.Container) ([]*spotlight.QuickLink, error) {
		return []*spotlight.QuickLink{spotlight.NewQuickLink(BiChatLink.Name, BiChatLink.Href)}, nil
	})

	composition.ContributeSpotlightAgent(builder, func(container *composition.Container) (spotlight.Agent, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		// NewBIChatAgent gracefully handles a nil KBSearcher; the agent
		// short-circuits the kb.Search path and only ranks SDK-supplied hits.
		return spotlight.NewBIChatAgent(b.config.KBSearcher), nil
	})

	if buildCtx.HasCapability(composition.CapabilityWorker) {
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			b, err := composition.Resolve[*bichatBundle](container)
			if err != nil {
				return nil, err
			}
			runtime := &runtimeComponent{
				config:      b.config,
				container:   b.services,
				eventBridge: b.eventBridge,
				pool:        buildCtx.DB(),
			}
			return []composition.Hook{{
				Name: runtime.Name(),
				Start: func(ctx context.Context) (composition.StopFn, error) {
					if err := runtime.Start(ctx); err != nil {
						return nil, err
					}
					return runtime.Stop, nil
				},
			}}, nil
		})
	}

	if buildCtx.HasCapability(composition.CapabilityAPI) {
		basePathOverride := c.options.streamBasePath
		requirePermissionOverride := c.options.streamRequirePermission
		readAllPermissionOverride := c.options.streamReadAllPermission

		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
			app, err := composition.RequireApplication(container)
			if err != nil {
				return nil, err
			}
			b, err := composition.Resolve[*bichatBundle](container)
			if err != nil {
				return nil, err
			}

			streamRequirePermission := bichatperm.BiChatAccess
			if b.config.StreamRequireAccessPermission != nil {
				streamRequirePermission = b.config.StreamRequireAccessPermission
			}
			if requirePermissionOverride != nil {
				streamRequirePermission = requirePermissionOverride
			}

			streamOpts := []controllers.ControllerOption{
				controllers.WithRequireAccessPermission(streamRequirePermission),
			}
			if readAllPermissionOverride != nil {
				streamOpts = append(streamOpts, controllers.WithReadAllPermission(readAllPermissionOverride))
			} else if b.config.StreamReadAllPermission != nil {
				streamOpts = append(streamOpts, controllers.WithReadAllPermission(b.config.StreamReadAllPermission))
			}

			if basePathOverride != "" {
				streamOpts = append(streamOpts, controllers.WithBasePath(basePathOverride))
			}

			if b.config.Logger != nil {
				b.config.Logger.Info("Registered BiChat stream endpoint")
			}

			return []application.Controller{
				controllers.NewStreamController(
					app,
					b.services.StreamCommands(),
					b.services.SessionQueries(),
					b.services.AttachmentService(),
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
