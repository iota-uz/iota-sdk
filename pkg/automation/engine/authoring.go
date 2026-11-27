package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opAuthoring = serrors.Op("automation.engine.authoring")

// Authoring is what an author checks a graph with before enabling it.
type Authoring interface {
	// DryRun: the interpreter of #1322/#1328 over the given events and context. It persists no run, step
	// or event; in cached mode a decision it had to make live is written to the decision cache. Nodes with
	// an effect are captured; execute calls run for real unless Decisions is scripted.
	DryRun(ctx context.Context, a Actor, in DryRunInput) (DryRunResult, error)
	// TestNode: one real execution of one node; no run, no steps, no outbox events.
	TestNode(ctx context.Context, a Actor, in TestNodeInput) (TestNodeResult, error)
	// SearchSubjects: subjects for dry run and test, visible to the actor.
	SearchSubjects(ctx context.Context, a Actor, entity, query string, limit int) ([]SubjectOption, error)
	// Validate checks a graph against the author's catalog as a save does (*ValidationError); nothing is
	// written. A graph that validates may still have warnings.
	Validate(ctx context.Context, a Actor, raw json.RawMessage) ([]graph.GraphError, error)
}

// Decision modes of a dry run: how its execute nodes answer.
const (
	DecisionsLive     = "live"     // a real call
	DecisionsCached   = "cached"   // the cache, else a real call recorded in the cache
	DecisionsScripted = "scripted" // the Context; no credentials are read and no network call is made
)

// Which execute nodes a dry run calls for real (DryRunInput.RealCalls).
const (
	RealCallsAll       = ""
	RealCallsDecisions = "decisions"
	RealCallsNone      = "none"
)

// MaxDryRunEvents bounds DryRunInput.Events.
const MaxDryRunEvents = 50

// DryRunInput is a dry run of a graph.
type DryRunInput struct {
	Graph   json.RawMessage
	Subject SubjectRef
	// AutomationID is the saved automation whose memory (state) the dry run reads; the zero value: the
	// graph is not saved, so no key is there. A dry run never writes the memory.
	AutomationID uuid.UUID
	Context      map[string]json.RawMessage // overrides of nodes.<id>.output / subject fields
	// Events is the sequence fed to triggers and waits (≤ 50): an event {kind, ...payload}, {"timeout":
	// true} for the earliest timer, or {"events": [...]} for the batch a trigger's collect merges into
	// the run that starts (events; event is its last entry).
	Events []json.RawMessage
	// Decisions is live, cached or scripted (default).
	Decisions string
	// MaxCalls bounds the real calls the dry run makes (live decisions, cache misses, execute nodes): the
	// call past it is not made and the dry run fails with ErrLimitExceeded. 0: no bound.
	MaxCalls int
	// RealCalls narrows which execute nodes a live or cached dry run calls for real: RealCallsAll (the
	// default, the editor's), RealCallsDecisions (decision nodes only) or RealCallsNone (cached reads the
	// cache only). A node not called is answered from Context, as a captured one.
	RealCalls string
	// Vars, when set, are the subject's variables as given (a backtest's snapshot of the record at its
	// event) instead of the record as the author reads it now: a JSON object.
	Vars json.RawMessage
	// Now is the time the context's now reads (a backtest's event time); zero: the current time.
	Now time.Time
	// Fence, when set, guards the real calls of a dry run made on behalf of a lease holder (a backtest
	// worker): see CallFence. nil: every call gets a fresh idempotency key and nothing is rechecked.
	Fence CallFence
}

// CallFence guards the real provider calls of a dry run that a worker makes while it holds a lease on
// the work, and that the next holder repeats when the lease passes to it.
//
// A lease check followed by a provider call is not atomic: a worker that stalls past its lease while a
// call is in flight still makes that call. The fence keeps such a repeat from being paid twice where the
// provider can deduplicate it, and bounds it where the provider cannot:
//
//   - every real call carries the idempotency key Scope:node:n (the n-th call of the node in the dry
//     run), the same for every holder of the work, so a provider that deduplicates by key
//     (catalog.SupportsIdempotencyKey) makes a repeated call once;
//   - Hold is asked right before each real call; once it fails the dry run makes no further call and
//     DryRun returns its error;
//   - Strict refuses (ErrUnfenced) a real call of a node that cannot deduplicate.
//
// Residual risk without Strict: a node whose provider cannot deduplicate may be paid twice for the call
// that was in flight (or past its Hold) when the lease passed: at most one call per example the superseded
// holder was running.
type CallFence interface {
	// Scope is the stable part of the calls' idempotency keys: what identifies the unit of work (a
	// backtest and an example), never the holder or its lease.
	Scope() string
	// Hold is nil while the caller may still make a real call.
	Hold(ctx context.Context) error
	// Strict refuses a real call of a node whose provider cannot deduplicate a repeat of it.
	Strict() bool
}

// fencedKey is the idempotency key of the n-th call of a node under a fence.
func fencedKey(scope, node string, n int) string {
	return scope + ":" + node + ":" + strconv.Itoa(n)
}

// DryRunResult is the trace of a dry run.
type DryRunResult struct {
	Trace []TraceStep
	// LiveCalls is the number of calls the dry run made to providers (a cached decision is not one).
	LiveCalls int
}

// StatusCached marks a dry run's execute step answered from the decision cache.
const StatusCached = "cached"

// TraceStep is one step of a dry run.
type TraceStep struct {
	Node    string
	Token   string // "" main path; "split.legal", "each[0]"
	Kind    string // triggered | not_triggered | step | wait | woke | timeout | joined | cancelled | ended | error
	Outcome string
	Path    string // error path (missing_context, expression_failed)
	Message string
	// Status is captured (an effect the dry run did not make, its params rendered), executed (a call
	// the dry run made for real) or cached (a decision answered from the cache, no call made).
	Status  string
	Effects []json.RawMessage // calls that would be made, params rendered, secrets as "***"
	Output  json.RawMessage
}

// Trace kinds.
const (
	TraceTriggered    = "triggered"
	TraceNotTriggered = "not_triggered"
	TraceStepKind     = "step"
	TraceWait         = "wait"
	TraceWoke         = "woke"
	TraceTimeout      = "timeout"
	TraceJoined       = "joined"
	TraceCancelled    = "cancelled"
	TraceEnded        = "ended"
	TraceError        = "error"
)

// TestNodeInput is one real execution of one node.
type TestNodeInput struct {
	Graph  json.RawMessage
	NodeID string
	// AutomationID is the saved automation whose memory (state) the node reads (see DryRunInput).
	AutomationID uuid.UUID
	// Key identifies the test the author confirmed (≤ 100 characters). The first request with a key
	// claims it in automation.node_tests (every process sees the claim) and runs the test; a repeat — a
	// retry after a lost answer, on any process — gets the stored answer, or, while the first is running
	// or its outcome is unknown (its process died), outcome_unknown: it is never run a second time. The
	// call is made with the idempotency key "test:<Key>". "": every request is a new test.
	Key     string
	Subject *SubjectRef
	Context map[string]json.RawMessage
}

// TestNodeResult is what the node did.
//
// The JSON names are the Go field names: automation.node_tests.result stores a result in this form.
type TestNodeResult struct {
	Outcome string          `json:"Outcome"`
	Output  json.RawMessage `json:"Output"`
	Logs    []string        `json:"Logs"`
	// Request is the rendered params of the call the node made (its prompt, its input), as JSON with the
	// connection's secrets masked; the credentials themselves are never in it. Empty for a node that
	// makes no call.
	Request    json.RawMessage `json:"Request"`
	DurationMS int64           `json:"DurationMS"`
	Error      *StepError      `json:"Error"` // reason: exception | timeout | network_blocked | connection_unavailable | ...
}

// SubjectOption is a subject an author may pick.
type SubjectOption struct {
	Subject     SubjectRef
	Label, Note string
}

// SubjectSearcher is an optional interface of the SubjectLoader: the subjects of an entity the subject
// (the author) may see, matching a query.
type SubjectSearcher interface {
	Search(ctx context.Context, as spi.Subject, entity, query string, limit int) ([]SubjectOption, error)
}

// Authoring is the API of dry runs and node tests.
func (r *Runtime) Authoring() Authoring { return &authoring{r: r} }

type authoring struct{ r *Runtime }

// authorGraph validates a graph against the author's catalog and builds its interpreter.
func (au *authoring) authorGraph(ctx context.Context, a Actor, raw json.RawMessage) (validated, *graph.Interpreter, error) {
	v, err := au.r.validate(ctx, a, raw)
	if err != nil {
		return validated{}, nil, err
	}
	in, err := graph.NewInterpreter(v.view.Catalog, v.g, au.r.graphOptions())
	if err != nil {
		return validated{}, nil, err
	}
	return v, in, nil
}

func (au *authoring) Validate(ctx context.Context, a Actor, raw json.RawMessage) ([]graph.GraphError, error) {
	if err := au.r.service.authorize(a, PermAuthor); err != nil {
		return nil, err
	}
	v, err := au.r.validate(ctx, a, raw)
	if err != nil {
		return nil, err
	}
	return v.warnings, nil
}

// decodeContext reads the overrides of a dry run or a test.
func decodeContext(c map[string]json.RawMessage) (map[string]any, error) {
	out := make(map[string]any, len(c))
	for k, raw := range c {
		var v any
		if err := expr.UnmarshalJSON(raw, &v); err != nil {
			return nil, fmt.Errorf("%w: context %s is not JSON", ErrInvalidInput, k)
		}
		out[k] = v
	}
	return out, nil
}

func (au *authoring) DryRun(ctx context.Context, a Actor, in DryRunInput) (DryRunResult, error) {
	if err := au.r.service.authorize(a, PermAuthor); err != nil {
		return DryRunResult{}, err
	}
	if n := DryRunEventCount(in.Events); n > MaxDryRunEvents {
		return DryRunResult{}, fmt.Errorf("%w: a dry run takes at most %d events, batches counted by their events; got %d", ErrLimitExceeded, MaxDryRunEvents, n)
	}
	switch in.RealCalls {
	case RealCallsAll, RealCallsDecisions, RealCallsNone:
	default:
		return DryRunResult{}, fmt.Errorf("%w: real_calls is all, decisions or none", ErrInvalidInput)
	}
	mode := in.Decisions
	switch mode {
	case "":
		mode = DecisionsScripted
	case DecisionsLive, DecisionsCached, DecisionsScripted:
	default:
		return DryRunResult{}, fmt.Errorf("%w: decisions is live, cached or scripted", ErrInvalidInput)
	}
	v, interp, err := au.authorGraph(ctx, a, in.Graph)
	if err != nil {
		return DryRunResult{}, err
	}
	overrides, err := decodeContext(in.Context)
	if err != nil {
		return DryRunResult{}, err
	}
	inputs := make([]graph.DryRunInput, 0, len(in.Events))
	for i, raw := range in.Events {
		var m map[string]any
		if err := expr.UnmarshalJSON(raw, &m); err != nil || m == nil {
			return DryRunResult{}, fmt.Errorf("%w: events[%d] is not an object", ErrInvalidInput, i)
		}
		if t, _ := m["timeout"].(bool); t && len(m) == 1 {
			inputs = append(inputs, graph.DryRunInput{Timeout: true})
			continue
		}
		if _, isBatch := dryRunBatch(raw); isBatch {
			// The rule DryRunEventCount counts by: the only field, an array.
			batch, _ := m["events"].([]any)
			if len(batch) == 0 || len(batch) > graph.CollectMaxEvents {
				return DryRunResult{}, fmt.Errorf("%w: events[%d] is a batch of 1..%d events", ErrInvalidInput, i, graph.CollectMaxEvents)
			}
			evs := make([]graph.Event, 0, len(batch))
			for _, e := range batch {
				em, ok := e.(map[string]any)
				if !ok {
					return DryRunResult{}, fmt.Errorf("%w: events[%d] holds a batch entry that is not an object", ErrInvalidInput, i)
				}
				evs = append(evs, graph.Event(em))
			}
			inputs = append(inputs, graph.DryRunInput{Events: evs})
			continue
		}
		inputs = append(inputs, graph.DryRunInput{Event: graph.Event(m)})
	}
	sub := a.Subject
	sub.TenantID = a.TenantID
	rc := RunContextInput{TenantID: a.TenantID, AutomationID: in.AutomationID, Subject: in.Subject, RunAs: sub, Now: time.Now(), Overrides: overrides}
	if !in.Now.IsZero() {
		rc.Now = in.Now
	}
	if len(in.Vars) > 0 {
		var vars map[string]any
		if err := expr.UnmarshalJSON(in.Vars, &vars); err != nil {
			return DryRunResult{}, fmt.Errorf("%w: vars is not a JSON object", ErrInvalidInput)
		}
		if vars == nil {
			vars = map[string]any{}
		}
		rc.Vars = vars
	}
	gctx, err := au.r.BuildRunContext(ctx, rc)
	if err != nil {
		return DryRunResult{}, &host.InfraError{Err: err}
	}
	opts := graph.DryRunOptions{Modes: true}
	var budget *callBudget
	if in.MaxCalls > 0 {
		budget = &callBudget{left: in.MaxCalls}
	}
	// Whether each execute call, in order per node, was answered from the cache: the trace's steps of a
	// node come in the same order, so a cached one is reported as such and not as a live call.
	cachedBy := map[string][]bool{}
	live := 0
	// stopped is the fence's error that ended the dry run's real calls; calls per node number the keys
	// of a fenced dry run.
	var stopped error
	calls := map[string]int{}
	if mode != DecisionsScripted {
		opts.Execute = func(node graph.Node, effects []graph.Effect) graph.ExecAnswer {
			if stopped != nil {
				return graph.ExecAnswer{Code: CodeCancelled, Message: "the dry run stopped"}
			}
			params := map[string]any{}
			var stepCtx map[string]any
			var templated []string
			if len(effects) > 0 {
				params, stepCtx, templated = effects[0].Params, effects[0].Context, effects[0].Templated
			}
			key := "dry-run:" + uuid.NewString()
			if in.Fence != nil {
				calls[node.ID]++
				key = fencedKey(in.Fence.Scope(), node.ID, calls[node.ID])
			}
			res := au.r.execDirect(ctx, directCall{tenant: a.TenantID, as: sub, view: v.view, node: node, params: params, context: stepCtx, templated: templated,
				subject: normSubject(in.Subject), key: key, decisions: mode, budget: budget, realCalls: in.RealCalls, fence: in.Fence})
			if res.stop != nil {
				stopped = res.stop
				return graph.ExecAnswer{Code: CodeCancelled, Message: "the dry run stopped"}
			}
			cachedBy[node.ID] = append(cachedBy[node.ID], res.cached)
			if res.called {
				live++
			}
			if res.script {
				return graph.ExecAnswer{Script: true}
			}
			if res.err != nil {
				return graph.ExecAnswer{Code: res.err.Code, Message: res.err.Message, Retryable: res.err.Retryable}
			}
			out, _ := decodeMap(res.output)
			return graph.ExecAnswer{Outcome: res.outcome, Output: out}
		}
	}
	opts.Preview = func(node graph.Node, effects []graph.Effect, started graph.Event) (graph.ExecAnswer, bool) {
		// The event that started the dry run, as the interpreter chose it, and the author as its initiator.
		trigger := triggerOfEvent(started, "dry-run")
		if trigger != nil {
			trigger.ActorUserID = a.UserID
		}
		return au.r.previewEffects(ctx, a.TenantID, sub, normSubject(in.Subject), trigger, node, effects, in.RealCalls != RealCallsAll)
	}
	steps := interp.DryRun(normSubject(in.Subject), gctx, inputs, opts)
	if stopped != nil {
		return DryRunResult{LiveCalls: live}, stopped
	}
	if budget.wasExceeded() {
		return DryRunResult{}, fmt.Errorf("%w: a dry run makes at most %d real calls", ErrLimitExceeded, in.MaxCalls)
	}
	trace := traceOf(interp, steps)
	for i := range trace {
		if trace[i].Status != graph.StatusExecuted {
			continue
		}
		if q := cachedBy[trace[i].Node]; len(q) > 0 {
			if q[0] {
				trace[i].Status = StatusCached
			}
			cachedBy[trace[i].Node] = q[1:]
		}
	}
	return DryRunResult{Trace: trace, LiveCalls: live}, nil
}

// DryRunEventCount is how many events a dry run's input feeds: a batch counts its entries, anything
// else one. A batch is what DryRun takes for one: an object whose only field is the array events (an
// event with its own payload field events is one event).
func DryRunEventCount(events []json.RawMessage) int {
	n := 0
	for _, raw := range events {
		if batch, ok := dryRunBatch(raw); ok {
			n += len(batch)
			continue
		}
		n++
	}
	return n
}

// dryRunBatch is the entries of a batch input {"events": [...]}: the only field, an array.
func dryRunBatch(raw json.RawMessage) ([]json.RawMessage, bool) {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || len(m) != 1 {
		return nil, false
	}
	var batch []json.RawMessage
	if json.Unmarshal(m["events"], &batch) != nil || batch == nil {
		return nil, false
	}
	return batch, true
}

// traceOf maps the interpreter's trace to the engine's.
func traceOf(in *graph.Interpreter, steps []graph.TraceStep) []TraceStep {
	out := make([]TraceStep, 0, len(steps))
	waiting := map[string]bool{} // token|node that waits
	for _, s := range steps {
		t := TraceStep{Node: s.Node, Token: s.Token, Outcome: s.Outcome, Status: s.Status}
		if eff, ok := s.ContextPatch["effects"].([]graph.Effect); ok {
			for _, e := range eff {
				b, err := json.Marshal(e)
				if err == nil {
					t.Effects = append(t.Effects, b)
				}
			}
		}
		if o, ok := s.ContextPatch["output"]; ok {
			if b, err := json.Marshal(o); err == nil {
				t.Output = b
			}
		}
		key := s.Token + "|" + s.Node
		_, isTrigger := in.Trigger(s.Node)
		switch {
		case s.Outcome == graph.TraceNotTriggered:
			t.Kind = TraceNotTriggered
			// One step per trigger the input reached and failed: the trigger and the condition that failed.
			if missed, ok := s.ContextPatch["triggers"].([]map[string]any); ok && len(missed) > 0 {
				for _, m := range missed {
					id, _ := m["trigger"].(string)
					failed, _ := m["failed"].(string)
					out = append(out, TraceStep{Node: id, Kind: TraceNotTriggered, Path: failed, Message: triggerCondition(in, id, failed)})
				}
				continue
			}
		case isTrigger && s.Outcome == graph.TriggerOutcome:
			t.Kind = TraceTriggered
		case s.Outcome == graph.TraceWaiting:
			t.Kind = TraceWait
			waiting[key] = true
		case s.Outcome == graph.TraceEnd || s.Outcome == graph.TraceEnded:
			t.Kind = TraceEnded
		case s.Outcome == graph.TraceCancelled:
			t.Kind = TraceCancelled
		case s.Outcome == graph.TraceArrived:
			t.Kind = TraceJoined
		case s.Outcome == graph.TraceMissingContext:
			t.Kind, t.Path, t.Message = TraceError, CodeMissingContext, fmt.Sprint(s.ContextPatch["path"])
			t.Outcome = ""
		case s.Outcome == graph.TraceFailed && s.ContextPatch["output"] != nil:
			// A node's own failed outcome, taken with output.error: a step, not a stop.
			t.Kind = TraceStepKind
		case s.Outcome == graph.TraceFailed:
			t.Kind = TraceError
			code, _ := s.ContextPatch["code"].(string)
			msg, _ := s.ContextPatch["message"].(string)
			t.Path, t.Message, t.Outcome = code, msg, ""
		case waiting[key] && s.Outcome == CodeTimeout:
			t.Kind = TraceTimeout
			delete(waiting, key)
		case waiting[key]:
			t.Kind = TraceWoke
			delete(waiting, key)
		default:
			t.Kind = TraceStepKind
		}
		out = append(out, t)
	}
	return out
}

// triggerCondition is the condition of a trigger that did not pass, as the author wrote it: its CEL
// filter, or the trigger parameter the event did not match (param = value).
func triggerCondition(in *graph.Interpreter, id, failed string) string {
	t, ok := in.Trigger(id)
	if !ok {
		return ""
	}
	switch failed {
	case graph.MismatchFilter:
		return t.Filter
	case graph.MismatchHook, "":
		return ""
	}
	if v, ok := t.Params[failed]; ok {
		if b, err := json.Marshal(v); err == nil {
			return failed + " = " + string(b)
		}
	}
	return failed
}

// MaxTestKey bounds TestNodeInput.Key.
const MaxTestKey = 100

// testedNodeNeedsReauth reports whether the node a test names needs a connection that waits for a new
// sign-in (needsReauth).
func (au *authoring) testedNodeNeedsReauth(ctx context.Context, a Actor, in TestNodeInput) bool {
	g, errs := graph.Decode(in.Graph)
	if len(errs) > 0 {
		return false
	}
	for _, n := range g.Nodes {
		if n.ID != in.NodeID {
			continue
		}
		nt, ok := au.r.cfg.Catalog.Node(n.Type)
		return ok && len(nt.Requires()) > 0 && needsReauth(ctx, au.r.cfg.DB, a.TenantID, n.Params[catalog.ConnectionParam], nt.Requires())
	}
	return false
}

func (au *authoring) TestNode(ctx context.Context, a Actor, in TestNodeInput) (TestNodeResult, error) {
	if err := au.r.service.authorize(a, PermAuthor); err != nil {
		return TestNodeResult{}, err
	}
	if len(in.Key) > MaxTestKey {
		return TestNodeResult{}, fmt.Errorf("%w: the test key is longer than %d characters", ErrInvalidInput, MaxTestKey)
	}
	if in.Key == "" {
		return au.testNode(ctx, a, in, "test:"+uuid.NewString())
	}
	return au.keyedTest(ctx, a, in)
}

// testWait is how long a repeat of a node test waits for the first one to record its answer.
var testWait = 30 * time.Second

// testRetention is how long the answer of a keyed node test is kept for its repeats. The claim itself is
// kept: a key once used never runs a test again (a repeat past the retention is refused as unknown).
const testRetention = 24 * time.Hour

// keyedTest runs a node test once per (tenant, author, key), whatever process a request reaches.
func (au *authoring) keyedTest(ctx context.Context, a Actor, in TestNodeInput) (TestNodeResult, error) {
	db := au.r.cfg.DB
	if _, err := db.Exec(ctx, `UPDATE automation.node_tests SET result = NULL WHERE tenant_id = $1 AND created_at < $2 AND result IS NOT NULL`,
		a.TenantID, time.Now().Add(-testRetention)); err != nil {
		return TestNodeResult{}, serrors.FromDB(opAuthoring, err)
	}
	tag, err := db.Exec(ctx, `INSERT INTO automation.node_tests (tenant_id, user_id, key, node_id) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
		a.TenantID, a.UserID, in.Key, in.NodeID)
	if err != nil {
		return TestNodeResult{}, serrors.FromDB(opAuthoring, err)
	}
	if tag.RowsAffected() == 0 {
		return au.repeatedTest(ctx, a, in)
	}
	// The claim is ours: the test runs to its end even if the request goes away, and its answer is kept.
	run := context.WithoutCancel(ctx)
	res, err := au.testNode(run, a, in, "test:"+in.Key)
	if err != nil {
		// Nothing was called (the graph, the context or the store refused first): the key is free again.
		if _, derr := db.Exec(run, `DELETE FROM automation.node_tests WHERE tenant_id = $1 AND user_id = $2 AND key = $3 AND result IS NULL`,
			a.TenantID, a.UserID, in.Key); derr != nil {
			au.r.cfg.Logger.Warn("automation: a node test's claim was not released", "error", derr)
		}
		return TestNodeResult{}, err
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return TestNodeResult{}, err
	}
	if _, err := db.Exec(run, `UPDATE automation.node_tests SET result = $4 WHERE tenant_id = $1 AND user_id = $2 AND key = $3`,
		a.TenantID, a.UserID, in.Key, raw); err != nil {
		return TestNodeResult{}, serrors.FromDB(opAuthoring, err)
	}
	return res, nil
}

// repeatedTest is the answer of a test whose key was claimed before: the stored answer, or outcome_unknown
// when none is recorded within testWait (it is still running, or its process died mid-call).
func (au *authoring) repeatedTest(ctx context.Context, a Actor, in TestNodeInput) (TestNodeResult, error) {
	deadline := time.Now().Add(testWait)
	for {
		var raw []byte
		var node string
		var claimed time.Time
		err := au.r.cfg.DB.QueryRow(ctx, `SELECT node_id, result, created_at FROM automation.node_tests WHERE tenant_id = $1 AND user_id = $2 AND key = $3`,
			a.TenantID, a.UserID, in.Key).Scan(&node, &raw, &claimed)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			// The first request released its claim (it ran nothing): this one may run the test.
			return au.keyedTest(ctx, a, in)
		case err != nil:
			return TestNodeResult{}, serrors.FromDB(opAuthoring, err)
		case node != in.NodeID:
			return TestNodeResult{}, fmt.Errorf("%w: the test key was used for another node", ErrInvalidInput)
		case len(raw) > 0:
			var res TestNodeResult
			if err := json.Unmarshal(raw, &res); err != nil {
				return TestNodeResult{}, err
			}
			return res, nil
		}
		// A claim older than the wait has no answer coming (its process died, or its answer expired).
		if !time.Now().Before(deadline) || time.Since(claimed) > testWait {
			return TestNodeResult{Error: &StepError{Code: CodeOutcomeUnknown,
				Message: "the test with this key has no recorded answer: it is still running or its outcome is unknown, so it is not run again"}}, nil
		}
		select {
		case <-ctx.Done():
			return TestNodeResult{}, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (au *authoring) testNode(ctx context.Context, a Actor, in TestNodeInput, key string) (TestNodeResult, error) {
	v, interp, err := au.authorGraph(ctx, a, in.Graph)
	if err != nil {
		// A node whose connection waits for a new sign-in left the catalog, so the graph does not
		// validate: the test answers as the call would, unauthorized.
		if errors.Is(err, ErrConnectionUnavailable) && au.testedNodeNeedsReauth(ctx, a, in) {
			return TestNodeResult{Error: &StepError{Code: catalog.CallUnauthorized, Message: msgNeedsReauth}}, nil
		}
		return TestNodeResult{}, err
	}
	node, ok := interp.View().Node(in.NodeID)
	if !ok {
		return TestNodeResult{}, fmt.Errorf("%w: no node %s", ErrInvalidInput, in.NodeID)
	}
	nt, _ := au.r.cfg.Catalog.Node(node.Type)
	if h, ok := nt.(HumanNode); ok && h.Human() {
		return TestNodeResult{Error: &StepError{Code: CodeInvalidInput, Message: "a human node cannot be tested: it waits for people"}}, nil
	}
	overrides, err := decodeContext(in.Context)
	if err != nil {
		return TestNodeResult{}, err
	}
	subject := SubjectRef{Entity: graph.SubjectNone}
	if in.Subject != nil {
		subject = normSubject(*in.Subject)
	}
	// The node runs as the author, not as the graph's owner.
	sub := a.Subject
	sub.TenantID = a.TenantID
	event := map[string]any{}
	if raw, ok := overrides["event"].(map[string]any); ok {
		event = raw
	}
	gctx, err := au.r.BuildRunContext(ctx, RunContextInput{TenantID: a.TenantID, AutomationID: in.AutomationID, Subject: subject, RunAs: sub, Event: event, Now: time.Now(), Overrides: overrides})
	if err != nil {
		return TestNodeResult{}, &host.InfraError{Err: err}
	}
	started := time.Now()
	res := TestNodeResult{}
	logs := &logCollector{}
	masker := code.NewMasker(nil)
	ctx = withLogs(ctx, logs)
	st := interp.Start(node.ID, &graph.StepContext{Ctx: gctx})
	switch st.Kind { //nolint:exhaustive // flow steps (end, fork, each, arrive) are reported by their kind
	case graph.StepFailed, graph.StepMissingContext:
		e := stepErr(st)
		res.Error = &e
	case graph.StepCall:
		params := map[string]any{}
		var stepCtx map[string]any
		var templated []string
		if len(st.Effects) > 0 {
			params, stepCtx, templated = st.Effects[0].Params, st.Effects[0].Context, st.Effects[0].Templated
		}
		dr := au.r.execDirect(ctx, directCall{tenant: a.TenantID, as: sub, view: v.view, node: node, params: params, context: stepCtx, templated: templated,
			subject: subject, key: key, decisions: DecisionsLive})
		res.Outcome, res.Output, res.Error = dr.outcome, dr.output, dr.err
		if dr.masker != nil {
			masker = dr.masker
		}
		if raw, err := json.Marshal(params); err == nil {
			res.Request = masker.MaskJSON(raw)
		}
		// As in a run: a call whose answer names its outcome by a choice field takes it from the output.
		if dr.err == nil && dr.outcome == "" && interp.View().Catalog().Nodes[node.Type].Answer != nil {
			out, _ := decodeMap(dr.output)
			o, fst := interp.CallOutcome(node.ID, out)
			if fst != nil {
				e := stepErr(*fst)
				res.Error = &e
			} else {
				res.Outcome = o
			}
		}
	case graph.StepOutcome, graph.StepWait:
		res.Outcome = st.Outcome
		if st.Output != nil {
			if res.Output, err = json.Marshal(st.Output); err != nil {
				return TestNodeResult{}, err
			}
		}
		if len(st.Effects) > 0 {
			applied, serr, err := au.r.effectRolledBack(ctx, a.TenantID, in.AutomationID, sub, subject, authorTrigger(event, a), node, interp.View().Catalog().Nodes[node.Type], st.Effects)
			if err != nil {
				return TestNodeResult{}, err
			}
			if serr != nil {
				res.Error = serr
			} else {
				if applied.Outcome != "" {
					res.Outcome = applied.Outcome
				}
				if applied.Output != nil {
					if res.Output, err = json.Marshal(applied.Output); err != nil {
						return TestNodeResult{}, err
					}
				}
			}
		}
		if st.Kind == graph.StepWait {
			w, err := json.Marshal(map[string]any{"waits_for": st.Wait.Event, "timeout_ms": st.Wait.Timeout.Milliseconds()})
			if err != nil {
				return TestNodeResult{}, err
			}
			res.Output = w
		}
	default:
		res.Outcome = string(st.Kind)
	}
	if res.Error == nil && len(res.Output) == 0 {
		res.Output = json.RawMessage(`{}`)
	}
	res.Logs = logs.lines()
	for i, l := range res.Logs {
		res.Logs[i] = masker.Mask(l)
	}
	res.DurationMS = time.Since(started).Milliseconds()
	return res, nil
}

// authorTrigger is the event a node test runs on, with the author as its initiator.
func authorTrigger(event map[string]any, a Actor) *TriggerEvent {
	t := triggerOfEvent(event, "test-node")
	if t != nil {
		t.ActorUserID = a.UserID
	}
	return t
}

// triggerOfEvent is the trigger event of an event as the context shows it (payload, and its kind); nil for
// an event that names no kind.
func triggerOfEvent(ev map[string]any, id string) *TriggerEvent {
	kind, _ := ev["kind"].(string)
	if kind == "" {
		return nil
	}
	payload := make(map[string]any, len(ev))
	for k, v := range ev {
		if k != "kind" && k != catalog.BySelfField {
			payload[k] = v
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return &TriggerEvent{Key: kind, ID: id, Payload: raw}
}

// previewEffects is the answer of a node that previews its effect (EffectPreviewer) in a dry run; offline
// (an agent's dry run) only an OfflinePreviewer is asked. ok is false
// for a node that does not.
func (r *Runtime) previewEffects(ctx context.Context, tenant uuid.UUID, as spi.Subject, subject SubjectRef, trigger *TriggerEvent, node graph.Node, effects []graph.Effect, offline bool) (graph.ExecAnswer, bool) {
	nt, _ := r.cfg.Catalog.Node(node.Type)
	pv, ok := nt.(EffectPreviewer)
	if !ok || len(effects) == 0 {
		return graph.ExecAnswer{}, false
	}
	if _, local := pv.(OfflinePreviewer); offline && !local {
		// An offline dry run (an agent's) runs no previewer that may call out or run package code.
		return graph.ExecAnswer{}, false
	}
	sc := StepContext{TenantID: tenant, RunAs: as, Subject: subject, NodeID: node.ID, Trigger: trigger, Offline: offline}
	res, err := pv.PreviewEffect(ctx, sc, node, effects[0])
	if err != nil {
		var se *StepError
		if errors.As(err, &se) {
			return graph.ExecAnswer{Code: se.Code, Message: se.Message, Retryable: se.Retryable}, true
		}
		return graph.ExecAnswer{Code: CodeEffectFailed, Message: err.Error()}, true
	}
	return graph.ExecAnswer{Outcome: res.Outcome, Output: res.Output}, true
}

// effectRolledBack applies an effect node's effects in a transaction it rolls back: the node's output
// is shown, and nothing it wrote (rows, outbox events) stays.
func (r *Runtime) effectRolledBack(ctx context.Context, tenant, automation uuid.UUID, as spi.Subject, subject SubjectRef, trigger *TriggerEvent, node graph.Node, spec graph.NodeSpec, effects []graph.Effect) (EffectResult, *StepError, error) {
	nt, _ := r.cfg.Catalog.Node(node.Type)
	en, ok := nt.(EffectNode)
	if !ok {
		// As in a run: an effect node without an executor applies nothing, so its test says so.
		return EffectResult{}, &StepError{Code: CodeUnsupported, Message: fmt.Sprintf("node type %s has no effect executor", node.Type)}, nil
	}
	sc := StepContext{TenantID: tenant, AutomationID: automation, RunAs: as, Subject: subject, NodeID: node.ID, Trigger: trigger}
	if prep, ok := en.(EffectPreparer); ok && len(effects) > 0 {
		// Prepared before the transaction opens, as a run does.
		v, err := prep.PrepareEffect(ctx, sc, node, effects[0])
		sc.Prepared = &PreparedEffect{Value: v, Err: err}
	}
	tx, err := r.cfg.DB.Begin(ctx)
	if err != nil {
		return EffectResult{}, nil, serrors.FromDB(opAuthoring, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	return applyEach(ctx, tx, sc, node, spec, en, effects, false)
}

func (au *authoring) SearchSubjects(ctx context.Context, a Actor, entity, query string, limit int) ([]SubjectOption, error) {
	if err := au.r.service.authorize(a, PermAuthor); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	s, ok := au.r.cfg.Subjects.(SubjectSearcher)
	if !ok || entity == "" {
		return []SubjectOption{}, nil
	}
	sub := a.Subject
	sub.TenantID = a.TenantID
	out, err := s.Search(ctx, sub, entity, strings.TrimSpace(query), limit)
	if err != nil {
		return nil, serrors.Wrap(opAuthoring, err)
	}
	if out == nil {
		out = []SubjectOption{}
	}
	return out, nil
}

// directCall is a call node executed outside a run: a test of a node, an execute node of a dry run.
type directCall struct {
	tenant    uuid.UUID
	as        spi.Subject
	view      catalog.View
	node      graph.Node
	params    map[string]any
	context   map[string]any // the step's context (graph.NodeSpec.StepContext)
	templated []string       // the Mixed params rendered as templates (graph.Effect.Templated)
	subject   SubjectRef
	key       string
	decisions string
	// budget, when set, bounds the real calls of a dry run.
	budget *callBudget
	// realCalls narrows the nodes called for real (DryRunInput.RealCalls).
	realCalls string
	// fence, when set, is asked before the real call (DryRunInput.Fence).
	fence CallFence
}

// callBudget is how many real calls a dry run may still make.
type callBudget struct {
	mu       sync.Mutex
	left     int
	exceeded bool
}

func (b *callBudget) take() bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.left <= 0 {
		b.exceeded = true
		return false
	}
	b.left--
	return true
}

func (b *callBudget) wasExceeded() bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.exceeded
}

type directResult struct {
	cached  bool // answered from the decision cache, no call made
	called  bool // the provider was called
	outcome string
	output  json.RawMessage
	err     *StepError
	masker  Masker // the connection's: what the node logged is masked with it
	// script: the dry run may not make this call; the node is answered from the context.
	script bool
	// stop: the fence refused the call (its holder lost the work, or a strict fence met a node that
	// cannot deduplicate); no call was made and the dry run ends with this error.
	stop error
}

// execDirect runs a call node once, as the actor, with the connection the actor may use; the output and
// errors are masked with its credentials. Asker nodes send and return the send's result.
func (r *Runtime) execDirect(ctx context.Context, c directCall) directResult {
	nt, ok := r.cfg.Catalog.Node(c.node.Type)
	cn, isCall := nt.(catalog.CallNode)
	if !ok || !isCall {
		return directResult{err: &StepError{Code: CodeUnsupported, Message: c.node.Type + " has no call executor"}}
	}
	if _, decides := cn.(DecisionNode); !decides && c.realCalls != RealCallsAll {
		return directResult{script: true}
	}
	if perm, ok := cn.(catalog.PermissionedNode); ok && perm.Permission() != "" && !c.as.Has(perm.Permission()) {
		return directResult{err: &StepError{Code: CodeForbidden, Message: "you may not use " + c.node.Type}}
	}
	var info *catalog.ConnectionInfo
	var creds catalog.Credentials
	if kinds := c.view.NodeRequires[c.node.Type]; len(kinds) > 0 {
		ref, _, err := catalog.ResolveConnection(c.view.Connections, kinds, c.params[catalog.ConnectionParam])
		if err != nil && needsReauth(ctx, r.cfg.DB, c.tenant, c.params[catalog.ConnectionParam], kinds) {
			return directResult{err: &StepError{Code: catalog.CallUnauthorized, Message: msgNeedsReauth}}
		}
		if err != nil || r.cfg.Credentials == nil {
			if c.realCalls == RealCallsNone {
				return directResult{script: true} // offline: answered from the context
			}
			return directResult{err: &StepError{Code: CodeConnectionUnavailable, Message: "the connection of " + c.node.ID + " is not available"}}
		}
		ci, cr, err := r.cfg.Credentials.Open(ctx, c.tenant, ref.ID)
		if err != nil && c.realCalls == RealCallsNone {
			// Offline: no cache key without the connection, so the context answers.
			return directResult{script: true}
		}
		switch {
		case errors.Is(err, connectors.ErrNeedsReauth):
			return directResult{err: &StepError{Code: catalog.CallUnauthorized, Message: msgNeedsReauth}}
		case errors.Is(err, connectors.ErrNotFound) && strings.HasPrefix(ref.ID, catalog.PlatformPrefix):
			// As in a run: the platform key was switched off since the catalog was read.
			return directResult{err: &StepError{Code: catalog.CallUnavailable, Message: platformOffMessage}}
		case errors.Is(err, connectors.ErrNotFound), errors.Is(err, connectors.ErrNotConnected), errors.Is(err, connectors.ErrUndecryptable):
			return directResult{err: &StepError{Code: CodeConnectionUnavailable, Message: "the connection is not connected"}}
		case err != nil:
			return directResult{err: &StepError{Code: CodeConnectionUnavailable, Message: "the connection could not be opened"}}
		}
		info, creds = &ci, cr
	}
	masker := code.NewMasker(nil)
	if info != nil {
		masker = r.credentialMasker(info.Kind, creds)
	}
	timeout := r.cfg.CallLimits.DefaultTimeout
	if t, ok := paramTimeout(c.params); ok {
		timeout = min(t, r.cfg.CallLimits.MaxTimeout)
	}
	deadline := time.Now().Add(timeout)
	in := catalog.CallInput{
		TenantID: c.tenant, NodeID: c.node.ID, Attempt: 1, IdempotencyKey: c.key, Params: c.params, Templated: c.templated, Context: c.context, Subject: c.subject,
		RunAs: c.as, Connection: info, Credentials: creds, Deadline: deadline, Usage: r.cfg.Usage,
	}
	dn, decides := cn.(DecisionNode)
	cacheKey := ""
	if decides && c.decisions == DecisionsCached {
		if k, ok := dn.DecisionKey(in); ok {
			cacheKey = k
			if raw, hit, err := r.cfg.Decisions.Get(ctx, c.tenant, cacheKey); err == nil && hit {
				var d struct {
					Outcome string          `json:"outcome"`
					Output  json.RawMessage `json:"output"`
				}
				if json.Unmarshal(raw, &d) == nil {
					return directResult{masker: masker, outcome: d.Outcome, output: nonNull(d.Output), cached: true}
				}
			}
		}
	}
	if c.realCalls == RealCallsNone {
		// Offline: only the cache answers; a miss is answered from the context.
		return directResult{script: true}
	}
	if c.fence != nil {
		if c.fence.Strict() && !catalog.SupportsIdempotencyKey(cn) {
			return directResult{stop: fmt.Errorf("%w: the provider of %s cannot deduplicate a repeated call, so a strict backtest does not call it", ErrUnfenced, c.node.Type)}
		}
		// The last check before the call: a holder that lost the work makes no further call. The call
		// itself may still outlive the lease; its key lets the next holder's repeat be made once.
		if err := c.fence.Hold(ctx); err != nil {
			return directResult{stop: err}
		}
	}
	if !c.budget.take() {
		return directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: CodeLimitExceeded, Message: "the dry run reached its limit of real calls"}}
	}
	callCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if _, asks := cn.(Asker); asks {
		// A test sends the question with a token that no wait listens for: an answer finds it closed.
		callCtx = WithCorrelation(callCtx, Correlation{Token: NewCorrelationToken()})
	}
	res, err := cn.Execute(callCtx, in)
	// Every answer from here on followed a real call to the provider.
	called := func(r directResult) directResult {
		r.called = true
		return r
	}
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			// As in a run's call: the code is kept only as an identifier with nothing a mask hides.
			return called(directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: callCode(ce.Code, masker), Message: masker.Mask(ce.Message), Retryable: ce.Retryable}})
		}
		if callCtx.Err() != nil {
			return called(directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: CodeTimeout, Message: c.node.ID + " got no answer by its deadline"}})
		}
		return called(directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: "exception", Message: masker.Mask(err.Error())}})
	}
	if res.RetryAfter > 0 {
		// The provider asked to repeat later: a refusal, as in a run (and never a cached decision).
		return called(directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: CodeRateLimited,
			Message: fmt.Sprintf("the provider asked to repeat the call in %s", res.RetryAfter), Retryable: true}})
	}
	out, err := json.Marshal(nonNilMap(res.Output))
	if err != nil {
		return called(directResult{masker: masker, err: &StepError{Code: graph.FailInvalidAnswer, Message: "the output is not JSON"}})
	}
	// One canonical value: what the cache holds (jsonb refuses U+0000) is what a live call returns, so a
	// cached decision and a fresh one give a downstream expression the same value.
	output := withoutNUL(masker.MaskJSON(out)) // a json.RawMessage: bytes would be stored as base64
	if cacheKey != "" {
		// The output is stored as JSON: bytes would be written as a base64 string, and a cached decision
		// would answer without its output.
		if b, err := json.Marshal(map[string]any{"outcome": res.Outcome, "output": output}); err == nil {
			if err := r.cfg.Decisions.Put(ctx, c.tenant, cacheKey, b); err != nil {
				r.cfg.Logger.Warn("automation: a decision was not cached", "error", err)
			}
		}
	}
	return called(directResult{masker: masker, outcome: res.Outcome, output: output})
}

// logCollector collects the log lines a node writes during a test (Log).
type logCollector struct {
	mu  sync.Mutex
	out []string
}

func (l *logCollector) add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.out) < 200 {
		l.out = append(l.out, s)
	}
}

func (l *logCollector) lines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.out...)
}

type logsKey struct{}

func withLogs(ctx context.Context, l *logCollector) context.Context {
	return context.WithValue(ctx, logsKey{}, l)
}

// Log writes a line to the log of a node test (TestNodeResult.Logs) or of a run's call (the step's logs,
// masked with the connection's secrets); elsewhere it does nothing. A
// node never logs credentials.
func Log(ctx context.Context, format string, args ...any) {
	if l, ok := ctx.Value(logsKey{}).(*logCollector); ok {
		l.add(fmt.Sprintf(format, args...))
	}
}
