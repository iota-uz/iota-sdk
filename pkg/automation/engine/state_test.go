package engine_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/builtin/setstate"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/state"
)

// stateGraph remembers the note of an order: the first order with a note takes the "first" branch and
// writes the key, the next one with the same note finds it and takes "dup".
const stateGraph = `{"name": "Once per chat", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
 "nodes": [
  {"id": "seen", "type": "branch", "params": {"expression": "state.has(\"chat:\" + event.note)"}},
  {"id": "mark", "type": "set_state", "params": {"key": "chat:{{ event.note }}", "value": "event.amount", "ttl": "1h"}},
  {"id": "first", "type": "acme.note", "params": {"text": "first"}},
  {"id": "dup", "type": "acme.note", "params": {"text": "dup {{ state.get(\"chat:\" + event.note) }}"}},
  {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.mark.output.error.code }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "seen"},
  {"from": {"node": "seen", "outcome": "false"}, "to": "mark"},
  {"from": {"node": "seen", "outcome": "true"}, "to": "dup"},
  {"from": {"node": "mark", "outcome": "set"}, "to": "first"},
  {"from": {"node": "mark", "outcome": "failed"}, "to": "bad"},
  {"from": {"node": "first", "outcome": "done"}, "to": "fin"}, {"from": {"node": "first", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "dup", "outcome": "done"}, "to": "fin"}, {"from": {"node": "dup", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}
 ]}`

func newStateEnv(t *testing.T) (*env, *proc) {
	t.Helper()
	e := newEnv(t)
	require.NoError(t, e.reg.RegisterNode(setstate.New()))
	return e, e.proc()
}

// order fires a run for a note and returns its effects once it ends.
func (e *env) orderEffects(p *proc, a engine.Actor, key, note string, amount int) []string {
	e.t.Helper()
	before := len(e.runs(p, a, engine.RunFilter{}))
	e.publish(e.tenant, orderCreated, key, order{Amount: amount, Note: note}, 0)
	var run engine.RunSummary
	p.until(func() bool {
		runs := e.runs(p, a, engine.RunFilter{})
		if len(runs) <= before {
			return false
		}
		for _, r := range runs {
			if r.Status != engine.RunDone && r.Status != engine.RunFailed {
				return false
			}
		}
		run = runs[0]
		return true
	}, "the run ends")
	return e.effects(run.RunID)
}

func TestSetStateIsSeenByTheNextRun(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	am := e.create(p, a, stateGraph)

	assert.Equal(t, []string{"first:first"}, e.orderEffects(p, a, "o-1", "42", 7))
	// The second run on the same trigger finds the key and reads its value.
	assert.Equal(t, []string{"dup:dup 7"}, e.orderEffects(p, a, "o-2", "42", 9))
	// Another key is another memory.
	assert.Equal(t, []string{"first:first"}, e.orderEffects(p, a, "o-3", "43", 1))

	var by *uuid.UUID
	var value string
	require.NoError(t, e.db.QueryRow(context.Background(),
		`SELECT value::text, updated_by_run FROM automation.state WHERE tenant_id = $1 AND automation_id = $2 AND key = 'chat:42'`, e.tenant, am.ID).Scan(&value, &by))
	assert.Equal(t, "7", value, "the second run wrote nothing")
	assert.NotNil(t, by, "the run that wrote the key")
}

func TestSetStateKeyExpires(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	e.create(p, a, stateGraph)
	assert.Equal(t, []string{"first:first"}, e.orderEffects(p, a, "o-1", "42", 7))

	// The ttl passes: the key is not seen even before the sweep removes its row.
	_, err := e.db.Exec(context.Background(), `UPDATE automation.state SET expires_at = now() - interval '1 second'`)
	require.NoError(t, err)
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.state`))
	assert.Equal(t, []string{"first:first"}, e.orderEffects(p, a, "o-2", "42", 8))

	// The sweep removes an expired row and keeps a live one.
	_, err = e.db.Exec(context.Background(), `INSERT INTO automation.state (tenant_id, automation_id, key, value, expires_at)
		SELECT tenant_id, automation_id, 'old', '1', now() - interval '1 hour' FROM automation.state LIMIT 1`)
	require.NoError(t, err)
	_, err = p.rt.Prune(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.state WHERE key = 'old'`))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.state WHERE key = 'chat:42'`))
}

func TestSetStateValueTooLargeFailsTheNode(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	am := e.create(p, a, stateGraph)
	// The value is the whole note: over 16 KiB.
	g := strings.Replace(stateGraph, `"value": "event.amount"`, `"value": "event.note"`, 1)
	g = strings.Replace(g, `"key": "chat:{{ event.note }}"`, `"key": "chat:big"`, 1)
	_, err := p.rt.Service().SaveVersion(context.Background(), a, am.ID, engine.SaveInput{BaseVersion: 1, Graph: json.RawMessage(g)})
	require.NoError(t, err)
	enableLatest(t, p, a, am.ID)

	big := strings.Repeat("x", state.MaxValueBytes)
	assert.Equal(t, []string{"bad:" + state.CodeValueTooLarge}, e.orderEffects(p, a, "o-1", big, 1))
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.state`), "a failed node writes nothing")
}

func enableLatest(t *testing.T, p *proc, a engine.Actor, id uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	am, err := p.rt.Service().Get(ctx, a, id)
	require.NoError(t, err)
	rv, err := p.rt.Service().EnableReview(ctx, a, id, am.LatestVersion)
	require.NoError(t, err)
	_, err = p.rt.Service().Enable(ctx, a, id, engine.EnableInput{Version: am.LatestVersion, CodeDigest: rv.CodeDigest})
	require.NoError(t, err)
}

func TestStateIsPerTenant(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	e.create(p, a, stateGraph)
	assert.Equal(t, []string{"first:first"}, e.orderEffects(p, a, "o-1", "42", 7))

	// The same key of another tenant's automation is not there, and the key of this tenant stays hidden from it.
	other := e.newTenant()
	var id uuid.UUID
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT automation_id FROM automation.state LIMIT 1`).Scan(&id))
	_, ok, err := state.Get(context.Background(), e.db, other, id, "chat:42")
	require.NoError(t, err)
	assert.False(t, ok, "another tenant does not see the key")
	_, ok, err = state.Get(context.Background(), e.db, e.tenant, id, "chat:42")
	require.NoError(t, err)
	assert.True(t, ok)
	page, err := state.List(context.Background(), e.db, other, id, state.Filter{})
	require.NoError(t, err)
	assert.Empty(t, page.Items)
}

func TestDryRunReadsStateAndWritesNothing(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	am := e.create(p, a, stateGraph)
	_, err := state.Put(context.Background(), e.db, e.tenant, am.ID, "chat:42", 5, 0, nil)
	require.NoError(t, err)

	run := func(note string) []string {
		res, err := p.rt.Authoring().DryRun(context.Background(), a, engine.DryRunInput{
			Graph: json.RawMessage(stateGraph), AutomationID: am.ID, Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
			Events: []json.RawMessage{json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 1, "note": "` + note + `", "secret": ""}`)},
		})
		require.NoError(t, err)
		var out []string
		for _, s := range res.Trace {
			out = append(out, s.Node+":"+s.Outcome+":"+s.Status)
		}
		return out
	}
	// The key is there: the dry run takes the branch of a repeat.
	assert.Equal(t, []string{"created:fired:", "seen:true:", "dup:done:captured", "fin:end:"}, run("42"))
	// A new key: set_state is captured, not executed.
	assert.Equal(t, []string{"created:fired:", "seen:false:", "mark:set:captured", "first:done:captured", "fin:end:"}, run("99"))
	assert.Equal(t, 1, e.count(`SELECT count(*) FROM automation.state`), "a dry run writes nothing")
}

func TestStatePutLimitsAndDelete(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	am := e.create(p, a, stateGraph)
	ctx := context.Background()
	_, err := e.db.Exec(ctx, `INSERT INTO automation.state (tenant_id, automation_id, key, value)
		SELECT $1, $2, 'k' || i, '1' FROM generate_series(1, $3) i`, e.tenant, am.ID, state.MaxKeys)
	require.NoError(t, err)

	// A new key over the limit is refused; a key that is there may change.
	_, err = state.Put(ctx, e.db, e.tenant, am.ID, "new", 1, 0, nil)
	var se *state.Error
	require.ErrorAs(t, err, &se)
	assert.Equal(t, state.CodeLimitExceeded, se.Code)
	_, err = state.Put(ctx, e.db, e.tenant, am.ID, "k1", "again", 0, nil)
	require.NoError(t, err)

	// An expired key does not count.
	_, err = e.db.Exec(ctx, `UPDATE automation.state SET expires_at = now() - interval '1 minute' WHERE key = 'k2'`)
	require.NoError(t, err)
	_, err = state.Put(ctx, e.db, e.tenant, am.ID, "new", 1, 0, nil)
	require.NoError(t, err)

	deleted, err := state.Delete(ctx, e.db, e.tenant, am.ID, "new")
	require.NoError(t, err)
	assert.True(t, deleted)
	deleted, err = state.Delete(ctx, e.db, e.tenant, am.ID, "new")
	require.NoError(t, err)
	assert.False(t, deleted)

	// The rest of the bounds.
	for _, c := range []struct {
		key  string
		val  any
		ttl  time.Duration
		code string
	}{
		{"", 1, 0, state.CodeKeyInvalid},
		{strings.Repeat("я", state.MaxKeyChars+1), 1, 0, state.CodeKeyInvalid},
		{"ok", strings.Repeat("x", state.MaxValueBytes), 0, state.CodeValueTooLarge},
		{"ok", 1, time.Second, state.CodeTTLInvalid},
		{"ok", 1, state.MaxTTL + time.Hour, state.CodeTTLInvalid},
	} {
		_, err := state.Put(ctx, e.db, e.tenant, am.ID, c.key, c.val, c.ttl, nil)
		require.ErrorAs(t, err, &se, c.code)
		assert.Equal(t, c.code, se.Code)
	}
	_, err = state.Put(ctx, e.db, e.tenant, am.ID, strings.Repeat("я", state.MaxKeyChars), "fits", state.MaxTTL, nil)
	require.NoError(t, err)

	// A deleted automation keeps no memory.
	require.NoError(t, p.rt.Service().Delete(ctx, a, am.ID))
	assert.Equal(t, 0, e.count(`SELECT count(*) FROM automation.state`))
}

func TestSetStateParamsAreChecked(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	g := func(params string) json.RawMessage {
		return json.RawMessage(`{"name": "x", "triggers": [{"id": "created", "event": "acme.order.created.v1"}],
		 "nodes": [{"id": "s", "type": "set_state", "params": ` + params + `}, {"id": "fin", "type": "end", "params": {}}],
		 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "s"}, {"from": {"node": "s", "outcome": "set"}, "to": "fin"}, {"from": {"node": "s", "outcome": "failed"}, "to": "fin"}]}`)
	}
	check := func(params string) string {
		_, err := p.rt.Authoring().Validate(context.Background(), a, g(params))
		if err == nil {
			return ""
		}
		var ve *engine.ValidationError
		require.ErrorAs(t, err, &ve)
		var codes []string
		for _, ge := range ve.Errors {
			codes = append(codes, ge.Code)
		}
		return strings.Join(codes, ",")
	}
	assert.Empty(t, check(`{"key": "a", "value": "1", "ttl": "1m"}`))
	assert.Empty(t, check(`{"key": "a", "delete": true}`))
	assert.Contains(t, check(`{"key": "a"}`), "state_value_required")
	assert.Contains(t, check(`{"key": "a", "value": "1", "delete": true}`), "state_value_unused")
	assert.Contains(t, check(`{"key": "a", "delete": true, "ttl": "1h"}`), "state_ttl_unused")
	assert.NotEmpty(t, check(`{"key": "a", "value": "1", "ttl": "30s"}`), "under a minute")
	assert.NotEmpty(t, check(`{"key": "a", "value": "1", "ttl": "366d"}`), "over a year")
	assert.Empty(t, check(`{"key": "a", "value": "1", "ttl": "365d"}`))
}

// Testing a set_state node of a saved graph runs it as the node would run, then rolls it back.
func TestTestNodeOfSetStateWritesNothing(t *testing.T) {
	e, p := newStateEnv(t)
	a := e.human()
	am := e.create(p, a, stateGraph)
	res, err := p.rt.Authoring().TestNode(context.Background(), a, engine.TestNodeInput{
		Graph: json.RawMessage(stateGraph), NodeID: "mark", AutomationID: am.ID,
		Subject: &engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
		Context: map[string]json.RawMessage{"event": json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 1, "note": "42", "secret": ""}`)},
	})
	require.NoError(t, err)
	assert.Nil(t, res.Error)
	assert.Equal(t, "set", res.Outcome)
	assert.Zero(t, e.count(`SELECT count(*) FROM automation.state`), "the test is rolled back")
}
