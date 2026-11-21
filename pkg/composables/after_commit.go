package composables

import (
	"context"
	"errors"
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

// ErrNoCommitOwner is returned by AfterCommit when the transaction of the
// context has no declared owner: nobody can say when, or whether, it commits.
var ErrNoCommitOwner = errors.New("composables: the transaction has no declared owner; begin it with composables.InTx or composables.OwnTx, or declare its owner with composables.WithAfterCommit")

// AfterCommit runs fn once the transaction of ctx has committed, so a side
// effect that must not outlive a rolled back change (an eventbus event, a
// wake-up) is not sent for it. fn is dropped when the transaction rolls back.
//
// A context without any transaction has no later commit to wait for and fn
// runs at once. A transaction whose owner did not declare itself
// (WithAfterCommit) cannot honor the callback, and sending it would risk an
// event for a change its owner rolls back, so AfterCommit reports
// ErrNoCommitOwner instead of dropping it silently; the caller fails the
// change, and the transaction's owner, who sees the error, rolls it back.
func AfterCommit(ctx context.Context, fn func()) error {
	if fn == nil {
		return nil
	}
	tx, hasTx := ctx.Value(constants.TxKey).(pgx.Tx)
	if !hasTx || tx == nil {
		fn()
		return nil
	}
	if owner, ok := ctx.Value(constants.AfterCommitKey).(*afterCommitOwner); ok && owner != nil && owner.tx == tx {
		owner.add(fn)
		return nil
	}
	return ErrNoCommitOwner
}

// CheckAfterCommit reports ErrNoCommitOwner when AfterCommit would, without
// registering anything. A change with a side effect that cannot be taken back
// (a sent message) calls it before it acts, so a transaction nobody owns is
// refused before, not after, the effect.
func CheckAfterCommit(ctx context.Context) error {
	tx, hasTx := ctx.Value(constants.TxKey).(pgx.Tx)
	if !hasTx || tx == nil {
		return nil
	}
	if owner, ok := ctx.Value(constants.AfterCommitKey).(*afterCommitOwner); ok && owner != nil && owner.tx == tx {
		return nil
	}
	return ErrNoCommitOwner
}

// OwnTx is the supported path of a caller that begins a transaction itself and
// passes it to services: it returns ctx carrying tx and the owner that keeps
// the services' after-commit callbacks. The caller calls Run on the result
// after tx committed, and not after a rollback.
func OwnTx(ctx context.Context, tx pgx.Tx) (context.Context, *TxCallbacks) {
	callbacks := &TxCallbacks{}
	return WithAfterCommit(WithTx(ctx, tx), tx, callbacks.Add), callbacks
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

// WithSilentTx carries tx like WithTx and declares that nobody wants the
// after-commit callbacks of the changes made in it: they are discarded. A
// seeding tool that fills a database with services uses it, so the eventbus
// events of the services do not fire for seed data.
func WithSilentTx(ctx context.Context, tx pgx.Tx) context.Context {
	return WithAfterCommit(WithTx(ctx, tx), tx, func(func()) {})
}
