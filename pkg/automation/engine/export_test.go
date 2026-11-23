package engine

import (
	"context"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"

	"github.com/jackc/pgx/v5"
)

// SetTestHook runs fn at the end of every fenced transaction of a run, before the counters and the
// receipt; its error rolls the transaction back.
func (r *Runtime) SetTestHook(fn func(ctx context.Context, tx pgx.Tx, label string) error) {
	r.testHook = fn
}

// SetContextHook calls fn with every context a live run's step is built with (BuildRunContext with a run).
func (r *Runtime) SetContextHook(fn func(event string, gctx graph.Context)) {
	r.contextHook = fn
}

// SweepWindow is the collect sweep's close of one window it read as due: it closes it only if it is due
// under the row's lock.
func (r *Runtime) SweepWindow(ctx context.Context, tenant, automation uuid.UUID, trigger, key string) error {
	return pgx.BeginFunc(ctx, r.cfg.DB, func(tx pgx.Tx) error {
		return r.closeWindow(ctx, tx, tenant, automation, trigger, key, true)
	})
}
