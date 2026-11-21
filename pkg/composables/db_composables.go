package composables

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoTx   = errors.New("no transaction found in context")
	ErrNoPool = errors.New("no database pool found in context")
)

// WithTx returns a context that runs in tx. The commit hooks of an enclosing
// transaction do not follow it (AfterCommit): tx has its own outcome, which
// only its owner knows; the owner attaches hooks with WithCommitHooks.
func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	ctx = context.WithValue(ctx, constants.TxKey, tx)
	if h, _ := ctx.Value(afterCommitKey{}).(*CommitHooks); h != nil {
		ctx = context.WithValue(ctx, afterCommitKey{}, (*CommitHooks)(nil))
	}
	return ctx
}

func UseTx(ctx context.Context) (repo.Tx, error) {
	tx := ctx.Value(constants.TxKey)
	if tx == nil {
		return UsePool(ctx)
	}
	return tx.(repo.Tx), nil
}

func WithPool(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, constants.PoolKey, pool)
}

func UsePool(ctx context.Context) (*pgxpool.Pool, error) {
	pool := ctx.Value(constants.PoolKey)
	if pool == nil {
		return nil, ErrNoPool
	}
	return pool.(*pgxpool.Pool), nil
}

func BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx := ctx.Value(constants.TxKey)
	if tx != nil {
		return tx.(pgx.Tx), nil
	}
	pool, err := UsePool(ctx)
	if err != nil {
		return nil, err
	}
	return pool.Begin(ctx)
}

// InTx runs the given function in a transaction. Reuses existing transaction if present,
// otherwise ALWAYS creates a new one.
func InTx(ctx context.Context, fn func(context.Context) error) error {
	// Check if there's already a transaction in the context
	existingTx := ctx.Value(constants.TxKey)
	if existingTx != nil {
		// Reuse existing transaction - don't commit/rollback as it's managed elsewhere
		return fn(ctx)
	}

	// No existing transaction, create a new one
	pool, err := UsePool(ctx)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		// A panic unwinding through fn would otherwise leave the transaction, and
		// any row locks it holds, open until the connection is reclaimed.
		if r := recover(); r != nil {
			_ = tx.Rollback(ctx)
			panic(r)
		}
	}()

	txCtx, hooks := WithCommitHooks(WithTx(ctx, tx))
	if err := fn(txCtx); err != nil {
		hooks.Discard()
		if rErr := tx.Rollback(ctx); rErr != nil {
			return errors.Join(err, rErr)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		hooks.Discard()
		return err
	}
	hooks.Run(ctx)
	return nil
}
