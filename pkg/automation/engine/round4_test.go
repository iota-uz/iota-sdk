package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
)

func TestEventDeliveredWhileTheWaitOpensStillWakesIt(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, followUp)
	// While the step that opens the wait is still uncommitted, the payment is published and delivered:
	// the router cannot see the wait yet, and the receipt is done.
	var once atomic.Bool
	p.rt.SetTestHook(func(ctx context.Context, tx pgx.Tx, label string) error {
		if label != "step" {
			return nil
		}
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`).Scan(&open); err != nil || open == 0 || once.Swap(true) {
			return err
		}
		e.publish(e.tenant, orderPaid, "o-1", order{Amount: 8}, 0)
		_, err := p.disp.RunOnce(ctx)
		return err
	})
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 8}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the wait catches up with the delivered payment")
	require.True(t, once.Load())
	assert.Equal(t, []string{"note:order 8", "paid:paid 8"}, e.effects(run.RunID))
}

func TestAFailedFilterDoesNotTakeAnotherTriggersStart(t *testing.T) {
	for _, order_ := range []string{"a_fails", "z_fails"} {
		t.Run(order_, func(t *testing.T) {
			e := newEnv(t)
			p := e.proc()
			a := e.human()
			failing, passing := "a", "z"
			if order_ == "z_fails" {
				failing, passing = "z", "a"
			}
			e.create(p, a, `{"name": "Two", "triggers": [
			  {"id": "`+failing+`", "event": "acme.order.created.v1", "filter": "10 / event.amount > 1"},
			  {"id": "`+passing+`", "event": "acme.order.created.v1"}],
			 "nodes": [{"id": "fin", "type": "end", "params": {}}],
			 "edges": [{"from": {"node": "`+failing+`", "outcome": "fired"}, "to": "fin"}, {"from": {"node": "`+passing+`", "outcome": "fired"}, "to": "fin"}]}`)
			e.publish(e.tenant, orderCreated, "o-1", order{Amount: 0}, 0)
			p.until(func() bool { return len(e.runs(p, a, engine.RunFilter{})) == 2 }, "a diagnostic row and a run")
			statuses := map[engine.RunStatus]int{}
			for _, r := range e.runs(p, a, engine.RunFilter{}) {
				statuses[r.Status]++
			}
			assert.Equal(t, 1, statuses[engine.RunFilterFailed])
			assert.Equal(t, 1, len(e.runs(p, a, engine.RunFilter{}))-statuses[engine.RunFilterFailed], "the passing trigger starts its run in either order")
		})
	}
}

func TestCancelOfABlockedRunWithoutInstance(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	am := e.create(p, human, noteGraph("x"))
	_, err := e.db.Exec(context.Background(), `DELETE FROM extruntime.trigger_owners WHERE trigger_id = $1`, "automation:"+am.ID.String())
	require.NoError(t, err)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	require.Equal(t, engine.RunBlocked, run.Status)
	require.NoError(t, p.rt.Service().Cancel(context.Background(), human, run.RunID))
	assert.Equal(t, engine.RunCancelled, e.status(p, run.RunID))
}

func TestSupersessionOutlivesTheReplacement(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Chat", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1h"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "fin"}]}`)
	chat := &engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"}
	first, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: chat})
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, first.RunID) == engine.RunWaiting }, "the first run waits")
	_, err = e.db.Exec(ctx, `UPDATE extruntime.workflow_instances i SET status = 'blocked', blocked_reason = 'ambiguous'
		FROM automation.runs r WHERE r.instance_id = i.id AND r.id = $1`, first.RunID)
	require.NoError(t, err)
	repl, err := p.rt.Service().Restart(ctx, human, first.RunID, 1)
	require.NoError(t, err)
	require.NoError(t, p.rt.Service().Cancel(ctx, human, repl.RunID))
	p.until(func() bool { return e.status(p, repl.RunID) == engine.RunCancelled }, "the replacement ends")
	// Retention prunes the replacement while the predecessor is still blocked.
	_, err = e.db.Exec(ctx, `DELETE FROM automation.runs WHERE id = $1`, repl.RunID)
	require.NoError(t, err)
	next, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: chat})
	require.NoError(t, err)
	assert.NotEqual(t, engine.RunQueued, next.Status, "the superseded predecessor still does not hold the conversation")
}

func TestCatchUpReadsPastAFullBatch(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, `{"name": "Filtered wait", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "w", "type": "wait", "params": {"event": "acme.order.paid.v1", "filter": "event.amount == 999", "timeout": "1h"}},
	  {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "w"}, {"from": {"node": "w", "outcome": "matched"}, "to": "fin"},
	  {"from": {"node": "w", "outcome": "timeout"}, "to": "fin"}]}`)
	// While the wait's step is uncommitted, 201 payments are delivered: 200 the filter refuses, then the one it takes.
	var once atomic.Bool
	p.rt.SetTestHook(func(ctx context.Context, tx pgx.Tx, label string) error {
		if label != "step" {
			return nil
		}
		var open int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`).Scan(&open); err != nil || open == 0 || once.Swap(true) {
			return err
		}
		for i := 0; i < 200; i++ {
			e.publish(e.tenant, orderPaid, "o-1", order{Amount: i}, 0)
		}
		e.publish(e.tenant, orderPaid, "o-1", order{Amount: 999}, 0)
		for i := 0; i < 10; i++ {
			if _, err := p.disp.RunOnce(ctx); err != nil {
				return err
			}
		}
		return nil
	})
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the 201st payment, past a full catch-up batch, wakes the wait")
	require.True(t, once.Load())
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1 AND node_id = 'w' AND outcome = 'matched'`, run.RunID))
}

func TestMaintenanceKeepsTheReleaseMarkOfABlockedRun(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	a := e.human()
	e.create(p, a, followUp)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")
	// A closed wait still holds its timer, and the instance blocks before its runner releases it.
	_, err := e.db.Exec(ctx, `UPDATE automation.waits SET status = 'cancelled', release_pending = true, updated_at = now() - interval '1 hour' WHERE run_id = $1`, run.RunID)
	require.NoError(t, err)
	_, err = e.db.Exec(ctx, `UPDATE extruntime.workflow_instances i SET status = 'blocked', blocked_reason = 'ambiguous', lease_owner = NULL, lease_until = NULL
		FROM automation.runs r WHERE r.instance_id = i.id AND r.id = $1`, run.RunID)
	require.NoError(t, err)
	require.NoError(t, p.rt.Maintain(ctx))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.waits WHERE run_id = $1 AND release_pending`, run.RunID),
		"the mark stays for the runner after Resume")
}

func TestRecoveredEventBeforeTheDeadlineBeatsTheTimer(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, followUp) // the payment wait times out after 1s
	var delivered, delayed atomic.Bool
	p.rt.SetTestHook(func(ctx context.Context, tx pgx.Tx, label string) error {
		switch label {
		case "step":
			// The payment arrives and is delivered while the wait's transaction is still open.
			var open int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`).Scan(&open); err != nil || open == 0 || delivered.Swap(true) {
				return err
			}
			e.publishAt(e.tenant, orderPaid, "o-1", order{Amount: 6}, e.deadline(tx).Add(-500*time.Millisecond))
			_, err := p.disp.RunOnce(ctx)
			return err
		case "catchup":
			// The catch-up that recovers it commits only after the deadline.
			if !delayed.Swap(true) {
				time.Sleep(1500 * time.Millisecond)
			}
		}
		return nil
	})
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 6}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	require.True(t, delivered.Load())
	require.True(t, delayed.Load())
	assert.Equal(t, []string{"note:order 6", "paid:paid 6"}, e.effects(run.RunID), "the payment came before the deadline: it wins over the timeout")
}

func TestConsumedEventIsAppliedBeforeTheTimerAfterARollback(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, followUp) // the payment wait times out after 1s
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 2}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")
	// The payment's apply fails once, after the deadline has passed; its inbox receipt is recorded.
	var failed atomic.Bool
	p.rt.SetTestHook(func(_ context.Context, _ pgx.Tx, label string) error {
		if label == "consume" && !failed.Swap(true) {
			time.Sleep(1500 * time.Millisecond)
			return &host.InfraError{Err: errors.New("apply rolled back")}
		}
		return nil
	})
	e.publishAt(e.tenant, orderPaid, "o-1", order{Amount: 2}, e.deadline(e.db).Add(-500*time.Millisecond))
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	require.True(t, failed.Load())
	assert.Equal(t, []string{"note:order 2", "paid:paid 2"}, e.effects(run.RunID), "the consumed payment is applied before the timer")
}

func TestRemovedNestedFieldUnderAProtectedParentStaysHidden(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	owner := e.human() // may read the current secret
	e.create(p, owner, noteGraph("x"))
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	// The step derived from secret.pin; the event's secret is a scalar now, so that child is gone.
	_, err := e.db.Exec(context.Background(), `UPDATE automation.steps SET sensitive = '[{"event": "acme.order.created.v1", "path": "secret.pin"}]' WHERE run_id = $1 AND node_id = 'note'`, run.RunID)
	require.NoError(t, err)
	notes := 0
	for _, st := range e.run(p, owner, run.RunID).Steps {
		if st.NodeID == "note" {
			notes++
			assert.True(t, st.Hidden, "a child the schema no longer has is not vouched for by its parent")
			continue
		}
		assert.False(t, st.Hidden, "step %s stays visible", st.NodeID)
	}
	require.Equal(t, 1, notes, "the history has the note step")
}

func TestTimelyPaymentRoutedBeforeTheTimeoutAppliesWins(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, followUp) // the payment wait times out after 1s
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 2}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")
	// The payment happened before the deadline and is delivered after the timer's receipt, before the
	// timeout's transaction.
	e.publishAt(e.tenant, orderPaid, "o-1", order{Amount: 2}, e.deadline(e.db).Add(-500*time.Millisecond))
	var routed atomic.Bool
	p.rt.SetTestHook(func(ctx context.Context, _ pgx.Tx, label string) error {
		if label == "timer-fired" && !routed.Swap(true) {
			_, err := p.disp.RunOnce(ctx)
			return err
		}
		return nil
	})
	// Only the workflow engine runs until the run ends: no dispatcher pass delivers the payment before
	// the timer fires.
	time.Sleep(1500 * time.Millisecond)
	ctx := context.Background()
	for end := time.Now().Add(20 * time.Second); time.Now().Before(end) && e.status(p, run.RunID) != engine.RunDone; {
		_, err := p.wf.RunOnce(ctx)
		require.NoError(t, err)
		time.Sleep(20 * time.Millisecond)
	}
	require.Equal(t, engine.RunDone, e.status(p, run.RunID))
	require.True(t, routed.Load())
	assert.Equal(t, []string{"note:order 2", "paid:paid 2"}, e.effects(run.RunID), "the timely payment wins over the timeout")
}
