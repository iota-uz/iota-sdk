package engine

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/automation/state"
)

type failingRow struct{ err error }

func (r failingRow) Scan(...any) error { return r.err }

type failingDB struct{ err error }

func (d failingDB) QueryRow(context.Context, string, ...any) pgx.Row { return failingRow(d) }

// A database failure while an expression reads the memory is a platform failure the step is retried
// for; a lookup that failed any other way stays the expression's.
func TestMemoryFailureIsRetriedOnlyForThePlatform(t *testing.T) {
	read := func(err error) graph.Context {
		r := state.NewReader(context.Background(), failingDB{err}, uuid.New(), uuid.New())
		_, _, lookupErr := r.Lookup("k")
		require.Error(t, lookupErr)
		return graph.Context{State: r}
	}
	// 57P01: the server is shutting down.
	require.Error(t, memoryFailure(context.Background(), read(&pgconn.PgError{Code: "57P01"})))
	assert.NoError(t, memoryFailure(context.Background(), read(&pgconn.PgError{Code: "22P02"})))
	assert.NoError(t, memoryFailure(context.Background(), graph.Context{}))
	assert.NoError(t, memoryFailure(context.Background(), graph.Context{State: expr.Lookup(state.Empty{})}))
}
