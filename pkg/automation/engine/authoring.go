package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

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
}

// Decision modes of a dry run: how its execute nodes answer.
const (
	DecisionsLive     = "live"     // a real call
	DecisionsCached   = "cached"   // the cache, else a real call recorded in the cache
	DecisionsScripted = "scripted" // the Context; no credentials are read and no network call is made
)

// MaxDryRunEvents bounds DryRunInput.Events.
const MaxDryRunEvents = 50

// DryRunInput is a dry run of a graph.
type DryRunInput struct {
	Graph   json.RawMessage
	Subject SubjectRef
	Context map[string]json.RawMessage // overrides of nodes.<id>.output / subject fields
	// Events is the sequence fed to triggers and waits (≤ 50): an event {kind, ...payload}, {"timeout":
	// true} for the earliest timer, or {"events": [...]} for the batch a trigger's collect merges into
	// the run that starts (events; event is its last entry).
	Events []json.RawMessage
	// Decisions is live, cached or scripted (default).
	Decisions string
}

// DryRunResult is the trace of a dry run.
type DryRunResult struct{ Trace []TraceStep }

// TraceStep is one step of a dry run.
type TraceStep struct {
	Node    string
	Token   string // "" main path; "split.legal", "each[0]"
	Kind    string // triggered | not_triggered | step | wait | woke | timeout | joined | cancelled | ended | error
	Outcome string
	Path    string // error path (missing_context, expression_failed)
	Message string
	// Status is captured (an effect the dry run did not make, its params rendered) or executed (a call
	// the dry run made for real).
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
	Graph   json.RawMessage
	NodeID  string
	Subject *SubjectRef
	Context map[string]json.RawMessage
}

// TestNodeResult is what the node did.
type TestNodeResult struct {
	Outcome    string
	Output     json.RawMessage
	Logs       []string
	DurationMS int64
	Error      *StepError // reason: exception | timeout | network_blocked | connection_unavailable | ...
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
	if len(in.Events) > MaxDryRunEvents {
		return DryRunResult{}, fmt.Errorf("%w: a dry run takes at most %d events", ErrInvalidInput, MaxDryRunEvents)
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
		if batch, ok := m["events"].([]any); ok && len(m) == 1 {
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
	gctx, err := au.r.BuildRunContext(ctx, RunContextInput{TenantID: a.TenantID, Subject: in.Subject, RunAs: sub, Now: time.Now(), Overrides: overrides})
	if err != nil {
		return DryRunResult{}, &host.InfraError{Err: err}
	}
	opts := graph.DryRunOptions{Modes: true}
	if mode != DecisionsScripted {
		opts.Execute = func(node graph.Node, effects []graph.Effect) graph.ExecAnswer {
			params := map[string]any{}
			if len(effects) > 0 {
				params = effects[0].Params
			}
			res := au.r.execDirect(ctx, directCall{tenant: a.TenantID, as: sub, view: v.view, node: node, params: params,
				subject: normSubject(in.Subject), key: "dry-run:" + uuid.NewString(), decisions: mode})
			if res.err != nil {
				return graph.ExecAnswer{Code: res.err.Code, Message: res.err.Message, Retryable: res.err.Retryable}
			}
			out, _ := decodeMap(res.output)
			return graph.ExecAnswer{Outcome: res.outcome, Output: out}
		}
	}
	trigger := dryRunTrigger(inputs)
	opts.Preview = func(node graph.Node, effects []graph.Effect) (graph.ExecAnswer, bool) {
		return au.r.previewEffects(ctx, a.TenantID, sub, normSubject(in.Subject), trigger, node, effects)
	}
	steps := interp.DryRun(normSubject(in.Subject), gctx, inputs, opts)
	return DryRunResult{Trace: traceOf(interp, steps)}, nil
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

func (au *authoring) TestNode(ctx context.Context, a Actor, in TestNodeInput) (TestNodeResult, error) {
	if err := au.r.service.authorize(a, PermAuthor); err != nil {
		return TestNodeResult{}, err
	}
	v, interp, err := au.authorGraph(ctx, a, in.Graph)
	if err != nil {
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
	gctx, err := au.r.BuildRunContext(ctx, RunContextInput{TenantID: a.TenantID, Subject: subject, RunAs: sub, Event: event, Now: time.Now(), Overrides: overrides})
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
		if len(st.Effects) > 0 {
			params = st.Effects[0].Params
		}
		dr := au.r.execDirect(ctx, directCall{tenant: a.TenantID, as: sub, view: v.view, node: node, params: params,
			subject: subject, key: "test:" + uuid.NewString(), decisions: DecisionsLive})
		res.Outcome, res.Output, res.Error = dr.outcome, dr.output, dr.err
		if dr.masker != nil {
			masker = dr.masker
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
			applied, serr, err := au.r.effectRolledBack(ctx, a.TenantID, sub, subject, node, interp.View().Catalog().Nodes[node.Type], st.Effects)
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

// dryRunTrigger is the event a dry run starts on: the first event input that names its kind.
func dryRunTrigger(inputs []graph.DryRunInput) *TriggerEvent {
	for _, in := range inputs {
		ev := in.Event
		if len(in.Events) > 0 {
			ev = in.Events[len(in.Events)-1]
		}
		kind, _ := ev["kind"].(string)
		if kind == "" {
			continue
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
		return &TriggerEvent{Key: kind, ID: "dry-run", Payload: raw}
	}
	return nil
}

// previewEffects is the answer of a node that previews its effect (EffectPreviewer) in a dry run; ok is false
// for a node that does not.
func (r *Runtime) previewEffects(ctx context.Context, tenant uuid.UUID, as spi.Subject, subject SubjectRef, trigger *TriggerEvent, node graph.Node, effects []graph.Effect) (graph.ExecAnswer, bool) {
	nt, _ := r.cfg.Catalog.Node(node.Type)
	pv, ok := nt.(EffectPreviewer)
	if !ok || len(effects) == 0 {
		return graph.ExecAnswer{}, false
	}
	sc := StepContext{TenantID: tenant, RunAs: as, Subject: subject, NodeID: node.ID, Trigger: trigger}
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
func (r *Runtime) effectRolledBack(ctx context.Context, tenant uuid.UUID, as spi.Subject, subject SubjectRef, node graph.Node, spec graph.NodeSpec, effects []graph.Effect) (EffectResult, *StepError, error) {
	nt, _ := r.cfg.Catalog.Node(node.Type)
	en, ok := nt.(EffectNode)
	if !ok {
		// As in a run: an effect node without an executor applies nothing, so its test says so.
		return EffectResult{}, &StepError{Code: CodeUnsupported, Message: fmt.Sprintf("node type %s has no effect executor", node.Type)}, nil
	}
	tx, err := r.cfg.DB.Begin(ctx)
	if err != nil {
		return EffectResult{}, nil, serrors.FromDB(opAuthoring, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	sc := StepContext{TenantID: tenant, RunAs: as, Subject: subject, NodeID: node.ID}
	return applyEach(ctx, tx, sc, node, spec, en, effects)
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
	subject   SubjectRef
	key       string
	decisions string
}

type directResult struct {
	outcome string
	output  json.RawMessage
	err     *StepError
	masker  Masker // the connection's: what the node logged is masked with it
}

// execDirect runs a call node once, as the actor, with the connection the actor may use; the output and
// errors are masked with its credentials. Asker nodes send and return the send's result.
func (r *Runtime) execDirect(ctx context.Context, c directCall) directResult {
	nt, ok := r.cfg.Catalog.Node(c.node.Type)
	cn, isCall := nt.(catalog.CallNode)
	if !ok || !isCall {
		return directResult{err: &StepError{Code: CodeUnsupported, Message: c.node.Type + " has no call executor"}}
	}
	if perm, ok := cn.(catalog.PermissionedNode); ok && perm.Permission() != "" && !c.as.Has(perm.Permission()) {
		return directResult{err: &StepError{Code: CodeForbidden, Message: "you may not use " + c.node.Type}}
	}
	var info *catalog.ConnectionInfo
	var creds catalog.Credentials
	if kinds := c.view.NodeRequires[c.node.Type]; len(kinds) > 0 {
		ref, _, err := catalog.ResolveConnection(c.view.Connections, kinds, c.params[catalog.ConnectionParam])
		if err != nil || r.cfg.Credentials == nil {
			return directResult{err: &StepError{Code: CodeConnectionUnavailable, Message: "the connection of " + c.node.ID + " is not available"}}
		}
		ci, cr, err := r.cfg.Credentials.Open(ctx, c.tenant, ref.ID)
		switch {
		case errors.Is(err, connectors.ErrNotFound), errors.Is(err, connectors.ErrNotConnected), errors.Is(err, connectors.ErrNeedsReauth), errors.Is(err, connectors.ErrUndecryptable):
			return directResult{err: &StepError{Code: CodeConnectionUnavailable, Message: "the connection is not connected"}}
		case err != nil:
			return directResult{err: &StepError{Code: CodeConnectionUnavailable, Message: "the connection could not be opened"}}
		}
		info, creds = &ci, cr
	}
	masker := code.NewMasker(map[string]string(creds))
	timeout := r.cfg.CallLimits.DefaultTimeout
	if ms, ok := graph.DurationMs(c.params["timeout"]); ok {
		timeout = min(time.Duration(ms)*time.Millisecond, r.cfg.CallLimits.MaxTimeout)
	}
	deadline := time.Now().Add(timeout)
	in := catalog.CallInput{
		TenantID: c.tenant, NodeID: c.node.ID, Attempt: 1, IdempotencyKey: c.key, Params: c.params, Subject: c.subject,
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
					return directResult{masker: masker, outcome: d.Outcome, output: nonNull(d.Output)}
				}
			}
		}
	}
	callCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	if _, asks := cn.(Asker); asks {
		// A test sends the question with a token that no wait listens for: an answer finds it closed.
		callCtx = WithCorrelation(callCtx, Correlation{Token: NewCorrelationToken()})
	}
	res, err := cn.Execute(callCtx, in)
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			// As in a run's call: the code is kept only as an identifier with nothing a mask hides.
			return directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: callCode(ce.Code, masker), Message: masker.Mask(ce.Message), Retryable: ce.Retryable}}
		}
		if callCtx.Err() != nil {
			return directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: CodeTimeout, Message: c.node.ID + " got no answer by its deadline"}}
		}
		return directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: "exception", Message: masker.Mask(err.Error())}}
	}
	if res.RetryAfter > 0 {
		// The provider asked to repeat later: a refusal, as in a run (and never a cached decision).
		return directResult{masker: masker, outcome: nt.Spec().FailedOutcome, err: &StepError{Code: CodeRateLimited,
			Message: fmt.Sprintf("the provider asked to repeat the call in %s", res.RetryAfter), Retryable: true}}
	}
	out, err := json.Marshal(nonNilMap(res.Output))
	if err != nil {
		return directResult{masker: masker, err: &StepError{Code: graph.FailInvalidAnswer, Message: "the output is not JSON"}}
	}
	out = masker.MaskJSON(out)
	if cacheKey != "" {
		if b, err := json.Marshal(map[string]any{"outcome": res.Outcome, "output": out}); err == nil {
			if err := r.cfg.Decisions.Put(ctx, c.tenant, cacheKey, b); err != nil {
				r.cfg.Logger.Warn("automation: a decision was not cached", "error", err)
			}
		}
	}
	return directResult{masker: masker, outcome: res.Outcome, output: out}
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

// Log writes a line to the log of a node test (TestNodeResult.Logs); outside a test it does nothing. A
// node never logs credentials.
func Log(ctx context.Context, format string, args ...any) {
	if l, ok := ctx.Value(logsKey{}).(*logCollector); ok {
		l.add(fmt.Sprintf(format, args...))
	}
}
