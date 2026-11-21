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
// back. InTx declares it for the transactions it begins; every other owner of
// a transaction (the native dispatcher, the outbox dispatcher, the extension
// host) declares it for theirs, and a test scope declares that it never
// commits and runs the callbacks at once.
func WithAfterCommit(ctx context.Context, tx pgx.Tx, add func(func())) context.Context {
	return context.WithValue(ctx, constants.AfterCommitKey, &afterCommitOwner{tx: tx, add: add})
}

// AfterCommit runs fn once the transaction of ctx has committed, so a side
// effect that must not outlive a rolled back change (an eventbus event, a
// wake-up) is not sent for it. fn is dropped when the transaction rolls back.
//
// A transaction whose owner did not declare itself (WithAfterCommit) offers no
// commit to wait for and the owner may well roll it back, so fn is dropped:
// nothing is delivered for a change in a transaction nobody answers for. Only
// a context without any transaction has no later commit to wait for, and fn
// runs at once.
func AfterCommit(ctx context.Context, fn func()) {
	if fn == nil {
		return
	}
	tx, hasTx := ctx.Value(constants.TxKey).(pgx.Tx)
	if !hasTx || tx == nil {
		fn()
		return
	}
	if owner, ok := ctx.Value(constants.AfterCommitKey).(*afterCommitOwner); ok && owner != nil && owner.tx == tx {
		owner.add(fn)
	}
}

// TxCallbacks collects the after-commit callbacks of a transaction its owner
// commits itself: declare it with WithAfterCommit(ctx, tx, c.Add), then call
// Run once the commit succeeded (and not at all after a rollback).
type TxCallbacks struct {
	mu  sync.Mutex
	fns []func()
}

// Add keeps a callback for Run.
func (c *TxCallbacks) Add(fn func()) {
	c.mu.Lock()
	c.fns = append(c.fns, fn)
	c.mu.Unlock()
}

// Run runs the kept callbacks in order, once.
func (c *TxCallbacks) Run() {
	c.mu.Lock()
	fns := c.fns
	c.fns = nil
	c.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}
