package engine_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// lostDispatch is a command host whose first dispatches are lost before the host records them, as when
// the process stops between the workflow's started record and the dispatch: the occurrence stays started
// with no invocation, and the next run of the instance sends the command again.
type lostDispatch struct {
	next     workflow.CommandInvoker
	lose     atomic.Int32  // dispatches still to lose
	delay    time.Duration // how long a lost dispatch takes
	deadline atomic.Bool   // the dispatch that reached the host carried a deadline
}

func (l *lostDispatch) Invoke(ctx context.Context, req host.Request) (host.Result, error) {
	if l.lose.Add(-1) >= 0 {
		time.Sleep(l.delay)
		return host.Result{}, &host.InfraError{Err: errors.New("the process stopped before the dispatch")}
	}
	_, has := ctx.Deadline()
	l.deadline.Store(has)
	return l.next.Invoke(ctx, req)
}

func newLostDispatchEnv(t *testing.T, lose int32, delay time.Duration) (*builtinEnv, *lostDispatch) {
	t.Helper()
	be := newBuiltinEnv(t)
	withPerm := append(append([]string{}, allPerms...), permBlock)
	be.subjects.Grant(1, withPerm...)
	ld := &lostDispatch{next: be.commands, delay: delay}
	ld.lose.Store(lose)
	be.commands = ld
	return be, ld
}

// A command the workflow sends again after a lost dispatch runs as a first dispatch does: within the
// call's deadline and holding one of the tenant's call slots.
func TestARecoveredActionHoldsASlotAndItsDeadline(t *testing.T) {
	be, ld := newLostDispatchEnv(t, 1, 0)
	var slots atomic.Int32
	be.action.onRun = func(ctx context.Context) {
		var n int32
		_ = be.db.QueryRow(context.WithoutCancel(ctx), `SELECT count(*) FROM automation.call_slots WHERE tenant_id = $1`, be.tenant).Scan(&n)
		slots.Store(n)
	}
	p := be.proc()
	a := be.actor(be.tenant, 1, engine.OriginHuman, append(append([]string{}, allPerms...), permBlock)...)
	be.create(p, a, actionGraph())
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the action runs")
	assert.Equal(t, []string{"ok:u-9"}, be.effects(run.RunID))
	assert.Equal(t, int32(1), be.action.calls.Load(), "the command ran once")
	assert.Equal(t, int32(1), slots.Load(), "the recovered command held a call slot")
	assert.True(t, ld.deadline.Load(), "the recovered command carried the call's deadline")
	assert.Zero(t, be.count(`SELECT count(*) FROM automation.call_slots`), "the slot is given back")
}

// A command whose deadline passed before the workflow could send it again is not sent: the call times out.
func TestARecoveredActionPastItsDeadlineIsNotSent(t *testing.T) {
	be, _ := newLostDispatchEnv(t, 1, 1500*time.Millisecond)
	p := be.proc(func(c *engine.Config) { c.CallLimits.DefaultTimeout = time.Second })
	a := be.actor(be.tenant, 1, engine.OriginHuman, append(append([]string{}, allPerms...), permBlock)...)
	be.create(p, a, actionGraph())
	be.publish(be.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, be.env, p, a)
	p.until(func() bool { return be.status(p, run.RunID) == engine.RunDone }, "the call times out")
	assert.Equal(t, []string{"bad:" + engine.CodeTimeout}, be.effects(run.RunID))
	assert.Zero(t, be.action.calls.Load(), "the command was not sent after its deadline")
}
