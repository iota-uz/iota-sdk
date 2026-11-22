package engine_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
)

// followUp notes the order, waits for its payment (timeout 1s) and notes a late order.
const followUp = `{"name": "Order follow-up", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "note", "type": "acme.note", "params": {"text": "order {{ event.amount }}"}},
  {"id": "pay", "type": "wait", "params": {"event": "acme.order.paid.v1", "timeout": "1s"}},
  {"id": "late", "type": "acme.note", "params": {"text": "late"}},
  {"id": "paid", "type": "acme.note", "params": {"text": "paid {{ nodes.pay.output.event.amount }}"}},
  {"id": "fin", "type": "end", "params": {"reason": "finished"}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "note"},
  {"from": {"node": "note", "outcome": "done"}, "to": "pay"},
  {"from": {"node": "note", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "pay", "outcome": "matched"}, "to": "paid"},
  {"from": {"node": "pay", "outcome": "timeout"}, "to": "late"},
  {"from": {"node": "late", "outcome": "done"}, "to": "fin"},
  {"from": {"node": "late", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "paid", "outcome": "done"}, "to": "fin"},
  {"from": {"node": "paid", "outcome": "failed"}, "to": "fin"}
 ]}`

func onlyRun(t *testing.T, e *env, p *proc, a engine.Actor) engine.RunSummary {
	t.Helper()
	var runs []engine.RunSummary
	p.until(func() bool { runs = e.runs(p, a, engine.RunFilter{}); return len(runs) > 0 }, "a run starts")
	require.Len(t, runs, 1)
	return runs[0]
}

func TestEventWaitTimeoutEndSurvivesRestart(t *testing.T) {
	e := newEnv(t)
	p1 := e.proc()
	a := e.human()
	e.create(p1, a, followUp)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 42}, 0)

	run := onlyRun(t, e, p1, a)
	p1.until(func() bool { return e.status(p1, run.RunID) == engine.RunWaiting }, "the run waits for the payment")
	assert.Equal(t, []string{"note:order 42"}, e.effects(run.RunID))
	steps := len(e.run(p1, a, run.RunID).Steps)

	// The process stops while the run waits; a new one takes over on the same database.
	p1.wf.Shutdown()
	p1.disp.Stop()
	p2 := e.proc()
	require.Len(t, e.run(p2, a, run.RunID).Steps, steps, "a restart takes no step")
	p2.until(func() bool { return e.status(p2, run.RunID) == engine.RunDone }, "the wait times out and the run ends")

	d := e.run(p2, a, run.RunID)
	assert.Equal(t, []string{"late:late", "note:order 42"}, e.effects(run.RunID), "every effect once")
	nodes := make([]string, 0, len(d.Steps))
	for _, s := range d.Steps {
		nodes = append(nodes, s.NodeID+"/"+s.Outcome)
	}
	assert.Equal(t, []string{"note/done", "pay/timeout", "late/done", "fin/end"}, nodes)
	require.Len(t, d.Waits, 1)
	assert.Equal(t, "timed_out", d.Waits[0].Status)
	// The finish is an event.
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1 AND subject_key = $2 AND payload->>'status' = 'done'`,
		engine.EventRunFinished, run.RunID.String()))
}

func TestEventWakesWait(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, followUp)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 7}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")
	// Another order's payment does not wake it; its own does.
	e.publish(e.tenant, orderPaid, "o-2", order{Amount: 1}, 0)
	e.publish(e.tenant, orderPaid, "o-1", order{Amount: 7}, 0)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the payment wakes the wait")
	assert.Equal(t, []string{"note:order 7", "paid:paid 7"}, e.effects(run.RunID))
}

func TestCrashBetweenInboxAndApplyTakesOneStep(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, followUp)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 3}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")

	// The signal is consumed (its own transaction) and the apply dies before its commit, twice; and a
	// step dies after its writes, before the receipt.
	var consumeFails, stepFails atomic.Int32
	consumeFails.Store(2)
	stepFails.Store(1)
	p.rt.SetTestHook(func(_ context.Context, _ pgx.Tx, label string) error {
		switch {
		case label == "consume" && consumeFails.Add(-1) >= 0:
			return &host.InfraError{Err: errors.New("process killed before apply")}
		case label == "step" && stepFails.Add(-1) >= 0:
			return &host.InfraError{Err: errors.New("process killed before the receipt")}
		}
		return nil
	})
	e.publish(e.tenant, orderPaid, "o-1", order{Amount: 3}, 0)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run recovers and ends")
	assert.Negative(t, consumeFails.Load(), "the apply was killed")
	assert.Negative(t, stepFails.Load(), "a step was killed")
	assert.Equal(t, []string{"note:order 3", "paid:paid 3"}, e.effects(run.RunID), "exactly one step of each node")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1 AND node_id = 'paid'`, run.RunID))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1 AND node_id = 'pay' AND outcome = 'matched'`, run.RunID))
}

func TestStaleLeaseCannotWrite(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, followUp)
	// The first step's transaction loses its lease before it ends: another runner owns the instance.
	var stolen atomic.Bool
	p.rt.SetTestHook(func(ctx context.Context, tx pgx.Tx, label string) error {
		if label != "step" || stolen.Swap(true) {
			return nil
		}
		_, err := e.db.Exec(ctx, `UPDATE extruntime.workflow_instances SET lease_epoch = lease_epoch + 1, lease_owner = 'other', lease_until = now() + interval '1 second' WHERE workflow = $1`, engine.WorkflowID)
		return err
	})
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 5}, 0)
	run := onlyRun(t, e, p, a)
	require.Eventually(t, stolen.Load, 10*time.Second, 20*time.Millisecond)
	assert.Empty(t, e.effects(run.RunID), "the fenced step wrote nothing")
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1`, run.RunID))
	// Once the stolen lease expires the run goes on, one step per node.
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run continues after the lease expires")
	assert.Equal(t, []string{"note:order 5"}, e.effects(run.RunID))
}

func TestRedeliveryAndTenantFanOut(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	t2 := e.newTenant()
	ids := map[uuid.UUID][]uuid.UUID{}
	for _, tenant := range []uuid.UUID{e.tenant, t2} {
		a := e.actor(tenant, 1, engine.OriginHuman)
		for i := 0; i < 3; i++ {
			am := e.create(p, a, followUp)
			ids[tenant] = append(ids[tenant], am.ID)
		}
	}
	ev1 := e.publish(e.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	e.publish(t2, orderCreated, "o-1", order{Amount: 2}, 0)
	p.until(func() bool { return e.count(`SELECT count(*) FROM automation.runs`) == 6 }, "six runs")

	// A redelivery of the event (a second consumer pass with the receipt reset) starts nothing new.
	_, err := e.db.Exec(context.Background(), `UPDATE extruntime.subscription_receipts SET status = 'pending' WHERE event_id = $1`, ev1)
	require.NoError(t, err)
	p.pass()
	p.pass()
	assert.Equal(t, 6, e.count(`SELECT count(*) FROM automation.runs`))
	for tenant, autos := range ids {
		runs := e.runs(p, e.actor(tenant, 1, engine.OriginHuman), engine.RunFilter{})
		require.Len(t, runs, 3, "each tenant sees its own runs")
		for _, r := range runs {
			assert.Contains(t, autos, r.AutomationID)
		}
	}
}

func TestFilterErrorIsFilterFailed(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	// 10 / amount fails on a zero amount.
	e.create(p, a, `{"name": "Filtered", "triggers": [{"id": "created", "event": "acme.order.created.v1", "filter": "10 / event.amount > 1"}],
	  "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "fin"}]}`)
	ev := e.publish(e.tenant, orderCreated, "o-1", order{Amount: 0}, 0)
	run := onlyRun(t, e, p, a)
	assert.Equal(t, engine.RunFilterFailed, run.Status)
	assert.Contains(t, run.Reason, "filter_error")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.subscription_receipts WHERE event_id = $1 AND status = 'done'`, ev), "the receipt is done, not dead")
}

func TestJoinAnyCancelsLosingWaitAndTask(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	// Branch pay waits for the payment, branch ask opens a task, branch quick wins after a second; the
	// run then waits again, so what the join cancelled is seen while the run lives.
	e.create(p, a, `{"name": "Race", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [
	  {"id": "split", "type": "parallel", "params": {"branches": ["pay", "ask", "quick"]}},
	  {"id": "wpay", "type": "wait", "params": {"event": "acme.order.paid.v1"}},
	  {"id": "appr", "type": "acme.approve", "params": {}},
	  {"id": "nap", "type": "delay", "params": {"duration": "1s"}},
	  {"id": "first", "type": "join", "params": {"of": "split", "mode": "any"}},
	  {"id": "after", "type": "wait", "params": {"event": "acme.order.paid.v1", "timeout": "1h"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "created", "outcome": "fired"}, "to": "split"},
	  {"from": {"node": "split", "outcome": "pay"}, "to": "wpay"},
	  {"from": {"node": "split", "outcome": "ask"}, "to": "appr"},
	  {"from": {"node": "split", "outcome": "quick"}, "to": "nap"},
	  {"from": {"node": "wpay", "outcome": "matched"}, "to": "first"},
	  {"from": {"node": "appr", "outcome": "approve"}, "to": "first"},
	  {"from": {"node": "appr", "outcome": "reject"}, "to": "first"},
	  {"from": {"node": "nap", "outcome": "done"}, "to": "first"},
	  {"from": {"node": "first", "outcome": "joined"}, "to": "after"},
	  {"from": {"node": "after", "outcome": "matched"}, "to": "fin"},
	  {"from": {"node": "after", "outcome": "timeout"}, "to": "fin"}
	 ]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool {
		return e.count(`SELECT count(*) FROM extruntime.human_tasks t JOIN automation.runs r ON r.instance_id = t.instance_id WHERE r.id = $1 AND t.status = 'open'`, run.RunID) == 1
	}, "the task is open")
	p.until(func() bool {
		return e.count(`SELECT count(*) FROM automation.waits WHERE run_id = $1 AND node_id = 'after' AND status = 'open'`, run.RunID) == 1
	}, "the quick branch wins and the run waits after the join")
	d := e.run(p, a, run.RunID)
	assert.Equal(t, engine.RunWaiting, d.Status)
	waits := map[string]string{}
	for _, w := range d.Waits {
		waits[w.Kind+":"+w.Status] = w.WaitID
	}
	assert.Contains(t, waits, "event:cancelled", "the losing wait is cancelled")
	assert.Contains(t, waits, "task:cancelled", "the losing task's wait is cancelled")
	p.until(func() bool {
		return e.count(`SELECT count(*) FROM extruntime.human_tasks t JOIN automation.runs r ON r.instance_id = t.instance_id WHERE r.id = $1 AND t.status = 'cancelled'`, run.RunID) == 1
	}, "the task left the inbox")
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM extruntime.human_tasks t JOIN automation.runs r ON r.instance_id = t.instance_id WHERE r.id = $1 AND t.status = 'open'`, run.RunID))
	// The cancelled wait no longer wakes on the payment: only the wait after the join does.
	e.publish(e.tenant, orderPaid, "o-1", order{Amount: 1}, 0)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the payment wakes the wait after the join")
	tokens := make([]string, 0, 4)
	for _, tk := range e.run(p, a, run.RunID).Tokens {
		tokens = append(tokens, tk.Branch+":"+tk.Status)
	}
	assert.ElementsMatch(t, []string{":done", "pay:cancelled", "ask:cancelled", "quick:joined"}, tokens)
}

func TestTaskDecisionContinuesTheRun(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, `{"name": "Approval", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [
	  {"id": "appr", "type": "acme.approve", "params": {}},
	  {"id": "yes", "type": "acme.note", "params": {"text": "approved"}},
	  {"id": "no", "type": "acme.note", "params": {"text": "rejected"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "created", "outcome": "fired"}, "to": "appr"},
	  {"from": {"node": "appr", "outcome": "approve"}, "to": "yes"},
	  {"from": {"node": "appr", "outcome": "reject"}, "to": "no"},
	  {"from": {"node": "yes", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "yes", "outcome": "failed"}, "to": "fin"},
	  {"from": {"node": "no", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "no", "outcome": "failed"}, "to": "fin"}
	 ]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	run := onlyRun(t, e, p, a)
	var task uuid.UUID
	p.until(func() bool {
		err := e.db.QueryRow(context.Background(), `SELECT t.id FROM extruntime.human_tasks t JOIN automation.runs r ON r.instance_id = t.instance_id WHERE r.id = $1 AND t.status = 'open'`, run.RunID).Scan(&task)
		return err == nil
	}, "the task is open")
	require.NoError(t, p.wf.DecideTask(context.Background(), spiSubject(e.tenant, 1), task, []byte(`{"outcome": "reject"}`)))
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the decision continues the run")
	assert.Equal(t, []string{"no:rejected"}, e.effects(run.RunID))
}

func TestWaitWithoutTimeoutFailsAtWaitMax(t *testing.T) {
	e := newEnv(t)
	p := e.proc(func(c *engine.Config) { c.Limits.WaitMax = time.Second })
	a := e.human()
	e.create(p, a, `{"name": "Forever", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "pay", "type": "wait", "params": {"event": "acme.order.paid.v1"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "pay"}, {"from": {"node": "pay", "outcome": "matched"}, "to": "fin"}]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunFailed }, "the wait fails at WaitMax")
	d := e.run(p, a, run.RunID)
	require.NotNil(t, d.Error)
	assert.Equal(t, engine.CodeTimeout, d.Error.Code)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1 AND payload->>'status' = 'failed'`, engine.EventRunFinished))
}
