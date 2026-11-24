package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/code"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opCall = serrors.Op("automation.engine.call")

// CallWorkflowID is the native workflow contract every call node runs in: a child instance of the run
// with its own lease, journal and blocked state. A call is never executed inside a step's transaction.
const CallWorkflowID = "automation.call"

// Error codes of calls.
const (
	CodeOutcomeUnknown = catalog.CallOutcomeUnknown
	CodeRateLimited    = catalog.CallRateLimited
	CodeForbidden      = "forbidden"
	CodeInvalidInput   = "invalid_input"
)

// Masker replaces secrets with "***" in text and JSON (code.NewMasker).
type Masker = code.Masker

// CallRequest is the input of a child call instance; immutable, part of the instance input.
type CallRequest struct {
	TenantID       uuid.UUID       `json:"tenant_id"`
	ParentInstance uuid.UUID       `json:"parent_instance"`
	AutomationID   uuid.UUID       `json:"automation_id"`
	RunID          uuid.UUID       `json:"run_id"`
	TokenID        uuid.UUID       `json:"token_id"`
	Seq            int             `json:"seq"`
	WaitID         string          `json:"wait_id"`
	NodeID         string          `json:"node_id"`
	NodeType       string          `json:"node_type"`
	ConnectionID   *uuid.UUID      `json:"connection_id,omitempty"` // a tenant connection
	Connection     string          `json:"connection,omitempty"`    // the connection ref: a UUID or "platform:<kind>"
	ConnectorKind  string          `json:"connector,omitempty"`     // the connection's kind
	Input          json.RawMessage `json:"input"`                   // rendered params, no secrets
	// Context is the step's context (graph.NodeSpec.StepContext). It is never part of the instance input,
	// which the workflow API shows without the automation's field rules: Begin keeps it in
	// automation.call_contexts and the child reads it from there.
	Context        json.RawMessage `json:"-"`
	IdempotencyKey string          `json:"idempotency_key"`      // "run:node:token:seq"; stable across retries
	Deadline       time.Time       `json:"deadline"`             // node timeout from params, ≤ CallLimits.MaxTimeout
	TimeoutMS      int64           `json:"timeout_ms,omitempty"` // the node's timeout: an operator's Retry gets a new deadline of it
	Subject        SubjectRef      `json:"subject"`
	// RunAs is who the call acts as; the child instance runs as this subject with the rights it holds now.
	RunAs spi.Subject `json:"-"`
	// FailedConnected: the node's failed outcome has an edge. An unknown outcome of a call that may not be
	// repeated then takes it; otherwise the child blocks until an operator resolves or retries it.
	FailedConnected bool `json:"failed_connected"`
	// AnswerWaitID is the wait an ask node's answer wakes (Asker): its correlation points at it.
	AnswerWaitID string `json:"answer_wait_id,omitempty"`
}

// maxCallContextBytes bounds the step's context a call carries (a Code node's ctx).
const maxCallContextBytes = 8 << 20

// CallLimits bound the calls of a process.
type CallLimits struct {
	DefaultTimeout time.Duration // 30s
	MaxTimeout     time.Duration // 5m
	MaxAttempts    int           // idempotent and 429 only; default 5
	BackoffBase    time.Duration // 1s, exponential, capped by RetryAfter and Deadline
	PerTenant      int           // concurrent call instances per tenant, default 20
	Concurrency    int           // slots of automation.call in the process, default 8
}

func (l CallLimits) withDefaults() CallLimits {
	if l.DefaultTimeout <= 0 {
		l.DefaultTimeout = 30 * time.Second
	}
	if l.MaxTimeout <= 0 {
		l.MaxTimeout = 5 * time.Minute
	}
	if l.DefaultTimeout > l.MaxTimeout {
		l.DefaultTimeout = l.MaxTimeout
	}
	if l.MaxAttempts <= 0 {
		l.MaxAttempts = 5
	}
	if l.BackoffBase <= 0 {
		l.BackoffBase = time.Second
	}
	if l.PerTenant <= 0 {
		l.PerTenant = 20
	}
	if l.Concurrency <= 0 {
		l.Concurrency = 8
	}
	return l
}

// CallRunner implements workflow.NativeRunner for "automation.call" and CallExecutor: Begin starts a
// child instance in the step's transaction, the child resolves the credentials, executes the node and
// returns the result to the run's inbox in the transaction of its own receipt.
type CallRunner struct {
	r *Runtime
}

var (
	_ CallExecutor          = (*CallRunner)(nil)
	_ workflow.NativeRunner = (*CallRunner)(nil)
)

// callRecord is what one Effect of a call recorded: a result, a repeat after a delay, or a failure.
type callRecord struct {
	Kind      string          `json:"kind"` // result | retry | failed
	Outcome   string          `json:"outcome,omitempty"`
	Output    json.RawMessage `json:"output,omitempty"`
	AfterMS   int64           `json:"after_ms,omitempty"`
	Code      string          `json:"code,omitempty"`
	Message   string          `json:"message,omitempty"`
	Retryable bool            `json:"retryable,omitempty"`
	Await     *AwaitSpec      `json:"await,omitempty"`
}

// ResolvedCall is the result an operator records for a blocked call (Operations.Resolve): the journal
// value the child continues with.
func resolvedCall(outcome string, output json.RawMessage) json.RawMessage {
	b, err := json.Marshal(callRecord{Kind: "result", Outcome: outcome, Output: nonNull(output)})
	if err != nil {
		return json.RawMessage(`{"kind":"result"}`)
	}
	return b
}

// AwaitSpec is the answer wait an ask node registers after its send: the parent opens it.
type AwaitSpec struct {
	Event     string `json:"event"`
	TimeoutMS int64  `json:"timeout_ms"`
	Single    bool   `json:"single"`
	Token     string `json:"token"`
	// AfterSeq is the outbox watermark when the token was issued: an answer that came before the wait
	// opened is found by its catch-up.
	AfterSeq int64 `json:"after_seq"`
}

// contract is the native contract of automation.call.
func (c *CallRunner) contract() workflow.NativeContract {
	return workflow.NativeContract{
		Runner:         c,
		Input:          &manifest.Type{Kind: manifest.KindJSON},
		Concurrency:    c.r.cfg.CallLimits.Concurrency,
		RunTimeout:     c.r.cfg.CallLimits.MaxTimeout + time.Minute,
		Completion:     workflow.CancelPending,
		StatusObserver: callObserver{c: c},
		// A call's request, journal and result are a run's data: GetRun shows them with its field rules.
		Private: true,
	}
}

// endedResult is the result of a call child that ended (status) without one: outcome_unknown when an
// attempt had started (its effect may have happened), effect_failed otherwise. Its parent gets it from
// the observer, or reconciles it itself when that signal could not be delivered.
func (c *CallRunner) endedResult(ctx context.Context, q querier, tenant, child uuid.UUID, req CallRequest, status string) (WaitResult, error) {
	// A child that committed its result before it ended gives that result (its signal may still be
	// queued in the parent's inbox): the provider's answer is never replaced by a failure.
	if wr, ok, err := committedResult(ctx, q, tenant, child); err != nil || ok {
		if ok {
			wr.WaitID = req.WaitID
		}
		return wr, err
	}
	// The last attempt decides: one that completed with an answer (its result transaction rolled back, a
	// signal refused) gives that answer; one that started without a record may have reached the provider.
	var (
		lastStatus string
		lastRaw    []byte
	)
	err := q.QueryRow(ctx, `SELECT status, result FROM extruntime.workflow_occurrences
		WHERE tenant_id = $1 AND instance_id = $2 AND name = 'call' ORDER BY n DESC LIMIT 1`, tenant, child).Scan(&lastStatus, &lastRaw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return WaitResult{}, serrors.FromDB(opCall, err)
	}
	if lastStatus == workflow.OccCompleted {
		if wr, ok := c.recordedAnswer(lastRaw, req); ok {
			return wr, nil
		}
	}
	started := lastStatus == workflow.OccStarted
	code, msg := CodeEffectFailed, "the call ended without a result"
	if status == workflow.StatusTerminated {
		msg = "the call was terminated"
	}
	if started {
		code, msg = CodeOutcomeUnknown, msg+"; an attempt may have reached the provider"
	}
	out, err := json.Marshal(graph.FailedOutput(code, msg, false))
	if err != nil {
		return WaitResult{}, err
	}
	fo := ""
	if nt, ok := c.r.cfg.Catalog.Node(req.NodeType); ok {
		fo = nt.Spec().FailedOutcome
	}
	return WaitResult{WaitID: req.WaitID, Outcome: fo, Output: out, Error: &StepError{Code: code, Message: msg}}, nil
}

// recordedAnswer is the result of a completed attempt's journal value: its answer, or its failure; a
// retry (the next attempt never started) has none.
func (c *CallRunner) recordedAnswer(raw []byte, req CallRequest) (WaitResult, bool) {
	var rec callRecord
	if len(raw) == 0 || json.Unmarshal(raw, &rec) != nil {
		return WaitResult{}, false
	}
	switch rec.Kind {
	case "result":
		return WaitResult{WaitID: req.WaitID, Outcome: rec.Outcome, Output: rec.Output, Await: rec.Await}, true
	case "failed":
		out, err := json.Marshal(graph.FailedOutput(rec.Code, rec.Message, rec.Retryable))
		if err != nil {
			return WaitResult{}, false
		}
		fo := ""
		if nt, ok := c.r.cfg.Catalog.Node(req.NodeType); ok {
			fo = nt.Spec().FailedOutcome
		}
		return WaitResult{WaitID: req.WaitID, Outcome: fo, Output: out, Error: &StepError{Code: rec.Code, Message: rec.Message, Retryable: rec.Retryable}}, true
	}
	return WaitResult{}, false
}

// committedResult is the result a call child's result transaction recorded in its receipt.
func committedResult(ctx context.Context, q querier, tenant, child uuid.UUID) (WaitResult, bool, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT result FROM extruntime.workflow_occurrences
		WHERE tenant_id = $1 AND instance_id = $2 AND name = 'result' AND n = 0 AND status = $3 AND result IS NOT NULL`,
		tenant, child, workflow.OccCompleted).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return WaitResult{}, false, nil
	}
	if err != nil {
		return WaitResult{}, false, serrors.FromDB(opCall, err)
	}
	wr, ok := decodeResult(raw) // a receipt without a result: the child's end decides
	return wr, ok, nil
}

// decodeResult reads the call result a result receipt holds.
func decodeResult(raw []byte) (WaitResult, bool) {
	var msg inboxMessage
	if json.Unmarshal(raw, &msg) != nil || msg.WaitID == "" {
		return WaitResult{}, false
	}
	return msg.WaitResult, true
}

// readCallRequest reads the input of a call instance.
func readCallRequest(raw json.RawMessage) (CallRequest, bool) {
	var req CallRequest
	return req, json.Unmarshal(raw, &req) == nil
}

// callObserver gives the parent's call wait its result when a call child ends without one: it failed
// (the runner panicked, the engine gave up) or an operator terminated it. A call that may have reached
// the provider (an attempt started without a result) ends as outcome_unknown, never a plain failure.
type callObserver struct{ c *CallRunner }

func (o callObserver) InstanceStatusChanged(ctx context.Context, tx pgx.Tx, inst workflow.Instance, _, to string) error {
	if to != workflow.StatusFailed && to != workflow.StatusTerminated {
		return nil
	}
	req, ok := readCallRequest(inst.Input)
	if !ok || req.ParentInstance == uuid.Nil || req.TenantID != inst.TenantID {
		return nil // not a call of a run of this tenant: nothing waits for it
	}
	// As in Run: only the instance's stored parent is the call's; a request naming another is signalled
	// nothing.
	var parent *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT parent_instance_id FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2`,
		inst.TenantID, inst.ID).Scan(&parent); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return serrors.FromDB(opCall, err)
	}
	if parent == nil || *parent != req.ParentInstance {
		return nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM automation.call_slots WHERE tenant_id = $1 AND instance_id = $2`, inst.TenantID, inst.ID); err != nil {
		return serrors.FromDB(opCall, err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM automation.call_contexts WHERE tenant_id = $1 AND instance_id = $2`, inst.TenantID, inst.ID); err != nil {
		return serrors.FromDB(opCall, err)
	}
	if o.c.r.quietCallObserver {
		return wakeParent(ctx, tx, inst.TenantID, req.ParentInstance)
	}
	wr, err := o.c.endedResult(ctx, tx, inst.TenantID, inst.ID, req, to)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(inboxMessage{WaitResult: wr})
	if err != nil {
		return err
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return serrors.FromDB(opCall, err)
	}
	err = o.c.r.cfg.Workflows.SignalInTx(ctx, sp, spi.Scope{Subject: spi.Subject{TenantID: inst.TenantID, UserID: inst.SubjectUserID, Kind: inst.SubjectKind}}, host.SignalIntent{
		Workflow: WorkflowID, Instance: req.ParentInstance.String(), Name: InboxSignal, Payload: payload,
		Key: "call:" + req.RunID.String() + ":" + req.WaitID,
	})
	if err != nil {
		_ = sp.Rollback(ctx)
		if we, ok := pool.AsWireError(err); ok && (we.Code == workflow.CodeInstanceNotAccepting || we.Code == workflow.CodeInstanceNotFound) {
			return nil // the run itself ended: nothing waits for the call
		}
		if we, ok := pool.AsWireError(err); ok && we.Code == workflow.CodeSignalDenied {
			// The run-as is refused now: the run reconciles the ended call itself (endedCalls) when it
			// runs again; it is woken for that (and blocks on its subject until the right is back).
			return wakeParent(ctx, tx, inst.TenantID, req.ParentInstance)
		}
		return err
	}
	return serrors.FromDB(opCall, sp.Commit(ctx))
}

// wakeParent makes a waiting run due now, so its runner looks at its ended calls.
func wakeParent(ctx context.Context, tx pgx.Tx, tenant, parent uuid.UUID) error {
	_, err := tx.Exec(ctx, `UPDATE extruntime.workflow_instances SET wake_at = now(), updated_at = now()
		WHERE tenant_id = $1 AND id = $2 AND status = 'waiting'`, tenant, parent)
	return serrors.FromDB(opCall, err)
}

// Begin starts the call's child instance in the step's transaction and registers it as the wait.
func (c *CallRunner) Begin(ctx context.Context, tx pgx.Tx, run StepContext, node graph.Node, input json.RawMessage) (WaitSpec, error) {
	nt, ok := c.r.cfg.Catalog.Node(node.Type)
	if !ok {
		return WaitSpec{}, &StepError{Code: CodeUnsupported, Message: node.Type + " is not a node type of this process"}
	}
	if _, isCall := nt.(catalog.CallNode); !isCall {
		return WaitSpec{}, &StepError{Code: CodeUnsupported, Message: node.Type + " has no call executor"}
	}
	params, err := decodeMap(input)
	if err != nil {
		return WaitSpec{}, &StepError{Code: CodeInvalidInput, Message: "the params of " + node.ID + " are not an object"}
	}
	limits := c.r.cfg.CallLimits
	timeout := limits.DefaultTimeout
	if ms, ok := graph.DurationMs(params["timeout"]); ok {
		timeout = time.Duration(ms) * time.Millisecond
	}
	if timeout > limits.MaxTimeout {
		timeout = limits.MaxTimeout
	}
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		return WaitSpec{}, serrors.FromDB(opCall, err)
	}
	waitID := fmt.Sprintf("%s/%d", run.TokenID, run.Seq)
	req := CallRequest{
		TenantID: run.TenantID, AutomationID: run.AutomationID, RunID: run.RunID, TokenID: run.TokenID, Seq: run.Seq,
		WaitID: waitID, NodeID: node.ID, NodeType: node.Type, Input: input, Subject: run.Subject,
		IdempotencyKey: fmt.Sprintf("%s:%s:%s:%d", run.RunID, node.ID, run.TokenID, run.Seq),
		Deadline:       now.Add(timeout), TimeoutMS: timeout.Milliseconds(), FailedConnected: run.FailedConnected, RunAs: run.RunAs,
	}
	if run.Native != nil {
		req.ParentInstance = run.Native.Instance().ID
	}
	if run.Context != nil {
		if req.Context, err = json.Marshal(replaceNUL(run.Context)); err != nil {
			return WaitSpec{}, &StepError{Code: CodeInvalidInput, Message: "the context of " + node.ID + " is not JSON"}
		}
		if len(req.Context) > maxCallContextBytes {
			return WaitSpec{}, &StepError{Code: CodeLimitExceeded, Message: "the context of " + node.ID + " is too large"}
		}
	}
	view, err := c.r.cfg.Catalog.ForTenant(ctx, run.TenantID)
	if err != nil {
		return WaitSpec{}, &host.InfraError{Err: err}
	}
	// The node's own kinds and its connector's.
	if kinds := view.NodeRequires[node.Type]; len(kinds) > 0 {
		ref, kind, err := catalog.ResolveConnection(view.Connections, kinds, params[catalog.ConnectionParam])
		if err != nil {
			return WaitSpec{}, &StepError{Code: CodeConnectionUnavailable, Message: "the connection of " + node.ID + " is not available"}
		}
		req.Connection, req.ConnectorKind = ref.ID, string(kind)
		if id, err := uuid.Parse(ref.ID); err == nil {
			req.ConnectionID = &id
		}
	}
	if _, ok := nt.(Asker); ok {
		req.AnswerWaitID = waitID + "/answer"
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return WaitSpec{}, err
	}
	sub := run.RunAs
	sub.TenantID = run.TenantID
	id, err := c.r.cfg.Workflows.Start(ctx, tx, spi.Scope{Subject: sub}, spi.StartIntent{
		Workflow: CallWorkflowID, Input: raw, Key: "automation.call:" + req.IdempotencyKey,
		Producer: spi.Producer{InstanceID: req.ParentInstance.String()},
	})
	if err != nil {
		return WaitSpec{}, err
	}
	child, err := uuid.Parse(id)
	if err != nil {
		return WaitSpec{}, err
	}
	if req.Context != nil {
		if _, err := tx.Exec(ctx, `INSERT INTO automation.call_contexts (tenant_id, instance_id, run_id, context) VALUES ($1, $2, $3, $4)
			ON CONFLICT (tenant_id, instance_id) DO NOTHING`, run.TenantID, child, run.RunID, req.Context); err != nil {
			return WaitSpec{}, serrors.FromDB(opCall, err)
		}
	}
	// No timer: the child answers by its deadline, and a call that blocks (an unknown outcome) waits for
	// an operator, not for a clock.
	return WaitSpec{WaitID: waitID, Kind: waitCall, Child: &child}, nil
}

// Run executes the call of one child instance.
func (c *CallRunner) Run(ctx context.Context, nr workflow.NativeRun) (json.RawMessage, error) {
	var req CallRequest
	if err := json.Unmarshal(nr.Instance().Input, &req); err != nil || req.RunID == uuid.Nil {
		return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation.invalidInput", Message: "automation.call needs a call request"}
	}
	// The request is trusted only for the instance's own tenant (Begin writes both): a start from
	// elsewhere naming another tenant's connection opens nothing.
	if req.TenantID != nr.Instance().TenantID {
		return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation.invalidInput", Message: "the call request is not of the instance's tenant"}
	}
	if req.ParentInstance != uuid.Nil {
		var parent uuid.UUID
		err := c.r.cfg.DB.QueryRow(ctx, `SELECT parent_instance_id FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2`,
			req.TenantID, nr.Instance().ID).Scan(&parent)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, infra(serrors.FromDB(opCall, err)) // a database failure is retried, never the call's
		}
		if err != nil || parent != req.ParentInstance {
			return nil, &pool.WireError{Kind: pool.ErrKindDomain, Code: "automation.invalidInput", Message: "the call request names another parent than the instance's"}
		}
	}
	req.RunAs = nr.Subject()
	// An operator's Retry of a blocked call gives it a new deadline (automation.call_deadlines).
	var renewed *time.Time
	if err := c.r.cfg.DB.QueryRow(ctx, `SELECT deadline FROM automation.call_deadlines WHERE tenant_id = $1 AND instance_id = $2`,
		req.TenantID, nr.Instance().ID).Scan(&renewed); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, infra(serrors.FromDB(opCall, err))
	}
	if renewed != nil && renewed.After(req.Deadline) {
		req.Deadline = *renewed
	}
	if err := c.r.cfg.DB.QueryRow(ctx, `SELECT context FROM automation.call_contexts WHERE tenant_id = $1 AND instance_id = $2`,
		req.TenantID, nr.Instance().ID).Scan(&req.Context); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, infra(serrors.FromDB(opCall, err))
	}
	x := &callExec{c: c, nr: nr, req: req}
	res, err := x.run(ctx)
	if err != nil {
		return nil, infra(err)
	}
	res.WaitID = req.WaitID
	payload, err := json.Marshal(inboxMessage{WaitResult: res})
	if err != nil {
		return nil, err
	}
	if h := c.r.callResultHook; h != nil {
		h(ctx, nr.Instance().ID, "before")
	}
	// The receipt of the result transaction keeps the result: a child ended (terminated) after it
	// committed, before its completion, still gives its parent that result, never a synthesized one.
	_, err = nr.Transaction(ctx, "result", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
		if _, err := tx.Exec(ctx, `DELETE FROM automation.call_deadlines WHERE tenant_id = $1 AND instance_id = $2`, req.TenantID, nr.Instance().ID); err != nil {
			return nil, serrors.FromDB(opCall, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM automation.call_contexts WHERE tenant_id = $1 AND instance_id = $2`, req.TenantID, nr.Instance().ID); err != nil {
			return nil, serrors.FromDB(opCall, err)
		}
		if req.ParentInstance == uuid.Nil {
			return payload, nil
		}
		err := c.r.cfg.Workflows.SignalInTx(ctx, tx, spi.Scope{Subject: nr.Subject()}, host.SignalIntent{
			Workflow: WorkflowID, Instance: req.ParentInstance.String(), Name: InboxSignal, Payload: payload,
			Key: "call:" + req.RunID.String() + ":" + req.WaitID,
		})
		if we, ok := pool.AsWireError(err); ok && (we.Code == workflow.CodeInstanceNotAccepting || we.Code == workflow.CodeInstanceNotFound) {
			return payload, nil // the run finished meanwhile: the effect stays as it is
		}
		if err != nil {
			return nil, err
		}
		return payload, nil
	})
	if err != nil {
		return nil, infra(err)
	}
	if h := c.r.callResultHook; h != nil {
		h(ctx, nr.Instance().ID, "after")
	}
	return payload, nil
}

// callExec is one run of a child call instance.
type callExec struct {
	c   *CallRunner
	nr  workflow.NativeRun
	req CallRequest
	// answer and single are what an ask named when its token was issued (AnswerEventer), from the
	// receipt of that transaction: its wait keeps them.
	answer   string
	single   bool
	answerer AnswerEventer
	// terminalUnknown: the node declared its unknown outcome terminal (catalog.TerminalUnknown).
	terminalUnknown bool
}

func (x *callExec) run(ctx context.Context) (WaitResult, error) {
	req := x.req
	nt, ok := x.c.r.cfg.Catalog.Node(req.NodeType)
	cn, isCall := nt.(catalog.CallNode)
	if !ok || !isCall {
		return x.failed(nil, CodeUnsupported, req.NodeType+" has no call executor in this process", false), nil
	}
	if tu, ok := nt.(catalog.TerminalUnknown); ok {
		x.terminalUnknown = tu.UnknownIsTerminal()
	}
	if cc, ok := cn.(catalog.CommandCall); ok {
		return x.command(ctx, cn, cc)
	}
	token, afterSeq := "", int64(0)
	if _, asks := nt.(Asker); asks && req.AnswerWaitID != "" {
		var err error
		if token, afterSeq, err = x.correlation(ctx, nt); err != nil {
			var se *StepError
			if errors.As(err, &se) {
				return x.failed(nt, se.Code, se.Message, false), nil
			}
			return WaitResult{}, err
		}
	}
	for k := 0; ; k++ {
		rec, err := x.attempt(ctx, cn, k, token, afterSeq)
		if err != nil {
			return WaitResult{}, err
		}
		switch rec.Kind {
		case "result":
			x.c.r.metrics.call(ctx, x.connector(nt), req.NodeType, "ok")
			wr := WaitResult{Outcome: rec.Outcome, Output: rec.Output}
			if rec.Await != nil {
				wr.Await = rec.Await
			}
			return wr, nil
		case "retry":
			x.c.r.metrics.call(ctx, x.connector(nt), req.NodeType, "retried")
			if err := x.nr.Sleep(ctx, "backoff", k, time.Duration(rec.AfterMS)*time.Millisecond); err != nil {
				return WaitResult{}, err
			}
			continue
		default:
			return x.failed(nt, rec.Code, rec.Message, rec.Retryable), nil
		}
	}
}

// failed is a failed call's result: outcome failed with output.error, or the error alone for a node
// without a failed outcome (the run fails, as for any failed node).
func (x *callExec) failed(nt catalog.NodeType, code, message string, retryable bool) WaitResult {
	result := "failed"
	if code == CodeOutcomeUnknown {
		result = CodeOutcomeUnknown
	}
	x.c.r.metrics.call(context.Background(), x.connector(nt), x.req.NodeType, result)
	out, err := json.Marshal(graph.FailedOutput(code, message, retryable))
	if err != nil {
		out = json.RawMessage(`{}`)
	}
	out = withoutNUL(out)
	message = strings.ReplaceAll(message, "\x00", "\uFFFD")
	fo := ""
	if nt != nil {
		fo = nt.Spec().FailedOutcome
	}
	return WaitResult{Outcome: fo, Output: out, Error: &StepError{Code: code, Message: message, Retryable: retryable}}
}

func (x *callExec) connector(nt catalog.NodeType) string {
	if x.req.ConnectorKind != "" {
		return x.req.ConnectorKind
	}
	if x.req.Connection != "" {
		if kind, ok := strings.CutPrefix(x.req.Connection, catalog.PlatformPrefix); ok {
			return kind
		}
	}
	if nt != nil {
		if kinds := nt.Requires(); len(kinds) > 0 {
			return string(kinds[0])
		}
	}
	return "none"
}

// errSlotDeadline: the call's deadline passed while it waited for a slot.
var errSlotDeadline = errors.New("automation: the call's deadline passed while it waited for a slot")

// slot takes one of the tenant's PerTenant call slots before an attempt runs: under the tenant's
// advisory lock it drops the slots of calls that no longer run (finished, or a lease that expired with
// its process), counts the rest and takes one. A call that finds none sleeps and checks again on its
// wake. release gives the slot back after the attempt.
func (x *callExec) slot(ctx context.Context) error {
	for n := 0; n < 100000; n++ {
		o, ok, err := x.nr.Lookup(ctx, "slot", n)
		if err != nil {
			return err
		}
		if ok && o.Status == workflow.OccCompleted {
			continue // an earlier wait for a slot is over
		}
		if !ok {
			acquired, err := x.acquire(ctx)
			if err != nil {
				return err
			}
			if acquired {
				return nil
			}
		}
		// No slot (or a wait already pending): wait, then check again with the next occurrence. A wait
		// never outlives the call's deadline.
		left := time.Until(x.req.Deadline)
		if left <= 0 {
			return errSlotDeadline
		}
		if err := x.nr.Sleep(ctx, "slot", n, min(time.Second, left)); err != nil {
			return err
		}
	}
	return nil
}

func (x *callExec) acquire(ctx context.Context) (bool, error) {
	acquired := false
	err := pgx.BeginFunc(ctx, x.c.r.cfg.DB, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "automation.call|"+x.req.TenantID.String()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			DELETE FROM automation.call_slots s WHERE s.tenant_id = $1 AND s.instance_id <> $2 AND NOT EXISTS (
				SELECT 1 FROM extruntime.workflow_instances i
				WHERE i.id = s.instance_id AND i.status = 'running' AND i.lease_until > now())`, x.req.TenantID, x.nr.Instance().ID); err != nil {
			return err
		}
		var taken int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.call_slots WHERE tenant_id = $1 AND instance_id <> $2`,
			x.req.TenantID, x.nr.Instance().ID).Scan(&taken); err != nil {
			return err
		}
		if taken >= x.c.r.cfg.CallLimits.PerTenant {
			return nil
		}
		_, err := tx.Exec(ctx, `INSERT INTO automation.call_slots (tenant_id, instance_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			x.req.TenantID, x.nr.Instance().ID)
		acquired = err == nil
		return err
	})
	if err != nil {
		return false, &host.InfraError{Err: serrors.FromDB(opCall, err)}
	}
	return acquired, nil
}

// release gives the call's slot back.
func (x *callExec) release(ctx context.Context) {
	// Bounded: a stalled database must not hold the worker; a slot left behind is dropped as stale.
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if _, err := x.c.r.cfg.DB.Exec(rctx, `DELETE FROM automation.call_slots WHERE tenant_id = $1 AND instance_id = $2`,
		x.req.TenantID, x.nr.Instance().ID); err != nil {
		x.c.r.cfg.Logger.Warn("automation: a call slot was not released; it expires with the call", "error", err)
	}
}

// correlation issues the ask's correlation token once (a journaled transaction): it points at the
// answer wait the parent opens.
func (x *callExec) correlation(ctx context.Context, nt catalog.NodeType) (string, int64, error) {
	req := x.req
	// The answer, as the node names it before sending: only that event takes the token from the start.
	ae, ok := nt.(AnswerEventer)
	if !ok {
		return "", 0, &StepError{Code: CodeUnsupported, Message: req.NodeType + " asks without naming its answer event"}
	}
	// The answer is named once, in the journaled transaction that issues the token: its receipt keeps
	// it, and a replay (after a sleep, a crash) takes it from there, whatever the node would name now.
	raw, err := x.nr.Transaction(ctx, "correlation", 0, func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error) {
		event, single := x.answerOf(ae)
		if event == "" {
			return json.Marshal(map[string]any{"refused": req.NodeType + " names no answer event", "code": CodeUnsupported})
		}
		// No token outlives its run: under the run row's lock (the run's terminal cleanup takes it before
		// it deletes the run's tokens) the run must still be live.
		if _, err := tx.Exec(ctx, `SELECT 1 FROM automation.runs WHERE tenant_id = $1 AND id = $2 FOR SHARE`, req.TenantID, req.RunID); err != nil {
			return nil, serrors.FromDB(opCall, err)
		}
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM extruntime.workflow_instances WHERE tenant_id = $1 AND id = $2`, req.TenantID, req.ParentInstance).Scan(&status); err != nil {
			return nil, serrors.FromDB(opCall, err)
		}
		if status != workflow.StatusRunning && status != workflow.StatusWaiting && status != workflow.StatusBlocked {
			return json.Marshal(map[string]any{"refused": "the run ended before the question was sent", "code": CodeCancelled})
		}
		mark, err := watermark(ctx, tx)
		if err != nil {
			return nil, err
		}
		expires := req.Deadline.Add(x.c.r.cfg.Limits.WaitMax)
		for i := 0; i < 5; i++ {
			token := NewCorrelationToken()
			tag, err := tx.Exec(ctx, `
				INSERT INTO automation.correlations (token, tenant_id, run_id, wait_id, expires_at, event, single)
				VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (token) DO NOTHING`, token, req.TenantID, req.RunID, req.AnswerWaitID, expires, event, single)
			if err != nil {
				return nil, serrors.FromDB(opCall, err)
			}
			if tag.RowsAffected() == 1 {
				return json.Marshal(map[string]any{"token": token, "after_seq": mark, "event": event, "single": single})
			}
		}
		return nil, errors.New("no free correlation token")
	})
	if err != nil {
		return "", 0, err
	}
	var got struct {
		Token    string `json:"token"`
		AfterSeq int64  `json:"after_seq"`
		Event    string `json:"event"`
		Single   bool   `json:"single"`
		Refused  string `json:"refused"`
		Code     string `json:"code"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		return "", 0, err
	}
	if got.Refused != "" || got.Event == "" {
		code := got.Code
		if code == "" {
			code = CodeUnsupported
		}
		return "", 0, &StepError{Code: code, Message: got.Refused}
	}
	x.answer, x.single, x.answerer = got.Event, got.Single, ae
	return got.Token, got.AfterSeq, nil
}

// answerOf is the answer the node names now, for the call's params and run-as.
func (x *callExec) answerOf(ae AnswerEventer) (string, bool) {
	req := x.req
	params, _ := decodeMap(req.Input)
	return ae.AnswerEvent(catalog.CallInput{TenantID: req.TenantID, AutomationID: req.AutomationID, RunID: req.RunID,
		NodeID: req.NodeID, Params: params, Subject: req.Subject, RunAs: req.RunAs})
}

// attempt is the k-th Effect of the call. The credentials are opened outside the Effect, so a failure
// to open them is never an attempt whose outcome is unknown, and nothing of them is journaled.
func (x *callExec) attempt(ctx context.Context, cn catalog.CallNode, k int, token string, afterSeq int64) (callRecord, error) {
	o, done, err := x.nr.Lookup(ctx, "call", k)
	if err != nil {
		return callRecord{}, err
	}
	// Every attempt the call made counts toward MaxAttempts: the earlier occurrences' (with the ones a
	// crash interrupted) and this one's.
	prior := 0
	for j := 0; j < k; j++ {
		pj, ok, err := x.nr.Lookup(ctx, "call", j)
		if err != nil {
			return callRecord{}, err
		}
		if ok {
			prior += pj.Attempts
		}
	}
	params, _ := decodeMap(x.req.Input)
	idempotent := catalog.IsIdempotent(cn, params)
	var conn *openConnection
	switch {
	case done && o.Status == workflow.OccStarted && !idempotent:
		// An attempt that may have reached the provider and may not repeat: its outcome is unknown, and
		// nothing (not even an outage of the credential store) keeps that from being recorded.
		conn = &openConnection{masker: code.NewMasker(nil)}
	case !done || o.Status == workflow.OccStarted:
		var refused *callRecord
		conn, refused, err = x.open(ctx, cn)
		if err != nil {
			return callRecord{}, err
		}
		if refused != nil {
			return *refused, nil
		}
	}
	// An attempt that will call the provider holds a slot. A started attempt of a call that may not
	// repeat calls nothing (its outcome is unknown); one whose deadline passed while it waited goes on
	// without a slot to record its timeout (or its unknown outcome) without calling the provider.
	if (!done || o.Status == workflow.OccStarted) && (!done || idempotent) {
		switch err := x.slot(ctx); {
		case errors.Is(err, errSlotDeadline):
		case err != nil:
			return callRecord{}, err
		default:
			defer x.release(ctx)
		}
	}
	raw, err := x.nr.Effect(ctx, "call", k, func(ctx context.Context, a workflow.EffectAttempt) (json.RawMessage, error) {
		rec, err := x.execute(ctx, cn, prior, a, conn, token, afterSeq)
		if err != nil {
			return nil, err
		}
		b, err := json.Marshal(rec)
		return withoutNUL(b), err // the journal is jsonb: a provider's message with a NUL byte is kept
	})
	if err != nil {
		var we *pool.WireError
		if errors.As(err, &we) && we.Kind == pool.ErrKindDomain {
			return callRecord{Kind: "failed", Code: strings.TrimPrefix(we.Code, "automation."), Message: we.Message}, nil
		}
		return callRecord{}, err
	}
	return readRecord(raw), nil
}

// readRecord reads the journal value of an attempt; an unreadable one fails the call.
func readRecord(raw json.RawMessage) callRecord {
	var rec callRecord
	if json.Unmarshal(raw, &rec) != nil || rec.Kind == "" {
		return callRecord{Kind: "failed", Code: graph.FailInvalidAnswer, Message: "the recorded result of the call is not readable"}
	}
	return rec
}

// openConnection is what a call attempt runs with: the connection, its credentials and their masker.
type openConnection struct {
	info   *catalog.ConnectionInfo
	creds  catalog.Credentials
	masker Masker
}

// open checks the run-as's right to the node and opens its connection; refused is a call that may not run.
func (x *callExec) open(ctx context.Context, cn catalog.CallNode) (*openConnection, *callRecord, error) {
	req := x.req
	if perm, ok := cn.(catalog.PermissionedNode); ok && perm.Permission() != "" && !req.RunAs.Has(perm.Permission()) {
		return nil, &callRecord{Kind: "failed", Code: CodeForbidden, Message: "the run-as may not use " + req.NodeType}, nil
	}
	conn := &openConnection{masker: code.NewMasker(nil)}
	if req.Connection == "" {
		return conn, nil, nil
	}
	src := x.c.r.cfg.Credentials
	if src == nil {
		return nil, &callRecord{Kind: "failed", Code: CodeConnectionUnavailable, Message: "this process has no connection store"}, nil
	}
	ci, cr, err := src.Open(ctx, req.TenantID, req.Connection)
	switch {
	case errors.Is(err, connectors.ErrNotFound), errors.Is(err, connectors.ErrNotConnected), errors.Is(err, connectors.ErrNeedsReauth), errors.Is(err, connectors.ErrUndecryptable):
		return nil, &callRecord{Kind: "failed", Code: CodeConnectionUnavailable, Message: "the connection is not connected"}, nil
	case err != nil:
		return nil, nil, &host.InfraError{Err: fmt.Errorf("open the connection of %s: %w", req.NodeID, err)}
	}
	conn.info, conn.creds, conn.masker = &ci, cr, code.NewMasker(map[string]string(cr))
	return conn, nil, nil
}

// execute is one attempt: it decides whether the call may run and calls the node.
// prior is the attempts the earlier occurrences of the call made (interrupted ones included).
func (x *callExec) execute(ctx context.Context, cn catalog.CallNode, prior int, a workflow.EffectAttempt, conn *openConnection, token string, afterSeq int64) (callRecord, error) {
	req := x.req
	limits := x.c.r.cfg.CallLimits
	params, err := decodeMap(req.Input)
	if err != nil {
		return callRecord{Kind: "failed", Code: CodeInvalidInput, Message: "the params are not an object"}, nil
	}
	idempotent := catalog.IsIdempotent(cn, params)
	if a.Attempt > 1 && !idempotent {
		// An earlier attempt may have reached the provider: the call is not repeated blindly.
		return x.unknown("an earlier attempt of the call may have run; its outcome is unknown")
	}
	if prior+a.Attempt > limits.MaxAttempts {
		// Interrupted attempts count too: past the cap the last one's outcome is recorded as unknown.
		return x.unknown(fmt.Sprintf("the call reached its %d attempts without a recorded result", limits.MaxAttempts))
	}
	if conn == nil || (req.Connection != "" && conn.info == nil) {
		return callRecord{Kind: "failed", Code: CodeConnectionUnavailable, Message: "the connection is not connected"}, nil
	}
	if !time.Now().Before(req.Deadline) {
		return callRecord{Kind: "failed", Code: CodeTimeout, Message: req.NodeID + " ran out of time"}, nil
	}
	if x.answerer != nil {
		// The token's answer is fixed: a node that would name another one now is not sent again.
		if event, single := x.answerOf(x.answerer); event != x.answer || single != x.single {
			return callRecord{Kind: "failed", Code: graph.FailInvalidAnswer,
				Message: fmt.Sprintf("%s names %s as its answer now; its token was issued for %s", req.NodeID, event, x.answer)}, nil
		}
	}
	masker := conn.masker
	var callContext map[string]any
	if len(req.Context) > 0 {
		if callContext, err = decodeMap(req.Context); err != nil {
			return callRecord{Kind: "failed", Code: CodeInvalidInput, Message: "the context is not an object"}, nil
		}
	}
	in := catalog.CallInput{
		Context:  callContext,
		TenantID: req.TenantID, AutomationID: req.AutomationID, RunID: req.RunID, NodeID: req.NodeID,
		Attempt: prior + a.Attempt, IdempotencyKey: req.IdempotencyKey, Params: params, Subject: req.Subject,
		RunAs: req.RunAs, Connection: conn.info, Credentials: conn.creds, Deadline: req.Deadline, Usage: x.c.r.cfg.Usage,
	}
	callCtx, cancel := context.WithDeadline(ctx, req.Deadline)
	defer cancel()
	if token != "" {
		callCtx = WithCorrelation(callCtx, Correlation{Token: token})
	}
	started := time.Now()
	res, err := cn.Execute(callCtx, in)
	x.c.r.metrics.callDuration(ctx, x.connector(cn), time.Since(started))
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			msg := masker.Mask(ce.Message)
			ce = &catalog.CallError{Code: callCode(ce.Code, masker), Message: ce.Message, Retryable: ce.Retryable}
			switch {
			case ce.Code == catalog.CallOutcomeUnknown:
				return x.unknown(msg)
			case ce.Code == catalog.CallRateLimited:
				return x.repeat(prior+a.Attempt, 0, limits, CodeRateLimited, msg)
			case clientRefusal(ce.Code):
				// A refusal of the request itself: repeating it gives the same answer.
				return callRecord{Kind: "failed", Code: ce.Code, Message: msg}, nil
			case ce.Retryable && idempotent:
				return x.repeat(prior+a.Attempt, 0, limits, ce.Code, msg)
			}
			return callRecord{Kind: "failed", Code: ce.Code, Message: msg, Retryable: ce.Retryable}, nil
		}
		if ctx.Err() != nil {
			return callRecord{}, ctx.Err() // the process stops: the next attempt decides
		}
		msg := masker.Mask(err.Error())
		if callCtx.Err() != nil {
			msg = req.NodeID + " got no answer by its deadline"
		}
		if idempotent {
			return x.repeat(prior+a.Attempt, 0, limits, catalog.CallUnavailable, msg)
		}
		// The request may have reached the provider before the failure.
		return x.unknown(msg)
	}
	if res.RetryAfter > 0 {
		return x.repeat(prior+a.Attempt, res.RetryAfter, limits, CodeRateLimited, "the provider asked to repeat the call later")
	}
	out, err := json.Marshal(nonNilMap(res.Output))
	if err != nil {
		return callRecord{Kind: "failed", Code: graph.FailInvalidAnswer, Message: "the output of " + req.NodeID + " is not JSON"}, nil
	}
	rec := callRecord{Kind: "result", Outcome: res.Outcome, Output: withoutNUL(masker.MaskJSON(out))}
	if asker, ok := cn.(Asker); ok && token != "" {
		aw, err := asker.Await(in, res)
		if err != nil {
			return callRecord{Kind: "failed", Code: graph.FailInvalidAnswer, Message: masker.Mask(err.Error())}, nil
		}
		timeout := aw.Timeout
		if most := x.c.r.cfg.Limits.WaitMax; timeout <= 0 || timeout > most {
			timeout = most
		}
		if aw.Event != x.answer {
			return callRecord{Kind: "failed", Code: graph.FailInvalidAnswer, Message: fmt.Sprintf("%s awaits %s but named %s as its answer", req.NodeID, aw.Event, x.answer)}, nil
		}
		// The token was issued with the named answer and policy: the wait keeps them.
		rec.Await = &AwaitSpec{Event: x.answer, TimeoutMS: timeout.Milliseconds(), Single: x.single, Token: token, AfterSeq: afterSeq}
	}
	return rec, nil
}

// InvocationIDOf is the invocation ID (a UUID) a call node passes to SDK commands (host.Request): derived
// from the call's idempotency key, so every attempt of one call is the same invocation.
func InvocationIDOf(idempotencyKey string) string { return catalog.InvocationIDOf(idempotencyKey) }

// unknown is a call whose outcome is unknown: the failed outcome when the graph connects it, else the
// child blocks (ambiguous) until an operator resolves or retries it.
func (x *callExec) unknown(msg string) (callRecord, error) {
	if !x.req.FailedConnected && !x.terminalUnknown {
		return callRecord{}, workflow.Ambiguous(fmt.Sprintf("the outcome of %s (%s) is unknown: %s", x.req.NodeID, x.req.IdempotencyKey, msg))
	}
	return callRecord{Kind: "failed", Code: CodeOutcomeUnknown, Message: msg}, nil
}

// repeat asks for the next attempt after max(retryAfter, backoff), when it fits in the deadline; else
// the call fails with code.
// made is the attempts the call made so far, this one included.
func (x *callExec) repeat(made int, retryAfter time.Duration, limits CallLimits, code, msg string) (callRecord, error) {
	backoff := limits.BackoffBase << min(max(made-1, 0), 16)
	d := max(retryAfter, backoff)
	if made >= limits.MaxAttempts || !time.Now().Add(d).Before(x.req.Deadline) {
		return callRecord{Kind: "failed", Code: code, Message: msg, Retryable: true}, nil
	}
	return callRecord{Kind: "retry", AfterMS: d.Milliseconds()}, nil
}

// callCode is the code of a connector's CallError as the call records it: an identifier (lower case,
// digits, "_", ".", "-", at most 64) with nothing a mask hides, else effect_failed: a code is journaled
// and shown as output.error.code, so provider text in it is never kept.
func callCode(code string, masker Masker) string {
	if len(code) == 0 || len(code) > 64 || masker.Mask(code) != code {
		return CodeEffectFailed
	}
	for _, r := range code {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' && r != '.' && r != '-' {
			return CodeEffectFailed
		}
	}
	return code
}

// clientRefusal reports a CallError code the provider gives for the request itself (4xx except 429):
// it is never repeated automatically.
func clientRefusal(code string) bool {
	switch code {
	case catalog.CallInvalidRequest, catalog.CallUnauthorized, catalog.CallRejected, CodeForbidden, CodeInvalidInput:
		return true
	}
	return false
}

func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

// watermark is the outbox watermark in q.
func watermark(ctx context.Context, q querier) (int64, error) {
	var seq int64
	err := q.QueryRow(ctx, `SELECT CASE WHEN is_called THEN last_value ELSE 0 END FROM extruntime.outbox_events_seq_seq`).Scan(&seq)
	return seq, serrors.FromDB(opCall, err)
}

// command is a call whose node calls a command (catalog.CommandCall): wf.call through the workflow's call
// primitive under the call's invocation ID. The workflow journals it (KindCall) and the host records the
// invocation: a call that started and has no recorded outcome is resolved from that record, or blocks the
// instance as ambiguous, and is never repeated blindly.
func (x *callExec) command(ctx context.Context, cn catalog.CallNode, cc catalog.CommandCall) (WaitResult, error) {
	req := x.req
	conn, refused, err := x.open(ctx, cn)
	if err != nil {
		return WaitResult{}, err
	}
	if refused != nil {
		return x.failed(cn, refused.Code, refused.Message, false), nil
	}
	params, err := decodeMap(req.Input)
	if err != nil {
		return x.failed(cn, CodeInvalidInput, "the params are not an object", false), nil
	}
	in := catalog.CallInput{
		TenantID: req.TenantID, AutomationID: req.AutomationID, RunID: req.RunID, NodeID: req.NodeID, Attempt: 1,
		IdempotencyKey: req.IdempotencyKey, Params: params, Subject: req.Subject, RunAs: req.RunAs,
		Connection: conn.info, Credentials: conn.creds, Deadline: req.Deadline, Usage: x.c.r.cfg.Usage,
	}
	command, input, err := cc.Command(in)
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			return x.failed(cn, callCode(ce.Code, conn.masker), conn.masker.Mask(ce.Message), ce.Retryable), nil
		}
		return WaitResult{}, err
	}
	// A recorded command is read back as it is. One that never started runs within the call's deadline
	// and holds one of the tenant's call slots, as every call does; one that started (a crash) is resolved
	// by the workflow from the host's record of its invocation.
	_, seen, err := x.nr.Lookup(ctx, "command", 0)
	if err != nil {
		return WaitResult{}, err
	}
	callCtx := ctx
	if !seen {
		if !time.Now().Before(req.Deadline) {
			return x.failed(cn, CodeTimeout, req.NodeID+" ran out of time before its command", false), nil
		}
		switch err := x.slot(ctx); {
		case errors.Is(err, errSlotDeadline):
			return x.failed(cn, CodeTimeout, req.NodeID+" ran out of time waiting for a call slot", false), nil
		case err != nil:
			return WaitResult{}, err
		default:
			defer x.release(ctx)
		}
		var cancel context.CancelFunc
		callCtx, cancel = context.WithDeadline(ctx, req.Deadline)
		defer cancel()
	}
	out, err := x.nr.CallAs(callCtx, "command", 0, command, input, catalog.InvocationIDOf(req.IdempotencyKey))
	if err != nil {
		if we, ok := pool.AsWireError(err); ok && (we.Kind == pool.ErrKindDomain || we.Kind == pool.ErrKindValidation) {
			return x.failed(cn, callCode(CommandErrorCode(we.Code), conn.masker), conn.masker.Mask(we.Message), false), nil
		}
		return WaitResult{}, err // suspend, a blocked instance, infrastructure: the workflow decides
	}
	res, err := cc.CommandResult(in, out)
	if err != nil {
		var ce *catalog.CallError
		if errors.As(err, &ce) {
			return x.failed(cn, callCode(ce.Code, conn.masker), conn.masker.Mask(ce.Message), ce.Retryable), nil
		}
		return WaitResult{}, err
	}
	raw, err := json.Marshal(nonNilMap(res.Output))
	if err != nil {
		return x.failed(cn, graph.FailInvalidAnswer, "the output of "+req.NodeID+" is not JSON", false), nil
	}
	x.c.r.metrics.call(ctx, x.connector(cn), req.NodeType, "ok")
	return WaitResult{Outcome: res.Outcome, Output: withoutNUL(conn.masker.MaskJSON(raw))}, nil
}

// CommandErrorPrefix starts the code of a domain error a command of an automation action raises; the
// call's output.error.code is the code without it (an action's own code keeps its dots).
const CommandErrorPrefix = "automation."

// CommandErrorCode is the call's error code of a command's domain error code.
func CommandErrorCode(code string) string { return strings.TrimPrefix(code, CommandErrorPrefix) }

// withoutNUL is catalog.WithoutNUL: jsonb refuses U+0000.
func withoutNUL(raw json.RawMessage) json.RawMessage { return catalog.WithoutNUL(raw) }

func replaceNUL(v any) any { return catalog.ReplaceNUL(v) }
