package composables

import (
	"context"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/constants"
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
// When ctx carries no commit hooks the transaction is owned by code that does
// not offer them (a test transaction, a host with its own commit): fn then runs
// at once, as it did before AfterCommit existed, and the owner must make sure
// that is acceptable. Without a transaction there is nothing to wait for and fn
// runs at once as well.
func AfterCommit(ctx context.Context, fn func(context.Context)) {
	if h, ok := ctx.Value(afterCommitKey{}).(*CommitHooks); ok && h != nil {
		h.mu.Lock()
		h.fns = append(h.fns, fn)
		h.mu.Unlock()
		return
	}
	runHook(ctx, fn)
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
