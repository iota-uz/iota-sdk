package composables

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/gorm"
)

var (
	ErrNoTx   = errors.New("no transaction found in context")
	ErrNoDB   = errors.New("no database found in context")
	ErrNoPool = errors.New("no database pool found in contenxt")
)

// WithTx returns a new context with the database transaction.
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, constants.TxKey, tx)
}

func WithPoolTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, constants.PoolTxKey, tx)
}

// UseTx returns the database transaction from the context.
func UseTx(ctx context.Context) (*gorm.DB, bool) {
	tx, ok := ctx.Value(constants.TxKey).(*gorm.DB)
	if !ok {
		return nil, false
	}
	return tx, true
}

func UsePoolTx(ctx context.Context) (pgx.Tx, error) {
	tx, ok := ctx.Value(constants.PoolTxKey).(pgx.Tx)
	if !ok {
		return nil, ErrNoTx
	}
	return tx, nil
}

// WithDB returns a new context with the database.
func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, constants.DBKey, db)
}

func WithPool(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, constants.PoolKey, pool)
}

// UseDB returns the database from the context.
func UseDB(ctx context.Context) (*gorm.DB, error) {
	db := ctx.Value(constants.DBKey)
	if db == nil {
		return nil, ErrNoDB
	}
	return db.(*gorm.DB), nil
}

func UsePool(ctx context.Context) (*pgxpool.Pool, error) {
	pool := ctx.Value(constants.PoolKey)
	if pool == nil {
		return nil, ErrNoPool
	}
	return pool.(*pgxpool.Pool), nil
}
