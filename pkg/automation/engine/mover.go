package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/automation/state"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// mover moves the tokens of a run inside one fenced transaction. It mirrors graph.DryRun, durably: a
// token's context is its ancestors' writes and its own, a fork parks the forking token until its join, a
// foreach parks it until its items end.
type mover struct {
	x    *execution
	tx   pgx.Tx
	s    *stepState
	vars map[string]any
	// varsHash is a hash of the subject's variables this step read with the run-as's rights now: a
	// preparation made for the step is reused only while it is the same.
	varsHash string
	// memory is what this step reads the automation's memory through: a preparation is reused only while
	// the keys the step that asked for it read are what they were.
	memory *state.Reader
}

func (m *mover) in() *graph.Interpreter { return m.x.ie.in }

func (m *mover) spec(nodeID string) (graph.Node, graph.NodeSpec, bool) {
	node, ok := m.in().View().Node(nodeID)
	if !ok {
		return node, graph.NodeSpec{}, false
	}
	spec, ok := m.x.ie.specs[node.Type]
	return node, spec, ok
}

// stepErr is the error of a stopping step of the interpreter.
func stepErr(st graph.Step) StepError {
	if st.Kind == graph.StepMissingContext {
		return StepError{Code: CodeMissingContext, Message: st.Message, Path: st.Path}
	}
	switch st.Code {
	case graph.FailValueTooLarge, graph.FailLimit:
		return StepError{Code: CodeLimitExceeded, Message: st.Message}
	}
	code := st.Code
	if code == "" {
		code = graph.FailInvalidGraph
	}
	return StepError{Code: code, Message: st.Message}
}

// enter takes the step of a running token on the node it is at.
func (m *mover) enter(ctx context.Context, tok *tokenRow) (string, error) {
	in := m.in()
	if tok.NodeID == graph.Ignore {
		if _, err := m.record(ctx, tok, graph.Ignore, "", nil, graph.TraceEnded, nil, nil, true); err != nil {
			return "", err
		}
		return "ignore", m.finish(ctx, tok, false, nil, "ended at "+graph.Ignore, nil)
	}
	node, spec, ok := m.spec(tok.NodeID)
	if !ok {
		return "", m.fail(ctx, tok, tok.NodeID, 0, StepError{Code: graph.FailInvalidGraph, Message: "no node " + tok.NodeID})
	}
	behavior := string(spec.Behavior)
	if m.x.ie.unavailable[node.Type] {
		// A connection that waits for a new sign-in left the catalog: the node is unauthorized.
		if nt, ok := m.x.r.cfg.Catalog.Node(node.Type); ok && needsReauth(ctx, m.x.r.cfg.DB, m.x.tenant, node.Params[catalog.ConnectionParam], nt.Requires()) {
			return behavior, m.nodeFailed(ctx, tok, node.ID, behavior, 0, nil, StepError{Code: catalog.CallUnauthorized, Message: msgNeedsReauth})
		}
		// The node names the platform credential, or the platform credential is the one it would use, and
		// the tenant's administrator has not enabled it or switched it off after the run started.
		if ref, _ := node.Params[catalog.ConnectionParam].(string); strings.HasPrefix(ref, catalog.PlatformPrefix) || (ref == "" && m.x.ie.platformOff[node.Type]) {
			return behavior, m.nodeFailed(ctx, tok, node.ID, behavior, 0, nil, StepError{Code: catalog.CallUnavailable, Message: platformOffMessage})
		}
		return behavior, m.nodeFailed(ctx, tok, node.ID, behavior, 0, nil, StepError{Code: CodeConnectionUnavailable,
			Message: fmt.Sprintf("%s needs a connection the tenant does not have", node.Type)})
	}
	gctx, err := m.context(ctx, tok)
	if err != nil {
		return behavior, err
	}
	m.varsHash = hashVars(gctx.Vars)
	m.memory, _ = gctx.State.(*state.Reader)
	active, err := m.activeTokens(ctx)
	if err != nil {
		return behavior, err
	}
	st := in.Start(node.ID, &graph.StepContext{Ctx: gctx, Token: tok.Path, From: tok.From, Tokens: active})
	if err := memoryFailure(ctx, gctx); err != nil {
		return behavior, err
	}
	st.Effects = m.preparedEffects(tok, node.ID, st.Effects)
	switch st.Kind {
	case graph.StepFailed, graph.StepMissingContext:
		if stopsRun(st) {
			return behavior, m.stop(ctx, tok, node.ID, behavior, 0, nil, stepErr(st))
		}
		return behavior, m.nodeFailed(ctx, tok, node.ID, behavior, 0, nil, stepErr(st))
	case graph.StepEnd:
		if _, err := m.record(ctx, tok, node.ID, behavior, nil, graph.TraceEnd, st.End, nil, true); err != nil {
			return behavior, err
		}
		reason, _ := st.End["reason"].(string)
		if reason == "" {
			reason = "ended at " + node.ID
		}
		return behavior, m.finish(ctx, tok, false, nil, reason, st.End)
	case graph.StepCall:
		return behavior, m.call(ctx, tok, node, spec, st)
	case graph.StepWait:
		return behavior, m.wait(ctx, tok, node, spec, st)
	case graph.StepFork:
		return behavior, m.fork(ctx, tok, node, st)
	case graph.StepEach:
		return behavior, m.each(ctx, tok, node, st)
	case graph.StepArrive:
		if _, err := m.record(ctx, tok, node.ID, behavior, nil, graph.TraceArrived, map[string]any{"value": st.Arrive.Value}, nil, true); err != nil {
			return behavior, err
		}
		return behavior, m.finish(ctx, tok, true, st.Arrive.Value, "", nil)
	case graph.StepOutcome:
	}
	output := st.Output
	outcome := st.Outcome
	var input any
	if len(st.Effects) > 0 {
		input = effectInput(st.Effects)
		res, serr, err := m.applyEffects(ctx, tok, node, spec, st.Effects)
		if err != nil {
			return behavior, err
		}
		if serr != nil {
			return behavior, m.nodeFailed(ctx, tok, node.ID, behavior, 0, input, *serr)
		}
		if res.Output != nil {
			output = res.Output
		}
		if res.Outcome != "" {
			outcome = res.Outcome
		}
	}
	if output != nil {
		// jsonb refuses U+0000: a value with a NUL byte (a CEL string) keeps the rest of its text.
		output, _ = replaceNUL(output).(map[string]any)
	}
	if _, err := m.record(ctx, tok, node.ID, behavior, input, outcome, output, nil, true); err != nil {
		return behavior, err
	}
	if output != nil {
		tok.set(node.ID, graph.NodeState{Output: output})
	}
	return behavior, m.advance(ctx, tok, node.ID, outcome)
}

// preparedEffects are the effects the step applies: those its preparation was made for, when the step is the
// one prepared (the same token, step number and node) and what the run-as may read through the subject's
// variables is unchanged since — so a right lost in between is honored by the fresh evaluation, never bypassed
// by an effect rendered earlier — and so are the keys of the memory the preparing step read, else the ones the fresh evaluation rendered. A step whose fresh evaluation
// renders no effects (it failed) takes its failure, as it would without a preparation: the preparation read
// and wrote nothing, so nothing is lost.
func (m *mover) preparedEffects(tok *tokenRow, node string, fresh []graph.Effect) []graph.Effect {
	p := m.x.preparedStepFor(tok.ID, tok.StepSeq+1, node)
	if p == nil || len(fresh) == 0 || len(p.Effects) != len(fresh) || !m.fits(p) {
		return fresh
	}
	return p.Effects
}

// fits reports that what a preparation was made over is what the step reads now: the subject's variables and
// the keys of the memory.
func (m *mover) fits(p *preparedStep) bool {
	return p.varsHash == m.varsHash && m.memory.Same(p.memory)
}

// maxPrepareAttempts bounds how many times a step is prepared again because what it reads changed in between.
const maxPrepareAttempts = 3

// preparedFor is the preparation the effect node of this step gets: the one made for it, while the subject's
// variables the run-as may read and the memory keys read are what they were when it was made. A preparation that no longer fits is not
// handed over: the step asks for a new one, outside the transaction. After maxPrepareAttempts the preparation is
// marked Exhausted and the step is not applied but retried later (a platform error), so a node that asked for a
// check never acts without it.
func (m *mover) preparedFor(tok *tokenRow, seq int) *PreparedEffect {
	p := m.x.preparedStepFor(tok.ID, seq, tok.NodeID)
	switch {
	case p == nil:
		return nil
	case m.fits(p):
		return &p.PreparedEffect
	case p.attempts >= maxPrepareAttempts:
		return &PreparedEffect{Exhausted: true}
	}
	return nil
}

// hashVars fingerprints the subject's variables.
func hashVars(vars map[string]any) string {
	b, err := json.Marshal(vars)
	if err != nil {
		return "unhashable"
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func effectInput(effects []graph.Effect) any {
	if len(effects) == 1 {
		return effects[0].Params
	}
	return effects
}

// applyEffects applies a node's effects inside the step's transaction. They share one savepoint that is
// held until the node's output is checked: a node failure (an effect's error, an output over
// PayloadBytes) rolls every effect of the node back before the failed outcome is taken, so a handler on
// that edge never sees a write of the failed node. A failure of the platform aborts the step (err); any
// other is the node's.
func (m *mover) applyEffects(ctx context.Context, tok *tokenRow, node graph.Node, spec graph.NodeSpec, effects []graph.Effect) (EffectResult, *StepError, error) {
	nt, ok := m.x.r.cfg.Catalog.Node(node.Type)
	en, isEffect := nt.(EffectNode)
	if !ok || !isEffect {
		return EffectResult{}, &StepError{Code: CodeUnsupported, Message: fmt.Sprintf("node type %s has no effect executor", node.Type)}, nil
	}
	sp, err := m.tx.Begin(ctx)
	if err != nil {
		return EffectResult{}, nil, serrors.FromDB(opRun, err)
	}
	res, serr, err := applyEach(ctx, sp, m.stepContext(tok, tok.StepSeq+1), node, spec, en, effects, true)
	var np *needsPrepareError
	if errors.As(err, &np) {
		np.varsHash = m.varsHash
		if m.memory != nil {
			np.memory = m.memory.Snapshot()
		}
	}
	if err == nil && serr == nil && res.Output != nil {
		serr = m.outputTooLarge(node.ID, res.Output, "the output of "+node.ID)
	}
	if err != nil || serr != nil {
		if rerr := sp.Rollback(ctx); rerr != nil && err == nil {
			return EffectResult{}, nil, serrors.FromDB(opRun, rerr)
		}
		return EffectResult{}, serr, err
	}
	if err := sp.Commit(ctx); err != nil {
		return EffectResult{}, nil, serrors.FromDB(opRun, err)
	}
	return res, nil, nil
}

// applyEach applies the effects in order in the node's savepoint; the result is the last effect's. The
// outcome is empty unless the node picks its own (OutcomeEffectNode).
//
// A failure of the platform (an error host.IsInfrastructure names, or a cancelled context) is returned as an
// error when platformErrors is set, so the step is retried; a node test sets it off and shows every error
// as the node's.
func applyEach(ctx context.Context, sp pgx.Tx, sc StepContext, node graph.Node, spec graph.NodeSpec, en EffectNode, effects []graph.Effect, platformErrors bool) (EffectResult, *StepError, error) {
	var out EffectResult
	for _, e := range effects {
		o, err := applyOne(ctx, sp, sc, node, spec, en, e)
		if err != nil {
			var np *needsPrepareError
			if errors.As(err, &np) {
				np.effects = effects
				return EffectResult{}, nil, err
			}
			if platformErrors && (host.IsInfrastructure(err) || ctx.Err() != nil) {
				return EffectResult{}, nil, err
			}
			var se *StepError
			if errors.As(err, &se) {
				return EffectResult{}, se, nil
			}
			return EffectResult{}, &StepError{Code: CodeEffectFailed, Message: err.Error()}, nil
		}
		if o.Outcome != "" || o.Output != nil {
			out = o
		}
	}
	return out, nil, nil
}

// applyOne applies one effect; a node that picks its outcome must pick one it declares.
func applyOne(ctx context.Context, tx pgx.Tx, sc StepContext, node graph.Node, spec graph.NodeSpec, en EffectNode, e graph.Effect) (EffectResult, error) {
	if prep, ok := en.(EffectPreparer); ok && sc.Prepared == nil && sc.RunID != uuid.Nil {
		// The step is rolled back and prepared outside the transaction first (a node test prepares itself).
		return EffectResult{}, &needsPrepareError{preparer: prep, sc: sc, node: node, effect: e}
	}
	if sc.Prepared != nil && sc.Prepared.Exhausted {
		return EffectResult{}, &host.InfraError{Err: fmt.Errorf("%s: what the step reads kept changing while it was prepared; it is tried again later", node.ID)}
	}
	pick, picks := en.(OutcomeEffectNode)
	if !picks {
		o, err := en.ApplyEffect(ctx, tx, sc, node, e)
		return EffectResult{Output: o}, err
	}
	res, err := pick.ApplyEffectResult(ctx, tx, sc, node, e)
	if err != nil {
		return EffectResult{}, err
	}
	if res.Outcome != "" && !slices.Contains(spec.Outcomes, res.Outcome) {
		return EffectResult{}, &StepError{Code: CodeEffectFailed, Message: fmt.Sprintf("%s took the outcome %q it does not declare", node.Type, res.Outcome)}
	}
	if res.Outcome != "" && res.Outcome == spec.FailedOutcome {
		// A failure is an error: it rolls the effect back and carries output.error.
		return EffectResult{}, &StepError{Code: CodeEffectFailed, Message: fmt.Sprintf("%s took its failed outcome %q without an error", node.Type, res.Outcome)}
	}
	return res, nil
}

// outputTooLarge is tooLarge for a node's output: within the node type's MaxOutputBytes when it declares
// more than the payload limit.
func (m *mover) outputTooLarge(nodeID string, v any, what string) *StepError {
	most := m.x.r.cfg.Limits.PayloadBytes
	if _, spec, ok := m.spec(nodeID); ok && spec.MaxOutputBytes > most {
		most = spec.MaxOutputBytes
	}
	return tooLargeFor(v, what, most)
}

func (m *mover) tooLarge(v any, what string) *StepError {
	return tooLargeFor(v, what, m.x.r.cfg.Limits.PayloadBytes)
}

func tooLargeFor(v any, what string, most int) *StepError {
	b, err := json.Marshal(v)
	if err != nil {
		return &StepError{Code: graph.FailInvalidInput, Message: fmt.Sprintf("%s is not JSON: %v", what, err)}
	}
	if len(b) > most {
		return &StepError{Code: CodeLimitExceeded, Message: fmt.Sprintf("%s has %d bytes, at most %d", what, len(b), most)}
	}
	return nil
}

func (m *mover) stepContext(tok *tokenRow, seq int) StepContext {
	return StepContext{
		TenantID: m.x.tenant, RunID: m.x.run.ID, TokenID: tok.ID, Seq: seq,
		AutomationID: m.x.run.AutomationID, Version: m.x.run.Version,
		RunAs: m.x.subject, Subject: m.x.run.Subject, Depth: m.x.run.Depth, Native: m.x.nr,
		NodeID: tok.NodeID, Trigger: triggerEventOf(m.x.run), Prepared: m.preparedFor(tok, seq),
	}
}

// triggerEventOf is the event a run started on: nil when the run has none (its input has no kind).
func triggerEventOf(run runRow) *TriggerEvent {
	input, err := decodeMap(run.Input)
	if err != nil {
		return nil
	}
	id := ""
	if run.TriggerEventID != nil {
		id = *run.TriggerEventID
	}
	t := triggerOfEvent(input, id)
	if t != nil && run.InitiatorID != nil {
		t.ActorUserID = *run.InitiatorID
	}
	return t
}

// call registers a call or a human task as a wait: the executor starts it, its result comes later.
func (m *mover) call(ctx context.Context, tok *tokenRow, node graph.Node, spec graph.NodeSpec, st graph.Step) error {
	behavior := string(spec.Behavior)
	input := effectInput(st.Effects)
	nt, _ := m.x.r.cfg.Catalog.Node(node.Type)
	human := false
	if h, ok := nt.(HumanNode); ok && h.Human() {
		human = true
	}
	var begin func(ctx context.Context, tx pgx.Tx, run StepContext, node graph.Node, input json.RawMessage) (WaitSpec, error)
	kind := waitCall
	switch {
	case human && m.x.r.cfg.Tasks != nil:
		begin, kind = m.x.r.cfg.Tasks.Begin, waitTask
	case !human && m.x.r.cfg.Calls != nil:
		begin = m.x.r.cfg.Calls.Begin
	default:
		return m.nodeFailed(ctx, tok, node.ID, behavior, 0, input, StepError{Code: CodeConnectionUnavailable, Message: node.Type + " cannot run in this process"})
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	raw = withoutNUL(raw) // the call's request is a workflow input (jsonb)
	if len(raw) > m.x.r.cfg.Limits.PayloadBytes {
		return m.nodeFailed(ctx, tok, node.ID, behavior, 0, input, StepError{Code: CodeLimitExceeded, Message: "the request of " + node.ID + " is too large"})
	}
	seq := tok.StepSeq + 1
	sp, err := m.tx.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	sc := m.stepContext(tok, seq)
	if len(st.Effects) > 0 {
		sc.Context = st.Effects[0].Context
	}
	if fo := m.in().FailedOutcome(node.ID); fo != "" {
		// Connected to a node that handles the failure: an edge to ignore (or none, inside a construct)
		// would end the path silently on a call that may have been made, so such a call blocks instead.
		to, ok := m.in().Next(node.ID, fo)
		sc.FailedConnected = ok && to != graph.Ignore
	}
	ws, err := begin(ctx, sp, sc, node, raw)
	if err != nil {
		_ = sp.Rollback(ctx)
		if host.IsInfrastructure(err) || ctx.Err() != nil {
			return err
		}
		var se *StepError
		if !errors.As(err, &se) {
			se = &StepError{Code: CodeEffectFailed, Message: err.Error()}
		}
		return m.nodeFailed(ctx, tok, node.ID, behavior, 0, input, *se)
	}
	if err := sp.Commit(ctx); err != nil {
		return serrors.FromDB(opRun, err)
	}
	if ws.WaitID == "" {
		ws.WaitID = fmt.Sprintf("%s/%d", tok.ID, seq)
	}
	if ws.Kind != "" {
		kind = ws.Kind
	}
	if kind != waitCall && kind != waitTask {
		kind = waitCall
	}
	if _, err := m.record(ctx, tok, node.ID, behavior, input, "", nil, nil, false); err != nil {
		return err
	}
	w := waitRow{RunID: m.x.run.ID, TokenID: tok.ID, WaitID: ws.WaitID, Kind: kind, NodeID: node.ID, Spec: waitSpecJSON{Seq: seq, Exec: &ws}}
	if t := m.capWait(ws.Timeout); t > 0 {
		at, err := m.deadline(ctx, t)
		if err != nil {
			return err
		}
		w.TimeoutAt, w.Spec.Timer = &at, true
	}
	if err := insertWait(ctx, m.tx, m.x.tenant, w); err != nil {
		return err
	}
	tok.Status = tokWaiting
	return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok)
}

// deadline is when a wait registered now for d ends: counted from the database's clock at the
// registration, so neither the subject's lock nor the step's work before it (loading its context,
// an executor's call) eats into the wait.
func (m *mover) deadline(ctx context.Context, d time.Duration) (time.Time, error) {
	var now time.Time
	if err := m.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return time.Time{}, serrors.FromDB(opRun, err)
	}
	return now.Add(d), nil
}

func (m *mover) capWait(d time.Duration) time.Duration {
	if most := m.x.r.cfg.Limits.WaitMax; d > most {
		return most
	}
	return d
}

// wait registers a wait for an event (with its filter and timeout) or a delay.
func (m *mover) wait(ctx context.Context, tok *tokenRow, node graph.Node, spec graph.NodeSpec, st graph.Step) error {
	behavior := string(spec.Behavior)
	var input any
	params, _ := m.in().View().Params(node.ID)
	var effectOut map[string]any
	if len(st.Effects) > 0 {
		input = effectInput(st.Effects)
		params = st.Effects[0].Params
		res, serr, err := m.applyEffects(ctx, tok, node, spec, st.Effects)
		if err != nil {
			return err
		} else if serr != nil {
			return m.nodeFailed(ctx, tok, node.ID, behavior, 0, input, *serr)
		}
		effectOut = res.Output
	}
	wi := st.Wait
	kind := waitEvent
	if wi.Event == "" {
		kind = waitTimer
	}
	// A node type that names the conversation it waits in (WaitKeyer) listens there or nowhere: a run that
	// cannot say which conversation (the connection is not the tenant's own, cannot be resolved) never
	// falls back to the run's own conversation, or to none, where another chat's message could answer it.
	var keyed *SubjectRef
	if kind == waitEvent {
		var serr *StepError
		var err error
		if keyed, serr, err = m.keyedSubject(ctx, node, params); err != nil {
			return err // the catalog could not be read: the step is tried again, the run is not failed for it
		}
		if serr != nil {
			return m.nodeFailed(ctx, tok, node.ID, behavior, 0, input, *serr)
		}
	}
	timeout := m.capWait(wi.Timeout)
	if kind == waitEvent && timeout == 0 {
		timeout = m.x.r.cfg.Limits.WaitMax // a wait never outlives WaitMax
	}
	seq, err := m.record(ctx, tok, node.ID, behavior, input, "", nil, nil, false)
	if err != nil {
		return err
	}
	// As for a subscription's watermark (events.Watermark): an event whose seq was drawn before the
	// wait opened is an event before it, whatever its commit order.
	after, err := events.Watermark(ctx, m.tx)
	if err != nil {
		return err
	}
	w := waitRow{RunID: m.x.run.ID, TokenID: tok.ID, WaitID: fmt.Sprintf("%s/%d", tok.ID, seq), Kind: kind, NodeID: node.ID, AfterSeq: after, Spec: waitSpecJSON{Seq: seq}}
	if kind == waitEvent {
		ev := wi.Event
		w.EventKind = &ev
		if wi.Filter != "" {
			f := wi.Filter
			w.Filter = &f
		}
		if keyed != nil {
			w.Subject = keyed
		} else {
			w.Subject = m.waitSubject(node, ev, params)
		}
		if nt, ok := m.x.r.cfg.Catalog.Node(node.Type); ok && effectOut != nil {
			if ek, ok := nt.(EffectWaitKeyer); ok {
				if s, ok := ek.EffectWaitSubject(params, effectOut); ok {
					w.Subject = &s
				}
			}
		}
	}
	if timeout > 0 {
		at, err := m.deadline(ctx, timeout)
		if err != nil {
			return err
		}
		w.TimeoutAt, w.Spec.Timer = &at, true
	}
	if err := insertWait(ctx, m.tx, m.x.tenant, w); err != nil {
		return err
	}
	tok.Status = tokWaiting
	return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok)
}

// keyedSubject is the conversation a wait of a WaitKeyer node listens on, nil for a node type that does not
// name one. The connection it is keyed by is the one in the params, or, when they leave it out, the one the
// node resolves to as a call does (the tenant's own one for a node that takes no platform credential). It
// is a step error when that connection or the conversation cannot be named; a failure to read the tenant's
// connections is an ordinary error (storage), which retries the step rather than failing the node.
func (m *mover) keyedSubject(ctx context.Context, node graph.Node, params map[string]any) (*SubjectRef, *StepError, error) {
	nt, ok := m.x.r.cfg.Catalog.Node(node.Type)
	if !ok {
		return nil, nil, nil
	}
	wk, keyed := nt.(WaitKeyer)
	if !keyed {
		return nil, nil, nil
	}
	unnamed := &StepError{Code: CodeConnectionUnavailable, Message: node.ID + " cannot name the conversation it waits in: its connection or its chat is not usable"}
	view, err := m.x.r.cfg.Catalog.ForTenant(ctx, m.x.tenant)
	if err != nil {
		return nil, nil, fmt.Errorf("the connections of the tenant could not be read for %s: %w", node.ID, err)
	}
	if kinds := view.NodeRequires[node.Type]; len(kinds) > 0 {
		connections := view.Connections
		if view.TenantOnlyNodes[node.Type] {
			connections = catalog.OwnConnections(connections)
		}
		ref, _, err := catalog.ResolveConnection(connections, kinds, params[catalog.ConnectionParam])
		if err != nil {
			return nil, unnamed, nil
		}
		out := make(map[string]any, len(params)+1)
		for k, v := range params {
			out[k] = v
		}
		out[catalog.ConnectionParam] = ref.ID
		params = out
	}
	s, ok := wk.WaitSubject(params)
	if !ok {
		return nil, unnamed, nil
	}
	return &s, nil, nil
}

// waitSubject is the subject a wait listens on: the run's when the event is about the run's entity, the
// node type's (WaitKeyer) when it names one, else none (match by kind and filter).
func (m *mover) waitSubject(node graph.Node, event string, params map[string]any) *SubjectRef {
	if nt, ok := m.x.r.cfg.Catalog.Node(node.Type); ok {
		if wk, ok := nt.(WaitKeyer); ok {
			if s, ok := wk.WaitSubject(params); ok {
				return &s
			}
		}
	}
	es, ok := m.x.ie.events[event]
	run := normSubject(m.x.run.Subject)
	if ok && run.Entity != graph.SubjectNone && es.Subject == run.Entity {
		return &run
	}
	return nil
}

// fork starts a token per branch of a parallel; the forking token waits for the join.
func (m *mover) fork(ctx context.Context, tok *tokenRow, node graph.Node, st graph.Step) error {
	f := &flowState{Kind: string(graph.BehaviorParallel), Node: node.ID, Join: m.x.ie.joinOf[node.ID],
		State: graph.JoinState{Arrived: map[string]graph.Arrival{}, Gone: map[string]bool{}}}
	branches := make([]string, 0, len(st.Fork))
	for _, fk := range st.Fork {
		seg := fk.Token[strings.LastIndex(fk.Token, "/")+1:]
		branch := strings.TrimPrefix(seg, node.ID+".")
		parent := tok.ID
		child := tokenRow{ID: uuid.New(), Parent: &parent, Path: fk.Token, NodeID: fk.Node, From: node.ID, Status: tokRunning, Branch: &branch}
		if err := insertToken(ctx, m.tx, m.x.tenant, m.x.run.ID, child); err != nil {
			return err
		}
		f.Children = append(f.Children, child.ID)
		branches = append(branches, branch)
	}
	if _, err := m.record(ctx, tok, node.ID, string(graph.BehaviorParallel), nil, "fork", map[string]any{"branches": branches}, nil, true); err != nil {
		return err
	}
	if st.Wait != nil && st.Wait.Timeout > 0 && f.Join != "" {
		at, err := m.deadline(ctx, m.capWait(st.Wait.Timeout))
		if err != nil {
			return err
		}
		w := waitRow{RunID: m.x.run.ID, TokenID: tok.ID, WaitID: fmt.Sprintf("%s/%d", tok.ID, tok.StepSeq), Kind: waitJoin, NodeID: f.Join,
			TimeoutAt: &at, Spec: waitSpecJSON{Seq: tok.StepSeq, Timer: true}}
		if err := insertWait(ctx, m.tx, m.x.tenant, w); err != nil {
			return err
		}
	}
	tok.Status, tok.Flow = tokWaiting, f
	return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok)
}

// each starts the first items of a foreach (at most its concurrency); the forking token waits for all.
func (m *mover) each(ctx context.Context, tok *tokenRow, node graph.Node, st graph.Step) error {
	target, ok := m.in().Next(node.ID, graph.OutcomeItem)
	if !ok {
		return m.fail(ctx, tok, node.ID, 0, StepError{Code: graph.FailInvalidGraph, Message: node.ID + " has no edge from item"})
	}
	items := st.Each.Items
	conc := st.Each.Concurrency
	if conc < 1 {
		conc = 1
	}
	f := &flowState{Kind: string(graph.BehaviorForeach), Node: node.ID, Count: len(items), Target: target, Left: len(items), Concurrency: conc}
	raw, err := json.Marshal(items)
	if err != nil {
		return err
	}
	if _, err := m.tx.Exec(ctx, `UPDATE automation.tokens SET items = $4 WHERE tenant_id = $1 AND run_id = $2 AND id = $3`, m.x.tenant, m.x.run.ID, tok.ID, raw); err != nil {
		return serrors.FromDB(opRun, err)
	}
	if _, err := m.record(ctx, tok, node.ID, string(graph.BehaviorForeach), nil, "each", map[string]any{"items": len(items)}, nil, true); err != nil {
		return err
	}
	for f.Next < len(items) && f.Next < conc {
		if err := m.startItem(ctx, tok, f, items[f.Next]); err != nil {
			return err
		}
	}
	tok.Status, tok.Flow = tokWaiting, f
	return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok)
}

func (m *mover) startItem(ctx context.Context, parent *tokenRow, f *flowState, item any) error {
	k := f.Next
	f.Next++
	raw, err := json.Marshal(item)
	if err != nil {
		return err
	}
	pid := parent.ID
	ref := f.Node
	child := tokenRow{ID: uuid.New(), Parent: &pid, Path: graph.ItemToken(parent.Path, f.Node, k), NodeID: f.Target, From: f.Node,
		Status: tokRunning, ForeachRef: &ref, ForeachIndex: &k, Item: raw}
	return insertToken(ctx, m.tx, m.x.tenant, m.x.run.ID, child)
}

// advance moves a token along the edge of an outcome.
func (m *mover) advance(ctx context.Context, tok *tokenRow, from, outcome string) error {
	tok.From = from
	next, ok := m.in().Next(from, outcome)
	if !ok {
		return m.noEdge(ctx, tok)
	}
	tok.NodeID, tok.Status = next, tokRunning
	return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok)
}

// noEdge ends a token on an outcome without an edge: inside a construct its branch does not arrive (its
// item is not ok); on the main path (a valid graph connects every outcome there) the run ends.
func (m *mover) noEdge(ctx context.Context, tok *tokenRow) error {
	reason := "an outcome of " + tok.From + " has no edge"
	for _, w := range tok.Writes {
		if w.Node == tok.From {
			if msg := graph.FailureMessage(w.State.Output); msg != "" {
				reason = msg
			}
		}
	}
	return m.finish(ctx, tok, false, nil, reason, nil)
}

// stopsRun reports a step that ends the whole run wherever its token is, as in a dry run: missing
// context, and a fork or a foreach over Limits.MaxTokens, whatever outcomes the node has.
func stopsRun(st graph.Step) bool {
	return st.Kind == graph.StepMissingContext || (st.Kind == graph.StepFailed && st.Code == graph.FailLimit)
}

// stop fails the token and the run, even inside a branch or an item.
func (m *mover) stop(ctx context.Context, tok *tokenRow, nodeID, behavior string, seq int, input any, e StepError) error {
	if seq > 0 {
		if err := m.closeStep(ctx, tok, seq, graph.TraceFailed, nil, &e); err != nil {
			return err
		}
	} else if _, err := m.record(ctx, tok, nodeID, behavior, input, graph.TraceFailed, nil, &e, true); err != nil {
		return err
	}
	tok.Status = tokFailed
	if err := saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok); err != nil {
		return err
	}
	return m.endRun(ctx, nil, &e)
}

// nodeFailed sends a node's failure down its failed outcome when its type has one, else fails the token.
// Missing context stops the run. seq > 0 closes the token's open step instead of recording one.
func (m *mover) nodeFailed(ctx context.Context, tok *tokenRow, nodeID, behavior string, seq int, input any, e StepError) error {
	if e.Code == CodeMissingContext {
		return m.stop(ctx, tok, nodeID, behavior, seq, input, e)
	}
	fo := m.in().FailedOutcome(nodeID)
	if fo == "" {
		return m.fail(ctx, tok, nodeID, seq, e, behavior, input)
	}
	var out map[string]any
	if m.in().StructuredFailure(nodeID) {
		// output.error {code, message, retryable}: templates of the failed branch read it. The message
		// was masked by the call that failed.
		out = graph.FailedOutput(e.Code, "", e.Retryable)
		out = graph.FailedOutput(e.Code, m.fitError(e.Message, out), e.Retryable)
	} else {
		out = map[string]any{"error": e.Message, "code": e.Code}
		out["error"] = m.fitError(e.Message, out)
	}
	if big := m.tooLarge(out, "the failed output of "+nodeID); big != nil {
		// Even the fixed fields do not fit: the run fails.
		return m.stop(ctx, tok, nodeID, behavior, seq, input, *big)
	}
	if seq > 0 {
		if err := m.closeStep(ctx, tok, seq, fo, out, &e); err != nil {
			return err
		}
	} else if _, err := m.record(ctx, tok, nodeID, behavior, input, fo, out, &e, true); err != nil {
		return err
	}
	tok.set(nodeID, graph.NodeState{Output: out})
	return m.advance(ctx, tok, nodeID, fo)
}

// fitError shortens a failure message so the failed output stays within PayloadBytes.
func (m *mover) fitError(message string, with map[string]any) string {
	out := map[string]any{}
	for k, v := range with {
		out[k] = v
	}
	structured, _ := out["error"].(map[string]any)
	fits := func(s string) bool {
		if structured != nil {
			e := map[string]any{}
			for k, v := range structured {
				e[k] = v
			}
			e["message"] = s
			out["error"] = e
		} else {
			out["error"] = s
		}
		return m.tooLarge(out, "") == nil
	}
	for !fits(message) && message != "" {
		cut := len(message) - len(message)/4 - 1
		for cut > 0 && !utf8.RuneStart(message[cut]) {
			cut--
		}
		message = message[:cut]
	}
	return message
}

// fail ends a token on a failure without a failed outcome: the run on the main path, only the token
// inside a branch or an item. extra is the behavior and input of the step to record.
func (m *mover) fail(ctx context.Context, tok *tokenRow, nodeID string, seq int, e StepError, extra ...any) error {
	behavior := ""
	var input any
	if len(extra) > 0 {
		behavior, _ = extra[0].(string)
	}
	if len(extra) > 1 {
		input = extra[1]
	}
	if seq > 0 {
		if err := m.closeStep(ctx, tok, seq, graph.TraceFailed, nil, &e); err != nil {
			return err
		}
	} else if _, err := m.record(ctx, tok, nodeID, behavior, input, graph.TraceFailed, nil, &e, true); err != nil {
		return err
	}
	tok.Status = tokFailed
	if err := saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok); err != nil {
		return err
	}
	if tok.Parent == nil {
		return m.endRun(ctx, nil, &e)
	}
	return m.inGroup(ctx, tok, false, nil, e.Message)
}

// finish ends a token: it arrived at a join or a collect with a value, or it ended without arriving.
// A main token ends the run with endOutput.
func (m *mover) finish(ctx context.Context, tok *tokenRow, arrived bool, value any, reason string, endOutput map[string]any) error {
	tok.Status = tokDone
	if arrived {
		tok.Status = tokJoined
	}
	tok.Flow = nil
	if err := saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok); err != nil {
		return err
	}
	if tok.Parent == nil {
		if endOutput == nil {
			endOutput = map[string]any{}
		}
		return m.endRun(ctx, endOutput, nil)
	}
	return m.inGroup(ctx, tok, arrived, value, reason)
}

// inGroup tells the forking token that one of its branches or items ended.
func (m *mover) inGroup(ctx context.Context, tok *tokenRow, arrived bool, value any, reason string) error {
	a := arrival{OK: arrived, Value: value}
	if !arrived {
		a.Error = m.fitError(reason, nil)
	}
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	if _, err := m.tx.Exec(ctx, `UPDATE automation.tokens SET arrival = $4 WHERE tenant_id = $1 AND run_id = $2 AND id = $3`, m.x.tenant, m.x.run.ID, tok.ID, raw); err != nil {
		return serrors.FromDB(opRun, err)
	}
	parent, err := loadToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok.Parent, true)
	if err != nil {
		return err
	}
	if parent.Flow == nil || tokenTerminal(parent.Status) {
		return nil // the group closed: a join any already continued, or the parent was cancelled
	}
	f := parent.Flow
	switch f.Kind {
	case string(graph.BehaviorParallel):
		branch := ""
		if tok.Branch != nil {
			branch = *tok.Branch
		}
		if arrived {
			f.State.Arrived[branch] = graph.Arrival{Of: f.Node, Token: tok.Path, Value: value}
		} else {
			f.State.Gone[branch] = true
		}
		return m.decideJoin(ctx, &parent)
	case string(graph.BehaviorForeach):
		f.Left--
		if f.Left <= 0 {
			return m.foreachDone(ctx, &parent)
		}
		if f.Next < f.Count {
			var item json.RawMessage
			if err := m.tx.QueryRow(ctx, `SELECT items -> $4::int FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND id = $3`,
				m.x.tenant, m.x.run.ID, parent.ID, f.Next).Scan(&item); err != nil {
				return serrors.FromDB(opRun, err)
			}
			var v any
			if err := expr.UnmarshalJSON(item, &v); err != nil {
				return err
			}
			if err := m.startItem(ctx, &parent, f, v); err != nil {
				return err
			}
		}
		return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, parent)
	}
	return nil
}

// decideJoin decides a parallel's join for the token that forked it.
func (m *mover) decideJoin(ctx context.Context, parent *tokenRow) error {
	f := parent.Flow
	if f.Join == "" {
		var live int
		if err := m.tx.QueryRow(ctx, `SELECT count(*) FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND id = ANY($3) AND status IN ('running', 'waiting')`,
			m.x.tenant, m.x.run.ID, f.Children).Scan(&live); err != nil {
			return serrors.FromDB(opRun, err)
		}
		if live > 0 {
			return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *parent)
		}
		return m.finish(ctx, parent, false, nil, "the branches of "+f.Node+" ended", nil)
	}
	st := m.in().Join(f.Join, parent.Path, f.State)
	switch st.Kind { //nolint:exhaustive // a join waits, fails or takes an outcome
	case graph.StepWait:
		return saveToken(ctx, m.tx, m.x.tenant, m.x.run.ID, *parent)
	case graph.StepFailed, graph.StepMissingContext:
		if err := m.cancel(ctx, parent, st.Cancel); err != nil {
			return err
		}
		if err := m.closeJoinWait(ctx, parent); err != nil {
			return err
		}
		join := f.Join
		parent.Flow = nil
		if stopsRun(st) {
			return m.stop(ctx, parent, join, string(graph.BehaviorJoin), 0, nil, stepErr(st))
		}
		return m.fail(ctx, parent, join, 0, stepErr(st), string(graph.BehaviorJoin))
	}
	if err := m.cancel(ctx, parent, st.Cancel); err != nil {
		return err
	}
	if err := m.closeJoinWait(ctx, parent); err != nil {
		return err
	}
	// Only what an arrived branch wrote since the fork, in the order of the branches.
	children, err := queryTokens(ctx, m.tx, `SELECT `+tokenCols+` FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND id = ANY($3) ORDER BY created_seq`,
		m.x.tenant, m.x.run.ID, f.Children)
	if err != nil {
		return err
	}
	for _, c := range children {
		if c.Branch == nil {
			continue
		}
		if _, ok := f.State.Arrived[*c.Branch]; !ok {
			continue
		}
		for _, w := range c.Writes {
			parent.set(w.Node, w.State)
		}
	}
	join := f.Join
	parent.Flow = nil
	parent.set(join, graph.NodeState{Output: st.Output})
	if _, err := m.record(ctx, parent, join, string(graph.BehaviorJoin), nil, st.Outcome, st.Output, nil, true); err != nil {
		return err
	}
	return m.advance(ctx, parent, join, st.Outcome)
}

// closeJoinWait cancels the timeout of a parallel's join.
func (m *mover) closeJoinWait(ctx context.Context, parent *tokenRow) error {
	ws, err := queryWaits(ctx, m.tx, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND token_id = $3 AND kind = 'join' AND status = 'open'`,
		m.x.tenant, m.x.run.ID, parent.ID)
	if err != nil {
		return err
	}
	for _, w := range ws {
		if err := closeWait(ctx, m.tx, m.x.tenant, w, waitCancelled); err != nil {
			return err
		}
	}
	return nil
}

// foreachDone continues the foreach's token once every item ended: done, partial_failed, or its failed
// outcome when the results do not fit.
func (m *mover) foreachDone(ctx context.Context, parent *tokenRow) error {
	f := parent.Flow
	rows, err := m.tx.Query(ctx, `SELECT foreach_index, arrival FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND parent_token_id = $3 AND foreach_ref = $4 ORDER BY foreach_index`,
		m.x.tenant, m.x.run.ID, parent.ID, f.Node)
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	var results []graph.ItemResult
	for rows.Next() {
		var idx int
		var raw []byte
		if err := rows.Scan(&idx, &raw); err != nil {
			rows.Close()
			return serrors.FromDB(opRun, err)
		}
		var a arrival
		if len(raw) > 0 {
			if err := expr.UnmarshalJSON(raw, &a); err != nil {
				rows.Close()
				return err
			}
		} else {
			a.Error = "the item did not end"
		}
		results = append(results, graph.ItemResult{Index: idx, OK: a.OK, Value: a.Value, Error: a.Error})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return serrors.FromDB(opRun, err)
	}
	node := f.Node
	parent.Flow = nil
	st := m.in().ForeachDone(node, results)
	if st.Kind == graph.StepFailed {
		// The output keeps the interpreter's code (VALUE_TOO_LARGE), as a dry run's does.
		return m.nodeFailed(ctx, parent, node, string(graph.BehaviorForeach), 0, nil, StepError{Code: st.Code, Message: st.Message})
	}
	parent.set(node, graph.NodeState{Output: st.Output})
	if _, err := m.record(ctx, parent, node, string(graph.BehaviorForeach), nil, st.Outcome, st.Output, nil, true); err != nil {
		return err
	}
	return m.advance(ctx, parent, node, st.Outcome)
}

// cancel ends live tokens named by their paths, and every token they started, with their waits.
func (m *mover) cancel(ctx context.Context, parent *tokenRow, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	rows, err := m.tx.Query(ctx, `
		UPDATE automation.tokens SET status = 'cancelled', flow = NULL, updated_at = now()
		WHERE tenant_id = $1 AND run_id = $2 AND status IN ('running', 'waiting') AND id <> $4
		  AND EXISTS (SELECT 1 FROM unnest($3::text[]) p WHERE path = p OR starts_with(path, p || '/'))
		RETURNING id`, m.x.tenant, m.x.run.ID, paths, parent.ID)
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	return m.cancelWaits(ctx, ids)
}

func (m *mover) cancelWaits(ctx context.Context, tokens []uuid.UUID) error {
	if len(tokens) == 0 {
		return nil
	}
	ws, err := queryWaits(ctx, m.tx, `SELECT `+waitCols+` FROM automation.waits WHERE tenant_id = $1 AND run_id = $2 AND token_id = ANY($3) AND status = 'open'`,
		m.x.tenant, m.x.run.ID, tokens)
	if err != nil {
		return err
	}
	for _, w := range ws {
		if err := closeWait(ctx, m.tx, m.x.tenant, w, waitCancelled); err != nil {
			return err
		}
		if _, err := m.tx.Exec(ctx, `UPDATE automation.steps SET outcome = $5, ended_at = now() WHERE tenant_id = $1 AND run_id = $2 AND token_id = $3 AND seq = $4 AND ended_at IS NULL`,
			m.x.tenant, m.x.run.ID, w.TokenID, w.Spec.Seq, graph.TraceCancelled); err != nil {
			return serrors.FromDB(opRun, err)
		}
	}
	return nil
}

// endRun ends the run: every live token is cancelled with its waits, and the run keeps its output (or its
// failure) for the workflow engine.
func (m *mover) endRun(ctx context.Context, output map[string]any, e *StepError) error {
	rows, err := m.tx.Query(ctx, `
		UPDATE automation.tokens SET status = 'cancelled', flow = NULL, updated_at = now()
		WHERE tenant_id = $1 AND run_id = $2 AND status IN ('running', 'waiting') RETURNING id`, m.x.tenant, m.x.run.ID)
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	if err := m.cancelWaits(ctx, ids); err != nil {
		return err
	}
	if e != nil {
		b, err := json.Marshal(e)
		if err != nil {
			return err
		}
		m.s.failure = b
		return nil
	}
	b, err := json.Marshal(output)
	if err != nil {
		return err
	}
	m.s.output = b
	return nil
}

// timedOut fires the timer of a wait.
func (m *mover) timedOut(ctx context.Context, tok *tokenRow, w waitRow) error {
	if err := closeWait(ctx, m.tx, m.x.tenant, w, waitTimedOut); err != nil {
		return err
	}
	switch w.Kind {
	case waitJoin:
		if tok.Flow == nil {
			return nil
		}
		tok.Flow.State.TimedOut = true
		return m.decideJoin(ctx, tok)
	case waitTask, waitCall, waitEvent:
		if w.Kind == waitEvent && w.Spec.Ask == nil {
			break
		}
		_, spec, _ := m.spec(w.NodeID)
		for _, o := range m.in().View().Outcomes(w.NodeID) {
			if o == CodeTimeout {
				return m.applyResult(ctx, tok, w, CodeTimeout, map[string]any{}, nil)
			}
		}
		return m.nodeFailed(ctx, tok, w.NodeID, string(spec.Behavior), w.Spec.Seq, nil, StepError{Code: CodeTimeout, Message: w.NodeID + " got no answer in time"})
	}
	outcome, output := m.in().Wake(w.NodeID, nil)
	if _, connected := m.in().Next(w.NodeID, outcome); !connected {
		// A wait without a timeout of its own reached WaitMax: the node fails, the run does not end
		// on an outcome the graph does not draw.
		_, spec, _ := m.spec(w.NodeID)
		return m.nodeFailed(ctx, tok, w.NodeID, string(spec.Behavior), w.Spec.Seq, nil,
			StepError{Code: CodeTimeout, Message: fmt.Sprintf("%s waited the longest a wait may (%s)", w.NodeID, m.x.r.cfg.Limits.WaitMax)})
	}
	return m.applyResult(ctx, tok, w, outcome, output, nil)
}

// wakeEvent wakes a wait with a matched event, when its filter still matches in the token's context.
func (m *mover) wakeEvent(ctx context.Context, tok *tokenRow, w waitRow, ev inboxEvent) error {
	if !ev.OccurredAt.IsZero() && w.TimeoutAt != nil && ev.OccurredAt.After(*w.TimeoutAt) {
		// It came after the deadline: the timer decides the wait, and nothing of the graph is read for it.
		return nil
	}
	payload, err := decodeMap(ev.Payload)
	if err != nil {
		return err
	}
	// The signal was projected for the rights the run-as had when it was routed; the step reads only
	// what it may read now, and remembers which sensitive fields it read.
	proj := m.x.r.projector(m.x.tenant)
	if payload, err = proj.project(ctx, ev.Kind, payload, &m.x.subject); err != nil {
		return &host.InfraError{Err: err}
	}
	refs, err := proj.sensitiveIn(ctx, ev.Kind, payload)
	if err != nil {
		return &host.InfraError{Err: err}
	}
	gev := graph.Event{}
	for k, v := range payload {
		gev[k] = v
	}
	gev["kind"] = ev.Kind
	if w.Spec.Ask != nil {
		var changed bool
		m.s.sensitive, changed = mergeRefs(m.s.sensitive, refs)
		m.s.sensitiveChanged = m.s.sensitiveChanged || changed
		return m.answer(ctx, tok, w, gev)
	}
	gctx, err := m.context(ctx, tok)
	if err != nil {
		return err
	}
	if !ev.OccurredAt.IsZero() {
		// A wait's filter reads `now` as the event's time, as the router did when it routed it.
		gctx.Now = ev.OccurredAt
	}
	matched, st := m.in().WaitMatches(w.NodeID, gev, gctx)
	if err := memoryFailure(ctx, gctx); err != nil {
		return err
	}
	if st != nil {
		if err := closeWait(ctx, m.tx, m.x.tenant, w, waitCancelled); err != nil {
			return err
		}
		if stopsRun(*st) {
			return m.stop(ctx, tok, w.NodeID, "", w.Spec.Seq, nil, stepErr(*st))
		}
		return m.fail(ctx, tok, w.NodeID, w.Spec.Seq, stepErr(*st))
	}
	if !matched {
		return nil
	}
	if err := closeWait(ctx, m.tx, m.x.tenant, w, waitFired); err != nil {
		return err
	}
	var changed bool
	m.s.sensitive, changed = mergeRefs(m.s.sensitive, refs)
	m.s.sensitiveChanged = m.s.sensitiveChanged || changed
	if err := m.withheldOutput(ctx, gev); err != nil {
		return err
	}
	outcome, output := m.in().Wake(w.NodeID, gev)
	if node, _, ok := m.spec(w.NodeID); ok {
		if nt, ok := m.x.r.cfg.Catalog.Node(node.Type); ok {
			if wr, ok := nt.(WakeResolver); ok {
				o, out := wr.ResolveWake(gev)
				if !slices.Contains(m.in().View().Outcomes(w.NodeID), o) {
					_, spec, _ := m.spec(w.NodeID)
					return m.nodeFailed(ctx, tok, w.NodeID, string(spec.Behavior), w.Spec.Seq, nil,
						StepError{Code: graph.FailUnknownOutcome, Message: fmt.Sprintf("%s has no outcome %q", w.NodeID, o)})
				}
				outcome, output = o, out
			}
		}
	}
	return m.applyResult(ctx, tok, w, outcome, output, nil)
}

// result applies the result of a call or a task.
func (m *mover) result(ctx context.Context, tok *tokenRow, w waitRow, wr WaitResult) error {
	if w.Kind == waitEvent || w.Kind == waitTimer || w.Kind == waitJoin {
		return nil // only calls and tasks take results
	}
	if err := closeWait(ctx, m.tx, m.x.tenant, w, waitFired); err != nil {
		return err
	}
	if len(wr.Logs) > 0 {
		// The call's step keeps what the node logged (masked by the call), whatever the outcome.
		logs, err := json.Marshal(wr.Logs)
		if err != nil {
			return err
		}
		if _, err := m.tx.Exec(ctx, `UPDATE automation.steps SET logs = $5 WHERE tenant_id = $1 AND run_id = $2 AND token_id = $3 AND seq = $4`,
			m.x.tenant, m.x.run.ID, tok.ID, w.Spec.Seq, logs); err != nil {
			return serrors.FromDB(opRun, err)
		}
	}
	_, spec, _ := m.spec(w.NodeID)
	behavior := string(spec.Behavior)
	output, err := decodeMap(wr.Output)
	if err != nil {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, StepError{Code: graph.FailInvalidAnswer, Message: "the answer is not a JSON object"})
	}
	if serr := m.outputTooLarge(w.NodeID, output, "the answer of "+w.NodeID); serr != nil {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, *serr)
	}
	outcome := wr.Outcome
	fo := m.in().FailedOutcome(w.NodeID)
	if wr.Await != nil && wr.Error == nil {
		return m.awaitAnswer(ctx, tok, w, *wr.Await, wr.Output)
	}
	if wr.Error != nil && (outcome == "" || outcome == fo) {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, *wr.Error)
	}
	if outcome == "" && spec.Answer != nil {
		o, st := m.in().CallOutcome(w.NodeID, output)
		if st != nil {
			return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, stepErr(*st))
		}
		outcome = o
	}
	known := false
	for _, o := range m.in().View().Outcomes(w.NodeID) {
		if o == outcome {
			known = true
		}
	}
	if !known {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, StepError{Code: graph.FailUnknownOutcome, Message: fmt.Sprintf("%s has no outcome %q", w.NodeID, outcome)})
	}
	return m.applyResult(ctx, tok, w, outcome, output, nil)
}

// awaitAnswer opens the answer wait of an ask node whose question was sent: the token keeps waiting on
// the node's step; the inbound event that carries the correlation token wakes it (Asker.Answer), its
// timeout takes the timeout outcome.
func (m *mover) awaitAnswer(ctx context.Context, tok *tokenRow, call waitRow, aw AwaitSpec, sent json.RawMessage) error {
	if aw.Event == "" || !validToken(aw.Token) {
		_, spec, _ := m.spec(call.NodeID)
		return m.nodeFailed(ctx, tok, call.NodeID, string(spec.Behavior), call.Spec.Seq, nil, StepError{Code: graph.FailInvalidAnswer, Message: call.NodeID + " asked for an answer without an event"})
	}
	ev, token := aw.Event, aw.Token
	w := waitRow{RunID: m.x.run.ID, TokenID: tok.ID, WaitID: call.WaitID + "/answer", Kind: waitEvent, NodeID: call.NodeID,
		EventKind: &ev, AfterSeq: aw.AfterSeq, Correlation: &token, Spec: waitSpecJSON{Seq: call.Spec.Seq, Ask: &askSpec{Single: aw.Single, Sent: sent}}}
	timeout := m.capWait(time.Duration(aw.TimeoutMS) * time.Millisecond)
	if timeout <= 0 {
		timeout = m.x.r.cfg.Limits.WaitMax
	}
	at := m.s.now.Add(timeout)
	w.TimeoutAt, w.Spec.Timer = &at, true
	// The token was issued with the answer event and policy the node named before sending; the wait
	// keeps them, and the token lives as long as the wait (an answer that came early spent a single
	// token already: it stays, spent, and the wait's catch-up delivers that answer).
	tag, err := m.tx.Exec(ctx, `UPDATE automation.correlations SET expires_at = $4 WHERE tenant_id = $1 AND run_id = $2 AND token = $3 AND expires_at > now()`,
		m.x.tenant, m.x.run.ID, token, at)
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	if tag.RowsAffected() == 0 {
		// The token is gone (it expired before the answer wait could open): no answer can find the wait.
		_, spec, _ := m.spec(call.NodeID)
		return m.nodeFailed(ctx, tok, call.NodeID, string(spec.Behavior), call.Spec.Seq, nil,
			StepError{Code: CodeTimeout, Message: call.NodeID + "'s correlation expired before its answer wait opened"})
	}
	if err := insertWait(ctx, m.tx, m.x.tenant, w); err != nil {
		return err
	}
	if _, err := m.tx.Exec(ctx, `UPDATE automation.steps SET output = $5 WHERE tenant_id = $1 AND run_id = $2 AND token_id = $3 AND seq = $4`,
		m.x.tenant, m.x.run.ID, tok.ID, call.Spec.Seq, []byte(nonNull(sent))); err != nil {
		return serrors.FromDB(opRun, err)
	}
	return nil
}

// answer applies the inbound answer of an ask node: Asker.Answer maps it to an outcome.
func (m *mover) answer(ctx context.Context, tok *tokenRow, w waitRow, gev graph.Event) error {
	_, spec, _ := m.spec(w.NodeID)
	behavior := string(spec.Behavior)
	if err := closeWait(ctx, m.tx, m.x.tenant, w, waitFired); err != nil {
		return err
	}
	nt, _ := m.x.r.cfg.Catalog.Node(spec.Type)
	asker, ok := nt.(Asker)
	if !ok {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, StepError{Code: CodeUnsupported, Message: spec.Type + " does not take answers in this process"})
	}
	payload := map[string]any{}
	for k, v := range gev {
		if k != "kind" {
			payload[k] = v
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var (
		outcome string
		out     json.RawMessage
	)
	if pa, ok := asker.(ParamAnswerer); ok {
		params, _ := m.in().View().Params(w.NodeID)
		outcome, out, err = pa.AnswerFor(params, raw)
	} else {
		outcome, out, err = asker.Answer(raw)
	}
	if err != nil {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, StepError{Code: graph.FailInvalidAnswer, Message: err.Error()})
	}
	output, err := decodeMap(out)
	if err != nil {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, StepError{Code: graph.FailInvalidAnswer, Message: "the answer is not a JSON object"})
	}
	if serr := m.outputTooLarge(w.NodeID, output, "the answer of "+w.NodeID); serr != nil {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, *serr)
	}
	known := false
	for _, o := range m.in().View().Outcomes(w.NodeID) {
		known = known || o == outcome
	}
	if !known {
		return m.nodeFailed(ctx, tok, w.NodeID, behavior, w.Spec.Seq, nil, StepError{Code: graph.FailUnknownOutcome, Message: fmt.Sprintf("%s has no outcome %q", w.NodeID, outcome)})
	}
	return m.applyResult(ctx, tok, w, outcome, output, nil)
}

// applyResult closes a waiting node's step with its outcome and moves the token on.
func (m *mover) applyResult(ctx context.Context, tok *tokenRow, w waitRow, outcome string, output map[string]any, e *StepError) error {
	if err := m.closeStep(ctx, tok, w.Spec.Seq, outcome, output, e); err != nil {
		return err
	}
	tok.set(w.NodeID, graph.NodeState{Output: output}) // an earlier wake's event is not this one's output
	return m.advance(ctx, tok, w.NodeID, outcome)
}

// record inserts a step of the token; ended=false leaves it open (a wait).
func (m *mover) record(ctx context.Context, tok *tokenRow, node, behavior string, input any, outcome string, output any, e *StepError, ended bool) (int, error) {
	tok.StepSeq++
	seq := tok.StepSeq
	in, err := jsonOrNil(input)
	if err != nil {
		return 0, err
	}
	out, err := jsonOrNil(output)
	if err != nil {
		return 0, err
	}
	errj, err := jsonOrNil(e)
	if err != nil {
		return 0, err
	}
	var oc any
	if outcome != "" {
		oc = outcome
	}
	_, err = m.tx.Exec(ctx, `
		INSERT INTO automation.steps (tenant_id, run_id, token_id, seq, node_id, attempt, behavior, input, outcome, output, error, sensitive, started_at, ended_at)
		VALUES ($1, $2, $3, $4, $5, 1, $6, $7, $8, $9, $10, $13, $11, CASE WHEN $12 THEN $11::timestamptz END)`,
		m.x.tenant, m.x.run.ID, tok.ID, seq, node, behavior, in, oc, out, errj, m.s.now, ended, m.s.sensitiveJSON())
	return seq, serrors.FromDB(opRun, err)
}

func (m *mover) closeStep(ctx context.Context, tok *tokenRow, seq int, outcome string, output any, e *StepError) error {
	out, err := jsonOrNil(output)
	if err != nil {
		return err
	}
	errj, err := jsonOrNil(e)
	if err != nil {
		return err
	}
	_, err = m.tx.Exec(ctx, `UPDATE automation.steps SET outcome = $5, output = $6, error = $7, ended_at = $8, sensitive = $9
		WHERE tenant_id = $1 AND run_id = $2 AND token_id = $3 AND seq = $4`, m.x.tenant, m.x.run.ID, tok.ID, seq, outcome, out, errj, m.s.now, m.s.sensitiveJSON())
	return serrors.FromDB(opRun, err)
}

// jsonOrNil is v as JSON, or nil (SQL NULL) for a nil value.
func jsonOrNil(v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case *StepError:
		if x == nil {
			return nil, nil
		}
	case map[string]any:
		if x == nil {
			return nil, nil
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return withoutNUL(b), nil // jsonb refuses U+0000
}

// activeTokens counts the tokens that run: not finished and not waiting for their own branches.
func (m *mover) activeTokens(ctx context.Context) (int, error) {
	var n int
	err := m.tx.QueryRow(ctx, `SELECT count(*) FROM automation.tokens WHERE tenant_id = $1 AND run_id = $2 AND status IN ('running', 'waiting') AND flow IS NULL`,
		m.x.tenant, m.x.run.ID).Scan(&n)
	return n, serrors.FromDB(opRun, err)
}

// context is what a token's step reads: the trigger's event, the node outputs of its ancestors and its
// own, the subject's variables (with the run-as rights), the tenant and the step's time.
func (m *mover) context(ctx context.Context, tok *tokenRow) (graph.Context, error) {
	chain, err := ancestors(ctx, m.tx, m.x.tenant, m.x.run.ID, *tok)
	if err != nil {
		return graph.Context{}, err
	}
	nodes := map[string]graph.NodeState{}
	each := map[string]graph.ForeachItem{}
	all := append([]tokenRow{*tok}, chain...)
	for i := len(all) - 1; i >= 0; i-- {
		t := all[i]
		for _, w := range t.Writes {
			nodes[w.Node] = w.State
		}
		if t.ForeachRef != nil && t.ForeachIndex != nil {
			var item any
			if len(t.Item) > 0 {
				// Numbers as json.Number, as the format reads values: decoding to float64 first would round
				// an integer past 2^53.
				if err := expr.UnmarshalJSON(t.Item, &item); err != nil {
					return graph.Context{}, err
				}
			}
			each[*t.ForeachRef] = graph.ForeachItem{Item: item, Index: *t.ForeachIndex}
		}
	}
	event, err := decodeMap(m.x.run.Input)
	if err != nil {
		return graph.Context{}, err
	}
	vars, err := m.subjectVars(ctx)
	if err != nil {
		return graph.Context{}, err
	}
	evs, err := runEvents(m.x.run.Events)
	if err != nil {
		return graph.Context{}, err
	}
	return m.x.r.BuildRunContext(ctx, RunContextInput{
		TenantID: m.x.tenant, AutomationID: m.x.run.AutomationID, RunID: m.x.run.ID, Subject: m.x.run.Subject, RunAs: m.x.subject,
		Event: event, Events: evs, Nodes: nodes, Foreach: each, Now: m.s.now, Vars: vars, StateDB: m.tx,
	})
}

// runEvents reads the merged events of a run (runs.events).
func runEvents(raw json.RawMessage) ([]map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var list []any
	if err := expr.UnmarshalJSON(raw, &list); err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(list))
	for _, v := range list {
		if m, ok := v.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// subjectVars loads the subject's variables with the run-as rights, once per step.
func (m *mover) subjectVars(ctx context.Context) (map[string]any, error) {
	if m.vars != nil {
		return m.vars, nil
	}
	m.vars = map[string]any{}
	s := normSubject(m.x.run.Subject)
	if m.x.r.cfg.Subjects == nil || s.Entity == graph.SubjectNone {
		return m.vars, nil
	}
	raw, err := m.x.r.cfg.Subjects.Load(ctx, m.x.subject, s)
	if errors.Is(err, ErrNotFound) {
		return m.vars, nil // not visible: an expression that reads it is missing_context
	}
	if err != nil {
		return nil, err
	}
	vars, err := decodeMap(raw)
	if err != nil {
		return nil, err
	}
	m.vars = vars
	return vars, nil
}

// withheldOutput gives a wait woken by the finish of a run whose output the event withheld (the run read
// sensitive fields) that output, read from the run, when this run's run-as may read those fields; the
// fields then go with this run's data too.
func (m *mover) withheldOutput(ctx context.Context, gev graph.Event) error {
	if gev.Kind() != EventRunFinished {
		return nil
	}
	if withheld, _ := gev["outputWithheld"].(bool); !withheld {
		return nil
	}
	id, err := uuid.Parse(fmt.Sprint(gev["runId"]))
	if err != nil {
		return nil //nolint:nilerr // not a run's ID: the output stays withheld
	}
	var out, raw []byte
	err = m.tx.QueryRow(ctx, `SELECT output, sensitive FROM automation.runs WHERE tenant_id = $1 AND id = $2`, m.x.tenant, id).Scan(&out, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return serrors.FromDB(opRun, err)
	}
	var refs []sensitiveRef
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &refs); err != nil {
			return err
		}
	}
	ok, err := m.x.r.projector(m.x.tenant).readable(ctx, refs, &m.x.subject)
	if err != nil {
		return &host.InfraError{Err: err}
	}
	if !ok {
		return nil
	}
	var output any
	if len(out) > 0 {
		if err := json.Unmarshal(out, &output); err != nil {
			return err
		}
	}
	gev["output"], gev["outputWithheld"] = output, false
	var changed bool
	m.s.sensitive, changed = mergeRefs(m.s.sensitive, refs)
	m.s.sensitiveChanged = m.s.sensitiveChanged || changed
	return nil
}

// memoryFailure is the platform failure an expression of the step met reading the automation's memory: the
// expression failed with the database's text, but the step is retried, not failed. Any other failure of a
// lookup stays the expression's own.
func memoryFailure(ctx context.Context, gctx graph.Context) error {
	r, ok := gctx.State.(*state.Reader)
	if !ok {
		return nil
	}
	if err := r.Err(); err != nil && (host.IsInfrastructure(err) || ctx.Err() != nil) {
		return err
	}
	return nil
}
