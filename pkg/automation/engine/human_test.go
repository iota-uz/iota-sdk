package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
)

// humanGraph asks a group for a decision (quorum required) and notes it.
func humanGraph(params string, timeout bool) string {
	late, lateNode := "", ""
	if timeout {
		late = `, {"from": {"node": "ok", "outcome": "timeout"}, "to": "late"},
		 {"from": {"node": "late", "outcome": "done"}, "to": "fin"}, {"from": {"node": "late", "outcome": "failed"}, "to": "fin"}`
		lateNode = `{"id": "late", "type": "acme.note", "params": {"text": "late"}},`
	}
	return graphJSON("Approval", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "ok", "type": "human", "params": `+params+`},
		 {"id": "yes", "type": "acme.note", "params": {"text": "{{ nodes.ok.output.outcome }} {{ string(size(nodes.ok.output.decisions)) }}"}},
		 {"id": "no", "type": "acme.note", "params": {"text": "rejected"}},
		 `+lateNode+`
		 {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "ok"},
		 {"from": {"node": "ok", "outcome": "approve"}, "to": "yes"},
		 {"from": {"node": "ok", "outcome": "reject"}, "to": "no"},
		 {"from": {"node": "yes", "outcome": "done"}, "to": "fin"}, {"from": {"node": "yes", "outcome": "failed"}, "to": "fin"},
		 {"from": {"node": "no", "outcome": "done"}, "to": "fin"}, {"from": {"node": "no", "outcome": "failed"}, "to": "fin"}`+late)
}

const humanButtons = `"buttons": [{"outcome": "approve", "label": "Approve", "style": "primary"}, {"outcome": "reject", "label": "Reject", "style": "danger", "require_comment": true}]`

// failOnce fails the first created change of a task: its transaction rolls back.
type failOnce struct{ failed atomic.Bool }

func (f *failOnce) TaskChanged(_ context.Context, _ pgx.Tx, _ workflow.Task, c workflow.TaskChange) error {
	if c.Kind == workflow.TaskChangeCreated && f.failed.CompareAndSwap(false, true) {
		return errors.New("the change of the task fails once")
	}
	return nil
}

func humanEnv(t *testing.T) (*acmeEnv, *failOnce) {
	t.Helper()
	ae := newAcmeEnv(t)
	aud := fakeAudience{members: map[string][]int64{"group:6f0c1c1e-0000-4000-8000-000000000001": {1, 2, 3}, "role:77": {2, 4}}}
	fail := &failOnce{}
	ae.audience = aud
	ae.observers = []workflow.TaskObserver{engine.NewTaskAssignedObserver(aud, ae.logger), fail}
	for _, u := range []int64{3, 4} {
		ae.subjects.Grant(u, allPerms...)
	}
	return ae, fail
}

func (ae *acmeEnv) openTask(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT id FROM extruntime.human_tasks WHERE status = 'open'`).Scan(&id))
	return id
}

// Quorum 2 of a group of 3: after two equal decisions the task is closed for the third and the run
// goes on by the outcome.
func TestHumanQuorumOfAGroup(t *testing.T) {
	ae, _ := humanEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), humanGraph(`{"title": "Approve order {{ event.amount }}", "assignees": [{"kind": "group", "ref": "6f0c1c1e-0000-4000-8000-000000000001"}], "required": 2, `+humanButtons+`}`, false))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 7}, 0)
	p.until(func() bool { return ae.count(`SELECT count(*) FROM extruntime.human_tasks WHERE status = 'open'`) == 1 }, "the task opens")
	task := ae.openTask(t)
	ctx := context.Background()
	decide := func(user int64, outcome string) error {
		return p.wf.DecideTask(ctx, spiSubject(ae.tenant, user), task, json.RawMessage(`{"outcome": "`+outcome+`"}`))
	}
	require.NoError(t, decide(1, "approve"))
	p.pass()
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM extruntime.human_tasks WHERE status = 'open'`), "one decision is not the quorum")
	require.NoError(t, decide(2, "approve"))
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the quorum moves the run")
	assert.Equal(t, []string{"approve 2"}, noteTexts(t, ae.env))
	require.ErrorIs(t, decide(3, "reject"), workflow.ErrTaskClosed, "the task is closed for the third")
	run := runOf(t, ae.env, p)
	p.until(func() bool { return ae.status(p, run.RunID) == engine.RunDone }, "done")
	var title string
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT title->>'en' FROM extruntime.human_tasks WHERE id = $1`, task).Scan(&title))
	assert.Equal(t, "Approve order 7", title, "the title is rendered")
}

// A human node on a group of 3 publishes one automation.task.assigned.v1 with three users; the
// escalation that adds the managers publishes one with the manager not yet assigned only. The rolled
// back creation leaves no event.
func TestTaskAssignedEvents(t *testing.T) {
	ae, fail := humanEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), humanGraph(`{"title": "Approve", "assignees": [{"kind": "group", "ref": "6f0c1c1e-0000-4000-8000-000000000001"}], `+humanButtons+`,
		"timeout": "2s", "escalation": [{"add": [{"kind": "role", "ref": "77"}], "extend": "1h"}]}`, true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 7}, 0)
	for i := 0; i < 20 && ae.count(`SELECT count(*) FROM extruntime.human_tasks WHERE status = 'open'`) == 0; i++ {
		p.pass()
		time.Sleep(50 * time.Millisecond)
	}
	if ae.count(`SELECT count(*) FROM extruntime.human_tasks WHERE status = 'open'`) == 0 {
		rows, _ := ae.db.Query(context.Background(), `SELECT status || ' ' || blocked_reason || ' ' || blocked_detail || ' ' || coalesce(error::text, '') FROM extruntime.workflow_instances`)
		dbg, _ := pgx.CollectRows(rows, pgx.RowTo[string])
		t.Fatalf("the task did not open; instances: %v", dbg)
	}
	require.True(t, fail.failed.Load(), "the first creation rolled back")
	assigned := func() []engine.TaskAssignedPayload {
		rows, err := ae.db.Query(context.Background(), `SELECT payload FROM extruntime.outbox_events WHERE event = $1 ORDER BY seq`, engine.EventTaskAssigned)
		require.NoError(t, err)
		raws, err := pgx.CollectRows(rows, pgx.RowTo[[]byte])
		require.NoError(t, err)
		out := make([]engine.TaskAssignedPayload, len(raws))
		for i, raw := range raws {
			require.NoError(t, json.Unmarshal(raw, &out[i]))
		}
		return out
	}
	got := assigned()
	require.Len(t, got, 1, "one event: the rolled back creation left none")
	assert.Equal(t, []int64{1, 2, 3}, got[0].UserIDs)
	assert.Equal(t, workflow.TaskChangeCreated, got[0].Reason)
	assert.Equal(t, "ok", got[0].NodeID)
	assert.Equal(t, "Approve", got[0].Title)
	run := runOf(t, ae.env, p)
	assert.Equal(t, run.RunID, got[0].RunID)

	p.until(func() bool { return len(assigned()) == 2 }, "the escalation adds the managers")
	got = assigned()
	assert.Equal(t, workflow.TaskChangeEscalated, got[1].Reason)
	assert.Equal(t, []int64{4}, got[1].UserIDs, "user 2 of the managers is assigned already")
	require.NotNil(t, got[1].DueAt)
	assert.True(t, got[1].DueAt.After(time.Now().Add(30*time.Minute)), "the escalation extended the due time")
}

func TestHumanNodeRefusesBadParams(t *testing.T) {
	_, err := engine.ParseHumanParams(json.RawMessage(`{"title": "x", "assignees": [{"kind": "user", "user_id": 1}], "buttons": [{"outcome": "timeout", "label": "T"}]}`))
	require.NoError(t, err)
	p, _ := engine.ParseHumanParams(json.RawMessage(`{"title": "x", "assignees": [{"kind": "user", "user_id": 1}], "buttons": [{"outcome": "timeout", "label": "T"}]}`))
	_, err = p.TaskSpec(time.Hour, engine.SubjectRef{})
	require.Error(t, err, "timeout is not a button")
	p, _ = engine.ParseHumanParams(json.RawMessage(`{"title": "x", "assignees": [{"kind": "user", "user_id": 1}], "buttons": [{"outcome": "ok", "label": "OK"}], "escalation": [{"extend": "1h"}]}`))
	_, err = p.TaskSpec(time.Hour, engine.SubjectRef{})
	require.Error(t, err, "escalation needs a timeout")
	p, _ = engine.ParseHumanParams(json.RawMessage(`{"title": "x", "assignees": [{"kind": "user", "user_id": 1}], "buttons": [{"outcome": "ok", "label": "OK"}], "timeout": "90d"}`))
	spec, err := p.TaskSpec(time.Hour, engine.SubjectRef{})
	require.NoError(t, err)
	assert.Equal(t, time.Hour, spec.Timeout, "capped by WaitMax")
}
