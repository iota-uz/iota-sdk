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
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

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
	kind     pool.Kind
	scope    spi.Scope

	mu        sync.Mutex
	ctxTx     *txHandle // open ctx.transaction of an ordinary invocation
	snapshot  *txHandle
	committed int
}

func (e *execution) InvocationID() string { return e.id }
func (e *execution) Scope() spi.Scope     { return e.scope }

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

func (e *execution) run(ctx context.Context, res spi.ResolvedResource) (Result, error) {
	out := Result{InvocationID: e.id, Binding: e.binding}
	parentID := ""
	if e.spec.parent != nil {
		parentID = e.spec.parent.id
	}
	hash := res.Resource.ContractHash
	if hash == "" && e.contract != nil {
		hash = manifest.ContractHashOf(*e.contract)
	}
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
	b, _ := json.Marshal(we)
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
	var ownTx *txHandle
	switch {
	case e.spec.externalTx != nil:
		invCtx.Tx = e.root.txs.adopt(e.spec.externalTx).id
	case e.kind == pool.KindAtomic || e.kind == pool.KindOperation:
		if e.spec.join != nil {
			invCtx.Tx = e.spec.join.id
		} else {
			h, err := e.root.txs.begin(ctx, e.id, pgx.TxOptions{})
			if err != nil {
				return nil, err
			}
			ownTx, invCtx.Tx = h, h.id
		}
	case e.kind == pool.KindSubscription:
		return nil, fail("runtime.txRequired", "subscription handlers run in a caller-owned transaction")
	}

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
	if e.spec.parent != nil {
		inv.ParentID = e.spec.parent.id
	}
	output, err := e.h.cfg.Pool.Invoke(ctx, inv)

	if err == nil {
		err = e.checkOutput(output)
	} else {
		err = e.closedErrors(err)
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
	if e.contract == nil || e.contract.Output == nil {
		return nil
	}
	if err := manifest.ValidateValue(*e.contract.Output, output); err != nil {
		return invalidValue("output", err)
	}
	return nil
}

// closedErrors enforces that a contract only raises its declared domain
// errors; anything else is a bug of the implementation.
func (e *execution) closedErrors(err error) error {
	we, ok := pool.AsWireError(err)
	if !ok || we.Kind != pool.ErrKindDomain || e.contract == nil {
		return err
	}
	if slices.Contains(e.contract.Errors, we.Code) {
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
	out, err := e.call(ctx, method, args)
	if err != nil {
		return nil, asWire(err)
	}
	return out, nil
}

func (e *execution) call(ctx context.Context, method string, args json.RawMessage) (json.RawMessage, error) {
	switch method {
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
func (e *execution) OpenTx(ctx context.Context) (string, error) {
	h, err := e.root.txs.begin(ctx, e.id, pgx.TxOptions{})
	if err != nil {
		return "", err
	}
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
	return e.callJoined(ctx, h, contract, input)
}

func (e *execution) EndTx(ctx context.Context, handle string, outcome TxOutcome) error {
	h, err := e.root.txs.lookup(handle)
	if err != nil {
		return err
	}
	if err := e.root.txs.end(ctx, h, outcome); err != nil {
		return err
	}
	if outcome == TxCommit {
		e.countCommitted()
	}
	return nil
}

// txBegin opens the transaction of an ordinary invocation's ctx.transaction.
func (e *execution) txBegin(ctx context.Context) (json.RawMessage, error) {
	if e.kind != pool.KindCommand && e.kind != pool.KindEndpoint {
		return nil, fail("runtime.txNotAllowed", "%s invocations do not open transactions; they run in the transaction the host gave them", e.kind)
	}
	e.mu.Lock()
	if e.ctxTx != nil {
		e.mu.Unlock()
		return nil, fail("runtime.nestedTransaction", "ctx.transaction cannot be nested")
	}
	e.mu.Unlock()
	h, err := e.root.txs.begin(ctx, e.id, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.ctxTx = h
	e.mu.Unlock()
	return json.Marshal(map[string]string{"tx": h.id})
}

func (e *execution) txEnd(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Tx      string
		Outcome TxOutcome
	}
	if err := decode(args, &a); err != nil {
		return nil, err
	}
	e.mu.Lock()
	h := e.ctxTx
	e.mu.Unlock()
	if h == nil || h.id != a.Tx {
		return nil, fail("runtime.txUnknown", "transaction handle is not the open ctx.transaction of this invocation")
	}
	if a.Outcome != TxCommit && a.Outcome != TxRollback {
		return nil, fail(pool.CodeHostCall, "unknown tx.end outcome %q", a.Outcome)
	}
	err := e.root.txs.end(ctx, h, a.Outcome)
	e.mu.Lock()
	e.ctxTx = nil
	e.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if a.Outcome == TxCommit {
		e.countCommitted()
	}
	return json.RawMessage(`null`), nil
}

func (e *execution) invokeNested(ctx context.Context, contract string, input json.RawMessage, join *txHandle) (json.RawMessage, error) {
	res, err := e.h.execute(ctx, e.root, execSpec{
		contract:   contract,
		input:      input,
		subject:    e.spec.subject,
		trigger:    e.trigger(),
		occurrence: nil,
		parent:     e,
		join:       join,
	})
	if err != nil {
		return nil, err
	}
	return res.Output, nil
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
	joinable := res.Resource.Kind == manifest.ResOperation || (res.Resource.Kind == manifest.ResCommand && res.Resource.Atomic)
	if !joinable {
		return nil, fail("runtime.ordinaryCommandInTx", "tx.call accepts only operations and atomic commands; %s is not", contract)
	}
	return e.invokeNested(ctx, contract, input, h)
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
	res, err := e.h.resolveContract(ctx, a.Command)
	if err != nil {
		return nil, err
	}
	if res.Resource.Kind != manifest.ResCommand {
		return nil, fail("runtime.notCommand", "ctx.call accepts commands; %s is a %s", a.Command, res.Resource.Kind)
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
	}
	return nil, fail(pool.CodeHostCall, "unknown host call %q", method)
}

func (e *execution) readSnapshot(ctx context.Context) (*txHandle, error) {
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
	case "tx.emit", "tx.notify", "tx.start":
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
	if res.Native || res.Resource.Kind != want {
		return nil, &pool.WireError{Kind: pool.ErrKindDenied, Code: "runtime.notWritable", Message: fmt.Sprintf("%s is not a writable package-owned %s; native data changes through published operations", target, want)}
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
		err := h.do(ctx, func(ctx context.Context, tx pgx.Tx) (err error) {
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
		err = h.do(ctx, func(ctx context.Context, tx pgx.Tx) (err error) {
			id, err = e.h.cfg.Notifier.Enqueue(ctx, tx, e.scope, spi.NoticeIntent{
				Notice: a.Notice, Version: e.binding.Version, Subject: a.Subject, Payload: a.Payload,
				Recipients: recipients, Channels: a.Channels, Producer: e.producer(),
			})
			return err
		})
		return marshalID("noticeId", id, err)
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
			if e.root.startKeyBase == "" {
				return nil, fail("runtime.startKeyRequired", "tx.start needs {key} outside subscriptions and workflows")
			}
			key = e.root.startKeyBase + "#" + strconv.FormatInt(e.root.startSeq.Add(1), 10)
		}
		var id string
		err := h.do(ctx, func(ctx context.Context, tx pgx.Tx) (err error) {
			id, err = e.h.cfg.Starter.Start(ctx, tx, e.scope, spi.StartIntent{Workflow: a.Workflow, Input: a.Input, Key: key, Producer: e.producer()})
			return err
		})
		return marshalID("instance", id, err)
	}
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
