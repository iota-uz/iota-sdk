package engine_test

// Regression tests of the follow-up to the engine's review (#1395).

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// bigNode writes a row and answers with an output of size bytes.
type bigNode struct{ specNode }

func (bigNode) ApplyEffect(ctx context.Context, tx pgx.Tx, run engine.StepContext, node graph.Node, e graph.Effect) (map[string]any, error) {
	if _, err := tx.Exec(ctx, `INSERT INTO test_effects (run_id, node_id, text) VALUES ($1, $2, 'wrote')`, run.RunID, node.ID); err != nil {
		return nil, err
	}
	size, _ := e.Params["size"].(json.Number)
	n, _ := size.Int64()
	return map[string]any{"blob": strings.Repeat("x", int(n))}, nil
}

func TestOversizedEffectOutputRollsTheEffectBack(t *testing.T) {
	e := newEnv(t)
	require.NoError(t, e.reg.RegisterNode(bigNode{specNode{graph.NodeSpec{Type: "acme.big", Title: "Big", Behavior: graph.BehaviorEffect,
		FailedOutcome: "failed", Outcomes: []string{"done", "failed"},
		Params: graph.MustSchemaParams(`{"type":"object","properties":{"size":{"type":"integer"}},"required":["size"],"additionalProperties":false}`)}}}))
	p := e.proc(func(c *engine.Config) { c.Limits.PayloadBytes = 4096 })
	a := e.human()
	e.create(p, a, `{"name": "Big", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [{"id": "big", "type": "acme.big", "params": {"size": 8192}},
	  {"id": "after", "type": "acme.note", "params": {"text": "after failed"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "big"}, {"from": {"node": "big", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "big", "outcome": "failed"}, "to": "after"},
	  {"from": {"node": "after", "outcome": "done"}, "to": "fin"}, {"from": {"node": "after", "outcome": "failed"}, "to": "fin"}]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	assert.Equal(t, []string{"after:after failed"}, e.effects(run.RunID), "the failed node's write is rolled back before its failed edge")
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.steps WHERE run_id = $1 AND node_id = 'big' AND outcome = 'failed' AND error->>'code' = $2`,
		run.RunID, engine.CodeLimitExceeded))
}

func TestForeachKeepsTheDigitsOfLargeIntegers(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	a := e.human()
	e.create(p, a, `{"name": "Ids", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
	 "nodes": [
	  {"id": "each", "type": "foreach", "params": {"items": "[9007199254740993, 9007199254740995, 9007199254740997]", "concurrency": 1}},
	  {"id": "note", "type": "acme.note", "params": {"text": "{{ foreach.each.item }}"}},
	  {"id": "take", "type": "collect", "params": {"of": "each", "value": "foreach.each.item"}},
	  {"id": "fin", "type": "end", "params": {}}
	 ],
	 "edges": [
	  {"from": {"node": "created", "outcome": "fired"}, "to": "each"},
	  {"from": {"node": "each", "outcome": "item"}, "to": "note"},
	  {"from": {"node": "note", "outcome": "done"}, "to": "take"},
	  {"from": {"node": "note", "outcome": "failed"}, "to": "take"},
	  {"from": {"node": "each", "outcome": "done"}, "to": "fin"},
	  {"from": {"node": "each", "outcome": "partial_failed"}, "to": "fin"},
	  {"from": {"node": "each", "outcome": "failed"}, "to": "fin"}
	 ]}`)
	e.publish(e.tenant, orderCreated, "o-1", order{}, 0)
	run := onlyRun(t, e, p, a)
	p.until(func() bool { return e.status(p, run.RunID) == engine.RunDone }, "the run ends")
	// One at a time: the first item starts with the foreach, the others are read back from the token.
	assert.Equal(t, []string{"note:9007199254740993", "note:9007199254740995", "note:9007199254740997"}, e.effects(run.RunID))
	var out []byte
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT output FROM automation.steps WHERE run_id = $1 AND node_id = 'each' AND outcome = 'done'`, run.RunID).Scan(&out))
	var got struct {
		Results []struct {
			Value json.Number `json:"value"`
		} `json:"results"`
	}
	require.NoError(t, expr.UnmarshalJSON(out, &got))
	require.Len(t, got.Results, 3)
	for i, want := range []string{"9007199254740993", "9007199254740995", "9007199254740997"} {
		assert.Equal(t, want, got.Results[i].Value.String(), "collected value %d", i)
	}
}

func TestWaitDeadlineCountsFromAfterTheSubjectLock(t *testing.T) {
	e := newEnv(t)
	p := e.proc()
	ctx := context.Background()
	a := e.human()
	am := e.create(p, a, `{"name": "Pay", "triggers": [{"id": "go", "event": "automation.manual.v1"}],
	 "nodes": [{"id": "pay", "type": "wait", "params": {"event": "acme.order.paid.v1", "timeout": "1s"}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "go", "outcome": "fired"}, "to": "pay"}, {"from": {"node": "pay", "outcome": "matched"}, "to": "fin"},
	  {"from": {"node": "pay", "outcome": "timeout"}, "to": "fin"}]}`)
	ref, err := p.rt.Service().StartManual(ctx, a, am.ID, engine.ManualStart{TriggerID: "go", Input: json.RawMessage(`{}`)})
	require.NoError(t, err)
	// Another transaction holds the run's subject while its first step begins.
	holder, err := e.db.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = holder.Rollback(ctx) })
	_, err = holder.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, engine.SubjectLockKey(e.tenant, engine.SubjectRef{Entity: graph.SubjectNone}, ref.RunID))
	require.NoError(t, err)
	// The step runs while the holder keeps the lock; the holder lets go once the step waits for it.
	ran := make(chan error, 1)
	go func() {
		_, err := p.wf.RunOnce(ctx) // the step waits for the subject, then opens the wait
		ran <- err
	}()
	require.Eventually(t, func() bool {
		return e.count(`SELECT count(*) FROM pg_locks l JOIN pg_database d ON d.oid = l.database
			WHERE d.datname = current_database() AND l.locktype = 'advisory' AND NOT l.granted`) > 0
	}, 10*time.Second, 20*time.Millisecond, "the step waits for the subject's lock")
	time.Sleep(time.Second) // the lock is contended for a while: a transaction-start clock would lag behind
	var at time.Time
	require.NoError(t, holder.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&at))
	require.NoError(t, holder.Commit(ctx))
	require.NoError(t, <-ran)
	deadline := e.deadline(e.db)
	assert.False(t, deadline.Before(at.Add(time.Second)), "the wait gets its whole second after the lock: deadline %s, lock released %s", deadline, at)
}
