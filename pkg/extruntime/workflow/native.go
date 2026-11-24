package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const opNative = serrors.Op("extruntime.workflow.native")

// NativeContract is the Go implementation of one workflow contract.
type NativeContract struct {
	Runner NativeRunner
	// Signals are the contract's signal names and payload types; Signal and
	// SignalInTx check them as they check the catalog's Signals of package
	// workflows, and WaitFor accepts only these names.
	Signals map[string]manifest.Type
	// Input, when set, validates the input of a start.
	Input *manifest.Type
	// Permission, when set, is required of a subject that starts the workflow
	// through spi.Starter or NativeRun.Start, and of the sender of a
	// SignalInTx, with the rights it holds now.
	Permission string
	// Concurrency is how many runs of this contract the process executes at
	// once: its own slot pool, so these runs never take the slots of package
	// workflows. 0 → 4.
	Concurrency int
	// RunTimeout bounds one run (up to its next suspension). 0 → 5m.
	RunTimeout time.Duration
	// Completion says what happens to open waits and tasks when the instance
	// completes or fails. The zero value is CancelPending.
	Completion CompletionPolicy
	// StatusObserver, when set, is called in the transaction in which the
	// engine moves an instance of this contract to blocked, terminated,
	// failed or completed. Its error rolls the transition back.
	StatusObserver StatusObserver
	// HumanOnly, when set, marks the contract's instances and their human tasks as ones only a person
	// acts on through the product's own pages: agents' management tools (pkg/extruntime/mcptools) refuse
	// to signal, decide, terminate, restart, resolve, retry or resume them and point to the page it
	// returns.
	HumanOnly HumanPage
}

// HumanPage is the page where a person acts on an instance of a human-only contract; "" when there is
// none to name.
type HumanPage func(ctx context.Context, inst Instance) (string, error)

// CompletionPolicy decides the fate of an instance's open waits and tasks
// when it completes or fails.
type CompletionPolicy int

const (
	// CancelPending cancels the waiting occurrences and open tasks in the
	// transaction of the transition: the tasks leave the inbox.
	CancelPending CompletionPolicy = iota
	// KeepOpen leaves them as they are (what package workflows do).
	KeepOpen
)

// StatusObserver follows the status of the instances of a native contract.
// Consumers read the status from workflow_instances (a join) or react here;
// they never keep a copy.
type StatusObserver interface {
	InstanceStatusChanged(ctx context.Context, tx pgx.Tx, inst Instance, from, to string) error
}

// NativeRunner executes the runs of a native contract.
type NativeRunner interface {
	// Run executes one run of the instance. (out, nil) completes it with the
	// result out; ErrSuspend (or an error wrapping it) suspends it until a
	// wake time, signal or task decision; any other error fails it. A failure
	// of the platform (the database, the lease) is never a failure of the
	// instance: the run is retried.
	Run(ctx context.Context, run NativeRun) (json.RawMessage, error)
}

// NativeRunnerFunc adapts a function to NativeRunner.
type NativeRunnerFunc func(ctx context.Context, run NativeRun) (json.RawMessage, error)

func (f NativeRunnerFunc) Run(ctx context.Context, run NativeRun) (json.RawMessage, error) {
	return f(ctx, run)
}

var (
	// ErrSuspend suspends a run until its next wake condition.
	ErrSuspend = errors.New("extruntime workflow: suspend")
	// ErrCancelled is returned by WaitFor and Sleep for an occurrence that
	// was cancelled (NativeRun.Cancel, terminate).
	ErrCancelled = errors.New("extruntime workflow: occurrence cancelled")
	// ErrFenced is matched by the error of a primitive whose run lost its
	// lease or whose instance is no longer running: nothing was written.
	ErrFenced = errors.New("extruntime workflow: the run lost its lease or the instance is no longer running")
)

// NativeRun is the typed facade over the wf.* primitives for a native run;
// it is valid only during the run's lease. Every call addresses an
// occurrence (name, n), where the runner chooses n (it is not a per-name
// counter).
type NativeRun interface {
	Instance() Instance
	Subject() spi.Subject
	// Transaction runs fn in a fenced transaction: fn writes the consumer's
	// state and domain effects, and the receipt (name, n) commits with them.
	// The fence is checked at the end, after fn, so a lock fn takes comes
	// before the instance row's. A repeat with the same (name, n) after the
	// commit returns the recorded result without calling fn.
	Transaction(ctx context.Context, name string, n int, fn func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error)) (json.RawMessage, error)
	// WaitFor takes the oldest unconsumed signal of the name; ErrSuspend
	// while there is none and the timeout (0: none) has not passed.
	WaitFor(ctx context.Context, name string, n int, signal string, timeout time.Duration) (WaitResult, error)
	// Sleep returns ErrSuspend until the wake time.
	Sleep(ctx context.Context, name string, n int, d time.Duration) error
	// Task opens a human task; ErrSuspend while it is open.
	Task(ctx context.Context, name string, n int, spec TaskSpec) (TaskResult, error)
	// Effect runs an external effect outside any transaction. The engine
	// records the occurrence as started, counts the attempt and calls fn.
	// Attempt > 1 means an earlier attempt may have run: fn decides (repeat
	// with the same Key, or report the outcome as unknown).
	Effect(ctx context.Context, name string, n int, fn func(ctx context.Context, a EffectAttempt) (json.RawMessage, error)) (json.RawMessage, error)
	// Call is wf.call of a native or package command. An atomic command
	// commits the receipt in its own transaction; an ordinary one is recorded
	// as started before dispatch, and a started call without a result blocks
	// the instance (ambiguous).
	Call(ctx context.Context, name string, n int, command string, input json.RawMessage) (json.RawMessage, error)
	// Start starts a child instance with start key instance:name:n and the
	// current instance as parent.
	Start(ctx context.Context, name string, n int, workflow string, input json.RawMessage, opts StartOptions) (uuid.UUID, error)
	// Cancel cancels a waiting occurrence, and its task when it is one. A
	// repeat (or an occurrence that is not waiting) is a no-op, and so is the
	// cancel of a task that already closed: Task then returns its outcome.
	Cancel(ctx context.Context, name string, n int) error
	// Lookup reads the journal on demand.
	Lookup(ctx context.Context, name string, n int) (Occurrence, bool, error)
}

// WaitResult is the outcome of a WaitFor.
type WaitResult struct {
	Payload  json.RawMessage
	TimedOut bool
}

// EffectAttempt identifies one attempt of an Effect.
type EffectAttempt struct {
	// Attempt is 1 for the first attempt.
	Attempt int
	// Key is the stable idempotency key instance:name:n.
	Key string
}

// StartOptions tune NativeRun.Start.
type StartOptions struct {
	// Subject is who the child acts as; nil: the parent's subject. It must
	// belong to the parent's tenant.
	Subject *spi.Subject
}

// Ambiguous is what the fn of an Effect returns when the outcome of an earlier attempt is unknown and the
// effect may not be repeated (it is not idempotent): the occurrence stays started and the instance blocks
// as BlockAmbiguous. Engine.Resolve records the outcome found by hand (the Effect returns it after
// Resume); Engine.Retry forgets the occurrence, so the effect runs again as attempt 1.
func Ambiguous(detail string) error { return &ambiguousError{detail: detail} }

type ambiguousError struct{ detail string }

func (e *ambiguousError) Error() string { return "extruntime workflow: ambiguous effect: " + e.detail }

// fencedError is a fencing failure as a native runner sees it.
type fencedError struct{ we *pool.WireError }

func (f *fencedError) Error() string        { return f.we.Error() }
func (f *fencedError) Is(target error) bool { return target == ErrFenced }
func (f *fencedError) Unwrap() error        { return f.we }

func suspended(kind, name string, n int) error {
	return fmt.Errorf("%w: %s %s#%d", ErrSuspend, kind, name, n)
}

// runNative executes one run of a native instance. It skips everything that
// pins package code (PackageGate, artifacts, the journal preload).
func (e *Engine) runNative(runCtx, parent context.Context, rs *runState) error {
	inst := rs.inst
	if rs.nativeErr != nil {
		return e.finishTransient(parent, rs, rs.nativeErr)
	}
	if rs.native == nil {
		return e.finishBlocked(parent, rs, BlockUnavailable, fmt.Sprintf("workflow %s has no Go implementation in this process", inst.Workflow))
	}
	nc := *rs.native
	subject, err := e.cfg.Subjects.LoadSubject(runCtx, inst.TenantID, inst.SubjectUserID, inst.SubjectKind)
	switch {
	case parent.Err() != nil:
		// The engine stopped while the subject loaded: no failure is counted.
		return e.abandon(parent, rs)
	case errors.Is(err, host.ErrSubjectBlocked):
		return e.finishBlocked(parent, rs, BlockSubject, err.Error())
	case err != nil:
		return e.finishTransient(parent, rs, err)
	}
	subject.TenantID, subject.UserID, subject.Kind = inst.TenantID, inst.SubjectUserID, inst.SubjectKind
	rs.subject = subject

	out, runErr := safeRun(runCtx, nc.Runner, &nativeRun{e: e, rs: rs, nc: nc})

	if code, abandoned := rs.staleCode(); abandoned {
		e.cfg.Logger.Info("extruntime: native workflow run abandoned", "instance", inst.ID, "reason", code)
		return nil
	}
	if parent.Err() != nil {
		// The engine is stopping. Whatever the run ended with (an aborted
		// primitive whose database call the shutdown cut short, a block, a
		// result) is not decided here: the lease is released without counting
		// a failure, and the next process replays the run.
		return e.abandon(parent, rs)
	}
	if b := rs.blockedInfo(); b != nil {
		return e.finishBlocked(parent, rs, b.reason, b.detail)
	}
	if ab := rs.aborted(); ab != nil {
		return e.finishTransient(parent, rs, ab)
	}
	if err := runCtx.Err(); err != nil {
		// The run outlived its timeout: whatever the runner returned after
		// that is not accepted; the run is retried with backoff.
		return e.finishTransient(parent, rs, err)
	}
	switch {
	case runErr == nil:
		if len(out) > 0 && !json.Valid(out) {
			return e.finishFailed(parent, rs, internalErr("runtime.invalidOutput", "workflow %s returned a result that is not JSON", inst.Workflow))
		}
		return e.finishCompleted(parent, rs, out)
	case errors.Is(runErr, ErrSuspend):
		return e.finishSuspended(parent, rs)
	case errors.Is(runErr, ErrFenced):
		return nil
	case parent.Err() != nil, runCtx.Err() != nil, host.IsInfrastructure(runErr):
		// Shutdown, run timeout or a platform failure: nothing is known about
		// the business logic, so the run is retried.
		return e.finishTransient(parent, rs, runErr)
	}
	return e.finishFailed(parent, rs, failureOf(runErr))
}

// safeRun turns a panic of the runner into a failure of the run.
func safeRun(ctx context.Context, r NativeRunner, run NativeRun) (json.RawMessage, error) {
	var (
		out json.RawMessage
		err error
	)
	func() {
		defer func() {
			if p := recover(); p != nil {
				out, err = nil, internalErr("workflow.runnerPanic", "native workflow runner panicked: %v", p)
			}
		}()
		out, err = r.Run(ctx, run)
	}()
	return out, err
}

// failureOf is the recorded error of a failed native run or effect.
func failureOf(err error) *pool.WireError {
	if we, ok := pool.AsWireError(err); ok {
		return we
	}
	var carrier host.WireErrorer
	if errors.As(err, &carrier) {
		return carrier.WireError()
	}
	return internalErr("workflow.failed", "%s", err.Error())
}

// nativeRun implements NativeRun over one run state.
type nativeRun struct {
	e  *Engine
	rs *runState
	nc NativeContract
}

var _ NativeRun = (*nativeRun)(nil)

func (r *nativeRun) Instance() Instance   { return r.rs.inst }
func (r *nativeRun) Subject() spi.Subject { return r.rs.subject }

// enter refuses a primitive of a run that may record nothing more.
func (r *nativeRun) enter() error {
	if we := r.rs.stale(); we != nil {
		return &fencedError{we: we}
	}
	if r.rs.aborted() != nil {
		return errRunAborted
	}
	if b := r.rs.blockedInfo(); b != nil {
		return blockedWire(b)
	}
	return nil
}

// exit classifies a primitive's error: fencing becomes ErrFenced, a platform
// failure aborts the run (retried, never failed).
func (r *nativeRun) exit(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, errLeaseLost) || errors.Is(err, errTerminated) {
		if we := r.rs.stale(); we != nil {
			return &fencedError{we: we}
		}
		var we *pool.WireError
		if errors.As(err, &we) {
			return &fencedError{we: we}
		}
	}
	if host.IsInfrastructure(err) {
		r.rs.abort(err)
	}
	return err
}

// lookup returns the occurrence and checks that it is of kind (a different
// kind is nondeterminism and blocks the instance).
func (r *nativeRun) lookup(ctx context.Context, name string, n int, kind string) (Occurrence, bool, error) {
	if err := r.enter(); err != nil {
		return Occurrence{}, false, err
	}
	if name == "" || n < 0 {
		return Occurrence{}, false, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.invalidOccurrence", Message: fmt.Sprintf("occurrence %q#%d needs a name and n >= 0", name, n)}
	}
	o, ok, err := r.e.occ(ctx, r.rs, name, n)
	if err != nil {
		return o, ok, r.exit(err)
	}
	if ok && o.Kind != kind {
		return o, ok, r.rs.nondeterminism(name, n, kind, o)
	}
	return o, ok, nil
}

func (r *nativeRun) Lookup(ctx context.Context, name string, n int) (Occurrence, bool, error) {
	if err := r.enter(); err != nil {
		return Occurrence{}, false, err
	}
	o, ok, err := r.e.occ(ctx, r.rs, name, n)
	return o, ok, r.exit(err)
}

// txContext is what fn of a Transaction gets: the transaction for the
// repositories (composables.UseTx), the instance's tenant and the pool.
func (r *nativeRun) txContext(ctx context.Context, tx pgx.Tx) context.Context {
	ctx = composables.WithTx(ctx, tx)
	if tid, err := composables.UseTenantID(ctx); err != nil || tid != r.rs.inst.TenantID {
		ctx = composables.WithTenantID(ctx, r.rs.inst.TenantID)
	}
	if ctx.Value(constants.PoolKey) == nil {
		ctx = composables.WithPool(ctx, r.e.cfg.DB)
	}
	return ctx
}

func (r *nativeRun) Transaction(ctx context.Context, name string, n int, fn func(ctx context.Context, tx pgx.Tx) (json.RawMessage, error)) (json.RawMessage, error) {
	o, ok, err := r.lookup(ctx, name, n, KindTransaction)
	if err != nil {
		return nil, err
	}
	if ok {
		if o.Status != OccCompleted {
			return nil, r.rs.nondeterminism(name, n, KindTransaction, o)
		}
		return nonNull(o.Result), nil
	}
	tx, err := r.e.cfg.DB.Begin(ctx)
	if err != nil {
		return nil, r.exit(serrors.FromDB(opNative, err))
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	out, err := fn(r.txContext(ctx, tx), tx)
	if err != nil {
		return nil, r.exit(err)
	}
	if len(out) > 0 && !json.Valid(out) {
		return nil, internalErr("workflow.invalidResult", "transaction %s#%d returned a result that is not JSON", name, n)
	}
	// The fence comes last: the locks fn took (a subject lock of the
	// consumer) precede the instance row's, the order signals and decisions
	// keep too.
	if err := r.e.fence(ctx, tx, r.rs); err != nil {
		return nil, r.exit(err)
	}
	rec := Occurrence{Name: name, N: n, Kind: KindTransaction, Status: OccCompleted, Result: nonNull(out), Attempts: 1}
	if err := putOccurrence(ctx, tx, r.rs, rec); err != nil {
		return nil, r.exit(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, r.exit(serrors.FromDB(opNative, err))
	}
	r.rs.set(rec)
	return rec.Result, nil
}

func (r *nativeRun) WaitFor(ctx context.Context, name string, n int, signal string, timeout time.Duration) (WaitResult, error) {
	if _, declared := r.nc.Signals[signal]; !declared {
		return WaitResult{}, &pool.WireError{Kind: pool.ErrKindInternal, Code: CodeUnknownSignal, Message: fmt.Sprintf("workflow %s declares no signal %q", r.rs.inst.Workflow, signal)}
	}
	o, ok, err := r.lookup(ctx, name, n, KindWaitFor)
	if err != nil {
		return WaitResult{}, err
	}
	var result json.RawMessage
	switch {
	case ok && o.Status == OccCancelled:
		return WaitResult{}, fmt.Errorf("%w: waitFor %s#%d", ErrCancelled, name, n)
	case ok && o.Status == OccCompleted:
		result = o.Result
	case ok && o.Status != OccWaiting:
		return WaitResult{}, r.rs.nondeterminism(name, n, KindWaitFor, o)
	default:
		var wake *time.Time
		result, wake, err = r.e.waitRecord(ctx, r.rs, name, n, signal, timeout, timeout > 0, o, ok)
		if err != nil {
			return WaitResult{}, r.exit(err)
		}
		if result == nil {
			_ = wake
			return WaitResult{}, suspended("waitFor", name, n)
		}
	}
	var w struct {
		Outcome string          `json:"outcome"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(result, &w); err != nil {
		return WaitResult{}, internalErr("workflow.corruptJournal", "occurrence %s#%d has an unreadable result", name, n)
	}
	return WaitResult{Payload: w.Payload, TimedOut: w.Outcome == "timeout"}, nil
}

func (r *nativeRun) Sleep(ctx context.Context, name string, n int, d time.Duration) error {
	o, ok, err := r.lookup(ctx, name, n, KindSleep)
	if err != nil {
		return err
	}
	switch {
	case ok && o.Status == OccCancelled:
		return fmt.Errorf("%w: sleep %s#%d", ErrCancelled, name, n)
	case ok && o.Status == OccCompleted:
		return nil
	case ok && (o.Status != OccWaiting || o.WakeAt == nil):
		return r.rs.nondeterminism(name, n, KindSleep, o)
	}
	done, _, err := r.e.sleepRecord(ctx, r.rs, name, n, d, o, ok)
	if err != nil {
		return r.exit(err)
	}
	if !done {
		return suspended("sleep", name, n)
	}
	return nil
}

func (r *nativeRun) Effect(ctx context.Context, name string, n int, fn func(ctx context.Context, a EffectAttempt) (json.RawMessage, error)) (json.RawMessage, error) {
	o, ok, err := r.lookup(ctx, name, n, KindStep)
	if err != nil {
		return nil, err
	}
	if ok {
		switch o.Status {
		case OccCompleted, OccFailed:
			return terminalReply(o)
		case OccStarted:
		default:
			return nil, r.rs.nondeterminism(name, n, KindStep, o)
		}
	}
	started := Occurrence{Name: name, N: n, Kind: KindStep, Status: OccStarted, Attempts: o.Attempts + 1}
	if err := r.e.record(ctx, r.rs, started); err != nil {
		return nil, r.exit(err)
	}
	out, ferr := fn(ctx, EffectAttempt{Attempt: started.Attempts, Key: idempotencyKey(r.rs, name, n)})
	if ferr != nil {
		var amb *ambiguousError
		if errors.As(ferr, &amb) {
			// The effect's outcome is unknown and may not be repeated blindly: the occurrence stays
			// started and the instance blocks until an operator resolves or retries it.
			return nil, blockedWire(r.rs.block(BlockAmbiguous, fmt.Sprintf("effect %s#%d: %s; resolve, retry or terminate it manually", name, n, amb.detail)))
		}
		if ctx.Err() != nil || errors.Is(ferr, ErrFenced) || errors.Is(ferr, ErrSuspend) || host.IsInfrastructure(ferr) {
			// The outcome is unknown: the occurrence stays started and the
			// next attempt decides.
			return nil, r.exit(ferr)
		}
		failed := started
		failed.Status, failed.Error = OccFailed, marshal(failureOf(ferr))
		if err := r.e.record(ctx, r.rs, failed); err != nil {
			return nil, r.exit(err)
		}
		return nil, ferr
	}
	if len(out) > 0 && !json.Valid(out) {
		return nil, internalErr("workflow.invalidResult", "effect %s#%d returned a result that is not JSON", name, n)
	}
	done := started
	done.Status, done.Result = OccCompleted, nonNull(out)
	if err := r.e.record(ctx, r.rs, done); err != nil {
		return nil, r.exit(err)
	}
	return done.Result, nil
}

// nativeCaller dispatches the ordinary commands of a native run through the
// host, as a root invocation with a pre-allocated ID.
type nativeCaller struct {
	r    *nativeRun
	cmds CommandInvoker
	name string
	n    int
}

func (c nativeCaller) request(command string, input json.RawMessage) host.Request {
	inst := c.r.rs.inst
	return host.Request{
		Contract: command, Input: input, Subject: c.r.rs.subject,
		Trigger:  host.Trigger{Kind: host.TriggerWorkflow, Ref: inst.ID.String()},
		Producer: spi.Producer{InstanceID: inst.ID.String(), Occurrence: c.name + "#" + strconv.Itoa(c.n)},
	}
}

func (c nativeCaller) CallCommand(ctx context.Context, command string, input json.RawMessage, invocationID string) (host.CallResult, error) {
	req := c.request(command, input)
	req.InvocationID, req.StartKeyBase = invocationID, "wf.call:"+invocationID
	res, err := c.cmds.Invoke(ctx, req)
	return callResultOf(res), err
}

func callResultOf(res host.Result) host.CallResult {
	return host.CallResult{
		InvocationID: res.InvocationID, Binding: res.Binding, ContractHash: res.ContractHash, Output: res.Output, Outcome: res.Outcome,
		CommittedTxCount: res.CommittedTxCount, PartiallyCommitted: res.PartiallyCommitted,
	}
}

func (r *nativeRun) Call(ctx context.Context, name string, n int, command string, input json.RawMessage) (json.RawMessage, error) {
	o, ok, err := r.lookup(ctx, name, n, KindCall)
	if err != nil {
		return nil, err
	}
	if ok {
		switch o.Status {
		case OccCompleted, OccFailed:
			return terminalReply(o)
		case OccStarted:
		default:
			return nil, r.rs.nondeterminism(name, n, KindCall, o)
		}
	}
	deps := r.e.deps()
	if deps.commands == nil || deps.catalog == nil {
		return nil, blockedWire(r.rs.block(BlockUnavailable, fmt.Sprintf("command %s cannot be called: this process has no command host", command)))
	}
	rc, found, err := deps.catalog.Resolve(ctx, command)
	if err != nil {
		return nil, r.exit(err)
	}
	if !found || rc.Resource.Kind != manifest.ResCommand {
		return nil, blockedWire(r.rs.block(BlockUnavailable, fmt.Sprintf("command %s is not available in the catalog", command)))
	}
	hash := rc.Resource.ContractHash
	if hash == "" {
		hash = manifest.ContractHashOf(rc.Resource)
	}
	if len(input) == 0 {
		input = json.RawMessage(`null`)
	}
	caller := nativeCaller{r: r, cmds: deps.commands, name: name, n: n}
	if ok {
		out, handled, err := r.e.resolveStarted(ctx, r.rs, o, command)
		if handled {
			return out, r.exit(err)
		}
	} else if rc.Resource.Atomic {
		out, err := r.callAtomic(ctx, caller, name, n, command, input, hash)
		return out, r.exit(err)
	}
	out, err := r.e.callOrdinary(ctx, caller, r.rs, name, n, command, input, "", hash, o, ok)
	return out, r.exit(err)
}

// callAtomic runs an atomic command in a transaction that also holds the
// fenced receipt: both commit or neither does.
func (r *nativeRun) callAtomic(ctx context.Context, c nativeCaller, name string, n int, command string, input json.RawMessage, hash string) (json.RawMessage, error) {
	tx, err := r.e.cfg.DB.Begin(ctx)
	if err != nil {
		return nil, serrors.FromDB(opNative, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	req := c.request(command, input)
	req.Tx, req.StartKeyBase = tx, idempotencyKey(r.rs, name, n)
	res, err := c.cmds.Invoke(ctx, req)
	if err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		if isInfrastructure(ctx, err) {
			return nil, err
		}
		we, isWire := pool.AsWireError(err)
		switch {
		case isWire && (we.Kind == pool.ErrKindDomain || we.Kind == pool.ErrKindValidation):
			failed := Occurrence{Name: name, N: n, Kind: KindCall, Status: OccFailed, Error: marshal(we), Attempts: 1, InvocationID: res.InvocationID, ContractHash: hash, Meta: callMeta(callResultOf(res))}
			if rerr := r.e.record(ctx, r.rs, failed); rerr != nil {
				return nil, rerr
			}
		case isWire && we.Kind == pool.ErrKindDenied:
			return nil, blockedWire(r.rs.block(BlockDenied, fmt.Sprintf("command %s: %s", command, we.Error())))
		case errors.Is(err, host.ErrNoBinding):
			return nil, blockedWire(r.rs.block(BlockUnavailable, fmt.Sprintf("command %s has no active binding", command)))
		case isWire && we.Code == host.CodePackageUnavailable:
			return nil, blockedWire(r.rs.block(BlockUnavailable, we.Message))
		}
		return nil, err
	}
	if err := r.e.fence(ctx, tx, r.rs); err != nil {
		return nil, err
	}
	rec := Occurrence{Name: name, N: n, Kind: KindCall, Status: OccCompleted, Result: nonNull(res.Output), Attempts: 1, InvocationID: res.InvocationID, ContractHash: hash, Meta: callMeta(callResultOf(res))}
	if err := putOccurrence(ctx, tx, r.rs, rec); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, serrors.FromDB(opNative, err)
	}
	r.rs.set(rec)
	if res.AfterCommit != nil {
		res.AfterCommit(ctx)
	}
	return rec.Result, nil
}

func (r *nativeRun) Start(ctx context.Context, name string, n int, workflow string, input json.RawMessage, opts StartOptions) (uuid.UUID, error) {
	o, ok, err := r.lookup(ctx, name, n, KindStart)
	if err != nil {
		return uuid.Nil, err
	}
	if ok {
		if o.Status != OccCompleted {
			return uuid.Nil, r.rs.nondeterminism(name, n, KindStart, o)
		}
		return startedInstance(o)
	}
	if len(input) == 0 {
		input = json.RawMessage(`null`)
	}
	plan, err := r.e.prepareStart(ctx, workflow, input)
	if err != nil {
		var we *pool.WireError
		if errors.As(err, &we) && we.Kind == pool.ErrKindInternal && (we.Code == "workflow.noImplementation" || we.Code == "workflow.unknownWorkflow") {
			return uuid.Nil, blockedWire(r.rs.block(BlockUnavailable, we.Message))
		}
		return uuid.Nil, r.exit(err)
	}
	inst := r.rs.inst
	subject := spi.Subject{TenantID: inst.TenantID, UserID: inst.SubjectUserID, Kind: inst.SubjectKind}
	if opts.Subject != nil {
		if opts.Subject.TenantID != inst.TenantID {
			return uuid.Nil, &pool.WireError{Kind: pool.ErrKindValidation, Code: "workflow.foreignSubject", Message: "a child instance acts in the tenant of its parent"}
		}
		subject = *opts.Subject
		if err := r.e.authorizeStart(ctx, subject, workflow, plan.permission); err != nil {
			var we *pool.WireError
			if errors.As(err, &we) && we.Kind == pool.ErrKindDenied {
				return uuid.Nil, blockedWire(r.rs.block(BlockDenied, we.Message))
			}
			return uuid.Nil, r.exit(err)
		}
	} else if !r.rs.subject.Has(plan.permission) {
		return uuid.Nil, blockedWire(r.rs.block(BlockDenied, fmt.Sprintf("permission %s is required to start %s", plan.permission, workflow)))
	}
	var child uuid.UUID
	err = r.e.inFence(ctx, r.rs, func(tx pgx.Tx) (func(), error) {
		parent := inst.ID
		id, _, err := r.e.insertInstance(ctx, tx, startSpec{
			workflow: workflow, input: input, impl: plan.impl, subject: subject,
			key: idempotencyKey(r.rs, name, n), parent: &parent,
		})
		if err != nil {
			return nil, err
		}
		child = id
		rec := Occurrence{Name: name, N: n, Kind: KindStart, Status: OccCompleted, Result: marshal(map[string]string{"instance": id.String()}), Attempts: 1}
		if err := putOccurrence(ctx, tx, r.rs, rec); err != nil {
			return nil, err
		}
		return func() { r.rs.set(rec) }, nil
	})
	if err != nil {
		return uuid.Nil, r.exit(err)
	}
	return child, nil
}

func startedInstance(o Occurrence) (uuid.UUID, error) {
	var res struct {
		Instance string `json:"instance"`
	}
	if err := json.Unmarshal(o.Result, &res); err != nil {
		return uuid.Nil, internalErr("workflow.corruptJournal", "occurrence %s#%d has an unreadable result", o.Name, o.N)
	}
	id, err := uuid.Parse(res.Instance)
	if err != nil {
		return uuid.Nil, internalErr("workflow.corruptJournal", "occurrence %s#%d has an unreadable result", o.Name, o.N)
	}
	return id, nil
}

func (r *nativeRun) Cancel(ctx context.Context, name string, n int) error {
	if err := r.enter(); err != nil {
		return err
	}
	o, ok, err := r.e.occ(ctx, r.rs, name, n)
	if err != nil {
		return r.exit(err)
	}
	if !ok || o.Status != OccWaiting {
		return nil
	}
	err = r.e.inFence(ctx, r.rs, func(tx pgx.Tx) (func(), error) {
		if o.Kind == KindTask {
			// A task that closed (decided, expired) before the cancel keeps its
			// outcome: the occurrence stays waiting and the next Task returns
			// it, so the workflow never reports another outcome than the task.
			var status string
			err := tx.QueryRow(ctx, `
				SELECT status FROM extruntime.human_tasks
				WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 FOR UPDATE`,
				r.rs.inst.ID, r.rs.inst.TenantID, name, n).Scan(&status)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, serrors.FromDB(opNative, err)
			}
			if err == nil && status != "open" {
				return func() {}, nil // nothing to apply: the occurrence stays waiting
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE extruntime.workflow_occurrences SET status = 'cancelled', wake_at = NULL, updated_at = now()
			WHERE instance_id = $1 AND tenant_id = $2 AND name = $3 AND n = $4 AND status = 'waiting'`,
			r.rs.inst.ID, r.rs.inst.TenantID, name, n); err != nil {
			return nil, serrors.FromDB(opNative, err)
		}
		if o.Kind == KindTask {
			if err := r.e.cancelTasks(ctx, tx, r.rs.inst.TenantID, r.rs.inst.ID, &occKey{name: name, n: n}); err != nil {
				return nil, &host.InfraError{Err: err}
			}
		}
		cancelled := o
		cancelled.Status, cancelled.WakeAt = OccCancelled, nil
		return func() { r.rs.set(cancelled) }, nil
	})
	return r.exit(err)
}
