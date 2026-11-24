package engine

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"

	"github.com/jackc/pgx/v5"
)

// SetTestHook sets fn, called with a label at points of the engine. Inside a fenced transaction of a
// run (step, consume, timeout, task, release, fail, subject, catchup) it runs at the end, before the
// counters and the receipt, with that transaction; its error rolls the transaction back.
//
// Two labels are called outside any transaction, with tx == nil: "timer-fired" between a timer's receipt
// and the timeout's transaction (an error aborts the run's pass; the timer is already fired, so the
// next pass applies the timeout), and "prune-runs" in Prune between the step and the run deletes (an
// error ends Prune). A hook must not use tx for them.
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

// SetOperationsHook runs fn in Operations.Retry between reading the blocked attempt and acting on it.
func (r *Runtime) SetOperationsHook(fn func()) { r.opsHook = fn }

// SetCallResultHook runs fn in a call child before (stage "before") and after ("after") its result
// transaction (with the parent's signal) commits, before the child's completion.
func (r *Runtime) SetCallResultHook(fn func(ctx context.Context, child uuid.UUID, stage string)) {
	r.callResultHook = fn
}

// QuietCallObserver keeps the observer of automation.call from signalling the parent.
func (r *Runtime) QuietCallObserver() { r.quietCallObserver = true }

// TaskAssignedNewUsers is the new users of a task change as the TaskAssigned observer finds them.
func TaskAssignedNewUsers(ctx context.Context, obs workflow.TaskObserver, tenant uuid.UUID, t workflow.Task, change workflow.TaskChange) ([]int64, bool, error) {
	return obs.(*taskAssignedObserver).newUsers(ctx, tenant, t, change)
}

// SubjectLockKey is the advisory lock key steps, starts and the router take for a run's subject.
func SubjectLockKey(tenant uuid.UUID, s SubjectRef, run uuid.UUID) string {
	return subjectLockKey(tenant, s, run)
}

// RecordedAnswer is the result a call's journaled attempt gives when the child ended without its own
// (endedResult's recovery).
func (r *Runtime) RecordedAnswer(raw []byte, nodeType string) (WaitResult, bool) {
	return r.calls.recordedAnswer(raw, CallRequest{NodeType: nodeType, WaitID: "w"})
}

// SetTestWait bounds how long a repeat of a node test waits for the first one (default 30 s).
func SetTestWait(d time.Duration) func() {
	was := testWait
	testWait = d
	return func() { testWait = was }
}
