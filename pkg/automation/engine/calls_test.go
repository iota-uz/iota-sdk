package engine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/workflow"
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
	// The blocked call's detail names the node and the provider's message: the workflow history API keeps
	// the reason and leaves the text (and every payload) of an automation instance out.
	calls, err := p2.wf.List(context.Background(), workflow.ListFilter{TenantID: ae.tenant, Workflow: engine.CallWorkflowID})
	require.NoError(t, err)
	require.NotEmpty(t, calls)
	for _, c := range calls {
		assert.Empty(t, c.BlockedDetail)
		h, err := p2.wf.Get(context.Background(), ae.tenant, c.ID)
		require.NoError(t, err)
		assert.Equal(t, workflow.StatusBlocked, h.Instance.Status)
		assert.NotEmpty(t, h.Instance.BlockedReason)
		assert.Empty(t, h.Instance.BlockedDetail)
		assert.Nil(t, h.Instance.Input)
	}
	assert.Positive(t, ae.count(`SELECT count(*) FROM extruntime.workflow_instances WHERE workflow = 'automation.call' AND blocked_detail <> ''`), "the engine keeps it")
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

// What a call node logs during a run is kept with its step, masked with the connection's secrets: the
// run view shows the step's logs, and no table holds the secret.
func TestCallLogsAreKeptMaskedWithTheStep(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), graphJSON("HTTP", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "get", "type": "acme.http", "params": {}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "get"}, {"from": {"node": "get", "outcome": "ok"}, "to": "fin"}, {"from": {"node": "get", "outcome": "failed"}, "to": "fin"}`))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1, Secret: "s3cret"}, 0)
	p.until(func() bool {
		runs := ae.runs(p, ae.human(), engine.RunFilter{})
		return len(runs) == 1 && runs[0].Status == engine.RunDone
	}, "the run ends")
	run := runOf(t, ae.env, p)
	// The runs that took a step on a node (a node test's example) are found by the server.
	assert.Len(t, ae.runs(p, ae.human(), engine.RunFilter{Node: "get"}), 1)
	assert.Empty(t, ae.runs(p, ae.human(), engine.RunFilter{Node: "nope"}))
	d, err := p.rt.Service().GetRun(context.Background(), ae.human(), run.RunID)
	require.NoError(t, err)
	// The run's event comes with the run, projected for the viewer: a sensitive field only for a viewer
	// who may read it.
	assert.JSONEq(t, `{"kind": "acme.order.created.v1", "amount": 1, "note": "", "secret": "s3cret"}`, string(d.Event))
	reader := ae.actor(ae.tenant, 1, engine.OriginHuman, engine.PermRead, permRead)
	dr, err := p.rt.Service().GetRun(context.Background(), reader, run.RunID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind": "acme.order.created.v1", "amount": 1, "note": ""}`, string(dr.Event))
	// It fails closed. A key the schema does not declare is dropped, recorded as sensitive or not; a field
	// of the schema guarded by a rule the viewer passes (the event's read permission) stays; an event the
	// catalog no longer has gives nothing.
	ctx := context.Background()
	_, err = ae.db.Exec(ctx, `UPDATE automation.runs SET input = input || '{"legacy": "x"}'::jsonb,
		sensitive = sensitive || '[{"event": "acme.order.created.v1", "path": "legacy"}]'::jsonb WHERE id = $1`, run.RunID)
	require.NoError(t, err)
	d, err = p.rt.Service().GetRun(ctx, ae.human(), run.RunID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind": "acme.order.created.v1", "amount": 1, "note": "", "secret": "s3cret"}`, string(d.Event))
	dr, err = p.rt.Service().GetRun(ctx, reader, run.RunID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind": "acme.order.created.v1", "amount": 1, "note": ""}`, string(dr.Event))
	_, err = ae.db.Exec(ctx, `UPDATE automation.runs SET input = jsonb_set(input, '{kind}', '"acme.gone.v1"') WHERE id = $1`, run.RunID)
	require.NoError(t, err)
	d, err = p.rt.Service().GetRun(ctx, ae.human(), run.RunID)
	require.NoError(t, err)
	assert.Nil(t, d.Event, "an event no rule can check is not shown")
	// Past the detail's bound of steps, StepsOf still brings the step of the named node (a node test's
	// example reads it).
	restore := engine.SetMaxDetailSteps(1)
	capped, err := p.rt.Service().GetRun(context.Background(), ae.human(), run.RunID)
	require.NoError(t, err)
	require.Len(t, capped.Steps, 1)
	assert.Equal(t, "fin", capped.Steps[0].NodeID)
	named, err := p.rt.Service().GetRun(context.Background(), ae.human(), run.RunID, engine.StepsOf("get"))
	require.NoError(t, err)
	nodes := []string{}
	for _, s := range named.Steps {
		nodes = append(nodes, s.NodeID)
	}
	assert.Equal(t, []string{"get", "fin"}, nodes)
	// A later visit of the node the viewer may not read (it carries a field no rule can check now) does
	// not hide the earlier one: StepsOf gives the viewer its latest readable visit, and the hidden one
	// stays hidden.
	_, err = ae.db.Exec(context.Background(), `INSERT INTO automation.steps (tenant_id, run_id, token_id, seq, node_id, outcome, output, sensitive, started_at, ended_at)
		SELECT tenant_id, run_id, token_id, 99, node_id, 'ok', '{"headers": {"x": "later"}}', '[{"event": "acme.gone.v1", "path": "x"}]',
			now() + interval '1 hour', now() + interval '1 hour'
		FROM automation.steps WHERE run_id = $1 AND node_id = 'get'`, run.RunID)
	require.NoError(t, err)
	read, err := p.rt.Service().GetRun(context.Background(), ae.human(), run.RunID, engine.StepsOf("get"))
	require.NoError(t, err)
	var gets []engine.StepView
	for _, s := range read.Steps {
		if s.NodeID == "get" {
			gets = append(gets, s)
		}
	}
	require.Len(t, gets, 2, "the latest step (bound 1) is the hidden visit; StepsOf adds the readable one")
	assert.False(t, gets[0].Hidden, "the earlier visit, readable, with its data")
	assert.NotEmpty(t, gets[0].Output)
	assert.Equal(t, 99, gets[1].Seq)
	assert.True(t, gets[1].Hidden, "the later visit stays hidden")
	assert.Empty(t, gets[1].Output)
	_, err = ae.db.Exec(context.Background(), `DELETE FROM automation.steps WHERE run_id = $1 AND seq = 99`, run.RunID)
	require.NoError(t, err)
	restore()
	var logs []string
	for _, s := range d.Steps {
		if s.NodeID == "get" {
			logs = s.Logs
		}
	}
	assert.Equal(t, []string{"GET /me with Bearer ***"}, logs)
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.steps WHERE logs::text LIKE '%' || $1 || '%'`, acmeToken))
	assert.Zero(t, ae.count(`SELECT count(*) FROM extruntime.workflow_occurrences WHERE result::text LIKE '%' || $1 || '%'`, acmeToken))
}

// Cancelling a run terminates its call and closes its correlations.
func TestCancelTerminatesTheCall(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	stop := p.drive()
	select {
	case <-ae.acme.entered:
	case <-time.After(20 * time.Second):
		stop()
		t.Fatal("the call did not start")
	}
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

// A Retry that read an attempt which another Retry already acted on changes nothing: forgetting the
// attempt, renewing its deadline and resuming the call are one transaction bound to the attempt read.
func TestStaleRetryChangesNothing(t *testing.T) {
	ae := newAcmeEnv(t)
	p, run, waitID, _ := blockedCall(t, ae, "")
	ops := p.rt.Operations()
	ctx := context.Background()
	var second error
	p.rt.SetOperationsHook(func() {
		p.rt.SetOperationsHook(nil)
		second = ops.Retry(ctx, ae.human(), run.RunID, waitID) // the other operator, first to act
	})
	err := ops.Retry(ctx, ae.human(), run.RunID, waitID)
	require.NoError(t, second)
	require.ErrorIs(t, err, engine.ErrInvalidInput, "the stale retry finds its attempt gone")
	p.until(func() bool { return ae.status(p, run.RunID) == engine.RunDone }, "the retried call completes")
	require.Len(t, ae.acme.callsOf("acme.send"), 2, "one retry, one repeat")
}

// A Retry of an ask whose correlation token expired is refused (a repeated question could not be
// answered) and changes nothing; Resolve still settles the call.
func TestRetryOfAnExpiredAskIsRefused(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	// The failed edge ignored: an unknown outcome of the question blocks the run.
	g := strings.Replace(askGraph, `"params": {"text": "approve?"}`, `"params": {"text": "approve?", "mode": "crash_once"}`, 1)
	g = strings.Replace(g, `{"from": {"node": "ask", "outcome": "failed"}, "to": "bad"}`, `{"from": {"node": "ask", "outcome": "failed"}, "to": "ignore"}`, 1)
	g = strings.Replace(g, `{"from": {"node": "ask", "outcome": "timeout"}, "to": "late"}`, `{"from": {"node": "ask", "outcome": "timeout"}, "to": "late"}, {"from": {"node": "late", "outcome": "done"}, "to": "bad"}`, 1)
	g = strings.Replace(g, `{"from": {"node": "late", "outcome": "done"}, "to": "fin"}, `, ``, 1)
	ae.create(p, ae.human(), g)
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
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
	ctx := context.Background()
	require.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.correlations WHERE run_id = $1`, run.RunID))
	_, err := ae.db.Exec(ctx, `UPDATE automation.correlations SET expires_at = now() - interval '1 second' WHERE run_id = $1`, run.RunID)
	require.NoError(t, err)
	ops := p2.rt.Operations()
	require.ErrorIs(t, ops.Retry(ctx, ae.human(), run.RunID, waitID), engine.ErrInvalidInput)
	assert.Equal(t, engine.RunBlocked, ae.status(p2, run.RunID), "the refused retry resumed nothing")
	assert.Len(t, ae.acme.callsOf("acme.ask"), 1)
	require.NoError(t, ops.Resolve(ctx, ae.human(), run.RunID, waitID, "no", nil))
	p2.until(func() bool { return ae.status(p2, run.RunID) == engine.RunDone }, "the resolved ask goes on")
	assert.Equal(t, []string{"no:no"}, ae.effects(run.RunID))
}

// The run reconciles a call child that ended without delivering its result (the observer could not
// signal): the wait gets outcome_unknown and the run goes on.
func TestEndedCallIsReconciledByTheRun(t *testing.T) {
	ae := newAcmeEnv(t)
	p, run, child := interruptedCall(t, ae)
	p.rt.QuietCallObserver()
	require.NoError(t, p.wf.Terminate(context.Background(), ae.tenant, child))
	p.until(func() bool { return ae.status(p, run.RunID) == engine.RunDone }, "the run reconciles the ended call")
	var code string
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT error->>'code' FROM automation.steps WHERE run_id = $1 AND node_id = 'send'`, run.RunID).Scan(&code))
	assert.Equal(t, engine.CodeOutcomeUnknown, code)
	requireBadNote(t, ae, run.RunID)
}

// interruptedCall stops a process mid-call of a graph whose failed edge leads to the bad note (so the
// call does not block on an unknown outcome) and returns a process that has not resumed it yet, the run
// and the call's child instance.
func interruptedCall(t *testing.T, ae *acmeEnv) (*proc, engine.RunSummary, uuid.UUID) {
	t.Helper()
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "crash", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)
	p2 := ae.proc()
	run := runOf(t, ae.env, p2)
	var child uuid.UUID
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT id FROM extruntime.workflow_instances WHERE workflow = $1`, engine.CallWorkflowID).Scan(&child))
	return p2, run, child
}

// A call child terminated after its result committed (the parent's signal queued), before its own
// completion, gives the run that result: the reconciliation of the ended child recovers it from the
// receipt instead of synthesizing a failure, and the run takes the sent edge.
func TestCallTerminatedAfterItsResultKeepsTheResult(t *testing.T) {
	for _, quiet := range []bool{false, true} {
		t.Run(fmt.Sprintf("quiet observer %v", quiet), func(t *testing.T) {
			ae := newAcmeEnv(t)
			p := ae.proc()
			if quiet {
				p.rt.QuietCallObserver()
			}
			var once sync.Once
			p.rt.SetCallResultHook(func(ctx context.Context, child uuid.UUID, stage string) {
				if stage == "after" {
					once.Do(func() { require.NoError(t, p.wf.Terminate(context.Background(), ae.tenant, child)) })
				}
			})
			ae.create(p, ae.human(), sendGraph("acme.send", "ok", "", true))
			ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
			p.until(func() bool { return ae.status(p, runOf(t, ae.env, p).RunID) == engine.RunDone }, "the run ends")
			run := runOf(t, ae.env, p)
			assert.Equal(t, 1, ae.count(`SELECT count(*) FROM extruntime.workflow_instances WHERE workflow = $1 AND status = 'terminated'`, engine.CallWorkflowID))
			effects := ae.effects(run.RunID)
			require.Len(t, effects, 1, effects)
			assert.True(t, strings.HasPrefix(effects[0], "ok:sent "), effects)
			require.Len(t, ae.acme.callsOf("acme.send"), 1)
		})
	}
}

// A call child that ends after its provider answered, before its result transaction committed (it was
// refused or rolled back), gives the run that answer: a completed attempt is never effect_failed.
func TestCallEndedBeforeItsResultKeepsTheAnswer(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	var once sync.Once
	p.rt.SetCallResultHook(func(ctx context.Context, child uuid.UUID, stage string) {
		if stage == "before" {
			once.Do(func() { require.NoError(t, p.wf.Terminate(context.Background(), ae.tenant, child)) })
		}
	})
	ae.create(p, ae.human(), sendGraph("acme.send", "ok", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	p.until(func() bool { return ae.status(p, runOf(t, ae.env, p).RunID) == engine.RunDone }, "the run ends")
	run := runOf(t, ae.env, p)
	assert.Zero(t, ae.count(`SELECT count(*) FROM extruntime.workflow_occurrences o JOIN extruntime.workflow_instances i ON i.id = o.instance_id
		WHERE i.workflow = $1 AND o.name = 'result' AND o.status = 'completed'`, engine.CallWorkflowID), "no result receipt")
	effects := ae.effects(run.RunID)
	require.Len(t, effects, 1, effects)
	assert.True(t, strings.HasPrefix(effects[0], "ok:sent "), effects)
	require.Len(t, ae.acme.callsOf("acme.send"), 1)
}

// A connector's error code is kept only as an identifier: provider text in it (here the credentials)
// is recorded as effect_failed, never journaled or shown as output.error.code.
func TestCallErrorCodeIsAnIdentifier(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.send", "badcode", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	p.until(func() bool { return ae.status(p, runOf(t, ae.env, p).RunID) == engine.RunDone }, "the run ends")
	run := runOf(t, ae.env, p)
	effects := ae.effects(run.RunID)
	require.Len(t, effects, 1, effects)
	assert.Equal(t, "bad:"+engine.CodeEffectFailed+": refused", effects[0])
	assert.Zero(t, ae.count(`SELECT count(*) FROM extruntime.workflow_occurrences WHERE result::text LIKE '%' || $1 || '%'`, acmeToken), "the journal keeps no credentials")
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.steps WHERE output::text LIKE '%' || $1 || '%' OR error::text LIKE '%' || $1 || '%'`, acmeToken))
}

// A call instance whose request names a parent other than its stored one is refused by Run, and its
// end signals that parent nothing.
func TestCallOfAForeignParentSignalsNothing(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), graphJSON("Wait", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "pay", "type": "wait", "params": {"event": "acme.order.paid.v1", "timeout": "1h"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "pay"}, {"from": {"node": "pay", "outcome": "matched"}, "to": "fin"}, {"from": {"node": "pay", "outcome": "timeout"}, "to": "fin"}`))
	ae.publish(ae.tenant, orderCreated, "o-1", order{}, 0)
	p.until(func() bool {
		return ae.count(`SELECT count(*) FROM automation.waits WHERE status = 'open' AND kind = 'event'`) == 1
	}, "the run waits")
	ctx := context.Background()
	var runID, parent uuid.UUID
	require.NoError(t, ae.db.QueryRow(ctx, `SELECT id, instance_id FROM automation.runs`).Scan(&runID, &parent))
	req, err := json.Marshal(map[string]any{"tenant_id": ae.tenant, "parent_instance": parent, "run_id": runID, "wait_id": "w/forged",
		"node_id": "x", "node_type": "acme.send", "input": map[string]any{}, "deadline": time.Now().Add(time.Minute), "failed_connected": true})
	require.NoError(t, err)
	require.NoError(t, pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error {
		_, err := p.wf.Start(ctx, tx, spi.Scope{Subject: spiSubject(ae.tenant, 1)}, spi.StartIntent{Workflow: engine.CallWorkflowID, Input: req, Key: "forged"})
		return err
	}))
	p.until(func() bool {
		return ae.count(`SELECT count(*) FROM extruntime.workflow_instances WHERE workflow = $1 AND status = 'failed'`, engine.CallWorkflowID) == 1
	}, "the forged call fails")
	assert.Zero(t, ae.count(`SELECT count(*) FROM extruntime.workflow_signals WHERE instance_id = $1 AND signal_key LIKE 'call:%'`, parent), "the parent got no signal")
	assert.Empty(t, ae.acme.callsOf("acme.send"))
}

// requireBadNote asserts the run took the call's failed edge: the bad note ran with the error's code.
func requireBadNote(t *testing.T, ae *acmeEnv, run uuid.UUID) {
	t.Helper()
	effects := ae.effects(run)
	require.Len(t, effects, 1, effects)
	assert.True(t, strings.HasPrefix(effects[0], "bad:"+engine.CodeOutcomeUnknown+": "), effects)
}

// MaxAttempts counts attempts across occurrences: an interrupted attempt and a retryable answer are two,
// so with MaxAttempts=2 there is no third call.
func TestAttemptsCountAcrossOccurrences(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	ae.create(p, ae.human(), sendGraph("acme.fetch", "crash_then_unavail", "", true))
	ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1}, 0)
	crash(t, ae)
	p2 := ae.proc(func(c *engine.Config) { c.CallLimits.MaxAttempts = 2 })
	p2.until(func() bool { return ae.status(p2, runOf(t, ae.env, p2).RunID) == engine.RunDone }, "the call fails")
	assert.Len(t, ae.acme.callsOf("acme.fetch"), 2, "no third provider call")
}

// A call child terminated without a result gives the parent's wait one: outcome_unknown when an attempt
// had started, and the run goes on instead of waiting for ever.
func TestTerminatedCallChildEndsTheWait(t *testing.T) {
	ae := newAcmeEnv(t)
	p, run, child := interruptedCall(t, ae)
	require.NoError(t, p.wf.Terminate(context.Background(), ae.tenant, child))
	p.until(func() bool { return ae.status(p, run.RunID) == engine.RunDone }, "the run takes the call's failed edge")
	require.Len(t, ae.acme.callsOf("acme.send"), 1)
	var code string
	require.NoError(t, ae.db.QueryRow(context.Background(), `SELECT error->>'code' FROM automation.steps WHERE run_id = $1 AND node_id = 'send'`, run.RunID).Scan(&code))
	assert.Equal(t, engine.CodeOutcomeUnknown, code)
	requireBadNote(t, ae, run.RunID)
}

// An automation.call started from elsewhere with another tenant's request opens nothing.
func TestCallOfAnotherTenantIsRefused(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	other := ae.newTenant()
	ctx := context.Background()
	req, err := json.Marshal(map[string]any{"tenant_id": ae.tenant, "run_id": uuid.New(), "node_id": "x", "node_type": "acme.http",
		"connection": ae.conn.ID.String(), "input": map[string]any{}, "deadline": time.Now().Add(time.Minute)})
	require.NoError(t, err)
	require.NoError(t, pgx.BeginFunc(ctx, ae.db, func(tx pgx.Tx) error {
		_, err := p.wf.Start(ctx, tx, spi.Scope{Subject: spiSubject(other, 1)}, spi.StartIntent{Workflow: engine.CallWorkflowID, Input: req, Key: "foreign"})
		return err
	}))
	p.until(func() bool {
		return ae.count(`SELECT count(*) FROM extruntime.workflow_instances WHERE workflow = $1 AND status = 'failed'`, engine.CallWorkflowID) == 1
	}, "the foreign call fails")
	assert.Empty(t, ae.acme.callsOf("acme.http"), "no provider call with the other tenant's connection")
}
