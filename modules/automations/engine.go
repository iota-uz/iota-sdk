package automations

import (
	"context"
	"log/slog"
	"net/url"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine/ownernotice"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/wiring"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// WithEngineConfig adjusts the configuration of the automation engine the module builds: limits,
// retention, a subject loader, a task bridge for human nodes, links. DB, Workflows, Users, Catalog and
// Credentials are set by the module before the function runs.
func WithEngineConfig(tune func(*engine.Config)) Option {
	return func(c *component) {
		if tune != nil {
			c.engineTune = append(c.engineTune, tune)
		}
	}
}

// WithSubjectLink gives the page of a subject entity's record (its key → a path or an absolute URL): the
// run view's «Open <subject>» and a run context's subject.url. core.user links to /users/<id> unless
// replaced.
func WithSubjectLink(entity string, resolve engine.SubjectURLResolver) Option {
	return func(c *component) {
		if entity == "" || resolve == nil {
			return
		}
		if c.subjectLinks == nil {
			c.subjectLinks = map[string]engine.SubjectURLResolver{}
		}
		c.subjectLinks[entity] = resolve
	}
}

// newLinks is the link registry of runs and subjects under origin (an empty origin keeps links relative).
func (c *component) newLinks(origin string) (*engine.LinkRegistry, error) {
	path := c.automationsPath
	if path == "" {
		path = "/automations"
	}
	links := engine.NewLinks(origin, strings.TrimRight(path, "/")+"/%s/runs/%s")
	resolvers := map[string]engine.SubjectURLResolver{
		"core.user": func(_ context.Context, _ uuid.UUID, key string) (string, error) {
			return "/users/" + url.PathEscape(key), nil
		},
	}
	for entity, r := range c.subjectLinks {
		resolvers[entity] = r
	}
	for entity, r := range resolvers {
		if err := links.Register(entity, r); err != nil {
			return nil, err
		}
	}
	return links, nil
}

// WithoutEngine leaves the automation engine out: no runs execute in this process and the automations
// pages are not mounted (the integrations page still is).
func WithoutEngine() Option {
	return func(c *component) { c.noEngine = true }
}

// WithoutBackground keeps the engine's maintenance loop and the inbound poller from running in this
// process (another process runs them).
func WithoutBackground() Option {
	return func(c *component) { c.noBackground = true }
}

// engineHolder is the composition's engine; rt is nil when the composition cannot run one (the module's
// engine is turned off, or no extension core provides the workflow engine and the subject loader).
type engineHolder struct{ rt *engine.Runtime }

// runtime is the composition's engine, nil when it cannot run one.
func (c *component) runtime(container *composition.Container) (*engine.Runtime, error) {
	if c.noEngine || !composition.Has[*engineHolder](container) {
		return nil, nil //nolint:nilnil // no engine is an outcome: the pages are not mounted
	}
	h, err := composition.Resolve[*engineHolder](container)
	if err != nil {
		return nil, err
	}
	return h.rt, nil
}

// buildEngine provides the engine over the extension core's workflow engine and registers what it
// contributes to the core: the native contracts automation.run and automation.call, the outbox
// consumer, the schedules, and the maintenance loop and inbound poller as hooks.
func (c *component) buildEngine(b *composition.Builder) {
	if c.noEngine {
		return
	}
	composition.Provide[*engineHolder](b, func(container *composition.Container) (*engineHolder, error) {
		if !composition.Has[*workflow.Engine](container) || !composition.Has[host.SubjectLoader](container) || !composition.Has[*pgxpool.Pool](container) {
			return &engineHolder{}, nil
		}
		db, err := composition.Resolve[*pgxpool.Pool](container)
		if err != nil {
			return nil, err
		}
		wf, err := composition.Resolve[*workflow.Engine](container)
		if err != nil {
			return nil, err
		}
		users, err := composition.Resolve[host.SubjectLoader](container)
		if err != nil {
			return nil, err
		}
		reg, err := composition.Resolve[catalog.Registry](container)
		if err != nil {
			return nil, err
		}
		// The store's usage index is the engine's: the credentials are resolved when a call first opens one.
		cfg := engine.Config{DB: db, Workflows: wf, Users: users, Catalog: reg, Credentials: lazyCredentials{container: container}, Logger: slog.Default()}
		if c.links != nil {
			cfg.Links = c.links
		}
		for _, tune := range c.engineTune {
			tune(&cfg)
		}
		rt, err := engine.NewRuntime(cfg)
		if err != nil {
			return nil, err
		}
		return &engineHolder{rt: rt}, nil
	})
	wiring.ContributeNativeWorkflows(b, func(container *composition.Container) (map[string]workflow.NativeContract, error) {
		rt, err := c.runtime(container)
		if err != nil || rt == nil {
			return nil, err
		}
		return rt.Natives(), nil
	})
	wiring.ContributeConsumers(b, func(container *composition.Container) ([]events.Consumer, error) {
		rt, err := c.runtime(container)
		if err != nil || rt == nil {
			return nil, err
		}
		return []events.Consumer{rt.Consumer()}, nil
	})
	wiring.ContributeSchedules(b, func(container *composition.Container) (events.ScheduleSource, error) {
		rt, err := c.runtime(container)
		if err != nil || rt == nil {
			return nil, err
		}
		return rt.Schedules(), nil
	})
	// The owner of a graph hears of its failed runs.
	ownernotice.Contribute(b)

	bc := b.Context()
	if c.noBackground || (!bc.HasCapability(composition.CapabilityAPI) && !bc.HasCapability(composition.CapabilityWorker)) {
		return
	}
	composition.ContributeHooks(b, func(container *composition.Container) ([]composition.Hook, error) {
		rt, err := c.runtime(container)
		if err != nil || rt == nil {
			return nil, err
		}
		store, err := composition.Resolve[connectors.ConnectionStore](container)
		if err != nil {
			return nil, err
		}
		dir, err := composition.Resolve[connectors.Directory](container)
		if err != nil {
			return nil, err
		}
		poller := rt.NewPoller(engine.PollerConfig{Connections: store, Connectors: dir})
		return []composition.Hook{{Name: "automations.engine", Start: func(ctx context.Context) (composition.StopFn, error) {
			ctx, cancel := context.WithCancel(composition.LifecycleContext(ctx))
			done := make(chan struct{}, 2)
			go func() { rt.Serve(ctx); done <- struct{}{} }()
			go func() { poller.Serve(ctx); done <- struct{}{} }()
			return func(context.Context) error {
				cancel()
				<-done
				<-done
				return nil
			}, nil
		}}}, nil
	})
}

// registerEngineCatalog registers what every graph of the engine may use: the engine's own events
// (schedule, manual start, run finished), the application's native events, and the built-in and flow
// node types, whose wait node waits for any of those events. A native event whose payload the catalog
// cannot express is left out with a warning instead of failing the application.
func registerEngineCatalog(reg catalog.Registry, natives *native.Set, logger *slog.Logger) error {
	if err := reg.RegisterTriggers(engine.Triggers()); err != nil {
		return err
	}
	waitEvents := []string{engine.EventRunFinished}
	if natives != nil {
		defs := natives.Events()
		sort.Slice(defs, func(i, j int) bool { return defs[i].ID() < defs[j].ID() })
		for _, def := range defs {
			if err := reg.RegisterTriggers(catalog.FromNativeEvents(oneEvent{set: natives, def: def})); err != nil {
				logger.Warn("automations: a native event is left out of the automation catalog", "event", def.ID(), "error", err)
				continue
			}
			waitEvents = append(waitEvents, def.ID())
		}
	}
	for _, spec := range append(graph.BuiltinNodes(nil, waitEvents), graph.FlowNodes(nil)...) {
		if err := reg.RegisterNode(builtinNode{spec: spec}); err != nil {
			return err
		}
	}
	return nil
}

// oneEvent is one native event of a set (with the set's entities, for the event's read permission).
type oneEvent struct {
	set *native.Set
	def native.EventDefinition
}

func (o oneEvent) Events() []native.EventDefinition { return []native.EventDefinition{o.def} }

func (o oneEvent) Entities() []data.NativeEntity { return o.set.Entities() }

// builtinNode is a built-in node type of package graph.
type builtinNode struct{ spec graph.NodeSpec }

func (n builtinNode) Spec() graph.NodeSpec            { return n.spec }
func (builtinNode) Requires() []catalog.ConnectorKind { return nil }

// lazyCredentials resolves the connection store when a call first opens a connection.
type lazyCredentials struct{ container *composition.Container }

func (l lazyCredentials) Open(ctx context.Context, tenantID uuid.UUID, ref string) (catalog.ConnectionInfo, connectors.Credentials, error) {
	src, err := composition.Resolve[connectors.CredentialSource](l.container)
	if err != nil {
		return catalog.ConnectionInfo{}, nil, err
	}
	return src.Open(ctx, tenantID, ref)
}
