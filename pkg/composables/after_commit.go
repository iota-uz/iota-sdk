package composables

import (
	"context"
	"errors"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/jackc/pgx/v5"
	"github.com/sirupsen/logrus"
)

type afterCommitKey struct{}

// CommitHooks collects the callbacks to run once a transaction has committed.
// The owner of the transaction (InTx, or a host that commits its own) creates
// it with WithCommitHooks, and calls Run after a successful commit or Discard
// when the transaction rolls back.
type CommitHooks struct {
	mu  sync.Mutex
	fns []func(context.Context)
}

// WithCommitHooks returns a context whose AfterCommit callbacks are collected
// by the returned CommitHooks instead of running at once.
func WithCommitHooks(ctx context.Context) (context.Context, *CommitHooks) {
	h := &CommitHooks{}
	return context.WithValue(ctx, afterCommitKey{}, h), h
}

// UseCommitHooks returns the commit hooks ctx registers its AfterCommit
// callbacks with, or nil when its transaction has none.
func UseCommitHooks(ctx context.Context) *CommitHooks {
	h, _ := ctx.Value(afterCommitKey{}).(*CommitHooks)
	return h
}

// Attach returns ctx with h as the collector of its AfterCommit callbacks: a
// nested call whose context was derived before the transaction's own context
// joins the same transaction's hooks.
func (h *CommitHooks) Attach(ctx context.Context) context.Context {
	return context.WithValue(ctx, afterCommitKey{}, h)
}

// Run runs the collected callbacks in registration order, once. A panicking
// callback is logged and does not stop the rest: the change is committed, so a
// failing announcement must not turn into a failed call.
func (h *CommitHooks) Run(ctx context.Context) {
	h.mu.Lock()
	fns := h.fns
	h.fns = nil
	h.mu.Unlock()
	ctx = context.WithoutCancel(ctx)
	for _, fn := range fns {
		runHook(ctx, fn)
	}
}

// Discard drops the collected callbacks: the transaction rolled back.
func (h *CommitHooks) Discard() {
	h.mu.Lock()
	h.fns = nil
	h.mu.Unlock()
}

func runHook(ctx context.Context, fn func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			UseLoggerOrDefault(ctx).Errorf("after-commit callback panicked: %v", r)
		}
	}()
	fn(ctx)
}

// AfterCommit registers fn to run after the transaction ctx belongs to has
// committed; a rollback drops it, so a nested service call made inside a
// caller's InTx announces its change only when the caller's transaction commits.
//
// The commit hooks belong to one transaction: WithTx starts a context without
// them, so a callback registered under another transaction (a savepoint opened
// by hand, a transaction a host or a test owns) never reaches the outer commit.
// An owner that wants its transaction's callbacks run attaches hooks with
// WithCommitHooks or CommitHooks.Attach and runs them after its commit (InTx,
// InSavepoint, the extension runtime hosts do).
//
// A context with a transaction but without hooks has an owner nobody can ask
// about the outcome. fn is then dropped and a warning is logged: running it
// would announce a change that may still roll back. Without a transaction
// there is nothing to wait for and fn runs at once.
func AfterCommit(ctx context.Context, fn func(context.Context)) {
	if h, ok := ctx.Value(afterCommitKey{}).(*CommitHooks); ok && h != nil {
		h.mu.Lock()
		h.fns = append(h.fns, fn)
		h.mu.Unlock()
		return
	}
	if ctx.Value(constants.TxKey) != nil {
		UseLoggerOrDefault(ctx).Warn("an after-commit callback was dropped: the transaction of the context has no commit hooks, so its outcome is unknown")
		return
	}
	runHook(ctx, fn)
}

// ErrNoCommitHooks means ctx runs in a transaction whose owner offers no commit
// hooks, so a change announced after commit would never be announced.
var ErrNoCommitHooks = errors.New("composables: the transaction of the context has no commit hooks (its owner must attach them with WithCommitHooks)")

// RequireCommitHooks is called by a service that promises an announcement after
// commit, before it changes anything: it fails when ctx runs in a transaction
// without hooks (a transaction opened with the pool and handed over with WithTx,
// a savepoint begun by hand), where AfterCommit could not keep the promise, and
// succeeds when the service owns the transaction (InTx opens it with hooks) or
// joins one that has them.
func RequireCommitHooks(ctx context.Context) error {
	if h, ok := ctx.Value(afterCommitKey{}).(*CommitHooks); ok && h != nil {
		return nil
	}
	if ctx.Value(constants.TxKey) != nil {
		return ErrNoCommitHooks
	}
	return nil
}

// InSavepoint runs fn in a savepoint of the transaction held by ctx. The
// callbacks fn registers with AfterCommit are dropped when the savepoint rolls
// back and move to the enclosing transaction's hooks when it is released, so
// they run only if the enclosing transaction commits.
func InSavepoint(ctx context.Context, fn func(context.Context) error) error {
	tx, ok := ctx.Value(constants.TxKey).(pgx.Tx)
	if !ok || tx == nil {
		return ErrNoTx
	}
	sp, err := tx.Begin(ctx)
	if err != nil {
		return err
	}
	spCtx, hooks := WithCommitHooks(WithTx(ctx, sp))
	defer func() {
		if r := recover(); r != nil {
			hooks.Discard()
			_ = sp.Rollback(context.WithoutCancel(ctx))
			panic(r)
		}
	}()
	if err := fn(spCtx); err != nil {
		hooks.Discard()
		if rErr := sp.Rollback(context.WithoutCancel(ctx)); rErr != nil {
			return errors.Join(err, rErr)
		}
		return err
	}
	if err := sp.Commit(ctx); err != nil {
		hooks.Discard()
		return err
	}
	AfterCommit(ctx, hooks.Run)
	return nil
}

// UseLoggerOrDefault is the logger of ctx, or the standard logger when ctx has
// none: code that runs after a commit must be able to report a failure from
// any context.
func UseLoggerOrDefault(ctx context.Context) *logrus.Entry {
	if logger, ok := ctx.Value(constants.LoggerKey).(*logrus.Entry); ok && logger != nil {
		return logger
	}
	return logrus.NewEntry(logrus.StandardLogger())
}
