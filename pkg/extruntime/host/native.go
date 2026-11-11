package host

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// workerOf is the invocation whose worker a nested package invocation runs
// on: the nearest of x and its ancestors that runs package code. Go defaults
// run on no worker; under a Go root the nested invocation takes a pool slot.
func workerOf(x *execution) string {
	for p := x; p != nil; p = p.spec.parent {
		if p.native == nil {
			return p.id
		}
	}
	return ""
}

// runNative runs the Go default of a native contract in the transaction shape
// its kind demands: operations and atomic commands in txh (the caller's
// transaction for tx.call, or the one dispatch opened), queries in the
// caller's snapshot or a read-only one, ordinary commands with no
// transaction (CommandContext.Transaction opens one).
func (e *execution) runNative(ctx context.Context, txh *txHandle) (json.RawMessage, error) {
	ctx = context.WithValue(ctx, execKey{}, e)
	in := e.spec.input
	if len(in) == 0 {
		in = json.RawMessage(`null`)
	}
	switch e.kind {
	case pool.KindAtomic, pool.KindOperation:
		if txh == nil {
			return nil, fail("runtime.txRequired", "%s runs in a transaction", e.resource())
		}
		var out json.RawMessage
		err := txh.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
			t := &hostTx{e: e, h: txh}
			defer t.done.Store(true)
			var err error
			out, err = e.native.RunTx(e.nativeCtx(ctx, tx), t, in)
			return err
		})
		return out, err
	case pool.KindQuery:
		h, err := e.readSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		var out json.RawMessage
		err = h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
			q := &hostTx{e: e, h: h}
			defer q.done.Store(true)
			var err error
			out, err = e.native.RunQuery(e.nativeCtx(ctx, tx), q, in)
			return err
		})
		return out, err
	case pool.KindCommand:
		c := &hostCommand{e: e}
		defer c.done.Store(true)
		return e.native.RunCommand(e.nativeCtx(ctx, nil), c, in)
	case pool.KindSubscription, pool.KindWorkflow, pool.KindGuard, pool.KindTemplate,
		pool.KindEndpoint, pool.KindSchedule:
		// not native contract shapes
	}
	return nil, fail("runtime.nativeKind", "%s invocations have no Go implementation", e.kind)
}

// nativeCtx is the context a Go handler gets: the caller's values (request
// user, session, localizer when the root came from HTTP), the transaction for
// repositories (composables.UseTx; none for an ordinary command), the
// subject's tenant and the database pool.
func (e *execution) nativeCtx(ctx context.Context, tx pgx.Tx) context.Context {
	if tx != nil {
		ctx = composables.WithTx(ctx, tx)
	} else if ctx.Value(constants.TxKey) != nil {
		ctx = context.WithValue(ctx, constants.TxKey, nil)
	}
	if tid, err := composables.UseTenantID(ctx); err != nil || tid != e.scope.Subject.TenantID {
		ctx = composables.WithTenantID(ctx, e.scope.Subject.TenantID)
	}
	if ctx.Value(constants.PoolKey) == nil {
		ctx = composables.WithPool(ctx, e.h.cfg.DB)
	}
	return ctx
}

func (e *execution) authorize(permission string) error {
	if permission == "" || e.scope.Subject.Has(permission) {
		return nil
	}
	return &pool.WireError{Kind: pool.ErrKindDenied, Code: native.CodePermissionDenied, Message: "permission " + permission + " is required"}
}

func errHandlerDone() error {
	return pool.Internal("runtime.txClosed", "the native context was used after its handler returned")
}

// hostTx implements native.Tx and native.QueryContext over a host handle.
type hostTx struct {
	e    *execution
	h    *txHandle
	done atomic.Bool
}

var (
	_ native.Tx           = (*hostTx)(nil)
	_ native.QueryContext = (*hostTx)(nil)
)

func (t *hostTx) DB() pgx.Tx { return t.h.tx }

// held marks ctx as running inside the handle's lock, which the handler's
// goroutine holds for its whole run, whatever context it passes on.
func (t *hostTx) held(ctx context.Context) context.Context {
	return context.WithValue(ctx, heldKey{}, t.h)
}

func (t *hostTx) Call(ctx context.Context, contract string, input json.RawMessage) (json.RawMessage, error) {
	if t.done.Load() {
		return nil, errHandlerDone()
	}
	if t.e.kind == pool.KindQuery || t.e.kind == pool.KindGuard {
		return nil, fail(CodeReadOnlyContext, "%s invocations are read-only; tx.call is not allowed", t.e.kind)
	}
	return t.e.callJoined(t.held(ctx), t.h, contract, input)
}

func (t *hostTx) Query(ctx context.Context, query string, input json.RawMessage) (json.RawMessage, error) {
	if t.done.Load() {
		return nil, errHandlerDone()
	}
	return t.e.queryVia(t.held(ctx), t.h, query, input)
}

func (t *hostTx) AfterCommit(fn func(context.Context)) error {
	if t.done.Load() {
		return errHandlerDone()
	}
	if t.e.kind == pool.KindQuery {
		return fail(CodeReadOnlyContext, "query invocations cannot register after-commit callbacks")
	}
	return t.h.afterCommit(fn)
}

func (t *hostTx) Authorize(permission string) error { return t.e.authorize(permission) }

func (e *execution) queryVia(ctx context.Context, h *txHandle, query string, input json.RawMessage) (json.RawMessage, error) {
	if len(input) == 0 {
		input = json.RawMessage(`null`)
	}
	res, err := e.h.resolveContract(ctx, query)
	if err != nil {
		return nil, err
	}
	if err := native.CheckCall(native.ViaQuery, res.Resource); err != nil {
		return nil, err
	}
	return e.runQuery(ctx, h, query, input)
}

// hostCommand implements native.CommandContext for an ordinary command.
type hostCommand struct {
	e    *execution
	done atomic.Bool
}

var _ native.CommandContext = (*hostCommand)(nil)

func (c *hostCommand) Transaction(ctx context.Context, fn func(ctx context.Context, tx native.Tx) error) error {
	if c.done.Load() {
		return errHandlerDone()
	}
	h, err := c.e.openCtxTx(ctx)
	if err != nil {
		return err
	}
	ferr := h.do(ctx, func(ctx context.Context, tx pgx.Tx) error {
		t := &hostTx{e: c.e, h: h}
		defer t.done.Store(true)
		return fn(c.e.nativeCtx(ctx, tx), t)
	})
	outcome := TxCommit
	if ferr != nil {
		outcome = TxRollback
	}
	if err := c.e.endCtxTx(ctx, h.id, outcome); err != nil && ferr == nil {
		return err
	}
	return ferr
}

func (c *hostCommand) Call(ctx context.Context, command string, input json.RawMessage) (json.RawMessage, error) {
	if c.done.Load() {
		return nil, errHandlerDone()
	}
	if err := c.e.rejectIntentInTx("ctx.call"); err != nil {
		return nil, err
	}
	res, err := c.e.h.resolveContract(ctx, command)
	if err != nil {
		return nil, err
	}
	if err := native.CheckCall(native.ViaCtxCall, res.Resource); err != nil {
		return nil, err
	}
	return c.e.invokeNested(ctx, command, input, nil)
}

func (c *hostCommand) Query(ctx context.Context, query string, input json.RawMessage) (json.RawMessage, error) {
	if c.done.Load() {
		return nil, errHandlerDone()
	}
	h, err := c.e.readSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	return c.e.queryVia(ctx, h, query, input)
}

func (c *hostCommand) Authorize(permission string) error { return c.e.authorize(permission) }

// Domain error codes the platform raises on behalf of any contract (the data
// layer); they pass the closed-errors check of every contract.
const (
	CodeDataNotFound        = "data.notFound"
	CodeDataVersionConflict = "data.versionConflict"
	CodeDataUnique          = "data.unique"
	CodeDataReference       = "data.reference"
	CodeDataNativeWrite     = "data.nativeWrite"
	CodeDataForeignWrite    = "data.foreignWrite"
	CodeDataDenied          = "data.denied"
	CodeDataFieldHidden     = "data.fieldNotAccessible"
	CodeDataUnknownField    = "data.unknownField"
	CodeDataInvalidQuery    = "data.invalidQuery"
	CodeDataInvalidValue    = "data.invalidValue"
)

func isPlatformDomainCode(code string) bool {
	return code == CodeDataNotFound || code == CodeDataVersionConflict
}

// dataError maps a data-layer failure to the stable author-facing error of
// its kind. Values never travel: a hidden field is reported without its
// name's value, a unique violation names the field, not the value.
func dataError(err error) *pool.WireError {
	switch {
	case errors.Is(err, data.ErrUniqueViolation):
		return writeIssue(err, CodeDataUnique, "a unique field already holds this value")
	case errors.Is(err, data.ErrReferenceViolation):
		return writeIssue(err, CodeDataReference, "a referenced record is missing or the record is still referenced")
	case errors.Is(err, data.ErrNativeWrite):
		return &pool.WireError{Kind: pool.ErrKindDenied, Code: CodeDataNativeWrite, Message: "native entities are read-only through generic data calls; use a published operation"}
	case errors.Is(err, data.ErrForeignWrite):
		return &pool.WireError{Kind: pool.ErrKindDenied, Code: CodeDataForeignWrite, Message: "entities of another package are read-only through generic data calls; use its published operations"}
	case errors.Is(err, data.ErrFieldNotAccessible):
		return &pool.WireError{Kind: pool.ErrKindDenied, Code: CodeDataFieldHidden, Message: "a field is not accessible"}
	case errors.Is(err, data.ErrDenied):
		return &pool.WireError{Kind: pool.ErrKindDenied, Code: CodeDataDenied, Message: "permission denied"}
	case errors.Is(err, data.ErrNotFound):
		return &pool.WireError{Kind: pool.ErrKindDomain, Code: CodeDataNotFound, Message: "record not found"}
	case errors.Is(err, data.ErrVersionConflict):
		return &pool.WireError{Kind: pool.ErrKindDomain, Code: CodeDataVersionConflict, Message: "record was changed concurrently"}
	case errors.Is(err, data.ErrInvalidValue):
		return writeIssue(err, CodeDataInvalidValue, "a value does not match its field type")
	case errors.Is(err, data.ErrUnknownField):
		// A query or write naming a field the entity does not have: a bug of
		// the calling code, reported with the stable code the test host uses.
		return &pool.WireError{Kind: pool.ErrKindInternal, Code: CodeDataUnknownField, Message: err.Error()}
	case errors.Is(err, data.ErrInvalidQuery):
		return &pool.WireError{Kind: pool.ErrKindInternal, Code: CodeDataInvalidQuery, Message: err.Error()}
	}
	return nil
}

func writeIssue(err error, code, message string) *pool.WireError {
	is := pool.Issue{Code: code, Phase: "write"}
	if fs := serrors.FieldsOf(err); len(fs) > 0 && fs[0].Field != "" {
		if path, perr := json.Marshal([]string{fs[0].Field}); perr == nil {
			is.Path = path
		}
	}
	return &pool.WireError{Kind: pool.ErrKindValidation, Code: code, Message: message, Issues: []pool.Issue{is}}
}
