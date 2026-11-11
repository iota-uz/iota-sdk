// Package host dispatches extension invocations: it resolves the active
// binding of a contract, validates input and output against the contract,
// owns every database transaction, serves the host calls of the Node worker
// and records each invocation in extruntime.invocations.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opInvoke = serrors.Op("extruntime.host.Invoke")
	opRun    = serrors.Op("extruntime.host.Run")
)

// Runner executes invocations in Node workers; *pool.Pool implements it.
type Runner interface {
	Invoke(ctx context.Context, inv pool.Invocation) (json.RawMessage, error)
}

// SecretResolver serves secrets.get for secrets a package declared.
type SecretResolver interface {
	Secret(ctx context.Context, sc spi.Scope, name string) (string, error)
}

// WorkflowCall is the view the durable engine gets of a workflow invocation
// while serving a wf.* host call. Transactions it opens are registered with
// the invocation, so Node reaches them through tx.* calls with the handle.
type WorkflowCall interface {
	InvocationID() string
	Scope() spi.Scope
	// Producer is what the run was started with (workflow instance).
	Producer() spi.Producer
	// Token is the opaque RunRequest.Token of the run, so the engine finds the
	// state of the run a wf.* call belongs to.
	Token() string
	// OpenTx opens the transaction. startKeyBase is the occurrence identity
	// ("instance:name:n") the host derives the dedupe keys of tx.start calls
	// made through the handle from, so they are stable across replays.
	OpenTx(ctx context.Context, startKeyBase string) (handle string, err error)
	WithTx(ctx context.Context, handle string, fn func(ctx context.Context, tx pgx.Tx) error) error
	EndTx(ctx context.Context, handle string, outcome TxOutcome) error
	// CallInTx runs an operation or atomic command joined to the handle's
	// transaction (wf.transaction(name, callable, input)). Ordinary commands
	// are rejected.
	CallInTx(ctx context.Context, handle, contract string, input json.RawMessage) (json.RawMessage, error)
	// CallCommand dispatches a command as a NEW invocation (wf.call): it
	// selects the currently active binding, records it, and runs on the worker
	// of this run without taking a pool slot. invocationID, when set, is the
	// pre-allocated ID of the invocation record, so the engine can journal
	// `started` before dispatch and find the record after a crash.
	CallCommand(ctx context.Context, contract string, input json.RawMessage, invocationID string) (CallResult, error)
	// CallCommandInTx runs an atomic command joined to the handle's
	// transaction as a new invocation record, returning its identity.
	CallCommandInTx(ctx context.Context, handle, contract string, input json.RawMessage) (CallResult, error)
	// InfraError is the first infrastructure failure a host call of this run
	// hit (database, connection, worker, pool), or nil. The engine aborts and
	// retries a run that has one instead of letting the workflow see it.
	InfraError() error
}

// CallResult is the outcome of a command dispatched for the durable engine.
// It is returned together with the error on failure; InvocationID is empty
// when the call failed before an invocation was recorded.
type CallResult struct {
	InvocationID       string
	Binding            Binding
	ContractHash       string
	Output             json.RawMessage
	Outcome            Outcome
	CommittedTxCount   int
	PartiallyCommitted bool
}

// WorkflowHandler serves wf.* host calls (wf.step.begin, wf.sleep, ...). The
// durable engine is another slice; the host only forwards. Return a
// *pool.WireError of kind suspend to unwind the invocation.
type WorkflowHandler interface {
	CallWorkflow(ctx context.Context, call WorkflowCall, method string, args json.RawMessage) (json.RawMessage, error)
}

// SignalIntent is a signal to a workflow instance, as sent by tx.signal and
// invoke.signal.
type SignalIntent struct {
	// Workflow is the workflow contract ID the instance runs; a mismatch with
	// the instance is rejected.
	Workflow string          `json:"workflow"`
	Instance string          `json:"instance"`
	Name     string          `json:"name"`
	Payload  json.RawMessage `json:"payload"`
	// Key makes a repeated delivery a no-op when set.
	Key      string       `json:"key,omitempty"`
	Producer spi.Producer `json:"producer"`
}

// Signaler writes a signal row in the caller's transaction, so it commits
// atomically with the caller's data. It validates the payload against the
// workflow contract's signal schema, checks the subject's current rights for
// the workflow and rejects unknown, finished or terminated instances with
// typed errors. *workflow.Engine implements it.
type Signaler interface {
	SignalInTx(ctx context.Context, tx pgx.Tx, sc spi.Scope, in SignalIntent) error
}

// Codes of errors raised by the host router itself.
const (
	// CodeIntentInTransaction rejects an ordinary intent (ctx.call, wf.call,
	// ctx.signal) while a Go transaction is open in the same invocation: the
	// intent would run its own transactions while this one holds a connection
	// and locks, and could commit before this one fails.
	CodeIntentInTransaction = "runtime.intentInTransaction"
	// CodeReadOnlyContext rejects a write host call from a guard or query
	// invocation, whatever context object the worker built.
	CodeReadOnlyContext = "runtime.readOnlyContext"
	// CodeNativeQueryUnavailable is returned by tx.query / q.query for a query
	// with no implementation: neither a Go default (native) nor a bound
	// package export.
	CodeNativeQueryUnavailable = "runtime.nativeQueryUnavailable"
	// CodeNotQuery is returned by tx.query / q.query for a non-query contract.
	CodeNotQuery = "runtime.notQuery"
)

// Config wires the host. Outbox, Starter, Notifier, Signals, Workflow and
// Secrets are optional: the matching host calls fail explicitly when absent.
type Config struct {
	Pool     Runner
	DB       *pgxpool.Pool
	Bindings Bindings
	Catalog  spi.Catalog
	Data     spi.DataStore
	// Artifacts resolves the artifact of guards (package@version). Required
	// when the catalog reports active guards.
	Artifacts ArtifactResolver

	Outbox   spi.Outbox
	Starter  spi.Starter
	Notifier spi.Notifier
	Signals  Signaler
	Workflow WorkflowHandler
	Secrets  SecretResolver
	// Natives are the Go default implementations of native contracts
	// (*native.Implementations). A native contract without a package binding
	// runs its Go default; without Natives it has no implementation.
	Natives NativeImplementations

	// DefaultTimeout bounds invocations whose context has no deadline.
	// Default 30s.
	DefaultTimeout time.Duration
	Logger         *slog.Logger
}

// NativeImplementations resolves native contracts to their definition and Go
// default implementation; *native.Implementations implements it.
type NativeImplementations interface {
	Definition(id string) (native.Definition, bool)
	Lookup(id string) (*native.Implementation, bool, error)
}

// Host is the invocation host.
type Host struct {
	cfg Config
	rec recorder
}

func New(cfg Config) (*Host, error) {
	switch {
	case cfg.Pool == nil, cfg.DB == nil, cfg.Bindings == nil, cfg.Catalog == nil, cfg.Data == nil:
		return nil, serrors.New(serrors.Invalid, "extruntime host: Pool, DB, Bindings, Catalog and Data are required")
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Host{cfg: cfg, rec: recorder{db: cfg.DB}}, nil
}

// Trigger says why a root invocation runs.
type Trigger struct {
	Kind TriggerKind
	Ref  string
}

// Request is a root invocation of a command, operation or query contract.
type Request struct {
	Contract string
	Input    json.RawMessage
	Subject  spi.Subject
	Trigger  Trigger
	// Producer links events, starts and notices written by this invocation
	// back to the event, workflow instance or occurrence that caused it.
	Producer spi.Producer
	// StartKeyBase is the dedupe key prefix the host derives tx.start keys
	// from inside subscription deliveries ("subscription:event") and schedule
	// slots. Workflow transactions derive theirs from the occurrence
	// ("instance:name:n", see WorkflowCall.OpenTx). Without a base, tx.start
	// needs an explicit key.
	StartKeyBase string
	// Occurrence is passed to the handler as ctx.occurrence.
	Occurrence json.RawMessage
}

// Result describes a finished invocation. It is returned together with the
// error on failure.
type Result struct {
	InvocationID string
	Output       json.RawMessage
	Binding      Binding
	ContractHash string
	Outcome      Outcome
	// CommittedTxCount is how many transactions this invocation committed.
	CommittedTxCount int
	// PartiallyCommitted is set when the invocation failed or lost its reply
	// after committing at least one transaction. Such invocations are never
	// retried by the host.
	PartiallyCommitted bool
}

// Invoke runs a command, operation or query contract through its active
// binding. A failure after an ordinary command committed a transaction is
// reported with Result.PartiallyCommitted; the host never re-runs it.
func (h *Host) Invoke(ctx context.Context, req Request) (Result, error) {
	root := &rootState{
		txs:      newTxRegistry(h.cfg.DB, h.cfg.Logger),
		producer: req.Producer,
		starts:   &startSource{base: req.StartKeyBase},
	}
	defer root.txs.rollbackAll()
	ctx, cancel := h.withDeadline(ctx)
	defer cancel()

	res, err := h.execute(ctx, root, execSpec{
		contract:   req.Contract,
		input:      req.Input,
		subject:    req.Subject,
		trigger:    req.Trigger,
		occurrence: req.Occurrence,
	})
	if err != nil {
		return res, serrors.Wrap(opInvoke, err)
	}
	return res, nil
}

// Export is an explicit implementation to run outside contract dispatch:
// subscription handlers, workflow runs, guards, templates and endpoints.
type Export struct {
	Kind     pool.Kind
	Resource string
	Binding  Binding
}

// RunRequest runs one export for the events and workflow slices.
type RunRequest struct {
	Export  Export
	Input   json.RawMessage
	Subject spi.Subject
	Trigger Trigger
	// Tx is a caller-owned transaction the handler works in (a subscription
	// delivery with its receipt). The host never commits or rolls it back.
	// Required for subscription and guard-with-tx kinds.
	Tx           pgx.Tx
	Producer     spi.Producer
	StartKeyBase string
	// Occurrence is passed to the handler as ctx.occurrence. For workflow
	// invocations it carries the replay journal ({"journal":[...]}) the worker
	// answers completed primitives from.
	Occurrence json.RawMessage
	// Token is an opaque value the engine reads back through
	// WorkflowCall.Token().
	Token string
}

// Run executes an export directly. The caller has already chosen the
// implementation (a subscription's or workflow instance's pinned version).
func (h *Host) Run(ctx context.Context, req RunRequest) (Result, error) {
	root := &rootState{
		txs:      newTxRegistry(h.cfg.DB, h.cfg.Logger),
		producer: req.Producer,
		starts:   &startSource{base: req.StartKeyBase},
		token:    req.Token,
	}
	defer root.txs.rollbackAll()
	ctx, cancel := h.withDeadline(ctx)
	defer cancel()

	res, err := h.execute(ctx, root, execSpec{
		export:     &req.Export,
		input:      req.Input,
		subject:    req.Subject,
		trigger:    req.Trigger,
		occurrence: req.Occurrence,
		externalTx: req.Tx,
	})
	if err != nil {
		return res, serrors.Wrap(opRun, root.withInfra(err))
	}
	return res, nil
}

func (h *Host) withDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, h.cfg.DefaultTimeout)
}

// rootState is shared by a root invocation and everything it nests.
type rootState struct {
	txs      *txRegistry
	producer spi.Producer
	// starts derives the dedupe keys of tx.start calls that carry no key.
	starts *startSource
	token  string
	// infra is the first infrastructure failure of a host call under this root.
	infra infraState
}

// startSource derives deterministic tx.start keys: base#1, base#2, ... in the
// order the calls arrive. The base identifies where the sequence lives (a
// subscription delivery, a schedule slot, one workflow occurrence, one
// ordinary command called by a workflow), so a replay of that unit yields the
// same keys and a different unit can never collide with it.
type startSource struct {
	base string
	seq  atomic.Int64
}

func (s *startSource) next() (string, bool) {
	if s == nil || s.base == "" {
		return "", false
	}
	return s.base + "#" + strconv.FormatInt(s.seq.Add(1), 10), true
}

type execSpec struct {
	contract   string
	export     *Export
	input      json.RawMessage
	subject    spi.Subject
	trigger    Trigger
	occurrence json.RawMessage
	parent     *execution
	// placement set: run on the worker of placeOn (nil: take a pool slot of its
	// own) instead of the parent's. Guards use it because they run before the
	// invocation they belong to is live.
	placement bool
	placeOn   *execution
	// join runs an operation or atomic command in an existing transaction
	// (tx.call) instead of opening its own.
	join       *txHandle
	externalTx pgx.Tx
	// id pre-allocates the invocation ID (wf.call journals it before dispatch).
	id string
	// startBase, when set, gives the invocation (and what it nests) its own
	// tx.start key sequence instead of the parent's.
	startBase string
}

// execute resolves, authorizes, validates, records and runs one invocation.
func (h *Host) execute(ctx context.Context, root *rootState, spec execSpec) (Result, error) {
	if spec.join != nil {
		// The transaction cannot commit while an invocation joined to it runs,
		// nor after one was cut short: the pool cancels the host calls still
		// running when the caller's handler returns, so an un-awaited tx.call
		// ends here with a cancelled context, after some of its writes.
		spec.join.inflight.Add(1)
		defer func() {
			spec.join.inflight.Add(-1)
			if ctx.Err() != nil {
				spec.join.abandoned.Store(true)
			}
		}()
	}
	var (
		res      spi.ResolvedResource
		contract *manifest.Resource
		binding  Binding
		impl     *native.Implementation
		kind     pool.Kind
		err      error
	)
	if spec.export != nil {
		binding, kind = spec.export.Binding, spec.export.Kind
		binding.Contract = spec.export.Resource
	} else {
		res, err = h.resolveContract(ctx, spec.contract)
		if err != nil {
			return Result{}, err
		}
		contract = &res.Resource
		if kind, err = kindOf(contract); err != nil {
			return Result{}, err
		}
		for _, perm := range h.permissionsOf(res) {
			if !spec.subject.Has(perm) {
				return Result{}, &pool.WireError{Kind: pool.ErrKindDenied, Code: "runtime.permissionDenied", Message: "permission " + perm + " is required for " + contract.ID}
			}
		}
		if err := validateInput(contract, spec.input); err != nil {
			return Result{}, err
		}
		if binding, impl, err = h.resolveBinding(ctx, res); err != nil {
			return Result{}, err
		}
		if kind == pool.KindCommand {
			// Preliminary input guards of an ordinary command run before the
			// invocation is recorded, on a read snapshot; independent of the
			// bound implementation.
			if err := h.runGuards(ctx, guardRun{
				root: root, parent: spec.parent, live: spec.parent, subject: spec.subject, trigger: triggerOf(spec),
				contract: contract, target: contract.ID, phase: manifest.GuardInput, input: spec.input,
			}); err != nil {
				return Result{}, err
			}
		}
	}

	e := &execution{
		h:        h,
		root:     root,
		id:       idOr(spec.id),
		spec:     spec,
		contract: contract,
		binding:  binding,
		native:   impl,
		kind:     kind,
		scope: spi.Scope{
			Subject: spec.subject,
			Caller:  spi.Caller{Package: binding.Package, Version: binding.Version},
		},
	}
	e.rootID = e.id
	e.starts = root.starts
	if spec.parent != nil {
		e.rootID = spec.parent.rootID
		e.starts = spec.parent.starts
	}
	if spec.startBase != "" {
		e.starts = &startSource{base: spec.startBase}
	}
	return e.run(ctx, res)
}

// permissionsOf lists the permissions an invocation of the contract needs:
// the published contract's, and for a native contract the RBAC permission of
// its Go definition (not a catalog resource, so not published). Both apply
// whichever implementation is bound.
func (h *Host) permissionsOf(res spi.ResolvedResource) []string {
	perms := []string{res.Resource.Permission}
	if res.Native && h.cfg.Natives != nil {
		if d, ok := h.cfg.Natives.Definition(res.Resource.ID); ok {
			perms = append(perms, d.Permission())
		}
	}
	return perms
}

// resolveBinding selects the one active implementation: the bound package
// export, or for a native contract without a binding its Go default. A
// binding that cannot be resolved is an error; there is no fallback.
func (h *Host) resolveBinding(ctx context.Context, res spi.ResolvedResource) (Binding, *native.Implementation, error) {
	b, err := h.cfg.Bindings.Resolve(ctx, res.Resource.ID)
	if err == nil || !errors.Is(err, ErrNoBinding) || !res.Native || h.cfg.Natives == nil {
		return b, nil, err
	}
	impl, ok, lerr := h.cfg.Natives.Lookup(res.Resource.ID)
	if lerr != nil {
		return Binding{}, nil, lerr
	}
	if !ok {
		return Binding{}, nil, err
	}
	return Binding{Contract: res.Resource.ID, Native: true}, impl, nil
}

func idOr(id string) string {
	if id != "" {
		return id
	}
	return uuid.NewString()
}

func triggerOf(spec execSpec) Trigger {
	if spec.parent != nil {
		return spec.parent.trigger()
	}
	return spec.trigger
}

func (h *Host) resolveContract(ctx context.Context, id string) (spi.ResolvedResource, error) {
	res, ok, err := h.cfg.Catalog.Resolve(ctx, id)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, serrors.New(serrors.NotFound, "extruntime: unknown contract "+id)
	}
	return res, nil
}

// kindOf maps the public contract to the worker invocation kind. Atomicity is
// part of the contract, so it decides whether the handler gets a Tx.
func kindOf(c *manifest.Resource) (pool.Kind, error) {
	switch {
	case c.Kind == manifest.ResCommand && c.Atomic:
		return pool.KindAtomic, nil
	case c.Kind == manifest.ResCommand:
		return pool.KindCommand, nil
	case c.Kind == manifest.ResOperation:
		return pool.KindOperation, nil
	case c.Kind == manifest.ResQuery:
		return pool.KindQuery, nil
	}
	return "", serrors.New(serrors.Invalid, fmt.Sprintf("extruntime: %s is a %s, not an invocable contract", c.ID, c.Kind))
}

func validateInput(c *manifest.Resource, input json.RawMessage) error {
	if c.Input == nil {
		return nil
	}
	if err := manifest.ValidateValue(*c.Input, input); err != nil {
		return invalidValue("input", err)
	}
	return nil
}

func invalidValue(phase string, err error) *pool.WireError {
	var pe *manifest.PathError
	we := &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.invalidValue", Message: err.Error()}
	if errors.As(err, &pe) {
		path, pathErr := json.Marshal(pe.Path)
		params, paramsErr := json.Marshal(map[string]string{"message": pe.Message})
		// Without an encodable path the error keeps its message and no
		// per-field issue.
		if pathErr == nil && paramsErr == nil {
			we.Issues = []pool.Issue{{Code: "runtime.invalidValue", Phase: phase, Path: path, Params: params}}
		}
	}
	if phase == "output" {
		we.Kind = pool.ErrKindInternal
		we.Code = "runtime.invalidOutput"
		we.Issues = nil
	}
	return we
}
