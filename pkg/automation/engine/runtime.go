// Package engine is the automation engine: graph versions, triggers and the durable execution of runs.
//
// A run is an instance of the native workflow contract automation.run (pkg/extruntime/workflow). The
// engine keeps no claim, lease or timer of its own: the workflow engine claims the instance and calls the
// runner, and every step of the run writes the automation.* tables inside a fenced NativeRun.Transaction
// together with its receipt. The graph is never replayed: the position of the tokens is the state, so an
// upgrade of the binary never repeats the effects of earlier steps. The journal only deduplicates inputs:
// inbox signals (matched events, call and task results) and timers.
//
// One outbox consumer (ConsumerID) fans every event out to the enabled graphs whose triggers match and to
// the open waits of running graphs. Schedules are a ScheduleSource of tenant schedules.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/triggerowner"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Config wires the engine.
type Config struct {
	DB *pgxpool.Pool
	// Workflows runs the instances of automation.run; the composition registers Runtime.Natives with it
	// (wiring.ContributeNativeWorkflows), the consumer (Runtime.Consumer, wiring.ContributeConsumers) and
	// the schedules (Runtime.Schedules, wiring.ContributeSchedules).
	Workflows *workflow.Engine
	// Owners reads trigger_owners (a pool); Users loads the current rights of a run-as user.
	Owners triggerowner.Querier
	Users  host.SubjectLoader
	// Catalog is catalog.Registry of #1330: ForTenant(..., AsAuthor) on save, the system view at run time,
	// Node(type) for execution.
	Catalog  catalog.Registry
	Subjects SubjectLoader
	// Calls runs call nodes; nil: the engine's own CallRunner (automation.call child instances).
	Calls CallExecutor
	Tasks TaskBridge // nil → human nodes fail validation; NewHumanBridge is the engine's
	// Credentials opens the connection a call runs with (connectors.Store); nil: only calls without a
	// connector run.
	Credentials connectors.CredentialSource
	// Connectors, when set, tells which fields of a connection's credentials are secret (the x-secret
	// fields of the connector's descriptor): only those are masked in the output and the errors of a call.
	// A connection's public fields (a provider's name, an endpoint) then reach the output as they are.
	// nil, or a connector that declares no secret field: every credential value is masked.
	Connectors connectors.Directory
	// CallLimits bound the calls of automation.call.
	CallLimits CallLimits
	// Usage records what calls spent (optional).
	Usage catalog.UsageRecorder
	// Links resolves subject.url and run.url of a run's context (optional).
	Links Links
	// Decisions caches the decisions of dry runs in cached mode; nil: the engine's table.
	Decisions DecisionCache
	// Behaviors are the behaviors graphs run with; the registry's own (default graph.DefaultBehaviors).
	Behaviors *graph.Behaviors
	// MatchHooks are the product's trigger matching hooks (graph.Options.MatchHooks).
	MatchHooks map[string]func(params map[string]any, event graph.Event) bool
	Limits     Limits
	Retention  Retention
	// MaintenanceInterval is how often Serve starts queued runs, cancels runs of disabled graphs and
	// prunes by Retention. Default 5s.
	MaintenanceInterval time.Duration
	Tracer              trace.TracerProvider
	Meter               metric.MeterProvider
	Logger              *slog.Logger
}

// Runtime is the engine: its service, the runner of automation.run, the event consumer and schedules.
type Runtime struct {
	cfg     Config
	service *service
	tracer  trace.Tracer
	metrics metrics

	calls *CallRunner
	// poke wakes Serve: a collect window opened or a new deadline came nearer.
	poke chan struct{}
	// passes counts the passes of Serve.
	passes atomic.Int64

	mu      sync.Mutex
	interps map[string]*interpEntry

	// testHook, set by tests (SetTestHook), runs at the end of every fenced transaction of a run and,
	// with tx == nil, at the labels SetTestHook names; its error rolls the transaction back.
	testHook func(ctx context.Context, tx pgx.Tx, label string) error
	// contextHook, set by tests, sees every context a live run's step is built with.
	contextHook func(event string, gctx graph.Context)
	// opsHook, set by tests, runs in Operations.Retry between reading the attempt and acting on it.
	opsHook func()
	// quietCallObserver, set by tests, keeps the call observer from signalling the parent (the run's
	// own reconciliation must give the wait its result).
	quietCallObserver bool
	// callResultHook, set by tests, runs in a call child before ("before") and after ("after") its
	// result transaction.
	callResultHook func(ctx context.Context, child uuid.UUID, stage string)
}

type interpEntry struct {
	in          *graph.Interpreter
	unavailable map[string]bool // node types the tenant's catalog lacks now
	platformOff map[string]bool // of those, the ones that lack the platform key the tenant's administrator withheld
	specs       map[string]graph.NodeSpec
	events      map[string]graph.EventSpec
	joinOf      map[string]string // a parallel → its join
	used        time.Time
}

const maxInterpreters = 512

// NewRuntime checks the configuration and builds the engine.
func NewRuntime(cfg Config) (*Runtime, error) {
	const op = serrors.Op("automation.engine.NewRuntime")
	if cfg.DB == nil || cfg.Workflows == nil || cfg.Catalog == nil || cfg.Users == nil {
		return nil, serrors.New(serrors.Invalid, "automation engine: DB, Workflows, Catalog and Users are required").WithOp(op)
	}
	if cfg.Owners == nil {
		cfg.Owners = cfg.DB
	}
	if cfg.Behaviors == nil {
		cfg.Behaviors = graph.DefaultBehaviors()
	}
	cfg.Limits = cfg.Limits.withDefaults()
	cfg.CallLimits = cfg.CallLimits.withDefaults()
	cfg.Retention = cfg.Retention.withDefaults()
	if cfg.MaintenanceInterval <= 0 {
		cfg.MaintenanceInterval = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Tracer == nil {
		cfg.Tracer = otel.GetTracerProvider()
	}
	if cfg.Meter == nil {
		cfg.Meter = otel.GetMeterProvider()
	}
	r := &Runtime{cfg: cfg, interps: map[string]*interpEntry{}}
	r.tracer = cfg.Tracer.Tracer("github.com/iota-uz/iota-sdk/pkg/automation/engine")
	m, err := newMetrics(cfg.Meter.Meter("github.com/iota-uz/iota-sdk/pkg/automation/engine"))
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	r.metrics = m
	r.service = &service{r: r}
	r.calls = &CallRunner{r: r}
	if r.cfg.Calls == nil {
		r.cfg.Calls = r.calls
	}
	if r.cfg.Decisions == nil {
		r.cfg.Decisions = &dbDecisionCache{db: cfg.DB}
	}
	r.poke = make(chan struct{}, 1)
	return r, nil
}

// Service is the API of graphs, versions and runs.
func (r *Runtime) Service() Service { return r.service }

// Natives are the native workflow contracts the engine implements: automation.run.
func (r *Runtime) Natives() map[string]workflow.NativeContract {
	call := r.calls.contract()
	call.HumanOnly = r.humanPage
	return map[string]workflow.NativeContract{
		WorkflowID: {
			HumanOnly:      r.humanPage,
			Runner:         &runner{r: r},
			Signals:        map[string]manifest.Type{InboxSignal: {Kind: manifest.KindJSON}, SubjectSignal: {Kind: manifest.KindJSON}},
			Input:          &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{{Name: "run_id", Type: manifest.Type{Kind: manifest.KindString}}}},
			Concurrency:    r.cfg.Limits.Concurrency,
			Completion:     workflow.CancelPending,
			StatusObserver: &observer{r: r},
			// A run's data is shown by GetRun, which hides the fields a viewer may not read.
			Private: true,
		},
		CallWorkflowID: call,
	}
}

// CallRunner is the engine's runner of call nodes (automation.call).
func (r *Runtime) CallRunner() *CallRunner { return r.calls }

// Serve runs the maintenance loop until ctx ends: queued conversation runs start when their subject is
// free, live runs of disabled or deleted graphs are cancelled, and finished runs are pruned.
//
// The loop wakes at its interval, or earlier only when a collect window is due: an idle engine runs no
// pass between its planned ones.
func (r *Runtime) Serve(ctx context.Context) {
	planned := time.Now()
	for {
		r.passes.Add(1)
		failed := false
		if err := r.Maintain(ctx); err != nil && ctx.Err() == nil {
			r.cfg.Logger.Warn("automation engine: maintenance failed", "error", err)
			failed = true
		}
		now := time.Now()
		for !planned.After(now) {
			planned = planned.Add(r.cfg.MaintenanceInterval)
		}
		for {
			wake := planned
			// After a failed pass, an overdue window (its flush failing) waits for the planned pass: it
			// never makes the loop spin.
			if at, ok, err := r.nextCollect(ctx); err == nil && ok && at.Before(wake) && (!failed || at.After(time.Now())) {
				wake = at
			}
			d := time.Until(wake)
			if d < 0 {
				d = 0
			}
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-r.poke:
				t.Stop()
				continue // a window opened: its deadline may come before the planned pass
			case <-t.C:
			}
			break
		}
	}
}

// Passes is how many maintenance passes Serve ran.
func (r *Runtime) Passes() int64 { return r.passes.Load() }

// Maintain runs one maintenance pass.
func (r *Runtime) Maintain(ctx context.Context) error {
	var errs []error
	if err := r.startQueued(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := r.cancelOrphans(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := r.wakeFreeAwaiting(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := r.releasePending(ctx); err != nil {
		errs = append(errs, err)
	}
	if err := r.flushCollects(ctx); err != nil {
		errs = append(errs, err)
	}
	if _, err := r.Prune(ctx); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("automation engine maintenance: %v", errs)
	}
	return nil
}

// interpreter is the interpreter of a version for the tenant's current catalog. A node type the tenant's
// catalog lacks now (its connection went away) is taken from the registry and marked unavailable: its
// step fails with connection_unavailable instead of the whole run.
func (r *Runtime) interpreter(ctx context.Context, tenant, automation uuid.UUID, version int, raw json.RawMessage) (*interpEntry, error) {
	view, err := r.cfg.Catalog.ForTenant(ctx, tenant)
	if err != nil {
		return nil, &host.InfraError{Err: err}
	}
	// The platform keys the tenant lacks decide the code of a node that lost its connection: they are part
	// of what the entry was built from.
	withheld := make([]string, 0, len(view.PlatformWithheld))
	for k, off := range view.PlatformWithheld {
		if off {
			withheld = append(withheld, string(k))
		}
	}
	sort.Strings(withheld)
	key := fmt.Sprintf("%s|%s|%d|%s|%s", tenant, automation, version, view.Catalog.Revision, strings.Join(withheld, ","))
	r.mu.Lock()
	if e, ok := r.interps[key]; ok {
		e.used = time.Now()
		r.mu.Unlock()
		return e, nil
	}
	r.mu.Unlock()
	g, errs := graph.Decode(raw)
	if len(errs) > 0 {
		return nil, fmt.Errorf("version %d of %s does not decode: %s", version, automation, errs[0].Error())
	}
	cat := view.Catalog
	unavailable := map[string]bool{}
	platformOff := map[string]bool{}
	for _, n := range g.Nodes {
		if _, ok := cat.Nodes[n.Type]; ok {
			continue
		}
		nt, ok := r.cfg.Catalog.Node(n.Type)
		if !ok {
			continue
		}
		if cat.Nodes == nil {
			cat.Nodes = map[string]graph.NodeSpec{}
		}
		spec := nt.Spec()
		spec.Type = n.Type
		if len(nt.Requires()) > 0 {
			spec.Params = permissiveParams{spec.Params}
		}
		cat.Nodes[n.Type] = spec
		unavailable[n.Type] = true
		for _, k := range nt.Requires() {
			if view.PlatformWithheld[k] {
				platformOff[n.Type] = true
			}
		}
	}
	in, err := graph.NewInterpreter(cat, g, r.graphOptions())
	if err != nil {
		return nil, err
	}
	e := &interpEntry{in: in, unavailable: unavailable, platformOff: platformOff, specs: cat.Nodes, events: cat.Events, joinOf: map[string]string{}, used: time.Now()}
	for _, n := range g.Nodes {
		if cat.Nodes[n.Type].Behavior != graph.BehaviorJoin {
			continue
		}
		if p, ok := in.View().Params(n.ID); ok {
			if of, _ := p["of"].(string); of != "" {
				if _, taken := e.joinOf[of]; !taken {
					e.joinOf[of] = n.ID
				}
			}
		}
	}
	r.mu.Lock()
	if len(r.interps) >= maxInterpreters {
		var oldest string
		var at time.Time
		for k, v := range r.interps {
			if oldest == "" || v.used.Before(at) {
				oldest, at = k, v.used
			}
		}
		delete(r.interps, oldest)
	}
	r.interps[key] = e
	r.mu.Unlock()
	return e, nil
}

// graphOptions are the interpreter's options for the engine's limits.
func (r *Runtime) graphOptions() graph.Options {
	l := r.cfg.Limits
	gl := graph.Limits{
		MaxValueBytes:         l.PayloadBytes,
		MaxForeachItems:       l.ForeachItems,
		MaxForeachConcurrency: l.ForeachParallel,
		MaxTokens:             l.MaxTokens,
	}
	gl.Expr.CostLimit = l.CELCost
	return graph.Options{Behaviors: r.cfg.Behaviors, Limits: gl, MatchHooks: r.cfg.MatchHooks}
}

// permissiveParams accepts a connector node's params as they were saved, connection included: the
// connection is checked when the node runs.
type permissiveParams struct{ inner graph.ParamsSpec }

func (p permissiveParams) JSONSchema() json.RawMessage { return p.inner.JSONSchema() }

func (p permissiveParams) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	rest := make(map[string]any, len(raw))
	for k, v := range raw {
		if k != catalog.ConnectionParam {
			rest[k] = v
		}
	}
	out, issues := p.inner.Parse(rest)
	if c, ok := raw[catalog.ConnectionParam]; ok && out != nil {
		out[catalog.ConnectionParam] = c
	}
	return out, issues
}

// credentialMasker is the masker of a call that runs with creds of a connector of kind: the secret
// fields of its descriptor when the engine knows them, else every value.
func (r *Runtime) credentialMasker(kind catalog.ConnectorKind, creds catalog.Credentials) Masker {
	if r.cfg.Connectors == nil {
		return code.NewMasker(map[string]string(creds))
	}
	c, ok := r.cfg.Connectors.Get(kind)
	if !ok {
		return code.NewMasker(map[string]string(creds))
	}
	secret := connectors.SecretFields(c.Descriptor())
	if len(secret) == 0 {
		return code.NewMasker(map[string]string(creds))
	}
	values := make(map[string]string, len(secret))
	for field, v := range creds {
		if secret[field] {
			values[field] = v
		}
	}
	return code.NewMasker(values)
}
