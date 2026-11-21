package composables

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/constants"
)

// afterCommitOwner is the owner of the transaction tx of a context: add keeps
// a callback to run once that owner has committed the transaction.
type afterCommitOwner struct {
	tx  pgx.Tx
	add func(func())
}

// WithAfterCommit declares that the owner of tx, which ctx carries, runs the
// callbacks add receives after it commits tx and drops them when it rolls
// back. InTx declares it for the transactions it begins; the other owners of a
// transaction (the native dispatcher, the outbox dispatcher) declare it for
// theirs.
func WithAfterCommit(ctx context.Context, tx pgx.Tx, add func(func())) context.Context {
	return context.WithValue(ctx, constants.AfterCommitKey, &afterCommitOwner{tx: tx, add: add})
}

// AfterCommit runs fn once the transaction of ctx has committed, so a side
// effect that must not outlive a rolled back change (an eventbus event, a
// wake-up) is not sent for it. fn is dropped when the transaction rolls back.
// Without a transaction in ctx, or when the owner of the transaction did not
// declare itself (a test's rolled-back scope), the change has no later commit
// to wait for and fn runs at once.
func AfterCommit(ctx context.Context, fn func()) {
	if fn == nil {
		return
	}
	if owner, ok := ctx.Value(constants.AfterCommitKey).(*afterCommitOwner); ok && owner != nil {
		if tx, ok := ctx.Value(constants.TxKey).(pgx.Tx); ok && tx == owner.tx {
			owner.add(fn)
			return
		}
	}
	fn()
}

// commitCallbacks collects the callbacks of the transaction InTx owns.
type commitCallbacks struct {
	mu  sync.Mutex
	fns []func()
}

func (c *commitCallbacks) add(fn func()) {
	c.mu.Lock()
	c.fns = append(c.fns, fn)
	c.mu.Unlock()
}

func (c *commitCallbacks) run() {
	c.mu.Lock()
	fns := c.fns
	c.fns = nil
	c.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}
