package host

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/pool"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	opTxBegin = serrors.Op("extruntime.host.tx.begin")
	opTxEnd   = serrors.Op("extruntime.host.tx.end")
)

// TxOutcome is how a Go-owned transaction ends.
type TxOutcome string

const (
	TxCommit   TxOutcome = "commit"
	TxRollback TxOutcome = "rollback"
)

type heldKey struct{}

// txHandle is one Go-owned transaction. Node only ever sees its ID.
type txHandle struct {
	id string
	tx pgx.Tx
	// owner is the invocation whose committed_tx_count is incremented in
	// this transaction; empty for transactions owned by the caller.
	owner    string
	external bool
	// starts, when set, is the key sequence of tx.start calls made through
	// this handle (a workflow occurrence's transaction).
	starts *startSource
	// inflight counts invocations joined to this handle that have not
	// returned. The transaction cannot commit while any is running: its
	// later writes would hit a closed handle after the earlier ones committed.
	inflight atomic.Int32
	// abandoned is set when a joined invocation ended because its caller
	// finished or was cancelled: part of its writes are in the transaction.
	abandoned atomic.Bool
	hookMu    sync.Mutex

	mu     sync.Mutex
	closed bool
	// hooks run after a commit of the transaction (native Tx.AfterCommit).
	hooks []func(context.Context)
}

// afterCommit registers fn to run once the transaction commits. A
// caller-owned transaction is committed outside the host, so it refuses.
func (h *txHandle) afterCommit(fn func(context.Context)) error {
	if h.external {
		return pool.Internal("runtime.afterCommitUnavailable", "the transaction belongs to the caller of the runtime (a subscription delivery or schedule slot); after-commit callbacks cannot be registered in it")
	}
	// Called from inside a data call on the handle (the handle lock may be
	// held by this goroutine), so the hooks have their own lock.
	h.hookMu.Lock()
	h.hooks = append(h.hooks, fn)
	h.hookMu.Unlock()
	return nil
}

// do serializes calls on the handle. A pgx transaction is single-connection;
// concurrent host calls from Node (Promise.all) queue here instead of failing
// with repo.ErrTxInUse. Calls made from inside a data call of the same handle
// (an entity write rule running in Node during an insert) carry the handle in
// their context and bypass the held lock; the guarded tx still fails fast on
// real concurrency.
func (h *txHandle) do(ctx context.Context, fn func(ctx context.Context, tx pgx.Tx) error) error {
	if held, _ := ctx.Value(heldKey{}).(*txHandle); held == h {
		if h.closed {
			return errTxClosed()
		}
		return fn(ctx, h.tx)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errTxClosed()
	}
	return fn(context.WithValue(ctx, heldKey{}, h), h.tx)
}

func errTxClosed() error {
	return pool.Internal("runtime.txClosed", "transaction handle is closed")
}

// txRegistry holds the open handles of one root invocation. Handles are
// scoped to the root: nested invocations join them, nothing else can see them.
type txRegistry struct {
	db  *pgxpool.Pool
	log *slog.Logger

	mu      sync.Mutex
	handles map[string]*txHandle
}

func newTxRegistry(db *pgxpool.Pool, log *slog.Logger) *txRegistry {
	return &txRegistry{db: db, log: log, handles: map[string]*txHandle{}}
}

func (r *txRegistry) begin(ctx context.Context, owner string, opts pgx.TxOptions) (*txHandle, error) {
	tx, err := r.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, serrors.FromDB(opTxBegin, err)
	}
	h := &txHandle{id: "tx-" + uuid.NewString(), tx: repo.NewGuardedTx(tx), owner: owner}
	r.add(h)
	return h, nil
}

// adopt registers a caller-owned transaction. The host never commits or rolls
// it back.
func (r *txRegistry) adopt(tx pgx.Tx) *txHandle {
	h := &txHandle{id: "tx-" + uuid.NewString(), tx: repo.NewGuardedTx(tx), external: true}
	r.add(h)
	return h
}

func (r *txRegistry) add(h *txHandle) {
	r.mu.Lock()
	r.handles[h.id] = h
	r.mu.Unlock()
}

func (r *txRegistry) lookup(id string) (*txHandle, error) {
	r.mu.Lock()
	h := r.handles[id]
	r.mu.Unlock()
	if h == nil {
		return nil, pool.Internal("runtime.txUnknown", "unknown transaction handle")
	}
	return h, nil
}

// end commits or rolls back the handle. Commit increments the owner's
// committed_tx_count inside the same transaction.
func (r *txRegistry) end(ctx context.Context, h *txHandle, outcome TxOutcome) error {
	if h.external {
		return pool.Internal("runtime.txExternal", "this transaction is owned by the caller and cannot be ended")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errTxClosed()
	}
	h.closed = true
	r.mu.Lock()
	delete(r.handles, h.id)
	r.mu.Unlock()

	if outcome == TxCommit && (h.inflight.Load() > 0 || h.abandoned.Load()) {
		// A tx.call / tx.query that nobody awaited is still running in this
		// transaction, or was cut short when its caller finished. Committing
		// would persist what it wrote so far and lose the rest; roll everything
		// back instead.
		_ = h.tx.Rollback(context.WithoutCancel(ctx))
		return pool.Internal("runtime.nestedInFlight", "a nested invocation joined to this transaction did not finish before its caller (an un-awaited tx.call?); the transaction was rolled back")
	}
	if outcome == TxCommit {
		if err := countCommit(ctx, h.tx, h.owner); err != nil {
			_ = h.tx.Rollback(context.WithoutCancel(ctx))
			return err
		}
		if err := h.tx.Commit(ctx); err != nil {
			_ = h.tx.Rollback(context.WithoutCancel(ctx))
			return serrors.FromDB(opTxEnd, err)
		}
		h.hookMu.Lock()
		hooks := h.hooks
		h.hooks = nil
		h.hookMu.Unlock()
		native.RunAfterCommit(context.WithoutCancel(ctx), r.log, hooks)
		return nil
	}
	if err := h.tx.Rollback(context.WithoutCancel(ctx)); err != nil && err != pgx.ErrTxClosed {
		return serrors.FromDB(opTxEnd, err)
	}
	return nil
}

// rollbackAll rolls back everything still open: called when the root
// invocation ends for any reason (error, deadline, worker loss, suspend).
func (r *txRegistry) rollbackAll() {
	r.mu.Lock()
	open := make([]*txHandle, 0, len(r.handles))
	for _, h := range r.handles {
		open = append(open, h)
	}
	r.handles = map[string]*txHandle{}
	r.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, h := range open {
		if h.external {
			continue
		}
		h.mu.Lock()
		if !h.closed {
			h.closed = true
			_ = h.tx.Rollback(ctx)
		}
		h.mu.Unlock()
	}
}
