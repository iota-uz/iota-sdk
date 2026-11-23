package engine

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// SetTestHook runs fn at the end of every fenced transaction of a run, before the counters and the
// receipt; its error rolls the transaction back.
func (r *Runtime) SetTestHook(fn func(ctx context.Context, tx pgx.Tx, label string) error) {
	r.testHook = fn
}
