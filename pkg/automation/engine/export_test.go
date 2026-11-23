package engine

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SetTestHook sets fn, called with a label at points of the engine. Inside a fenced transaction of a
// run (step, consume, timeout, task, release, fail, subject, catchup) it runs at the end, before the
// counters and the receipt, with that transaction; its error rolls the transaction back.
//
// One label is called outside any transaction, with tx == nil: "timer-fired" between a timer's receipt
// and the timeout's transaction (an error aborts the run's pass; the timer is already fired, so the
// next pass applies the timeout). A hook must not use tx for it.
func (r *Runtime) SetTestHook(fn func(ctx context.Context, tx pgx.Tx, label string) error) {
	r.testHook = fn
}

// SubjectLockKey is the advisory lock key steps, starts and the router take for a run's subject.
func SubjectLockKey(tenant uuid.UUID, s SubjectRef, run uuid.UUID) string {
	return subjectLockKey(tenant, s, run)
}
