package connectors

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

// rowReader is what the availability reads need.
type rowReader interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ambient is the transaction ctx carries (pkg/composables), else the pool. The availability of a
// tenant's connections is read while the engine validates a graph, which may happen inside a caller's
// transaction (a guarded change being applied): reading through it never waits for a second
// connection of the pool the caller holds one of.
func ambient(ctx context.Context, pool *pgxpool.Pool) rowReader {
	if tx, ok := ctx.Value(constants.TxKey).(repo.Tx); ok && tx != nil {
		return tx
	}
	return pool
}
