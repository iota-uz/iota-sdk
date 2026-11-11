package native

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// MaxNesting bounds nested calls under one Direct dispatch, like the host's
// limit.
const MaxNesting = 32

// Direct is the dispatcher of an application without the extension runtime:
// it runs the Go defaults in-process with the same input/output validation
// and transaction shape the host gives them (operations and atomic commands
// in one transaction, ordinary commands opening their own, queries on a
// read-only snapshot). There are no package replacements or guards without
// the runtime.
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
	return d.invoke(ctx, contract, input, nil, func(def Definition) error { return nil })
}

func (d *Direct) invoke(ctx context.Context, contract string, input json.RawMessage, join *directTxState, accept func(Definition) error) (json.RawMessage, error) {
	const op = serrors.Op("extruntime.native.Direct.Dispatch")
	depth, _ := ctx.Value(depthKey{}).(int)
	if depth >= MaxNesting {
		return nil, pool.Internal("runtime.nestingTooDeep", fmt.Sprintf("more than %d nested invocations (recursive call of %s?)", MaxNesting, contract))
	}
	ctx = context.WithValue(ctx, depthKey{}, depth+1)
	def, ok := d.impls.Definition(contract)
	if !ok {
		return nil, serrors.New(serrors.NotFound, "extruntime: unknown native contract "+contract).WithOp(op)
	}
	if err := accept(def); err != nil {
		return nil, err
	}
	if err := d.Authorize(ctx, def.Permission()); err != nil {
		return nil, err
	}
	res := def.Resource()
	if len(input) == 0 {
		input = json.RawMessage(`null`)
	}
	if res.Input != nil {
		if err := manifest.ValidateValue(*res.Input, input); err != nil {
			return nil, invalidInput(err)
		}
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
	if res.Output != nil {
		if verr := manifest.ValidateValue(*res.Output, out); verr != nil {
			return nil, pool.Internal("runtime.invalidOutput", verr.Error())
		}
	}
	return out, nil
}

func invalidInput(err error) error {
	we := &pool.WireError{Kind: pool.ErrKindValidation, Code: "runtime.invalidValue", Message: err.Error()}
	var pe *manifest.PathError
	if errors.As(err, &pe) {
		path, _ := json.Marshal(pe.Path)
		params, _ := json.Marshal(map[string]string{"message": pe.Message})
		we.Issues = []pool.Issue{{Code: "runtime.invalidValue", Phase: "input", Path: path, Params: params}}
	}
	return we
}

// Authorize checks permission with the configured authorizer.
func (d *Direct) Authorize(ctx context.Context, permission string) error {
	if permission == "" || d.authorize == nil || d.authorize(ctx, permission) {
		return nil
	}
	return Denied("runtime.permissionDenied", "permission "+permission+" is required")
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
	return t.d.invoke(ctx, contract, input, t.st, func(def Definition) error {
		if def.Kind() != KindOperation && def.Kind() != KindAtomic {
			return pool.Internal("runtime.ordinaryCommandInTx", "tx.call accepts only operations and atomic commands; "+contract+" is not")
		}
		return nil
	})
}

func (t *directTx) Query(ctx context.Context, query string, input json.RawMessage) (json.RawMessage, error) {
	return t.d.invoke(ctx, query, input, t.st, acceptQuery(query))
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

func acceptQuery(id string) func(Definition) error {
	return func(def Definition) error {
		if def.Kind() != KindQuery {
			return pool.Internal("runtime.notQuery", "query accepts queries; "+id+" is a "+string(def.Kind()))
		}
		return nil
	}
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
		return pool.Internal("runtime.nestedTransaction", "ctx.transaction cannot be nested")
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
		return nil, pool.Internal("runtime.intentInTransaction", "ctx.call is not allowed while a transaction is open")
	}
	return c.d.invoke(ctx, command, input, nil, func(def Definition) error {
		if def.Kind() != KindCommand && def.Kind() != KindAtomic {
			return pool.Internal("runtime.notCommand", "ctx.call accepts commands; "+command+" is a "+string(def.Kind()))
		}
		return nil
	})
}

func (c *directCommand) Query(ctx context.Context, query string, input json.RawMessage) (json.RawMessage, error) {
	return c.d.invoke(ctx, query, input, nil, acceptQuery(query))
}

func (c *directCommand) Authorize(permission string) error {
	return c.d.Authorize(c.ctx, permission)
}
