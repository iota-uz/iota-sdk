package engine_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

func TestRunHistoryHidesWhatDerivesFromSensitiveFields(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	owner := e.human() // may read the secret
	// The note transforms the secret (upper-cased): no literal match could find it in the output.
	e.create(p, owner, noteGraph("secret is {{ event.secret.upperAscii() }}"))
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 4, Secret: "s3cret"}, 0)
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")

	mine := e.run(p, owner, run.RunID).Steps
	require.NotEmpty(t, mine)
	var b strings.Builder
	for _, st := range mine {
		assert.False(t, st.Hidden)
		b.Write(st.Output)
	}
	assert.Contains(t, b.String(), "S3CRET", "the owner reads what derives from the secret")

	reader := e.actor(e.tenant, 2, engine.OriginHuman, engine.PermRead, permRead)
	theirs := e.run(p, reader, run.RunID).Steps
	require.Len(t, theirs, len(mine))
	for _, st := range theirs {
		assert.True(t, st.Hidden, "step %s ran after the run read the secret", st.NodeID)
		assert.Nil(t, st.Input)
		assert.Nil(t, st.Output)
	}
}

func TestNonSensitiveRunsStayVisible(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	owner := e.human()
	e.create(p, owner, noteGraph("amount {{ event.amount }}"))
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 4}, 0) // no secret: nothing sensitive was read
	run := onlyRun(t, e, p, owner)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	reader := e.actor(e.tenant, 2, engine.OriginHuman, engine.PermRead, permRead)
	for _, st := range e.run(p, reader, run.RunID).Steps {
		assert.False(t, st.Hidden)
	}
}

func TestWaitApplyReprojectsForRightsOfNow(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	e.create(p, human, `{"name": "Wait created", "triggers": [{"id": "paid", "event": "acme.order.paid.v1"}],
	 "nodes": [{"id": "w", "type": "wait", "params": {"event": "acme.order.created.v1", "timeout": "1h"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "paid", "outcome": "fired"}, "to": "w"}, {"from": {"node": "w", "outcome": "matched"}, "to": "fin"}, {"from": {"node": "w", "outcome": "timeout"}, "to": "fin"}]}`)
	e.publish(e.tenant, orderPaid, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")
	e.publish(e.tenant, orderCreated, "o-1", order{Secret: "s3cret"}, 0)
	// The signal is routed while the run-as may read the secret, and applied after it lost the right.
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := p.disp.RunOnce(ctx)
		require.NoError(t, err)
	}
	require.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.workflow_signals s JOIN automation.runs r ON r.instance_id = s.instance_id WHERE r.id = $1`, run.RunID))
	e.subjects.Grant(1, engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate, permRead)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the wait wakes")
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1 AND output::text LIKE '%s3cret%'`, run.RunID),
		"the woken event is what the run-as may read now")
}

func TestScheduleGraphsDoNotRunAsInitiator(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	_, _, err := p.rt.Service().Create(context.Background(), e.human(), engine.CreateInput{Graph: json.RawMessage(`{"name": "Nightly", "run_as": "initiator",
	 "triggers": [{"id": "night", "event": "automation.schedule.v1", "params": {"cron": "0 3 * * *"}}],
	 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "night", "outcome": "fired"}, "to": "fin"}]}`)})
	var verr *engine.ValidationError
	require.ErrorAs(t, err, &verr)
	assert.Equal(t, "triggers[0].event", verr.Errors[0].Path)
}

func TestRestartedBlockedConversationRunHandsOverTheSubject(t *testing.T) {
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
	ref, err := p.rt.Service().Restart(ctx, human, first.RunID, 1)
	require.NoError(t, err)
	assert.NotEqual(t, engine.RunQueued, ref.Status, "the blocked predecessor no longer holds the conversation")
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunWaiting }, "the replacement runs")
	assert.Equal(t, engine.RunBlocked, e.status(p, first.RunID), "the predecessor is untouched")
	// A new start of the conversation queues behind the replacement, not behind the superseded run only.
	next, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: chat})
	require.NoError(t, err)
	assert.Equal(t, engine.RunQueued, next.Status)
}

func TestDisableCancelsQueuedConversationRuns(t *testing.T) {
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
	second, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: chat})
	require.NoError(t, err)
	require.Equal(t, engine.RunQueued, second.Status)
	p.until(func() bool { return e.status(p, first.RunID) == engine.RunWaiting }, "the first run waits")

	_, err = p.rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, first.RunID) == engine.RunCancelled }, "the head is cancelled")
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Equal(t, engine.RunCancelled, e.status(p, second.RunID), "the queued run never starts")
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.runs WHERE id = $1 AND instance_id IS NOT NULL`, second.RunID))
}

func TestCancellationIntentSurvivesAProcessFailure(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Manual", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1h"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "fin"}]}`)
	ref, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`)})
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunWaiting }, "the manual run waits")
	// The process stopped after Disable committed (the run is marked) and before it cancelled the run; the
	// graph is enabled again meanwhile.
	_, err = e.db.Exec(ctx, `UPDATE automation.runs SET cancel_requested = true WHERE id = $1`, ref.RunID)
	require.NoError(t, err)
	_, err = p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: 1})
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunCancelled }, "maintenance finishes the cancellation")
}

func TestWaitRoutingSeesWhatTheRunAsMayRead(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	// The wait's filter reads a sensitive field the run-as (user 1) may read.
	e.create(p, human, `{"name": "Secret wait", "triggers": [{"id": "paid", "event": "acme.order.paid.v1"}],
	 "nodes": [
	  {"id": "w", "type": "wait", "params": {"event": "acme.order.created.v1", "filter": "event.secret == 'open'", "timeout": "1h"}},
	  {"id": "note", "type": "acme.note", "params": {"text": "woke"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "paid", "outcome": "fired"}, "to": "w"},
	  {"from": {"node": "w", "outcome": "matched"}, "to": "note"},
	  {"from": {"node": "w", "outcome": "timeout"}, "to": "fin"},
	  {"from": {"node": "note", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}
	 ]}`)
	e.publish(e.tenant, orderPaid, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")
	e.publish(e.tenant, orderCreated, "o-1", order{Secret: "closed"}, 0)
	e.publish(e.tenant, orderCreated, "o-1", order{Secret: "open"}, 0)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the matching event wakes the wait")
	assert.Equal(t, []string{"note:woke"}, e.effects(run.RunID))
}

func TestEachStepUsesTheRunAsRightsOfNow(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	e.create(p, human, `{"name": "Rights", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "p1", "type": "acme.perm", "params": {}}, {"id": "p2", "type": "acme.perm", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "p1"}, {"from": {"node": "p1", "outcome": "done"}, "to": "p2"}, {"from": {"node": "p2", "outcome": "done"}, "to": "fin"}]}`)
	// The secret permission is revoked while the first step commits, within one claim of the instance.
	var revoked atomic.Bool
	p.rt.SetTestHook(func(_ context.Context, _ pgx.Tx, label string) error {
		if label == "step" && !revoked.Swap(true) {
			e.subjects.Grant(1, engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate, permRead)
		}
		return nil
	})
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"p1:has", "p2:lacks"}, e.effects(run.RunID))
}

func TestRestartOfABlockedRunAndRunAsOfTheVersion(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, followUp)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 2}, 2)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")
	// The instance blocks (an ambiguous call, say): a restart starts a new run and leaves it as it is.
	_, err := e.db.Exec(ctx, `UPDATE extruntime.workflow_instances i SET status = 'blocked', blocked_reason = 'ambiguous', lease_owner = NULL, lease_until = NULL
		FROM automation.runs r WHERE r.instance_id = i.id AND r.id = $1`, run.RunID)
	require.NoError(t, err)
	require.Equal(t, engine.RunBlocked, e.status(p, run.RunID))

	// Version 2 runs as the initiator; a human enables it (run_as changed, so the save does not).
	initiator := strings.Replace(followUp, `"name": "Order follow-up",`, `"name": "Order follow-up", "run_as": "initiator",`, 1)
	v2, err := p.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(initiator)})
	require.NoError(t, err)
	_, err = p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: v2.Version})
	require.NoError(t, err)

	ref, err := p.rt.Service().Restart(ctx, human, run.RunID, v2.Version)
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunWaiting }, "the restarted run runs")
	assert.Equal(t, engine.RunBlocked, e.status(p, run.RunID), "the predecessor is untouched")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.runs WHERE id = $1 AND run_as_user_id = 2 AND run_as_kind = 'initiator'`, ref.RunID),
		"the new version's run-as: the original initiator")
}

func TestFailedRunNotifiesTheOwnerOnce(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	am := e.create(p, human, `{"name": "Broken", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "check", "type": "branch", "params": {"expression": "10 / event.amount > 1"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "check"}, {"from": {"node": "check", "outcome": "true"}, "to": "fin"}, {"from": {"node": "check", "outcome": "false"}, "to": "fin"}]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 0}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunFailed }, "the run fails")
	// The notification intent is the outbox event the ownernotice definition delivers to the owner.
	var payload engine.RunFailedPayload
	var raw []byte
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT payload FROM extruntime.outbox_events WHERE event = $1 AND subject_key = $2`,
		engine.EventRunFailed, run.RunID.String()).Scan(&raw))
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Equal(t, int64(1), payload.OwnerUserID)
	assert.Equal(t, am.ID.String(), payload.AutomationID)
	assert.Equal(t, "Broken", payload.AutomationName)
	assert.NotEmpty(t, payload.Code)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1`, engine.EventRunFailed), "once")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.runs WHERE id = $1 AND owner_notified_at IS NOT NULL`, run.RunID))
}
