package engine_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

func TestFinishedEventWithholdsOutputOfASensitiveRun(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	owner := e.human()
	e.create(p, owner, noteGraph("x"))
	e.publish(e.tenant, orderCreated, "o-1", order{Secret: "s3cret"}, 0)
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	var raw []byte
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT payload FROM extruntime.outbox_events WHERE event = $1 AND subject_key = $2`,
		engine.EventRunFinished, run.RunID.String()).Scan(&raw))
	var got engine.RunFinishedPayload
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.True(t, got.OutputWithheld)
	assert.JSONEq(t, `null`, string(got.Output))
}

func TestUnresolvableProvenanceHidesTheStep(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	owner := e.human()
	e.create(p, owner, noteGraph("x"))
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	// The event a step derived from left the tenant's catalog (its connector was removed).
	_, err := e.db.Exec(context.Background(), `UPDATE automation.steps SET sensitive = '[{"event": "acme.gone.v1", "path": "token"}]' WHERE run_id = $1 AND node_id = 'note'`, run.RunID)
	require.NoError(t, err)
	for _, st := range e.run(p, owner, run.RunID).Steps {
		assert.Equal(t, st.NodeID == "note", st.Hidden, "a reference that cannot be checked is not disclosed: %s", st.NodeID)
	}
}

func TestStaleScheduleDefinitionDoesNotRunTheSlot(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Nightly", "triggers": [{"id": "night", "event": "automation.schedule.v1", "params": {"cron": "0 3 * * *"}}],
	 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "night", "outcome": "fired"}, "to": "fin"}]}`)
	old, err := p.rt.Schedules().ActiveSchedules(ctx)
	require.NoError(t, err)
	require.Len(t, old, 1)
	// The graph is enabled again: the listed definition is of an earlier generation.
	_, err = p.rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	_, err = p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: 1})
	require.NoError(t, err)
	tick := time.Now().UTC().Truncate(24 * time.Hour).Add(27 * time.Hour) // 03:00 UTC tomorrow
	slot := events.Slot{ScheduleID: old[0].ID, TenantID: e.tenant, At: tick}
	err = pgx.BeginFunc(ctx, e.db, func(tx pgx.Tx) error { return old[0].Handler.RunSlot(ctx, tx, slot) })
	require.Error(t, err, "a stale definition hands the slot back for a retry with the current one")

	cur, err := p.rt.Schedules().ActiveSchedules(ctx)
	require.NoError(t, err)
	require.Len(t, cur, 1)
	run := func(at time.Time) {
		require.NoError(t, pgx.BeginFunc(ctx, e.db, func(tx pgx.Tx) error {
			return cur[0].Handler.RunSlot(ctx, tx, events.Slot{ScheduleID: cur[0].ID, TenantID: e.tenant, At: at})
		}))
	}
	run(tick.Add(-48 * time.Hour)) // before the current enable
	run(tick.Add(time.Minute))     // not on the cron
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.runs`))
	run(tick)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.runs`), "a slot of the current enable runs")
}

func TestResumedPredecessorWaitsForItsConversation(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Chat", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1h"}}, {"id": "note", "type": "acme.note", "params": {"text": "after"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "note"},
	  {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}]}`)
	chat := &engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"}
	first, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: chat})
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, first.RunID) == engine.RunWaiting }, "the first run waits")
	var inst [16]byte
	require.NoError(t, e.db.QueryRow(ctx, `SELECT instance_id FROM automation.runs WHERE id = $1`, first.RunID).Scan(&inst))
	_, err = e.db.Exec(ctx, `UPDATE extruntime.workflow_instances SET status = 'blocked', blocked_reason = 'ambiguous', lease_owner = NULL, lease_until = NULL WHERE id = $1`, inst)
	require.NoError(t, err)
	repl, err := p.rt.Service().Restart(ctx, human, first.RunID, 1)
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, repl.RunID) == engine.RunWaiting }, "the replacement runs")

	// The predecessor is resumed while its replacement lives: it takes no step until the conversation is free.
	require.NoError(t, p.wf.Resume(ctx, e.tenant, inst))

	p.until(func() bool {
		return e.count(`SELECT count(*) FROM automation.runs WHERE id = $1 AND awaiting_subject`, first.RunID) == 1 && e.status(p, first.RunID) == engine.RunWaiting
	}, "the resumed predecessor waits for its conversation")
	steps := e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1`, first.RunID)
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, steps, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1`, first.RunID), "no step beside the replacement")

	require.NoError(t, p.rt.Service().Cancel(ctx, human, repl.RunID))
	p.until(func() bool {
		return e.count(`SELECT count(*) FROM automation.runs WHERE id = $1 AND NOT awaiting_subject`, first.RunID) == 1
	}, "the predecessor takes the conversation back once it is free")
	assert.Equal(t, engine.RunWaiting, e.status(p, first.RunID))
}
