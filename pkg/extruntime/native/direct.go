package native

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Direct is the dispatcher of an application without the extension runtime:
// it runs the Go defaults in-process. Input/output validation, permissions,
// the call shapes of tx.call / ctx.call / query and the nesting limit are the
// host's own rules (CheckInput, CheckOutput, CheckPermissions, CheckCall,
// MaxNesting), and the transaction shape is the host's (operations and atomic
// commands in one transaction, ordinary commands opening their own, queries
// on a read-only snapshot). What Direct lacks by design is everything that
// needs the runtime: package replacements, guards of packages, invocation
// records.
type Direct struct {
	impls     *Implementations
	authorize func(ctx context.Context, permission string) bool
	log       *slog.Logger
}

// DirectOption configures Direct.
type DirectOption func(*Direct)

// WithAuthorizer makes Spec.Permission and Authorize calls check the
// request's rights. Without it they pass: the application authorizes its own
// entry points, as it did before the dispatcher.
func WithAuthorizer(fn func(ctx context.Context, permission string) bool) DirectOption {
	return func(d *Direct) { d.authorize = fn }
}

// WithLogger sets the logger of after-commit callback failures.
func WithLogger(l *slog.Logger) DirectOption { return func(d *Direct) { d.log = l } }

// NewDirect returns the in-process dispatcher over impls.
func NewDirect(impls *Implementations, opts ...DirectOption) *Direct {
	d := &Direct{impls: impls, log: slog.Default()}
	for _, o := range opts {
		o(d)
	}
	return d
}

var _ Dispatcher = (*Direct)(nil)

type depthKey struct{}

// Dispatch implements Dispatcher.
func (d *Direct) Dispatch(ctx context.Context, contract string, input json.RawMessage) (json.RawMessage, error) {
	return d.invoke(ctx, contract, input, nil, "")
}

// invoke runs contract; via is the call shape of a nested invocation ("" for
// a root dispatch).
func (d *Direct) invoke(ctx context.Context, contract string, input json.RawMessage, join *directTxState, via Via) (json.RawMessage, error) {
	const op = serrors.Op("extruntime.native.Direct.Dispatch")
	depth, _ := ctx.Value(depthKey{}).(int)
	if depth >= MaxNesting {
		return nil, NestingTooDeep(contract)
	}
	ctx = context.WithValue(ctx, depthKey{}, depth+1)
	def, ok := d.impls.Definition(contract)
	if !ok {
		return nil, serrors.New(serrors.NotFound, "extruntime: unknown native contract "+contract).WithOp(op)
	}
	res := def.Resource()
	if via != "" {
		if err := CheckCall(via, res); err != nil {
			return nil, err
		}
	}
	if err := CheckPermissions(func(p string) bool { return d.allowed(ctx, p) }, res, def); err != nil {
		return nil, err
	}
	if len(input) == 0 {
		input = json.RawMessage(`null`)
	}
	if err := CheckInput(res, input); err != nil {
		return nil, err
	}
	impl, ok, err := d.impls.Lookup(contract)
	if err != nil {
		return nil, serrors.Wrap(op, err)
	}
	if !ok {
		return nil, serrors.Wrap(op, errors.Join(ErrNoImplementation, errors.New(contract)))
	}
	var out json.RawMessage
	switch def.Kind() {
	case KindOperation, KindAtomic:
		out, err = d.inTx(ctx, join, func(ctx context.Context, tx *directTx) (json.RawMessage, error) {
			return impl.RunTx(ctx, tx, input)
		})
	case KindCommand:
		out, err = impl.RunCommand(ctx, &directCommand{d: d, ctx: ctx}, input)
	case KindQuery:
		out, err = d.inSnapshot(ctx, join, func(ctx context.Context, tx *directTx) (json.RawMessage, error) {
			return impl.RunQuery(ctx, tx, input)
		})
	}
	if err != nil {
		return nil, err
	}
	if err := CheckOutput(res, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *Direct) allowed(ctx context.Context, permission string) bool {
	return permission == "" || d.authorize == nil || d.authorize(ctx, permission)
}

// Authorize checks permission with the configured authorizer.
func (d *Direct) Authorize(ctx context.Context, permission string) error {
	if d.allowed(ctx, permission) {
		return nil
	}
	return Denied(CodePermissionDenied, "permission "+permission+" is required")
}

// directTxState is one transaction of a Direct dispatch.
type directTxState struct {
	tx pgx.Tx
	// external: the transaction came with the caller's context; its owner
	// commits it and after-commit callbacks cannot be honored.
	external bool
	mu       sync.Mutex
	hooks    []func(context.Context)
}

func (d *Direct) begin(ctx context.Context, opts pgx.TxOptions) (*directTxState, error) {
	if tx, ok := ctx.Value(constants.TxKey).(pgx.Tx); ok && tx != nil {
		return &directTxState{tx: tx, external: true}, nil
	}
	db, err := composables.UsePool(ctx)
	if err != nil {
		return nil, serrors.Wrap("extruntime.native.Direct.begin", err)
	}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return nil, serrors.FromDB("extruntime.native.Direct.begin", err)
	}
	return &directTxState{tx: tx}, nil
}

func (d *Direct) inTx(ctx context.Context, join *directTxState, fn func(ctx context.Context, tx *directTx) (json.RawMessage, error)) (json.RawMessage, error) {
	if join != nil {
		return fn(composables.WithTx(ctx, join.tx), &directTx{d: d, st: join, ctx: ctx})
	}
	st, err := d.begin(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	out, err := fn(composables.WithTx(ctx, st.tx), &directTx{d: d, st: st, ctx: ctx})
	if err := d.end(ctx, st, err == nil); err != nil {
		return nil, err
	}
	return out, err
}

func (d *Direct) inSnapshot(ctx context.Context, join *directTxState, fn func(ctx context.Context, tx *directTx) (json.RawMessage, error)) (json.RawMessage, error) {
	if join != nil {
		return fn(composables.WithTx(ctx, join.tx), &directTx{d: d, st: join, ctx: ctx})
	}
	st, err := d.begin(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	out, err := fn(composables.WithTx(ctx, st.tx), &directTx{d: d, st: st, ctx: ctx})
	if !st.external {
		_ = st.tx.Rollback(context.WithoutCancel(ctx))
	}
	return out, err
}

// end commits (ok) or rolls back a transaction the dispatch opened, then runs
// the after-commit callbacks of a commit.
func (d *Direct) end(ctx context.Context, st *directTxState, ok bool) error {
	if st.external {
		return nil
	}
	if !ok {
		_ = st.tx.Rollback(context.WithoutCancel(ctx))
		return nil
	}
	if err := st.tx.Commit(ctx); err != nil {
		_ = st.tx.Rollback(context.WithoutCancel(ctx))
		return serrors.FromDB("extruntime.native.Direct.commit", err)
	}
	st.mu.Lock()
	hooks := st.hooks
	st.hooks = nil
	st.mu.Unlock()
	RunAfterCommit(context.WithoutCancel(ctx), d.log, hooks)
	return nil
}

// RunAfterCommit runs after-commit callbacks in order; a panicking callback
// is logged and does not stop the others (the transaction is committed).
func RunAfterCommit(ctx context.Context, log *slog.Logger, hooks []func(context.Context)) {
	for _, fn := range hooks {
		func() {
			defer func() {
				if r := recover(); r != nil && log != nil {
					log.Error("extruntime: after-commit callback panicked", "panic", r)
				}
			}()
			fn(ctx)
		}()
	}
}

// directTx implements Tx and QueryContext over a Direct transaction.
type directTx struct {
	d   *Direct
	st  *directTxState
	ctx context.Context
}

func (t *directTx) DB() pgx.Tx { return t.st.tx }

func (t *directTx) Call(ctx context.Context, contract string, input json.RawMessage) (json.RawMessage, error) {
	return t.d.invoke(ctx, contract, input, t.st, ViaTxCall)
}

func (t *directTx) Query(ctx context.Context, query string, input json.RawMessage) (json.RawMessage, error) {
	return t.d.invoke(ctx, query, input, t.st, ViaQuery)
}

func (t *directTx) AfterCommit(fn func(context.Context)) error {
	if t.st.external {
		return pool.Internal("runtime.afterCommitUnavailable", "the transaction belongs to the caller; after-commit callbacks cannot be registered in it")
	}
	t.st.mu.Lock()
	t.st.hooks = append(t.st.hooks, fn)
	t.st.mu.Unlock()
	return nil
}

func (t *directTx) Authorize(permission string) error {
	return t.d.Authorize(t.ctx, permission)
}

// directCommand implements CommandContext.
type directCommand struct {
	d    *Direct
	ctx  context.Context
	mu   sync.Mutex
	open bool
}

func (c *directCommand) Transaction(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error {
	c.mu.Lock()
	if c.open {
		c.mu.Unlock()
		return pool.Internal(CodeNestedTransaction, "ctx.transaction cannot be nested")
	}
	c.open = true
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.open = false; c.mu.Unlock() }()
	st, err := c.d.begin(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	ferr := fn(composables.WithTx(ctx, st.tx), &directTx{d: c.d, st: st, ctx: ctx})
	if err := c.d.end(ctx, st, ferr == nil); err != nil {
		return err
	}
	return ferr
}

func (c *directCommand) Call(ctx context.Context, command string, input json.RawMessage) (json.RawMessage, error) {
	c.mu.Lock()
	open := c.open
	c.mu.Unlock()
	if open {
		return nil, pool.Internal(CodeIntentInTransaction, "ctx.call is not allowed while a transaction is open")
	}
	return c.d.invoke(ctx, command, input, nil, ViaCtxCall)
}

func (c *directCommand) Query(ctx context.Context, query string, input json.RawMessage) (json.RawMessage, error) {
	return c.d.invoke(ctx, query, input, nil, ViaQuery)
}

func (c *directCommand) Authorize(permission string) error {
	return c.d.Authorize(c.ctx, permission)
}
