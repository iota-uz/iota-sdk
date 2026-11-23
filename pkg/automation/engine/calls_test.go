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

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// sendGraph runs one call node; failed: the failed outcome notes the error code (else it is left out:
// the node's failure is the run's).
func sendGraph(nodeType, mode, timeout string, failed bool) string {
	params := `{"mode": "` + mode + `", "text": "hi"`
	if timeout != "" {
		params += `, "timeout": "` + timeout + `"`
	}
	params += `}`
	nodes := `{"id": "send", "type": "` + nodeType + `", "params": ` + params + `},
	  {"id": "ok", "type": "acme.note", "params": {"text": "sent {{ nodes.send.output.message_id }}"}},
	  {"id": "fin", "type": "end", "params": {}}`
	edges := `{"from": {"node": "created", "outcome": "fired"}, "to": "send"},
	  {"from": {"node": "send", "outcome": "sent"}, "to": "ok"},
	  {"from": {"node": "ok", "outcome": "done"}, "to": "fin"}, {"from": {"node": "ok", "outcome": "failed"}, "to": "fin"}`
	if failed {
		nodes += `, {"id": "bad", "type": "acme.note", "params": {"text": "{{ nodes.send.output.error.code }}: {{ nodes.send.output.error.message }}"}}`
		edges += `, {"from": {"node": "send", "outcome": "failed"}, "to": "bad"},
		  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"}, {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}`
	} else {
		edges += `, {"from": {"node": "send", "outcome": "failed"}, "to": "ignore"}`
	}
	return graphJSON("Send", `{"id": "created", "event": "acme.order.created.v1"}`, nodes, edges)
}

// crash runs a process until the call it starts is executing, then stops it as a process that dies.
func crash(t *testing.T, ae *acmeEnv) {
	t.Helper()
	p := ae.proc()
	stop := p.drive()
	select {
	case <-ae.acme.entered:
	case <-time.After(20 * time.Second):
		stop()
		t.Fatal("the call did not start")
	}
	stop()
}

func runOf(t *testing.T, e *env, p *proc) engine.RunSummary {
	t.Helper()
	runs := e.runs(p, e.human(), engine.RunFilter{})
	require.Len(t, runs, 1)
	return runs[0]
}

// The process stops after a call that may not repeat was sent and before its result was written: the
// call is not repeated; its failed outcome (connected) takes outcome_unknown.
func TestUnknownOutcomeOfANonIdempotentCallTakesFailed(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)

	p2 := ae.proc()
	p2.until(func() bool { return ae.status(p2, runOf(t, ae.env, p2).RunID) == engine.RunDone }, "the run continues by the failed outcome")
	run := runOf(t, ae.env, p2)
	require.Len(t, ae.acme.callsOf("acme.send"), 1, "the call is not repeated blindly")
	effects := ae.effects(run.RunID)
	require.Len(t, effects, 1)
	assert.True(t, strings.HasPrefix(effects[0], "bad:outcome_unknown: "), effects[0])
}

// Without a failed outcome the call blocks the run; Resolve continues it by the outcome the operator chose.
func TestUnknownOutcomeBlocksAndResolveContinues(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash", "", false))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)

	p2 := ae.proc()
	p2.until(func() bool { return ae.status(p2, runOf(t, ae.env, p2).RunID) == engine.RunBlocked }, "the run is blocked by its call")
	run := runOf(t, ae.env, p2)
	assert.Contains(t, run.Reason, "call_blocked: send")
	detail := ae.run(p2, ae.human(), run.RunID)
	var waitID string
	for _, w := range detail.Waits {
		if w.Kind == "call" && w.Status == "open" {
			waitID = w.WaitID
		}
	}
	require.NotEmpty(t, waitID)
	ops := p2.rt.Operations()
	ctx := context.Background()
	require.ErrorIs(t, ops.Resolve(ctx, ae.human(), run.RunID, waitID, "nope", nil), engine.ErrInvalidInput, "an outcome the node does not have")
	noOperate := ae.actor(ae.tenant, 1, engine.OriginHuman, engine.PermRead)
	require.ErrorIs(t, ops.Resolve(ctx, noOperate, run.RunID, waitID, "sent", nil), engine.ErrForbidden)
	require.NoError(t, ops.Resolve(ctx, ae.human(), run.RunID, waitID, "sent", json.RawMessage(`{"message_id": "found-by-hand"}`)))
	p2.until(func() bool { return ae.status(p2, run.RunID) == engine.RunDone }, "the resolved run continues")
	assert.Equal(t, []string{"ok:sent found-by-hand"}, ae.effects(run.RunID))
	require.Len(t, ae.acme.callsOf("acme.send"), 1)
}

// Retry repeats a blocked call with its idempotency key.
func TestRetryOfABlockedCallRepeatsIt(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash_once", "", false))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)
	p2 := ae.proc()
	p2.until(func() bool { return ae.status(p2, runOf(t, ae.env, p2).RunID) == engine.RunBlocked }, "blocked")
	run := runOf(t, ae.env, p2)
	detail := ae.run(p2, ae.human(), run.RunID)
	var waitID string
	for _, w := range detail.Waits {
		if w.Kind == "call" && w.Status == "open" {
			waitID = w.WaitID
		}
	}
	require.NoError(t, p2.rt.Operations().Retry(context.Background(), ae.human(), run.RunID, waitID))
	p2.until(func() bool { return ae.status(p2, run.RunID) == engine.RunDone }, "the retried call completes")
	calls := ae.acme.callsOf("acme.send")
	require.Len(t, calls, 2)
	assert.Equal(t, calls[0].Key, calls[1].Key, "the same idempotency key")
}

// An idempotent call is repeated after the same crash, with the same idempotency key.
func TestIdempotentCallIsRepeatedWithTheSameKey(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.fetch", "crash_once", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)

	p2 := ae.proc()
	p2.until(func() bool { return ae.status(p2, runOf(t, ae.env, p2).RunID) == engine.RunDone }, "the repeated call completes")
	run := runOf(t, ae.env, p2)
	calls := ae.acme.callsOf("acme.fetch")
	require.Len(t, calls, 2)
	assert.Equal(t, calls[0].Key, calls[1].Key)
	assert.Equal(t, run.RunID.String(), strings.Split(calls[0].Key, ":")[0], "run:node:token:seq")
	assert.Equal(t, 2, calls[1].Attempt)
	assert.Equal(t, []string{"ok:sent m-" + calls[0].Key}, ae.effects(run.RunID))
}

// A 429 whose Retry-After is past the node's deadline fails at once, without another attempt.
func TestRateLimitPastTheDeadlineFails(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.fetch", "429", "30s", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	p.until(func() bool {
		runs := ae.runs(p, ae.human(), engine.RunFilter{})
		return len(runs) == 1 && runs[0].Status == engine.RunDone
	}, "the run takes the failed outcome")
	run := runOf(t, ae.env, p)
	require.Len(t, ae.acme.callsOf("acme.fetch"), 1, "no attempt the deadline cannot hold")
	effects := ae.effects(run.RunID)
	require.Len(t, effects, 1)
	assert.True(t, strings.HasPrefix(effects[0], "bad:rate_limited: "), effects[0])
}

// The failed branch reads output.error.message; the secret in the provider's message is masked.
func TestErrorMessageInTheFailedBranchIsMasked(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "reject", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	p.until(func() bool {
		runs := ae.runs(p, ae.human(), engine.RunFilter{})
		return len(runs) == 1 && runs[0].Status == engine.RunDone
	}, "the run takes the failed outcome")
	run := runOf(t, ae.env, p)
	assert.Equal(t, []string{"bad:rejected: the provider refused token ***"}, ae.effects(run.RunID))
	// Neither the journal of the call nor the steps keep the secret.
	assert.Zero(t, ae.count(`SELECT count(*) FROM extruntime.workflow_occurrences WHERE result::text LIKE '%' || $1 || '%' OR error::text LIKE '%' || $1 || '%'`, acmeToken))
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.steps WHERE output::text LIKE '%' || $1 || '%' OR input::text LIKE '%' || $1 || '%' OR error::text LIKE '%' || $1 || '%'`, acmeToken))
	assert.Zero(t, ae.count(`SELECT count(*) FROM extruntime.workflow_instances WHERE input::text LIKE '%' || $1 || '%'`, acmeToken))
}

// Cancelling a run terminates its call and closes its correlations.
func TestCancelTerminatesTheCall(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	stop := p.drive()
	<-ae.acme.entered
	stop()
	p2 := ae.proc()
	run := runOf(t, ae.env, p2)
	require.NoError(t, p2.rt.Service().Cancel(context.Background(), ae.human(), run.RunID))
	p2.until(func() bool { return ae.status(p2, run.RunID) == engine.RunCancelled }, "cancelled")
	p2.until(func() bool {
		return ae.count(`SELECT count(*) FROM extruntime.workflow_instances WHERE workflow = $1 AND status = 'terminated'`, engine.CallWorkflowID) == 1
	}, "the call instance is terminated")
}

func TestInvocationIDIsStablePerKey(t *testing.T) {
	a, b := engine.InvocationIDOf("r:n:t:1"), engine.InvocationIDOf("r:n:t:1")
	assert.Equal(t, a, b)
	_, err := uuid.Parse(a)
	require.NoError(t, err)
	assert.NotEqual(t, a, engine.InvocationIDOf("r:n:t:2"))
}

// PerTenant=1: a second call of the tenant does not run while the first one still runs.
func TestPerTenantCallLimitHolds(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc(func(c *engine.Config) { c.CallLimits.PerTenant = 1 })
	ae.create(p, ae.human(), sendGraph("acme.send", "hold", "", true))
	for _, key := range []string{"o-1", "o-2", "o-3"} {
		ae.publish(ae.tenant, orderCreated, key, order{Amount: 1}, 0)
	}
	// Three processes drive the calls at once.
	for _, q := range []*proc{p, ae.proc(func(c *engine.Config) { c.CallLimits.PerTenant = 1 }), ae.proc(func(c *engine.Config) { c.CallLimits.PerTenant = 1 })} {
		stop := q.drive()
		defer stop()
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && len(noteTexts(t, ae.env)) < 3 {
		time.Sleep(50 * time.Millisecond)
	}
	require.Len(t, noteTexts(t, ae.env), 3, "every call ran")
	assert.Equal(t, int64(1), ae.acme.peak.Load(), "never two calls of the tenant at once")
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.call_slots`), "the slots are given back")
}

// A known client refusal is not repeated, even when the provider calls it retryable.
func TestClientRefusalIsNotRetried(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.fetch", "unauth", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the run takes the failed outcome")
	assert.Len(t, ae.acme.callsOf("acme.fetch"), 1, "one execution")
	assert.Equal(t, []string{"unauthorized: bad token"}, noteTexts(t, ae.env))
}

// output.error.retryable reaches the node's output.
func TestRetryableReachesTheFailedOutput(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	g := strings.Replace(sendGraph("acme.send", "unavail", "", true), `{{ nodes.send.output.error.code }}: {{ nodes.send.output.error.message }}`,
		`{{ nodes.send.output.error.code }} {{ string(nodes.send.output.error.retryable) }}`, 1)
	ae.create(p, ae.human(), g)
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the run takes the failed outcome")
	assert.Equal(t, []string{"unavailable true"}, noteTexts(t, ae.env))
	assert.Len(t, ae.acme.callsOf("acme.send"), 1, "a call that may not repeat is not retried")
}

// A call that waits for a tenant slot does not wait past its deadline: it times out without calling
// the provider.
func TestSlotWaitEndsAtTheDeadline(t *testing.T) {
	ae := newAcmeEnv(t)
	one := func(c *engine.Config) { c.CallLimits.PerTenant = 1 }
	p := ae.proc(one)
	ae.create(p, ae.human(), sendGraph("acme.send", "hold_long", "1s", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	ae.publish(ae.tenant, orderCreated, "o-2", order{Amount: 2}, 0)
	for _, q := range []*proc{p, ae.proc(one)} {
		stop := q.drive()
		defer stop()
	}
	started := time.Now()
	deadline := started.Add(20 * time.Second)
	for time.Now().Before(deadline) && len(noteTexts(t, ae.env)) < 2 {
		time.Sleep(50 * time.Millisecond)
	}
	notes := noteTexts(t, ae.env)
	require.Len(t, notes, 2)
	assert.True(t, strings.HasPrefix(notes[1], "sent ") || strings.HasPrefix(notes[0], "sent "), notes)
	timedOut := 0
	for _, n := range notes {
		if strings.HasPrefix(n, "timeout: ") {
			timedOut++
		}
	}
	assert.Equal(t, 1, timedOut, "the waiting call timed out: %v", notes)
	assert.Len(t, ae.acme.callsOf("acme.send"), 1, "the provider was called once")
}

// A Retry after the call's deadline passed gives it a new deadline: the provider is called again.
func TestRetryAfterTheDeadlineCallsTheProvider(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash_once", "1s", false))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)
	p2 := ae.proc()
	p2.until(func() bool { return ae.status(p2, runOf(t, ae.env, p2).RunID) == engine.RunBlocked }, "blocked")
	time.Sleep(1200 * time.Millisecond) // the first deadline is gone
	run := runOf(t, ae.env, p2)
	var waitID string
	for _, w := range ae.run(p2, ae.human(), run.RunID).Waits {
		if w.Kind == "call" && w.Status == "open" {
			waitID = w.WaitID
		}
	}
	require.NoError(t, p2.rt.Operations().Retry(context.Background(), ae.human(), run.RunID, waitID))
	p2.until(func() bool { return ae.status(p2, run.RunID) == engine.RunDone }, "the retried call completes")
	require.Len(t, ae.acme.callsOf("acme.send"), 2)
	assert.Len(t, ae.effects(run.RunID), 1)
	assert.True(t, strings.HasPrefix(ae.effects(run.RunID)[0], "ok:sent "), ae.effects(run.RunID))
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.call_deadlines`), "the renewed deadline goes with the call")
}

// blockedCall blocks a call (its process stopped mid-call) and returns the run, its call wait and the
// child instance.
func blockedCall(t *testing.T, ae *acmeEnv, timeout string) (*proc, engine.RunSummary, string, uuid.UUID) {
	t.Helper()
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash_once", timeout, false))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)
	p2 := ae.proc()
	p2.until(func() bool { return ae.status(p2, runOf(t, ae.env, p2).RunID) == engine.RunBlocked }, "blocked")
	run := runOf(t, ae.env, p2)
	var waitID string
	for _, w := range ae.run(p2, ae.human(), run.RunID).Waits {
		if w.Kind == "call" && w.Status == "open" {
			waitID = w.WaitID
		}
	}
	var child uuid.UUID
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT id FROM extruntime.workflow_instances WHERE workflow = $1`, engine.CallWorkflowID).Scan(&child))
	return p2, run, waitID, child
}

// A Retry that stopped after it forgot the attempt and before it resumed the call is finished by
// repeating it.
func TestInterruptedRetryIsFinishedByARepeat(t *testing.T) {
	ae := newAcmeEnv(t)
	p, run, waitID, child := blockedCall(t, ae, "1s")
	ctx := context.Background()
	time.Sleep(1200 * time.Millisecond) // the call's own deadline is gone
	// What Retry wrote before the process stopped: a new deadline (gone too by the repeat) and the
	// forgotten attempt.
	_, err := ae.db.Exec(ctx, `INSERT INTO automation.call_deadlines (tenant_id, instance_id, deadline) VALUES ($1, $2, now() - interval '1 second')`, ae.tenant, child)
	require.NoError(t, err)
	require.NoError(t, p.wf.Retry(ctx, ae.tenant, child, "call", 0))
	ops := p.rt.Operations()
	require.ErrorIs(t, ops.Resolve(ctx, ae.human(), run.RunID, waitID, "sent", nil), engine.ErrInvalidInput, "the pending retry is not resolved")
	require.NoError(t, ops.Retry(ctx, ae.human(), run.RunID, waitID))
	p.until(func() bool { return ae.status(p, run.RunID) == engine.RunDone }, "the repeated retry runs the call")
	require.Len(t, ae.acme.callsOf("acme.send"), 2, "the repeat renews the deadline: the provider is called")
	assert.True(t, strings.HasPrefix(ae.effects(run.RunID)[0], "ok:sent "), ae.effects(run.RunID))
}

// A Resolve that stopped after it recorded the outcome and before it resumed the call is finished by
// repeating it.
func TestInterruptedResolveIsFinishedByARepeat(t *testing.T) {
	ae := newAcmeEnv(t)
	p, run, waitID, child := blockedCall(t, ae, "")
	ctx := context.Background()
	require.NoError(t, p.wf.Resolve(ctx, ae.tenant, child, "call", 0, json.RawMessage(`{"kind": "result", "outcome": "sent", "output": {"message_id": "by-hand"}}`)))
	ops := p.rt.Operations()
	require.ErrorIs(t, ops.Retry(ctx, ae.human(), run.RunID, waitID), engine.ErrInvalidInput, "a resolved call is not retried")
	require.NoError(t, ops.Resolve(ctx, ae.human(), run.RunID, waitID, "sent", nil))
	p.until(func() bool { return ae.status(p, run.RunID) == engine.RunDone }, "the repeated resolve continues the run")
	assert.Equal(t, []string{"ok:sent by-hand"}, ae.effects(run.RunID))
	require.Len(t, ae.acme.callsOf("acme.send"), 1)
}
