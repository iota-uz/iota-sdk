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

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, constants.TxKey, tx)
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

func InTx(ctx context.Context, fn func(context.Context) error) error {
	pool, err := UsePool(ctx)
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}

	if err := fn(WithTx(ctx, tx)); err != nil {
		if rErr := tx.Rollback(ctx); rErr != nil {
			return errors.Join(err, rErr)
		}
		return err
	}
	return tx.Commit(ctx)
}
