package engine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
)

func noteGraph(text string) string {
	return `{"name": "Note", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "note", "type": "acme.note", "params": {"text": "` + text + `"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "note"},
	  {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}]}`
}

func codeGraph(src string) string {
	return `{"name": "Code", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "code", "type": "acme.code", "params": {"source": "` + src + `"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "code"}, {"from": {"node": "code", "outcome": "done"}, "to": "fin"}]}`
}

func TestAgentVersionDoesNotRunUntilAHumanEnablesIt(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	agent := e.actor(e.tenant, 1, engine.OriginAgent)
	am := e.create(p, human, noteGraph("v1"))

	v2, err := p.rt.Service().SaveVersion(ctx, agent, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(noteGraph("v2"))})
	require.NoError(t, err)
	got, err := p.rt.Service().Get(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, got.LatestVersion)
	require.NotNil(t, got.EnabledVersion)
	assert.Equal(t, 1, *got.EnabledVersion, "an agent's version does not become executable")

	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"note:v1"}, e.effects(run.RunID), "the enabled version runs")
	assert.Equal(t, 1, run.Version)

	_, err = p.rt.Service().Enable(ctx, agent, am.ID, engine.EnableInput{Version: v2.Version})
	require.ErrorIs(t, err, engine.ErrHumanRequired)

	// A stale base is a conflict that names who saved the latest version.
	_, err = p.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(noteGraph("v3"))})
	var conflict *engine.ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, 2, conflict.Latest)
	require.ErrorIs(t, err, engine.ErrVersionConflict)

	// A human who may enable switches the running graph when code and run-as are unchanged.
	v3, err := p.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 2, Graph: json.RawMessage(noteGraph("v3"))})
	require.NoError(t, err)
	got, err = p.rt.Service().Get(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, v3.Version, *got.EnabledVersion)
	versions, err := p.rt.Service().ListVersions(ctx, human, am.ID)
	require.NoError(t, err)
	require.Len(t, versions, 3)
	assert.True(t, versions[0].Enabled)
	assert.Equal(t, engine.OriginAgent, versions[1].SavedOrigin)
}

func TestEnableNeedsTheReviewedCode(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am, v1, err := p.rt.Service().Create(ctx, human, engine.CreateInput{Graph: json.RawMessage(codeGraph("return 1"))})
	require.NoError(t, err)
	rv, err := p.rt.Service().EnableReview(ctx, human, am.ID, v1.Version)
	require.NoError(t, err)
	require.Len(t, rv.CodeNodes, 1)
	assert.Equal(t, "return 1", rv.CodeNodes[0].Source)
	assert.Equal(t, []string{"TOKEN"}, rv.CodeNodes[0].Secrets)
	assert.NotEmpty(t, rv.CodeDigest)
	assert.Equal(t, "owner", rv.RunAs)
	require.Len(t, rv.Triggers, 1)
	assert.Equal(t, engine.KindEvent, rv.Triggers[0].Kind)

	_, err = p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: 1})
	require.ErrorIs(t, err, engine.ErrReviewRequired, "the code was not shown")

	// The code changes after the review: the old digest does not enable the new version.
	v2, err := p.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(codeGraph("return 2"))})
	require.NoError(t, err)
	_, err = p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: v2.Version, CodeDigest: rv.CodeDigest})
	require.ErrorIs(t, err, engine.ErrReviewRequired)
	rv2, err := p.rt.Service().EnableReview(ctx, human, am.ID, v2.Version)
	require.NoError(t, err)
	got, err := p.rt.Service().Enable(ctx, human, am.ID, engine.EnableInput{Version: v2.Version, CodeDigest: rv2.CodeDigest})
	require.NoError(t, err)
	assert.Equal(t, 2, *got.EnabledVersion)
	assert.Equal(t, int64(1), *got.OwnerUserID)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.trigger_owners WHERE trigger_id = $1 AND user_id = 1`, "automation:"+am.ID.String()))

	// A human's save of changed code on an enabled graph keeps the old version running.
	_, err = p.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: 2, Graph: json.RawMessage(codeGraph("return 3"))})
	require.NoError(t, err)
	got, err = p.rt.Service().Get(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, *got.EnabledVersion)
}

func TestEnableStartsNoRunForEarlierEvents(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	am := e.create(p, human, noteGraph("x"))
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Empty(t, e.runs(p, human, engine.RunFilter{}), "an event before the enable starts nothing")
	e.publish(e.tenant, orderCreated, "o-2", order{}, 0)
	p.until(func() bool { return len(e.runs(p, human, engine.RunFilter{})) == 1 }, "a later event starts a run")

	// Events between a disable and the next enable are not replayed.
	_, err := p.rt.Service().Disable(context.Background(), human, am.ID)
	require.NoError(t, err)
	e.publish(e.tenant, orderCreated, "o-3", order{}, 0)
	p.pass()
	_, err = p.rt.Service().Enable(context.Background(), human, am.ID, engine.EnableInput{Version: 1})
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		p.pass()
	}
	assert.Len(t, e.runs(p, human, engine.RunFilter{}), 1)
}

func TestValidationAndPermissions(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	_, _, err := p.rt.Service().Create(ctx, human, engine.CreateInput{Graph: json.RawMessage(`{"name": "Bad", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	  "nodes": [{"id": "x", "type": "acme.missing", "params": {}}], "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "x"}]}`)})
	var verr *engine.ValidationError
	require.ErrorAs(t, err, &verr)
	assert.NotEmpty(t, verr.Errors)
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.automations`), "nothing is written")

	reader := e.actor(e.tenant, 1, engine.OriginHuman, engine.PermRead)
	_, _, err = p.rt.Service().Create(ctx, reader, engine.CreateInput{Graph: json.RawMessage(noteGraph("x"))})
	require.ErrorIs(t, err, engine.ErrForbidden)

	am := e.create(p, human, noteGraph("x"))
	other := e.actor(e.newTenant(), 1, engine.OriginHuman)
	_, err = p.rt.Service().Get(ctx, other, am.ID)
	require.ErrorIs(t, err, engine.ErrNotFound, "another tenant's graph is not found")

	// An author who may not read the trigger's event cannot enable it.
	blind := e.actor(e.tenant, 2, engine.OriginHuman, engine.PermRead, engine.PermAuthor, engine.PermEnable)
	_, err = p.rt.Service().Enable(ctx, blind, am.ID, engine.EnableInput{Version: 1})
	require.ErrorIs(t, err, engine.ErrForbidden)

	// Layout: merged, and only for the latest version's ids.
	require.NoError(t, p.rt.Service().SaveLayout(ctx, human, am.ID, graph.Layout{"note": {X: 1, Y: 2}, "gone": {X: 3, Y: 4}}))
	got, err := p.rt.Service().Get(ctx, human, am.ID)
	require.NoError(t, err)
	assert.Equal(t, graph.Layout{"note": {X: 1, Y: 2}}, got.Layout)

	require.NoError(t, p.rt.Service().Delete(ctx, human, am.ID))
	_, err = p.rt.Service().Get(ctx, human, am.ID)
	require.ErrorIs(t, err, engine.ErrNotFound)
	page, err := p.rt.Service().List(ctx, human, engine.ListFilter{})
	require.NoError(t, err)
	assert.Empty(t, page.Items)
}

func TestDisableCancelsWaitingRunsAndRestart(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, followUp)
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 9}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunWaiting }, "the run waits")

	_, err := p.rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunCancelled }, "disable cancels the waiting run")
	require.ErrorIs(t, p.rt.Service().Cancel(ctx, human, run.RunID), engine.ErrRunFinished)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM extruntime.outbox_events WHERE event = $1 AND payload->>'status' = 'cancelled'`, engine.EventRunFinished))

	// A restart runs the same input again on the chosen version.
	ref, err := p.rt.Service().Restart(ctx, human, run.RunID, 1)
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunWaiting }, "the restarted run waits")
	assert.Equal(t, []string{"note:order 9"}, e.effects(ref.RunID))
	require.NoError(t, p.rt.Service().Cancel(ctx, human, ref.RunID))
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunCancelled }, "cancel terminates the run")
	_, err = p.rt.Service().Restart(ctx, human, uuid.New(), 1)
	assert.ErrorIs(t, err, engine.ErrNotFound)
}

func TestDepthLimitBlocksTheCascade(t *testing.T) {
	e := newEnv(t)
	p := e.proc(func(c *engine.Config) { c.Limits.MaxDepth = 2 })
	human := e.human()
	// Each run publishes the event that starts it (include_self: a graph does not start on its own
	// events otherwise).
	e.create(p, human, `{"name": "Loop", "triggers": [{"id": "created", "event": "acme.order.created.v1", "include_self": true}],
	 "nodes": [{"id": "emit", "type": "acme.emit", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "emit"}, {"from": {"node": "emit", "outcome": "done"}, "to": "fin"}]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool {
		return e.count(`SELECT count(*) FROM automation.runs WHERE start_state = 'blocked' AND start_reason = 'depth_exceeded'`) == 1
	}, "the cascade stops at the depth limit")
	assert.Equal(t, 4, e.count(`SELECT count(*) FROM automation.runs`), "depths 0, 1, 2 run; 3 is blocked")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.runs WHERE depth = 3 AND cause_run_id IS NOT NULL`))
	blocked := e.runs(p, human, engine.RunFilter{Status: []engine.RunStatus{engine.RunBlocked}})
	require.Len(t, blocked, 1)
	assert.Equal(t, "depth_exceeded", blocked[0].Reason)
}

func TestNoOwnerIsBlocked(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	am := e.create(p, human, noteGraph("x"))
	_, err := e.db.Exec(context.Background(), `DELETE FROM extruntime.trigger_owners WHERE trigger_id = $1`, "automation:"+am.ID.String())
	require.NoError(t, err)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	assert.Equal(t, engine.RunBlocked, run.Status)
	assert.Equal(t, "no_owner", run.Reason)
}

func TestSensitivePayloadIsProjectedForTheRunAs(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	// User 2 may read orders but not their secret, and owns the graph.
	e.subjects.Grant(2, engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate, permRead)
	owner := e.actor(e.tenant, 2, engine.OriginHuman, engine.PermRead, engine.PermAuthor, engine.PermEnable, engine.PermOperate, permRead)
	e.create(p, owner, noteGraph("x"))
	e.publish(e.tenant, orderCreated, "o-1", order{Amount: 4, Secret: "s3cret"}, 0)
	run := onlyRun(t, e, p, owner)
	var input string
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT input::text FROM automation.runs WHERE id = $1`, run.RunID).Scan(&input))
	assert.NotContains(t, input, "s3cret")
	assert.Contains(t, input, `"amount": 4`)
}

func TestConversationRunsQueue(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Chat", "triggers": [{"id": "go", "event": "automation.manual.v1", "params": {"input_schema": {"type": "object", "properties": {"n": {"type": "integer"}}, "required": ["n"]}}}],
	 "nodes": [{"id": "nap", "type": "delay", "params": {"duration": "1s"}}, {"id": "note", "type": "acme.note", "params": {"text": "n={{ event.input.n }}"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "nap"}, {"from": {"node": "nap", "outcome": "done"}, "to": "note"},
	  {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}]}`)
	chat := &engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"}
	_, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`), Subject: chat})
	require.ErrorIs(t, err, engine.ErrInvalidInput, "the input schema is checked")
	first, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{"n": 1}`), Subject: chat})
	require.NoError(t, err)
	assert.Equal(t, engine.RunRunning, first.Status)
	second, err := p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{"n": 2}`), Subject: chat})
	require.NoError(t, err)
	assert.Equal(t, engine.RunQueued, second.Status, "the conversation is busy")
	p.until(func() bool { return e.status(p, second.RunID) == engine.RunDone }, "the queued run starts after the first")
	assert.Equal(t, engine.RunDone, e.status(p, first.RunID))
	assert.Equal(t, []string{"note:n=2"}, e.effects(second.RunID))
	var firstEnd, secondStart time.Time
	require.NoError(t, e.db.QueryRow(ctx, `SELECT max(ended_at) FROM automation.steps WHERE run_id = $1`, first.RunID).Scan(&firstEnd))
	require.NoError(t, e.db.QueryRow(ctx, `SELECT min(started_at) FROM automation.steps WHERE run_id = $1`, second.RunID).Scan(&secondStart))
	assert.False(t, secondStart.Before(firstEnd), "the runs of one conversation do not overlap")

	_, err = p.rt.Service().Disable(ctx, human, am.ID)
	require.NoError(t, err)
	_, err = p.rt.Service().StartManual(ctx, human, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{"n": 3}`)})
	assert.ErrorIs(t, err, engine.ErrNotEnabled)
}

func TestScheduleSlotStartsOneRun(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, `{"name": "Nightly", "triggers": [{"id": "night", "event": "automation.schedule.v1", "params": {"cron": "0 3 * * *", "timezone": "Asia/Tashkent"}}],
	 "nodes": [{"id": "note", "type": "acme.note", "params": {"text": "at {{ event.scheduled_at }}"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "night", "outcome": "fired"}, "to": "note"}, {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}]}`)
	defs, err := p.rt.Schedules().ActiveSchedules(ctx)
	require.NoError(t, err)
	require.Len(t, defs, 1)
	def := defs[0]
	assert.Equal(t, "automation:"+am.ID.String()+":night", def.ID)
	assert.Equal(t, "0 3 * * *", def.Cron)
	assert.Equal(t, "Asia/Tashkent", def.Timezone)
	require.NotNil(t, def.TenantID)
	// The next 03:00 in Tashkent after now: a slot after the enable.
	tashkent, err := time.LoadLocation("Asia/Tashkent")
	require.NoError(t, err)
	local := time.Now().In(tashkent)
	tick := time.Date(local.Year(), local.Month(), local.Day(), 3, 0, 0, 0, tashkent)
	if !tick.After(local) {
		tick = tick.AddDate(0, 0, 1)
	}
	slot := events.Slot{ScheduleID: def.ID, TenantID: *def.TenantID, At: tick.UTC()}
	for i := 0; i < 2; i++ {
		require.NoError(t, pgx.BeginFunc(ctx, e.db, func(tx pgx.Tx) error { return def.Handler.RunSlot(ctx, tx, slot) }))
	}
	run := onlyRun(t, e, p, human)
	assert.Equal(t, engine.KindSchedule, run.TriggerKind)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the scheduled run ends")
	assert.Equal(t, []string{"note:at " + tick.UTC().Format(time.RFC3339)}, e.effects(run.RunID))
}

func TestForeachAndParallelAll(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	e.create(p, human, `{"name": "Fan", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [
	  {"id": "each", "type": "foreach", "params": {"items": "[1, 2, 0, 4]", "concurrency": 2}},
	  {"id": "check", "type": "branch", "params": {"expression": "10 / foreach.each.item > 1"}},
	  {"id": "take", "type": "collect", "params": {"of": "each", "value": "foreach.each.item * 10"}},
	  {"id": "split", "type": "parallel", "params": {"branches": ["a", "b"]}},
	  {"id": "na", "type": "acme.note", "params": {"text": "a"}},
	  {"id": "nb", "type": "acme.note", "params": {"text": "b"}},
	  {"id": "all", "type": "join", "params": {"of": "split", "mode": "all"}},
	  {"id": "sum", "type": "acme.note", "params": {"text": "{{ nodes.each.output.results.size() }}:{{ nodes.na.output.text }}{{ nodes.nb.output.text }}"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "created", "outcome": "fired"}, "to": "each"},
	  {"from": {"node": "each", "outcome": "item"}, "to": "check"},
	  {"from": {"node": "check", "outcome": "true"}, "to": "take"},
	  {"from": {"node": "check", "outcome": "false"}, "to": "take"},
	  {"from": {"node": "each", "outcome": "done"}, "to": "split"},
	  {"from": {"node": "each", "outcome": "partial_failed"}, "to": "split"},
	  {"from": {"node": "each", "outcome": "failed"}, "to": "fin"},
	  {"from": {"node": "split", "outcome": "a"}, "to": "na"},
	  {"from": {"node": "split", "outcome": "b"}, "to": "nb"},
	  {"from": {"node": "na", "outcome": "done"}, "to": "all"},
	  {"from": {"node": "na", "outcome": "failed"}, "to": "all"},
	  {"from": {"node": "nb", "outcome": "done"}, "to": "all"},
	  {"from": {"node": "nb", "outcome": "failed"}, "to": "all"},
	  {"from": {"node": "all", "outcome": "joined"}, "to": "sum"},
	  {"from": {"node": "sum", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "sum", "outcome": "failed"}, "to": "fin"}
	 ]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	effects := e.effects(run.RunID)
	assert.Contains(t, effects, "sum:4:ab", "the join merges both branches")
	d := e.run(p, human, run.RunID)
	var each engine.StepView
	for _, s := range d.Steps {
		if s.NodeID == "each" && s.Outcome == "partial_failed" {
			each = s
		}
	}
	require.NotEmpty(t, each.Output, "the item that failed (10 / 0) makes the foreach partial_failed")
	var out struct {
		Results []struct {
			Index int  `json:"index"`
			OK    bool `json:"ok"`
			Value any  `json:"value"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(each.Output, &out))
	require.Len(t, out.Results, 4)
	for i, r := range out.Results {
		assert.Equal(t, i, r.Index, "results in the order of the items")
		assert.Equal(t, i != 2, r.OK)
	}
	assert.EqualValues(t, 40, out.Results[3].Value)
}

func TestUsageIndexAndConcurrency(t *testing.T) {
	e := newEnv(t)
	p := e.proc(func(c *engine.Config) { c.Limits.Concurrency = 7 })
	assert.Equal(t, 7, p.rt.Natives()[engine.WorkflowID].Concurrency, "automation.run has its own slots")
	uses, err := p.rt.UsageIndex().Uses(context.Background(), e.tenant, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, uses)
}

func TestStatusOf(t *testing.T) {
	cases := []struct {
		name, instance, start string
		want                  engine.RunStatus
	}{
		{"completed instance is done", "completed", "", engine.RunDone},
		{"terminated instance is cancelled", "terminated", "", engine.RunCancelled},
		{"failed instance is failed", "failed", "", engine.RunFailed},
		{"waiting instance is waiting", "waiting", "", engine.RunWaiting},
		{"blocked instance is blocked", "blocked", "", engine.RunBlocked},
		{"queued run without instance", "", "queued", engine.RunQueued},
		{"filter_failed run without instance", "", "filter_failed", engine.RunFilterFailed},
		{"blocked run without instance", "", "blocked", engine.RunBlocked},
		{"cancelled run without instance", "", "cancelled", engine.RunCancelled},
		{"the instance wins over a stored start state", "completed", "blocked", engine.RunDone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, engine.StatusOf(c.instance, c.start))
		})
	}
}

func TestConflictErrorIsAVersionConflict(t *testing.T) {
	err := &engine.ConflictError{Latest: 2}
	require.ErrorIs(t, err, engine.ErrVersionConflict)
}

func TestValidationErrorNamesItsCode(t *testing.T) {
	assert.True(t, strings.HasPrefix((&engine.ValidationError{}).Error(), "automation: VALIDATION_FAILED"))
}

func TestMissingContextInABranchStopsTheRun(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	human := e.human()
	// Branch a reads what branch b writes before b ran: missing context ends the run, not the branch,
	// although the node has a failed outcome.
	e.create(p, human, `{"name": "Early", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [
	  {"id": "split", "type": "parallel", "params": {"branches": ["a", "b"]}},
	  {"id": "na", "type": "acme.note", "params": {"text": "{{ nodes.nb.output.text }}"}},
	  {"id": "nb", "type": "acme.note", "params": {"text": "b"}},
	  {"id": "all", "type": "join", "params": {"of": "split", "mode": "all"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "created", "outcome": "fired"}, "to": "split"},
	  {"from": {"node": "split", "outcome": "a"}, "to": "na"},
	  {"from": {"node": "split", "outcome": "b"}, "to": "nb"},
	  {"from": {"node": "na", "outcome": "done"}, "to": "all"},
	  {"from": {"node": "na", "outcome": "failed"}, "to": "all"},
	  {"from": {"node": "nb", "outcome": "done"}, "to": "all"},
	  {"from": {"node": "nb", "outcome": "failed"}, "to": "all"},
	  {"from": {"node": "all", "outcome": "joined"}, "to": "fin"}
	 ]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunFailed }, "missing context fails the run")
	d := e.run(p, human, run.RunID)
	require.NotNil(t, d.Error)
	assert.Equal(t, engine.CodeMissingContext, d.Error.Code)
	assert.Empty(t, e.effects(run.RunID), "branch b never ran its note")
}

func TestTokenLimitFailsTheRunWhateverTheOutcomes(t *testing.T) {
	e := newEnv(t)
	p := e.proc(func(c *engine.Config) { c.Limits.MaxTokens = 2 })
	human := e.human()
	e.create(p, human, `{"name": "Wide", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [
	  {"id": "each", "type": "foreach", "params": {"items": "[1, 2, 3]", "concurrency": 3}},
	  {"id": "take", "type": "collect", "params": {"of": "each"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "created", "outcome": "fired"}, "to": "each"},
	  {"from": {"node": "each", "outcome": "item"}, "to": "take"},
	  {"from": {"node": "each", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "each", "outcome": "partial_failed"}, "to": "fin"},
	  {"from": {"node": "each", "outcome": "failed"}, "to": "fin"}
	 ]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunFailed }, "the token limit fails the run")
	assert.Equal(t, engine.CodeLimitExceeded, e.run(p, human, run.RunID).Error.Code)
}

func TestRestartRunsOnlyEnabledCode(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	agent := e.actor(e.tenant, 1, engine.OriginAgent)
	am := e.create(p, human, noteGraph("v1"))
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, human)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	v2, err := p.rt.Service().SaveVersion(ctx, agent, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(noteGraph("v2"))})
	require.NoError(t, err)
	_, err = p.rt.Service().Restart(ctx, agent, run.RunID, v2.Version)
	require.ErrorIs(t, err, engine.ErrForbidden, "a version nobody enabled does not run through a restart")
	ref, err := p.rt.Service().Restart(ctx, agent, run.RunID, 1)
	require.NoError(t, err)
	p.until(func() bool { return e.status(p, ref.RunID) == engine.RunDone }, "the restart on the enabled version runs")
	assert.Equal(t, []string{"note:v1"}, e.effects(ref.RunID))
}

// A call joined to the caller's transaction (WithTx) reads what the transaction wrote, and leaves the
// transaction to the caller.
func TestJoinedCallsReadTheCallersTransaction(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	tx, err := e.db.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	joined := engine.WithTx(ctx, tx)
	am, _, err := p.rt.Service().Create(joined, e.human(), engine.CreateInput{Graph: json.RawMessage(noteGraph("v1"))})
	require.NoError(t, err)
	versions, err := p.rt.Service().ListVersions(joined, e.human(), am.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 1)
	page, err := p.rt.Service().List(joined, e.human(), engine.ListFilter{})
	require.NoError(t, err)
	assert.Len(t, page.Items, 1)
	require.NoError(t, tx.Rollback(ctx))
	_, err = p.rt.Service().Get(ctx, e.human(), am.ID)
	require.ErrorIs(t, err, engine.ErrNotFound, "rolled back with the caller's transaction")
}

// ListVersions in a caller's READ COMMITTED transaction, while a person saves versions that become the
// enabled one, lists exactly one enabled version, the latest it lists: the versions and the enabled one
// are one snapshot (two statements would see a version enabled after the first one read the list).
func TestListVersionsIsOneSnapshotInReadCommitted(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	human := e.human()
	am := e.create(p, human, noteGraph("v1"))

	const saves = 25
	done := make(chan error, 1)
	go func() {
		for v := 1; v <= saves; v++ {
			if _, err := p.rt.Service().SaveVersion(ctx, human, am.ID, engine.SaveInput{BaseVersion: v, Graph: json.RawMessage(noteGraph(fmt.Sprintf("v%d", v+1)))}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	tx, err := e.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	joined := engine.WithTx(ctx, tx)
	for reads, finished := 0, false; !finished || reads < 50; reads++ {
		select {
		case err := <-done:
			require.NoError(t, err)
			finished = true
		default:
		}
		versions, err := p.rt.Service().ListVersions(joined, human, am.ID)
		require.NoError(t, err)
		enabled := 0
		for _, v := range versions {
			if v.Enabled {
				enabled++
			}
		}
		require.Equal(t, 1, enabled, "read %d: one enabled version in %+v", reads, versions)
		require.True(t, versions[0].Enabled, "read %d: the latest listed version is the enabled one", reads)
	}
	versions, err := p.rt.Service().ListVersions(joined, human, am.ID)
	require.NoError(t, err)
	assert.Len(t, versions, saves+1)
}
