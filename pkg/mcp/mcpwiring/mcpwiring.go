// Package mcpwiring composes the MCP operations of an iota-sdk application: the tools modules
// contribute to the application's registry (pkg/mcp) and the guarded changes behind prepare_changes /
// apply_changes / operation_get (pkg/mcp/changes) with the change kinds modules contribute.
//
//	registry := mcp.NewRegistry()
//	components = append(components, mcpwiring.NewComponent(mcpwiring.Options{
//	    Registry: registry, Authorizer: authz, Auditor: auditor,
//	    ApprovalURL: func(id string) string { return origin + "/mcp/approvals/" + id },
//	}))
//	// after composition.Start: mcp.NewEngine(registry, authz, mcp.WithAuditor(auditor)) and the gateway
//
// Modules contribute with ContributeTools and ContributeKinds (the automations module contributes
// the automation tools and the kinds of entity "automation", pkg/automation/mcpops). The component
// registers them when the application starts, like the extension runtime's MCP tools, so the gateway
// is built over the registry after composition.Start.
//
// The component is on when its required configuration is present: Registry, Authorizer, Auditor and
// ApprovalURL. Without a Registry it is off and composes nothing; a Registry without the others is a
// configuration error. In a process with the API or the worker capability it calls
// changes.Service.DispatchEffects every DispatchInterval: external effects of applied changes, and the
// audit events of applied changes a lost commit response or a stopped process left undelivered.
package mcpwiring

import (
	"context"
	"embed"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/iota-uz/iota-sdk/pkg/mcp/changes"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Defaults of the dispatch loop.
const (
	DefaultDispatchInterval = 5 * time.Second
	DefaultDispatchLimit    = 100
)

// Options are the Go values of the composition; there are no environment variables.
type Options struct {
	// Registry is the application's MCP registry; nil turns the component off.
	Registry *mcp.Registry
	// Authorizer authorizes the principal of a change (the one the application's mcp.Engine uses).
	Authorizer mcp.Authorizer
	// Auditor stores the audit events of applied changes; an event it does not confirm is kept and
	// delivered again after a backoff (changes.Config.Auditor).
	Auditor mcp.ConfirmingAuditor
	// ApprovalURL is the trusted page that shows a plan's diff to a person (changes.Config.ApprovalURL).
	ApprovalURL func(planID string) string
	// Store keeps plans, operations, effects and audit; default changes.NewPostgresStore over the
	// container's pool.
	Store changes.Store
	// Effects deliver the external effects of applied changes by kind.
	Effects map[string]changes.EffectHandler
	// Tune adjusts the changes configuration (TTLs, limits, the tool policy) before the service is built.
	Tune func(cfg *changes.Config)
	// DispatchInterval is how often DispatchEffects runs (default DefaultDispatchInterval).
	DispatchInterval time.Duration
	// DispatchLimit bounds the effects of one dispatch (default DefaultDispatchLimit).
	DispatchLimit int
	// DisableBackground keeps the dispatch loop from running in this process (another one runs it).
	DisableBackground bool
}

// Enabled reports whether the options turn the component on.
func (o Options) Enabled() bool { return o.Registry != nil }

// Validate checks that the required configuration is complete.
func (o Options) Validate() error {
	if !o.Enabled() {
		return nil
	}
	var missing []string
	if o.Authorizer == nil {
		missing = append(missing, "Authorizer")
	}
	if o.Auditor == nil {
		missing = append(missing, "Auditor")
	}
	if o.ApprovalURL == nil {
		missing = append(missing, "ApprovalURL")
	}
	if len(missing) > 0 {
		return serrors.New(serrors.Invalid, "mcpwiring: a Registry also needs "+strings.Join(missing, ", "))
	}
	return nil
}

// ContributeTools adds tools to the application's registry; the factory runs once when the
// application is composed and may resolve any provider. Its tools are registered when it starts.
func ContributeTools(b *composition.Builder, factory func(*composition.Container) ([]mcp.Tool, error)) {
	if factory == nil {
		return
	}
	composition.Contribute(b, func(c *composition.Container) ([]toolSet, error) {
		tools, err := factory(c)
		if err != nil || len(tools) == 0 {
			return nil, err
		}
		return []toolSet{tools}, nil
	})
}

// toolSet and kindSet are the contributions of this package: private types, so values other packages
// contribute as bare []mcp.Tool or []changes.Kind never reach the application's registry.
type (
	toolSet []mcp.Tool
	kindSet []changes.Kind
)

// ContributeKinds adds change kinds to the changes service (changes.Config.Kinds).
func ContributeKinds(b *composition.Builder, factory func(*composition.Container) ([]changes.Kind, error)) {
	if factory == nil {
		return
	}
	composition.Contribute(b, func(c *composition.Container) ([]kindSet, error) {
		kinds, err := factory(c)
		if err != nil || len(kinds) == 0 {
			return nil, err
		}
		return []kindSet{kinds}, nil
	})
}

// Changes is the composition's changes service, which exists once the application started (an approval
// page reads plans and approves them through it).
type Changes struct {
	mu  sync.RWMutex
	svc *changes.Service
}

// Service is the changes service; false before the application started or with the component off.
func (c *Changes) Service() (*changes.Service, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.svc, c.svc != nil
}

func (c *Changes) set(s *changes.Service) {
	c.mu.Lock()
	c.svc = s
	c.mu.Unlock()
}

// NewComponent is the MCP composition.
func NewComponent(opts Options) composition.Component { return &component{opts: opts} }

type component struct{ opts Options }

func (c *component) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "mcp.wiring"}
}

func (c *component) LocaleFS() []*embed.FS { return nil }

func (c *component) Build(b *composition.Builder) error {
	const op = serrors.Op("mcpwiring.Build")
	holder := &Changes{}
	composition.Provide[*Changes](b, func() *Changes { return holder })
	if !c.opts.Enabled() {
		if logger := b.Context().Logger(); logger != nil {
			logger.Info("mcp operations are off: no MCP registry (mcpwiring.Options.Registry)")
		}
		return nil
	}
	if err := c.opts.Validate(); err != nil {
		return serrors.Wrap(op, err)
	}
	bc := b.Context()
	var log logger
	if l := bc.Logger(); l != nil {
		log = l
	}
	background := !c.opts.DisableBackground &&
		(bc.HasCapability(composition.CapabilityAPI) || bc.HasCapability(composition.CapabilityWorker))
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		store := c.opts.Store
		if store == nil {
			pool, err := composition.Resolve[*pgxpool.Pool](container)
			if err != nil {
				return nil, err
			}
			store = changes.NewPostgresStore(pool)
		}
		return []composition.Hook{{Name: "mcp.wiring", Start: func(ctx context.Context) (composition.StopFn, error) {
			svc, err := c.register(container, store)
			if err != nil {
				return nil, serrors.Wrap(op, err)
			}
			holder.set(svc)
			if !background {
				return nil, nil
			}
			return c.dispatch(composition.LifecycleContext(ctx), svc, log), nil
		}}}, nil
	})
	return nil
}

// register builds the changes service over the contributed kinds and registers the contributed tools
// and the changes tools in the registry.
func (c *component) register(container *composition.Container, store changes.Store) (*changes.Service, error) {
	toolSets, err := composition.Contributions[toolSet](container)
	if err != nil {
		return nil, err
	}
	kindSets, err := composition.Contributions[kindSet](container)
	if err != nil {
		return nil, err
	}
	var tools []mcp.Tool
	for _, set := range toolSets {
		tools = append(tools, set...)
	}
	var kinds []changes.Kind
	for _, set := range kindSets {
		kinds = append(kinds, set...)
	}
	cfg := changes.Config{
		Store: store, Authorizer: c.opts.Authorizer, Kinds: kinds, Effects: c.opts.Effects,
		ApprovalURL: c.opts.ApprovalURL, Auditor: c.opts.Auditor,
	}
	if c.opts.Tune != nil {
		c.opts.Tune(&cfg)
	}
	svc, err := changes.NewService(cfg)
	if err != nil {
		return nil, err
	}
	if err := c.opts.Registry.Register(append(tools, svc.Tools()...)...); err != nil {
		return nil, err
	}
	return svc, nil
}

// logger is what the dispatch loop logs with (the build context's logger).
type logger interface {
	Warnf(format string, args ...any)
}

// dispatch runs DispatchEffects every interval until the application stops.
func (c *component) dispatch(ctx context.Context, svc *changes.Service, log logger) composition.StopFn {
	interval := c.opts.DispatchInterval
	if interval <= 0 {
		interval = DefaultDispatchInterval
	}
	limit := c.opts.DispatchLimit
	if limit <= 0 {
		limit = DefaultDispatchLimit
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := svc.DispatchEffects(ctx, limit); err != nil && ctx.Err() == nil && log != nil {
					log.Warnf("mcp: dispatching the effects of applied changes failed: %v", err)
				}
			}
		}
	}()
	return func(context.Context) error {
		cancel()
		<-done
		return nil
	}
}
