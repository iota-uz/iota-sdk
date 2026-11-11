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
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
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
	OpenTx(ctx context.Context) (handle string, err error)
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

// Config wires the host. Outbox, Starter, Notifier, Workflow and Secrets are
// optional: the matching host calls fail explicitly when absent.
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
	Workflow WorkflowHandler
	Secrets  SecretResolver

	// DefaultTimeout bounds invocations whose context has no deadline.
	// Default 30s.
	DefaultTimeout time.Duration
	Logger         *slog.Logger
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
	// from inside subscription deliveries ("subscription:event") and workflow
	// transactions ("instance:name:n"). Without it, tx.start needs an
	// explicit key.
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
		txs:          newTxRegistry(h.cfg.DB),
		producer:     req.Producer,
		startKeyBase: req.StartKeyBase,
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
		txs:          newTxRegistry(h.cfg.DB),
		producer:     req.Producer,
		startKeyBase: req.StartKeyBase,
		token:        req.Token,
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
		return res, serrors.Wrap(opRun, err)
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
	txs          *txRegistry
	producer     spi.Producer
	startKeyBase string
	token        string
	startSeq     atomic.Int64
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
}

// execute resolves, authorizes, validates, records and runs one invocation.
func (h *Host) execute(ctx context.Context, root *rootState, spec execSpec) (Result, error) {
	var (
		res      spi.ResolvedResource
		contract *manifest.Resource
		binding  Binding
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
		if !spec.subject.Has(contract.Permission) {
			return Result{}, &pool.WireError{Kind: pool.ErrKindDenied, Code: "runtime.permissionDenied", Message: "permission " + contract.Permission + " is required for " + contract.ID}
		}
		if err := validateInput(contract, spec.input); err != nil {
			return Result{}, err
		}
		if binding, err = h.cfg.Bindings.Resolve(ctx, spec.contract); err != nil {
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
		kind:     kind,
		scope: spi.Scope{
			Subject: spec.subject,
			Caller:  spi.Caller{Package: binding.Package, Version: binding.Version},
		},
	}
	e.rootID = e.id
	if spec.parent != nil {
		e.rootID = spec.parent.rootID
	}
	return e.run(ctx, res)
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
		path, _ := json.Marshal(pe.Path)
		params, _ := json.Marshal(map[string]string{"message": pe.Message})
		we.Issues = []pool.Issue{{Code: "runtime.invalidValue", Phase: phase, Path: path, Params: params}}
	}
	if phase == "output" {
		we.Kind = pool.ErrKindInternal
		we.Code = "runtime.invalidOutput"
		we.Issues = nil
	}
	return we
}
