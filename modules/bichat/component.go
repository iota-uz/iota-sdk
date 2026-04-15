package bichat

import (
	"context"
	"embed"
	"errors"
	"strings"
	"time"

	bichatagents "github.com/iota-uz/iota-sdk/modules/bichat/agents"
	bichatperm "github.com/iota-uz/iota-sdk/modules/bichat/permissions"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/controllers"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/bichatconfig"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/iota-uz/iota-sdk/pkg/spotlight"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed presentation/locales/*.json
var LocaleFiles embed.FS

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
	// These influence the module-level configuration (attachment storage,
	// observability providers, prompt extensions, etc.) and are applied
	// AFTER the default parent agent is constructed.
	extraConfigOptions []ConfigOption
	// extraAgentOptions are applied to the default parent agent at
	// construction time. Use these for anything that changes the agent's
	// behaviour (KB searchers, model name, learning stores, code
	// interpreter, custom agent registries). They are distinct from
	// extraConfigOptions because config options are applied after the
	// agent is already built — a caller passing WithKBSearcher via
	// WithExtraConfigOptions would have no effect on the default agent.
	extraAgentOptions []bichatagents.BIAgentOption
	// streamControllerOptions overrides the BiChat stream endpoint wiring.
	streamBasePath          string
	streamRequirePermission permission.Permission
	streamReadAllPermission permission.Permission
}

// WithExtraConfigOptions appends ConfigOption values to the BiChat module
// configuration. Use this to register attachment storage, observability
// providers, or other module-level knobs. These options are applied AFTER
// the default parent agent has been constructed — see WithExtraAgentOptions
// for anything that needs to influence the agent itself.
func WithExtraConfigOptions(opts ...ConfigOption) Option {
	return func(o *componentOptions) {
		for _, opt := range opts {
			if opt != nil {
				o.extraConfigOptions = append(o.extraConfigOptions, opt)
			}
		}
	}
}

// WithExtraAgentOptions appends BIAgentOption values that are passed to
// NewDefaultBIAgent before the default parent agent is constructed. Use
// this for anything that configures the agent: custom KB searchers, model
// name overrides, learning stores, sub-agent registries, code-interpreter
// toggles. Options passed here actually affect the running agent; options
// passed via WithExtraConfigOptions do not (because config is applied on
// the ModuleConfig, not the already-built agent).
func WithExtraAgentOptions(opts ...bichatagents.BIAgentOption) Option {
	return func(o *componentOptions) {
		for _, opt := range opts {
			if opt != nil {
				o.extraAgentOptions = append(o.extraAgentOptions, opt)
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

// provideBundleField registers a provider keyed by I whose factory pulls
// the field out of the already-resolved bichatBundle. Centralizes the
// bundle decomposition boilerplate that previously appeared eleven times.
func provideBundleField[I any](builder *composition.Builder, field func(*bichatBundle) I) {
	composition.Provide[I](builder, func(container *composition.Container) (I, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			var zero I
			return zero, err
		}
		return field(b), nil
	})
}

func (c *component) Build(builder *composition.Builder) error {
	buildCtx := builder.Context()

	// Gate: resolve bichatconfig to check if OpenAI is configured.
	// Prefer the new typed-config path (config.Source); fall back to legacy.
	var gateCfg *bichatconfig.Config
	if src := buildCtx.Source(); src != nil {
		reg := config.NewRegistry(src)
		if ptr, err := config.Register[bichatconfig.Config](reg, "bichat"); err == nil {
			gateCfg = ptr
		}
	} else if legacyCfg := buildCtx.Config(); legacyCfg != nil {
		v := bichatconfig.FromLegacy(legacyCfg)
		gateCfg = &v
	}
	if gateCfg == nil {
		v := bichatconfig.Config{}
		v.SetDefaults()
		gateCfg = &v
	}

	composition.AddLocales(builder, &LocaleFiles)
	if !gateCfg.OpenAI.IsConfigured() {
		if logger := buildCtx.Logger(); logger != nil {
			logger.Info("bichat.openai.apikey not set - BiChat module disabled")
		}
		return nil
	}
	_ = strings.TrimSpace // keep strings import used below
	composition.AddNavItems(builder, NavItems...)
	composition.AddQuickLinks(builder, spotlight.NewQuickLink(BiChatLink.Name, BiChatLink.Href))

	// Single lazy provider backing the entire BiChat graph. Resolved once per
	// container instantiation; downstream providers read individual services
	// off the bundle.
	extraConfigOpts := append([]ConfigOption(nil), c.options.extraConfigOptions...)
	extraAgentOpts := append([]bichatagents.BIAgentOption(nil), c.options.extraAgentOptions...)
	composition.Provide[*bichatBundle](builder, func(*composition.Container) (*bichatBundle, error) {
		moduleConfig, servicesContainer, eventBridge, err := loadModule(buildCtx, extraAgentOpts, extraConfigOpts)
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

	provideBundleField(builder, func(b *bichatBundle) bichatservices.SessionCommands { return b.services.SessionCommands() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.SessionQueries { return b.services.SessionQueries() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.TurnCommands { return b.services.TurnCommands() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.TurnQueries { return b.services.TurnQueries() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.StreamCommands { return b.services.StreamCommands() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.HITLCommands { return b.services.HITLCommands() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.AgentService { return b.services.AgentService() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.AttachmentService { return b.services.AttachmentService() })
	provideBundleField(builder, func(b *bichatBundle) bichatservices.ArtifactService { return b.services.ArtifactService() })
	provideBundleField(builder, func(b *bichatBundle) *services.StreamObservability { return b.services.StreamObservability() })

	composition.ContributeApplets(builder, func(container *composition.Container) ([]application.Applet, error) {
		b, err := composition.Resolve[*bichatBundle](container)
		if err != nil {
			return nil, err
		}
		return []application.Applet{NewBiChatApplet(b.config, b.services)}, nil
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
		pool := buildCtx.DB()
		composition.ContributeHooks(builder, func(container *composition.Container) ([]composition.Hook, error) {
			b, err := composition.Resolve[*bichatBundle](container)
			if err != nil {
				return nil, err
			}
			return []composition.Hook{{
				Name:  "bichat-runtime",
				Start: newRuntimeStart(b, pool),
			}}, nil
		})
	}

	if buildCtx.HasCapability(composition.CapabilityAPI) {
		basePathOverride := c.options.streamBasePath
		requirePermissionOverride := c.options.streamRequirePermission
		readAllPermissionOverride := c.options.streamReadAllPermission

		composition.ContributeControllers(builder, func(container *composition.Container) ([]application.Controller, error) {
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
				streamOpts = append(streamOpts, controllers.WithLogger(b.config.Logger))
			}

			return []application.Controller{
				controllers.NewStreamController(
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

// newRuntimeStart returns a Hook.Start that captures the title worker and
// event bridge in closures so that Stop has direct references without any
// shared struct state. Previously this was a runtimeComponent with
// cross-phase fields — the closure captures make that state local.
func newRuntimeStart(b *bichatBundle, pool *pgxpool.Pool) func(ctx context.Context) (composition.StopFn, error) {
	return func(ctx context.Context) (composition.StopFn, error) {
		const op serrors.Op = "bichat.runtimeStart"

		if b.config == nil || b.services == nil {
			return func(context.Context) error { return nil }, nil
		}

		if b.config.ViewManager != nil {
			if err := b.config.ViewManager.Sync(ctx, pool); err != nil {
				return nil, serrors.E(op, err, "failed to sync analytics views")
			}
		}

		var (
			titleWorkerCancel context.CancelFunc
			titleWorkerDone   chan struct{}
		)
		worker, err := b.services.NewTitleJobWorker(pool)
		if err != nil {
			if !errors.Is(err, ErrTitleJobWorkerDisabled) {
				return nil, serrors.E(op, err, "failed to create title job worker")
			}
		} else if worker != nil {
			// The worker loop runs under a dedicated context rooted at
			// context.Background() so that the Start ctx ending (for
			// example, the request context used to boot the engine) does
			// not silently stop it. Teardown is via titleWorkerCancel +
			// waiting on the done channel.
			workerCtx, workerCancel := context.WithCancel(context.Background())
			titleWorkerCancel = workerCancel
			titleWorkerDone = make(chan struct{})
			logger := b.config.Logger
			go func() {
				defer close(titleWorkerDone)
				if startErr := worker.Start(workerCtx); startErr != nil && logger != nil {
					logger.WithError(startErr).Warn("bichat title job worker stopped with error")
				}
			}()
		}

		// Stale-run reaper: periodically scans active runs and marks
		// wedged ones (heartbeat silent for > StaleAfter) as failed so
		// the sidebar can transition + clients see a terminal error
		// event. Returns (nil, nil) when Redis is unconfigured.
		var (
			reaperCancel context.CancelFunc
			reaperDone   chan struct{}
		)
		reaper, err := b.services.NewRunReaper()
		if err != nil {
			return nil, serrors.E(op, err, "failed to create run reaper")
		}
		if reaper != nil {
			reaperCtx, reaperCancelFn := context.WithCancel(context.Background())
			reaperCancel = reaperCancelFn
			reaperDone = make(chan struct{})
			logger := b.config.Logger
			go func() {
				defer close(reaperDone)
				if startErr := reaper.Start(reaperCtx); startErr != nil && logger != nil && !errors.Is(startErr, context.Canceled) {
					logger.WithError(startErr).Warn("bichat run reaper stopped with error")
				}
			}()
		}

		return func(stopCtx context.Context) error {
			var stopErr error
			if titleWorkerCancel != nil {
				titleWorkerCancel()
			}
			if reaperCancel != nil {
				reaperCancel()
			}
			if titleWorkerDone != nil {
				// Cancel has been called; the worker loop will observe it
				// and exit. We block on the done channel to join the
				// goroutine cleanly; if the caller's stopCtx expires first
				// we surface that error but let the goroutine finish on
				// its own — it will exit shortly because its context is
				// already cancelled.
				select {
				case <-titleWorkerDone:
				case <-stopCtx.Done():
					stopErr = errors.Join(stopErr, stopCtx.Err())
				}
			}
			if reaperDone != nil {
				select {
				case <-reaperDone:
				case <-stopCtx.Done():
					stopErr = errors.Join(stopErr, stopCtx.Err())
				}
			}

			if b.services != nil {
				if closeErr := b.services.CloseTitleQueue(); closeErr != nil {
					stopErr = errors.Join(stopErr, closeErr)
				}
				if closeErr := b.services.CloseSharedRedis(); closeErr != nil {
					stopErr = errors.Join(stopErr, closeErr)
				}
			}

			if b.eventBridge != nil {
				shutdownCtx, cancel := context.WithTimeout(stopCtx, 30*time.Second)
				defer cancel()
				if bridgeErr := b.eventBridge.Shutdown(shutdownCtx); bridgeErr != nil {
					stopErr = errors.Join(stopErr, bridgeErr)
				}
			}
			return stopErr
		}, nil
	}
}
