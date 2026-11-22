package engine_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
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
