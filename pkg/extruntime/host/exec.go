package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opSignal = serrors.Op("extruntime.host.invoke.signal")

// WireErrorer lets data, event and workflow slices return structured errors
// (domain, validation, denied) without importing the host.
type WireErrorer interface {
	WireError() *pool.WireError
}

func asWire(err error) *pool.WireError {
	if we, ok := pool.AsWireError(err); ok {
		return we
	}
	var carrier WireErrorer
	if errors.As(err, &carrier) {
		return carrier.WireError()
	}
	if we := dataError(err); we != nil {
		return we
	}
	return pool.Internal(pool.CodeHostCall, err.Error())
}

func fail(code, format string, args ...any) *pool.WireError {
	return pool.Internal(code, fmt.Sprintf(format, args...))
}

// execution is one invocation: it records itself, owns its transaction and
// serves the host calls of the worker running it.
type execution struct {
	h        *Host
	root     *rootState
	id       string
	rootID   string
	spec     execSpec
	contract *manifest.Resource
	binding  Binding
	// native is the Go default running this invocation (no worker); nil for
	// package code.
	native *native.Implementation
	kind   pool.Kind
	scope  spi.Scope
	// starts is the tx.start key sequence this invocation draws from: its
	// parent's, unless the invocation was given a base of its own.
	starts *startSource

	mu        sync.Mutex
	ctxTx     *txHandle // open ctx.transaction of an ordinary invocation
	snapshot  *txHandle
	committed int
	// extTx is the adopted caller-owned transaction of a Run; a schedule's
	// ctx.transaction joins it, so its effects commit with the caller's receipt.
	extTx         *txHandle
	extRolledBack bool
	// engineTxs are the transactions the durable engine opened for this
	// invocation (wf.transaction) and has not ended yet.
	engineTxs map[string]struct{}
}

func (e *execution) InvocationID() string { return e.id }
func (e *execution) Scope() spi.Scope     { return e.scope }
func (e *execution) Token() string        { return e.root.token }

func (e *execution) Producer() spi.Producer {
	p := e.root.producer
	p.InvocationID = e.id
	return p
}

func (e *execution) trigger() Trigger {
	if e.spec.parent != nil {
		return e.spec.parent.trigger()
	}
	return e.spec.trigger
}

func (e *execution) resource() string {
	if e.contract != nil {
		return e.contract.ID
	}
	return e.binding.Contract
}

// MaxNesting bounds nested invocations (tx.call, tx.query, ctx.call, guards)
// under one root, so a query or operation that calls itself fails with a
// diagnostic instead of exhausting the worker stack. It is the limit of the
// native contract rules, shared with native.Direct.
const MaxNesting = native.MaxNesting

func (e *execution) depth() int {
	n := 0
	for p := e.spec.parent; p != nil; p = p.spec.parent {
		n++
	}
	return n
}

func (e *execution) run(ctx context.Context, res spi.ResolvedResource) (Result, error) {
	if e.depth() > MaxNesting {
		return Result{}, native.NestingTooDeep(e.resource())
	}
	out := Result{InvocationID: e.id, Binding: e.binding}
	parentID := ""
	if e.spec.parent != nil {
		parentID = e.spec.parent.id
	}
	hash := res.Resource.ContractHash
	if hash == "" && e.contract != nil {
		hash = manifest.ContractHashOf(*e.contract)
	}
	out.ContractHash = hash
	trig := e.trigger()
	if err := e.h.rec.start(ctx, startRecord{
		ID: e.id, ParentID: parentID, RootID: e.rootID,
		TenantID: e.scope.Subject.TenantID.String(), UserID: e.scope.Subject.UserID, SubjectKind: subjectKind(e.scope.Subject),
		Contract: e.resource(), ContractHash: hash,
		BindingID: e.binding.ID, Package: e.binding.Package, Version: e.binding.Version, Exp: e.binding.Export,
		TriggerKind: triggerKind(trig.Kind), TriggerRef: trig.Ref,
	}); err != nil {
		out.Outcome = OutcomeUnknown
		return out, err
	}

	output, err := e.dispatch(ctx)
	e.closeOpenTx(ctx)

	e.mu.Lock()
	out.CommittedTxCount = e.committed
	e.mu.Unlock()
	fin := finishRecord{ID: e.id}
	switch {
	case err == nil:
		out.Outcome = OutcomeOK
		out.Output = output
		fin.Result = output
	case pool.IsSuspend(err):
		// A workflow suspension is the invocation's normal end, not a failure.
		out.Outcome = OutcomeOK
		fin.Error = errorJSON(err)
	case errors.Is(err, pool.ErrWorkerLost):
		out.Outcome = OutcomeWorkerLost
		fin.Error = errorJSON(err)
	case e.root.infra.get() != nil:
		// A host call failed for an infrastructure reason. Whatever the handler
		// made of it, the invocation did not finish on its own terms: its
		// outcome is unknown, never a recorded business failure.
		out.Outcome = OutcomeUnknown
		fin.Error = errorJSON(err)
	default:
		out.Outcome = OutcomeError
		fin.Error = errorJSON(err)
	}
	if err != nil && !pool.IsSuspend(err) {
		out.PartiallyCommitted = out.CommittedTxCount > 0
	}
	fin.Outcome, fin.PartiallyCommitted = out.Outcome, out.PartiallyCommitted
	if ferr := e.h.rec.finish(ctx, fin); ferr != nil {
		e.h.cfg.Logger.Error("extruntime: invocation record not finished", "invocation", e.id, "error", ferr)
	}
	return out, err
}

func subjectKind(s spi.Subject) string {
	if s.Kind == spi.SubjectOwner {
		return string(spi.SubjectOwner)
	}
	return string(spi.SubjectInitiator)
}

func triggerKind(k TriggerKind) TriggerKind {
	if k == "" {
		return TriggerUI
	}
	return k
}

func errorJSON(err error) json.RawMessage {
	we := asWire(err)
	if errors.Is(err, pool.ErrWorkerLost) {
		we = &pool.WireError{Kind: pool.ErrKindInternal, Code: "runtime.workerLost", Message: err.Error()}
	}
	b, err2 := json.Marshal(we)
	if err2 != nil {
		// A payload that cannot be encoded must not hide the failure itself:
		// report its kind, code and message without the structured parts.
		b, err2 = json.Marshal(&pool.WireError{Kind: we.Kind, Code: we.Code, Message: we.Message})
		if err2 != nil {
			return json.RawMessage(`{"kind":"internal","code":"runtime.encodeError"}`)
		}
	}
	return b
}

// dispatch opens the invocation's transaction when its contract demands one,
// calls the worker, validates the output and commits or rolls back.
func (e *execution) dispatch(ctx context.Context) (json.RawMessage, error) {
	invCtx := pool.InvokeContext{
		Tenant: e.scope.Subject.TenantID.String(),
		User: pool.UserContext{
			ID:          strconv.FormatInt(e.scope.Subject.UserID, 10),
			Kind:        subjectKind(e.scope.Subject),
			Locale:      e.scope.Subject.Locale,
			Permissions: append([]string{}, e.scope.Subject.Permissions...),
		},
		Invocation: e.id,
		Trigger:    string(triggerKind(e.trigger().Kind)),
		Instance:   e.root.producer.InstanceID,
		Occurrence: e.spec.occurrence,
	}
	if e.spec.parent != nil {
		invCtx.Parent = e.spec.parent.id
	}
	var ownTx, txh *txHandle
	switch {
	case e.spec.externalTx != nil:
		txh = e.root.txs.adopt(e.spec.externalTx)
		e.mu.Lock()
		e.extTx = txh
		e.mu.Unlock()
		invCtx.Tx = txh.id
	case e.spec.join != nil && e.kind == pool.KindQuery:
		// A nested query (tx.query / q.query) reads through the joined
		// transaction via q.*; it is never handed the handle itself.
	case e.spec.join != nil:
		txh = e.spec.join
		invCtx.Tx = txh.id
	case e.kind == pool.KindAtomic || e.kind == pool.KindOperation:
		h, err := e.root.txs.begin(ctx, e.id, pgx.TxOptions{})
		if err != nil {
			return nil, err
		}
		ownTx, txh, invCtx.Tx = h, h, h.id
	case e.kind == pool.KindSubscription:
		return nil, fail("runtime.txRequired", "subscription handlers run in a caller-owned transaction")
	}
	if txh != nil && e.contract != nil && (e.kind == pool.KindAtomic || e.kind == pool.KindOperation) {
		// Operation guards run inside the transaction, before the bound
		// implementation, and cannot be bypassed by replacing it.
		if gerr := e.h.runGuards(ctx, guardRun{
			root: e.root, parent: e, live: e.spec.parent, subject: e.spec.subject, trigger: e.trigger(),
			contract: e.contract, target: e.contract.ID, phase: manifest.GuardOperation, input: e.spec.input, join: txh,
		}); gerr != nil {
			if ownTx != nil {
				_ = e.root.txs.end(ctx, ownTx, TxRollback)
			}
			return nil, gerr
		}
	}

	var (
		output json.RawMessage
		err    error
	)
	if e.native != nil {
		output, err = e.runNative(ctx, txh)
	} else {
		inv := pool.Invocation{
			ID:       e.id,
			Artifact: e.binding.Artifact,
			Kind:     e.kind,
			Export:   e.binding.Export,
			Resource: e.resource(),
			Input:    e.spec.input,
			Ctx:      invCtx,
			Calls:    e,
		}
		switch {
		case e.spec.placement:
			inv.ParentID = workerOf(e.spec.placeOn)
		case e.spec.parent != nil:
			inv.ParentID = workerOf(e.spec.parent)
		}
		output, err = e.h.cfg.Pool.Invoke(ctx, inv)
	}

	if err == nil {
		err = e.checkOutput(output)
	} else {
		err = e.closedErrors(err)
	}
	e.mu.Lock()
	rolledBack := e.extRolledBack
	e.mu.Unlock()
	if err == nil && rolledBack {
		err = fail("runtime.txRolledBack", "the handler rolled back its transaction and then returned normally; the caller-owned transaction is not committed")
	}
	if err == nil && e.spec.externalTx != nil && txh != nil && (txh.inflight.Load() > 0 || txh.abandoned.Load()) {
		// The caller commits this transaction: it must not see a handler whose
		// un-awaited tx.call is still running or was cut short.
		err = fail("runtime.nestedInFlight", "a nested invocation joined to the caller-owned transaction did not finish before the handler returned (an un-awaited tx.call?); the caller must roll back")
	}
	if ownTx != nil {
		if err != nil {
			_ = e.root.txs.end(ctx, ownTx, TxRollback)
		} else if cerr := e.root.txs.end(ctx, ownTx, TxCommit); cerr != nil {
			err = cerr
		} else {
			e.countCommitted()
		}
	}
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (e *execution) countCommitted() {
	e.mu.Lock()
	e.committed++
	e.mu.Unlock()
}

func (e *execution) checkOutput(output json.RawMessage) error {
	if e.contract == nil {
		return nil
	}
	return native.CheckOutput(*e.contract, output)
}

// closedErrors enforces that a contract only raises its declared domain
// errors; anything else is a bug of the implementation.
func (e *execution) closedErrors(err error) error {
	we, ok := pool.AsWireError(err)
	if !ok || we.Kind != pool.ErrKindDomain || e.contract == nil {
		return err
	}
	if slices.Contains(e.contract.Errors, we.Code) || isPlatformDomainCode(we.Code) {
		return err
	}
	return fail("runtime.undeclaredDomainError", "%s raised %q which its contract does not declare", e.contract.ID, we.Code)
}

// closeOpenTx rolls back what the invocation left open: its read snapshot and
// a ctx.transaction its handler abandoned (deadline, worker loss, suspend).
func (e *execution) closeOpenTx(ctx context.Context) {
	e.mu.Lock()
	snap, open := e.snapshot, e.ctxTx
	e.snapshot, e.ctxTx = nil, nil
	e.mu.Unlock()
	for _, h := range []*txHandle{snap, open} {
		if h != nil {
			_ = e.root.txs.end(ctx, h, TxRollback)
		}
	}
}

// Call serves one host call of the worker running this invocation.
func (e *execution) Call(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
	ctx = context.WithValue(ctx, execKey{}, e)
	out, err := e.call(ctx, method, args)
	if err != nil {
		if IsInfrastructure(err) {
			e.root.infra.note(err)
		}
		return nil, asWire(err)
	}
	return out, nil
}

// writeMethods are the host calls that change state or start intents. Guards
// and queries are read-only: the router rejects these from them whatever
// context object the worker built, so a hand-written artifact cannot write.
func isWriteMethod(method string) bool {
	switch method {
	case "tx.begin", "tx.end", "tx.insert", "tx.update", "tx.delete", "tx.putOverlay", "tx.emit", "tx.notify", "tx.start", "tx.signal", "tx.call",
		"invoke.command", "invoke.signal":
		return true
	}
	return strings.HasPrefix(method, "wf.")
}

func (e *execution) call(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
	if (e.kind == pool.KindGuard || e.kind == pool.KindQuery) && isWriteMethod(method) {
		return nil, fail(CodeReadOnlyContext, "%s invocations are read-only; %s is not allowed", e.kind, method)
	}
	switch method {
	case "invoke.signal":
		return e.invokeSignal(ctx, args)
	case "tx.begin":
		return e.txBegin(ctx)
	case "tx.end":
		return e.txEnd(ctx, args)
	case "tx.call":
		return e.txCall(ctx, args)
	case "invoke.command":
		return e.invokeCommand(ctx, args)
	case "secrets.get":
		return e.secret(ctx, args)
	}
	switch {
	case strings.HasPrefix(method, "tx."):
		return e.txData(ctx, method, args)
	case strings.HasPrefix(method, "q."):
		return e.queryData(ctx, method, args)
	case strings.HasPrefix(method, "wf."):
		if e.h.cfg.Workflow == nil {
			return nil, fail("runtime.notConfigured", "workflow engine is not configured")
		}
		return e.h.cfg.Workflow.CallWorkflow(ctx, e, method, args)
	}
	return nil, fail(pool.CodeHostCall, "unknown host call %q", method)
}

func decode(args json.RawMessage, into any) error {
	if err := json.Unmarshal(args, into); err != nil {
		return fail(pool.CodeHostCall, "malformed host call arguments: %v", err)
	}
	return nil
}

// OpenTx opens a transaction for the durable engine (wf.transaction); the
// worker reaches it through tx.* calls with the returned handle.
func (e *execution) OpenTx(ctx context.Context, startKeyBase string) (string, error) {
	h, err := e.root.txs.begin(ctx, e.id, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
	if startKeyBase != "" {
		h.starts = &startSource{base: startKeyBase}
	}
	e.mu.Lock()
	if e.engineTxs == nil {
		e.engineTxs = map[string]struct{}{}
	}
	e.engineTxs[h.id] = struct{}{}
	e.mu.Unlock()
	return h.id, nil
}

func (e *execution) WithTx(ctx context.Context, handle string, fn func(ctx context.Context, tx pgx.Tx) error) error {
	h, err := e.root.txs.lookup(handle)
	if err != nil {
		return err
	}
	return h.do(ctx, fn)
}

// CallInTx runs an operation or atomic command inside an engine-owned
// transaction (wf.transaction with a callable).
func (e *execution) CallInTx(ctx context.Context, handle, contract string, input json.RawMessage) (json.RawMessage, error) {
	h, err := e.root.txs.lookup(handle)
	if err != nil {
		return nil, err
	}
	out, err := e.callJoined(ctx, h, contract, input)
	return out, e.root.withInfra(err)
}

// InfraError implements WorkflowCall.
func (e *execution) InfraError() error { return e.root.infra.get() }

func (e *execution) EndTx(ctx context.Context, handle string, outcome TxOutcome) error {
	h, err := e.root.txs.lookup(handle)
	if err != nil {
		return err
	}
	err = e.root.txs.end(ctx, h, outcome)
	e.mu.Lock()
	delete(e.engineTxs, handle)
	e.mu.Unlock()
	if err != nil {
		return err
	}
	if outcome == TxCommit {
		e.countCommitted()
	}
	return nil
}

// txBegin opens the transaction of an ordinary invocation's ctx.transaction.
func (e *execution) txBegin(ctx context.Context) (json.RawMessage, error) {
	h, err := e.openCtxTx(ctx)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{"tx": h.id})
}

// openCtxTx opens the ctx.transaction of an ordinary invocation (package code
// through tx.begin, a Go default through CommandContext.Transaction).
func (e *execution) openCtxTx(ctx context.Context) (*txHandle, error) {
	if e.kind != pool.KindCommand && e.kind != pool.KindEndpoint && e.kind != pool.KindSchedule {
		return nil, fail("runtime.txNotAllowed", "%s invocations do not open transactions; they run in the transaction the host gave them", e.kind)
	}
	e.mu.Lock()
	if e.ctxTx != nil {
		e.mu.Unlock()
		return nil, fail("runtime.nestedTransaction", "ctx.transaction cannot be nested")
	}
	ext := e.extTx
	if ext != nil {
		// A run with a caller-owned transaction (a schedule slot with its
		// receipt): ctx.transaction joins it instead of opening another one.
		e.ctxTx = ext
		e.mu.Unlock()
		return ext, nil
	}
	e.mu.Unlock()
	h, err := e.root.txs.begin(ctx, e.id, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	if e.ctxTx != nil {
		// A concurrent ctx.transaction won the race.
		e.mu.Unlock()
		_ = e.root.txs.end(ctx, h, TxRollback)
		return nil, fail("runtime.nestedTransaction", "ctx.transaction cannot be nested")
	}
	e.ctxTx = h
	e.mu.Unlock()
	return h, nil
}

func (e *execution) txEnd(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Tx      string
		Outcome TxOutcome
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	if err := e.endCtxTx(ctx, a.Tx, a.Outcome); err != nil {
		return nil, err
	}
	return json.RawMessage(`null`), nil
}

// endCtxTx commits or rolls back the open ctx.transaction.
func (e *execution) endCtxTx(ctx context.Context, handle string, outcome TxOutcome) error {
	e.mu.Lock()
	h := e.ctxTx
	e.mu.Unlock()
	if h == nil || h.id != handle {
		return fail("runtime.txUnknown", "transaction handle is not the open ctx.transaction of this invocation")
	}
	if outcome != TxCommit && outcome != TxRollback {
		return fail(pool.CodeHostCall, "unknown tx.end outcome %q", outcome)
	}
	if h.external {
		// The caller commits; a rollback poisons the run, because part of the
		// caller's transaction would otherwise commit anyway.
		e.mu.Lock()
		e.ctxTx = nil
		if outcome == TxRollback {
			e.extRolledBack = true
		}
		e.mu.Unlock()
		return nil
	}
	err := e.root.txs.end(ctx, h, outcome)
	e.mu.Lock()
	e.ctxTx = nil
	e.mu.Unlock()
	if err != nil {
		return err
	}
	if outcome == TxCommit {
		e.countCommitted()
	}
	return nil
}

func (e *execution) invokeNested(ctx context.Context, contract string, input json.RawMessage, join *txHandle) (json.RawMessage, error) {
	res, err := e.invokeNestedResult(ctx, contract, input, join, "")
	if err != nil {
		return nil, err
	}
	return res.Output, nil
}

func (e *execution) invokeNestedResult(ctx context.Context, contract string, input json.RawMessage, join *txHandle, id string) (Result, error) {
	return e.h.execute(ctx, e.root, execSpec{
		contract:   contract,
		input:      input,
		subject:    e.spec.subject,
		trigger:    e.trigger(),
		occurrence: nil,
		parent:     e,
		join:       join,
		id:         id,
	})
}

func callResult(res Result) CallResult {
	return CallResult{
		InvocationID: res.InvocationID, Binding: res.Binding, ContractHash: res.ContractHash, Output: res.Output, Outcome: res.Outcome,
		CommittedTxCount: res.CommittedTxCount, PartiallyCommitted: res.PartiallyCommitted,
	}
}

// CallCommand is wf.call: a new invocation of a command that selects the
// currently active binding.
func (e *execution) CallCommand(ctx context.Context, contract string, input json.RawMessage, invocationID string) (CallResult, error) {
	if err := e.rejectIntentInTx("wf.call"); err != nil {
		return CallResult{}, err
	}
	var base string
	if invocationID != "" {
		// An ordinary command is never replayed: its pre-allocated invocation is
		// the stable identity its tx.start keys derive from.
		base = "wf.call:" + invocationID
	}
	res, err := e.h.execute(ctx, e.root, execSpec{
		contract: contract, input: input, subject: e.spec.subject, trigger: e.trigger(),
		parent: e, id: invocationID, startBase: base,
	})
	return callResult(res), e.root.withInfra(err)
}

// CallCommandInTx runs an atomic command joined to the engine's transaction.
func (e *execution) CallCommandInTx(ctx context.Context, handle, contract string, input json.RawMessage) (CallResult, error) {
	h, err := e.root.txs.lookup(handle)
	if err != nil {
		return CallResult{}, err
	}
	rc, err := e.h.resolveContract(ctx, contract)
	if err != nil {
		return CallResult{}, err
	}
	if rc.Resource.Kind != manifest.ResCommand || !rc.Resource.Atomic {
		return CallResult{}, fail("runtime.notAtomic", "%s is not an atomic command", contract)
	}
	res, err := e.invokeNestedResult(ctx, contract, input, h, "")
	return callResult(res), e.root.withInfra(err)
}

// txCall dispatches an operation or atomic command to its active
// implementation inside the caller's transaction. An ordinary command is
// rejected here even if the types were bypassed.
func (e *execution) txCall(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Tx       string
		Contract string
		Input    json.RawMessage
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	h, err := e.root.txs.lookup(a.Tx)
	if err != nil {
		return nil, err
	}
	return e.callJoined(ctx, h, a.Contract, a.Input)
}

func (e *execution) callJoined(ctx context.Context, h *txHandle, contract string, input json.RawMessage) (json.RawMessage, error) {
	res, err := e.h.resolveContract(ctx, contract)
	if err != nil {
		return nil, err
	}
	if err := native.CheckCall(native.ViaTxCall, res.Resource); err != nil {
		return nil, err
	}
	return e.invokeNested(ctx, contract, input, h)
}

// rejectIntentInTx refuses an ordinary intent while a Go transaction is open
// in this invocation: the open ctx.transaction of an ordinary invocation, the
// invocation-long transaction of an atomic command, operation or subscription
// handler, or a wf.transaction of the durable engine.
func (e *execution) rejectIntentInTx(what string) error {
	e.mu.Lock()
	open := e.ctxTx != nil || len(e.engineTxs) > 0
	e.mu.Unlock()
	switch e.kind {
	case pool.KindAtomic, pool.KindOperation, pool.KindSubscription:
		open = true
	case pool.KindCommand, pool.KindQuery, pool.KindWorkflow, pool.KindGuard,
		pool.KindTemplate, pool.KindEndpoint, pool.KindSchedule:
		// open only while a Go transaction is
	}
	if open {
		return fail(CodeIntentInTransaction, "%s is not allowed while a transaction is open; finish the transaction first (use tx.signal / tx.start inside it)", what)
	}
	return nil
}

// invokeCommand is ctx.call(Command): a new invocation that selects the
// current active binding.
func (e *execution) invokeCommand(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Command string
		Input   json.RawMessage
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	if err := e.rejectIntentInTx("ctx.call"); err != nil {
		return nil, err
	}
	res, err := e.h.resolveContract(ctx, a.Command)
	if err != nil {
		return nil, err
	}
	if err := native.CheckCall(native.ViaCtxCall, res.Resource); err != nil {
		return nil, err
	}
	return e.invokeNested(ctx, a.Command, a.Input, nil)
}

func (e *execution) secret(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct{ Name string }
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	if !slices.Contains(e.binding.Secrets, a.Name) {
		return nil, &pool.WireError{Kind: pool.ErrKindDenied, Code: "runtime.secretNotDeclared", Message: "secret " + a.Name + " is not declared in the package capabilities"}
	}
	if e.h.cfg.Secrets == nil {
		return nil, fail("runtime.notConfigured", "secret resolver is not configured")
	}
	v, err := e.h.cfg.Secrets.Secret(ctx, e.scope, a.Name)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// queryData serves q.* from a per-invocation read-only snapshot.
func (e *execution) queryData(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
	h, err := e.readSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	switch method {
	case "q.get", "q.find", "q.count":
		return e.dataRead(ctx, h, "tx."+strings.TrimPrefix(method, "q."), args)
	case "q.query":
		return e.nestedQuery(ctx, h, args)
	}
	return nil, fail(pool.CodeHostCall, "unknown host call %q", method)
}

// nestedQuery runs a query contract's active implementation as a nested
// invocation that reads through the handle h (the caller's transaction for
// tx.query, its read snapshot for q.query), with the caller's rights. It is
// validated against the contract, permission-checked and recorded with the
// caller as parent like any invocation.
func (e *execution) nestedQuery(ctx context.Context, h *txHandle, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Query string
		Input json.RawMessage
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	if len(a.Input) == 0 {
		a.Input = json.RawMessage(`null`)
	}
	res, err := e.h.resolveContract(ctx, a.Query)
	if err != nil {
		return nil, err
	}
	if err := native.CheckCall(native.ViaQuery, res.Resource); err != nil {
		return nil, err
	}
	return e.runQuery(ctx, h, a.Query, a.Input)
}

// runQuery runs a query contract nested in this invocation, reading through h.
func (e *execution) runQuery(ctx context.Context, h *txHandle, query string, input json.RawMessage) (json.RawMessage, error) {
	out, err := e.invokeNested(ctx, query, input, h)
	if errors.Is(err, ErrNoBinding) {
		return nil, fail(CodeNativeQueryUnavailable, "query %s has no implementation (neither a Go default nor a bound package export)", query)
	}
	return out, err
}

func (e *execution) readSnapshot(ctx context.Context) (*txHandle, error) {
	if e.spec.join != nil && e.kind == pool.KindQuery {
		return e.spec.join, nil
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.snapshot != nil {
		return e.snapshot, nil
	}
	h, err := e.root.txs.begin(ctx, "", pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	e.snapshot = h
	return h, nil
}

func (e *execution) txData(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
	var a struct{ Tx string }
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	h, err := e.root.txs.lookup(a.Tx)
	if err != nil {
		return nil, err
	}
	switch method {
	case "tx.get", "tx.find", "tx.count":
		return e.dataRead(ctx, h, method, args)
	case "tx.insert", "tx.update", "tx.delete", "tx.putOverlay":
		return e.dataWrite(ctx, h, method, args)
	case "tx.query":
		return e.nestedQuery(ctx, h, args)
	case "tx.emit", "tx.notify", "tx.start", "tx.signal":
		return e.intent(ctx, h, method, args)
	}
	return nil, fail(pool.CodeHostCall, "unknown host call %q", method)
}

func (e *execution) dataRead(ctx context.Context, h *txHandle, method string, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Entity string
		Key    string
		Query  spi.FindQuery
		Where  *manifest.Filter
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	var out json.RawMessage
	err := h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		switch method {
		case "tx.get":
			out, err = e.h.cfg.Data.Get(ctx, tx, e.scope, a.Entity, a.Key)
		case "tx.find":
			out, err = e.h.cfg.Data.Find(ctx, tx, e.scope, a.Entity, a.Query)
		case "tx.count":
			var n int64
			if n, err = e.h.cfg.Data.Count(ctx, tx, e.scope, a.Entity, a.Where); err == nil {
				out, err = json.Marshal(n)
			}
		}
		return err
	})
	return nullIfEmptyJSON(out), err
}

func nullIfEmptyJSON(b json.RawMessage) json.RawMessage {
	if len(b) == 0 {
		return json.RawMessage(`null`)
	}
	return b
}

// dataWrite applies generic writes. Only entities declared by packages ("own")
// are writable; native entities change through published operations.
func (e *execution) dataWrite(ctx context.Context, h *txHandle, method string, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Entity  string
		Overlay string
		Key     string
		BaseKey string
		Values  json.RawMessage
		Patch   json.RawMessage
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	target, want := a.Entity, manifest.ResEntity
	if method == "tx.putOverlay" {
		target, want = a.Overlay, manifest.ResOverlay
	}
	res, err := e.h.resolveContract(ctx, target)
	if err != nil {
		return nil, err
	}
	if res.Native {
		return nil, &pool.WireError{Kind: pool.ErrKindDenied, Code: CodeDataNativeWrite, Message: fmt.Sprintf("%s is native: native entities are read-only through generic data calls; use a published operation", target)}
	}
	if res.Resource.Kind != want {
		return nil, &pool.WireError{Kind: pool.ErrKindDenied, Code: "runtime.notWritable", Message: fmt.Sprintf("%s is not a writable package-owned %s", target, want)}
	}
	var out json.RawMessage
	err = h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
		var err error
		switch method {
		case "tx.insert":
			out, err = e.h.cfg.Data.Insert(ctx, tx, e.scope, a.Entity, a.Values)
		case "tx.update":
			out, err = e.h.cfg.Data.Update(ctx, tx, e.scope, a.Entity, a.Key, a.Patch)
		case "tx.delete":
			err = e.h.cfg.Data.Delete(ctx, tx, e.scope, a.Entity, a.Key)
		case "tx.putOverlay":
			out, err = e.h.cfg.Data.PutOverlay(ctx, tx, e.scope, a.Overlay, a.BaseKey, a.Values)
		}
		return err
	})
	return nullIfEmptyJSON(out), err
}

func (e *execution) producer() spi.Producer {
	p := e.root.producer
	p.InvocationID = e.id
	return p
}

// intent routes emit, notify and start to the pluggable slices, handing them
// the tx so they write atomically with the data.
func (e *execution) intent(ctx context.Context, h *txHandle, method string, args json.RawMessage) (json.RawMessage, error) {
	switch method {
	case "tx.emit":
		var a struct {
			Event   string
			Subject *spi.ObjectRef
			Payload json.RawMessage
		}
		if err := decode(args, &a); err != nil {
			return nil, err
		}
		if e.h.cfg.Outbox == nil {
			return nil, fail("runtime.notConfigured", "event outbox is not configured")
		}
		var id string
		err := h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
			var err error
			id, err = e.h.cfg.Outbox.Emit(ctx, tx, e.scope, spi.EventIntent{Event: a.Event, Subject: a.Subject, Payload: a.Payload, Producer: e.producer()})
			return err
		})
		return marshalID("eventId", id, err)
	case "tx.notify":
		var a struct {
			Notice   string
			Subject  *spi.ObjectRef
			Payload  json.RawMessage
			To       []wireRecipient
			Channels []string
		}
		if err := decode(args, &a); err != nil {
			return nil, err
		}
		if e.h.cfg.Notifier == nil {
			return nil, fail("runtime.notConfigured", "notifier is not configured")
		}
		recipients, err := toRecipients(a.To)
		if err != nil {
			return nil, err
		}
		var id string
		err = h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
			var err error
			id, err = e.h.cfg.Notifier.Enqueue(ctx, tx, e.scope, spi.NoticeIntent{
				Notice: a.Notice, Version: e.binding.Version, Subject: a.Subject, Payload: a.Payload,
				Recipients: recipients, Channels: a.Channels, Producer: e.producer(),
			})
			return err
		})
		return marshalID("noticeId", id, err)
	case "tx.signal":
		in, err := e.signalIntent(args)
		if err != nil {
			return nil, err
		}
		err = h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
			return e.h.cfg.Signals.SignalInTx(ctx, tx, e.scope, in)
		})
		if err != nil {
			return nil, err
		}
		return json.RawMessage(`null`), nil
	default: // tx.start
		var a struct {
			Workflow string
			Input    json.RawMessage
			Key      string
		}
		if err := decode(args, &a); err != nil {
			return nil, err
		}
		if e.h.cfg.Starter == nil {
			return nil, fail("runtime.notConfigured", "workflow starter is not configured")
		}
		key := a.Key
		if key == "" {
			// The handle's own sequence (a workflow occurrence) wins over the
			// invocation's (a delivery, a slot, a workflow-called command).
			src := h.starts
			if src == nil {
				src = e.starts
			}
			var ok bool
			if key, ok = src.next(); !ok {
				return nil, fail("runtime.startKeyRequired", "tx.start needs {key} outside subscriptions, schedules and workflows")
			}
		}
		var id string
		err := h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
			var err error
			id, err = e.h.cfg.Starter.Start(ctx, tx, e.scope, spi.StartIntent{Workflow: a.Workflow, Input: a.Input, Key: key, Producer: e.producer()})
			return err
		})
		return marshalID("instance", id, err)
	}
}

func (e *execution) signalIntent(args json.RawMessage) (SignalIntent, error) {
	var in SignalIntent
	if err := decode(args, &in); err != nil {
		return in, err
	}
	if e.h.cfg.Signals == nil {
		return in, fail("runtime.notConfigured", "workflow signals are not configured")
	}
	in.Producer = e.producer()
	return in, nil
}

// invokeSignal is ctx.signal: the signal is written in its own short
// transaction. It is an ordinary intent, so it is refused while this
// invocation holds a transaction (tx.signal is the atomic variant).
func (e *execution) invokeSignal(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	if err := e.rejectIntentInTx("ctx.signal"); err != nil {
		return nil, err
	}
	in, err := e.signalIntent(args)
	if err != nil {
		return nil, err
	}
	tx, err := e.h.cfg.DB.Begin(ctx)
	if err != nil {
		return nil, serrors.FromDB(opSignal, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if err := e.h.cfg.Signals.SignalInTx(ctx, tx, e.scope, in); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serrors.FromDB(opSignal, err)
	}
	return json.RawMessage(`null`), nil
}

func marshalID(field, id string, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]string{field: id})
}

type wireRecipient struct {
	Kind    spi.RecipientKind `json:"kind"`
	User    *spi.ObjectRef    `json:"user"`
	Record  *spi.ObjectRef    `json:"record"`
	Address map[string]string `json:"address"`
}

func toRecipients(in []wireRecipient) ([]spi.Recipient, error) {
	out := make([]spi.Recipient, 0, len(in))
	for _, r := range in {
		rec := spi.Recipient{Kind: r.Kind, Record: r.Record, Address: r.Address}
		if r.Kind == spi.RecipientUser {
			if r.User == nil {
				return nil, fail(pool.CodeHostCall, "user recipient needs a user ref")
			}
			id, err := strconv.ParseInt(r.User.Key, 10, 64)
			if err != nil {
				return nil, fail(pool.CodeHostCall, "user recipient key %q is not a user id", r.User.Key)
			}
			rec.UserID = id
		}
		out = append(out, rec)
	}
	return out, nil
}
