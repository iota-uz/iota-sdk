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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// classifyGraph: an inbound message → a classifier → send to sales or support.
const classifyGraph = `{"name": "Route", "triggers": [{"id": "m", "event": "acme.message.v1"}],
 "nodes": [
  {"id": "cls", "type": "acme.classify", "params": {"text": "{{ event.text }}"}},
  {"id": "sales", "type": "acme.send", "params": {"text": "to sales: {{ event.text }}"}},
  {"id": "support", "type": "acme.send", "params": {"text": "to support"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "m", "outcome": "fired"}, "to": "cls"},
  {"from": {"node": "cls", "outcome": "sales"}, "to": "sales"},
  {"from": {"node": "cls", "outcome": "support"}, "to": "support"},
  {"from": {"node": "cls", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "sales", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "sales", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "support", "outcome": "sent"}, "to": "fin"}, {"from": {"node": "support", "outcome": "failed"}, "to": "fin"}
 ]}`

// A cached dry run of "inbound → classifier → send" goes to the end: the decision is real, a second
// run takes it from the cache, the send is captured and nothing is sent.
func TestDryRunCachedDecidesAndCapturesTheSend(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	au := p.rt.Authoring()
	in := engine.DryRunInput{Graph: json.RawMessage(classifyGraph), Decisions: engine.DecisionsCached,
		Subject: engine.SubjectRef{Entity: graph.SubjectConversation, Key: "c-1"},
		Events:  []json.RawMessage{json.RawMessage(`{"kind": "acme.message.v1", "text": "I want to buy", "from": "ann"}`)}}
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		res, err := au.DryRun(ctx, ae.human(), in)
		require.NoError(t, err)
		kinds := make([]string, 0, len(res.Trace))
		for _, s := range res.Trace {
			kinds = append(kinds, s.Node+":"+s.Kind+":"+s.Outcome+":"+s.Status)
		}
		// The first run decides live; the second takes the decision from the cache and calls nobody.
		decided := map[int]string{0: "executed", 1: engine.StatusCached}[i]
		assert.Equal(t, []string{"m:triggered:fired:", "cls:step:sales:" + decided, "sales:step:sent:captured", "fin:ended:end:"}, kinds, "run %d", i)
		assert.Equal(t, 1-i, res.LiveCalls, "run %d: calls made to the provider", i)
		require.Len(t, res.Trace[2].Effects, 1)
		assert.Contains(t, string(res.Trace[2].Effects[0]), "to sales: I want to buy", "the send's params are rendered")
	}
	assert.Equal(t, int64(1), ae.acme.provider.Load(), "the second run took the decision from the cache")
	assert.Empty(t, ae.acme.callsOf("acme.send"), "nothing was sent")
	assert.Equal(t, 1, ae.count(`SELECT count(*) FROM automation.decision_cache`))
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.runs`))

	// Scripted: no call; the classifier's outcome comes from the context.
	in.Decisions = engine.DecisionsScripted
	in.Context = map[string]json.RawMessage{"nodes.cls": json.RawMessage(`{"output": {"label": "support"}, "outcome": "support"}`)}
	res, err := au.DryRun(ctx, ae.human(), in)
	require.NoError(t, err)
	assert.Equal(t, "support", res.Trace[1].Outcome)
	assert.Equal(t, "captured", res.Trace[1].Status)
	assert.Equal(t, int64(1), ae.acme.provider.Load())
}

// A trigger whose filter fails is named with the condition, so the author sees why nothing ran.
func TestDryRunNamesTheTriggerConditionThatFailed(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	res, err := p.rt.Authoring().DryRun(context.Background(), ae.human(), engine.DryRunInput{
		Graph: json.RawMessage(graphJSON("Big orders", `{"id": "big", "event": "acme.order.created.v1", "filter": "event.amount > 100"}`,
			`{"id": "fin", "type": "end", "params": {}}`, `{"from": {"node": "big", "outcome": "fired"}, "to": "fin"}`)),
		Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
		Events:  []json.RawMessage{json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 1, "note": "", "secret": ""}`)},
	})
	require.NoError(t, err)
	require.Len(t, res.Trace, 1)
	assert.Equal(t, engine.TraceStep{Node: "big", Kind: engine.TraceNotTriggered, Path: graph.MismatchFilter, Message: "event.amount > 100"}, res.Trace[0])
	assert.Zero(t, res.LiveCalls)
}

// A forbidden node stops the dry run.
func TestDryRunStopsOnAForbiddenNode(t *testing.T) {
	ae := newAcmeEnv(t)
	require.NoError(t, ae.reg.RegisterNode(specNode{graph.NodeSpec{Type: "acme.wipe", Title: "Wipe", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunForbid,
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}}))
	p := ae.proc()
	res, err := p.rt.Authoring().DryRun(context.Background(), ae.human(), engine.DryRunInput{
		Graph: json.RawMessage(graphJSON("Wipe", `{"id": "created", "event": "acme.order.created.v1"}`,
			`{"id": "w", "type": "acme.wipe", "params": {}}, {"id": "fin", "type": "end", "params": {}}`,
			`{"from": {"node": "created", "outcome": "fired"}, "to": "w"}, {"from": {"node": "w", "outcome": "done"}, "to": "fin"}`)),
		Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"},
		Events:  []json.RawMessage{json.RawMessage(`{"kind": "acme.order.created.v1", "amount": 1, "note": "", "secret": ""}`)},
	})
	require.NoError(t, err)
	last := res.Trace[len(res.Trace)-1]
	assert.Equal(t, engine.TraceError, last.Kind)
	assert.Equal(t, graph.FailDryRunForbidden, last.Path)
}

// A repeat of a confirmed test (its answer lost) gets the first answer on any process: the claim and the
// answer are in the database, so a non-idempotent send is made once. A claim without an answer (the first
// test still runs, or its process died mid-call) is refused as outcome_unknown, never run again; another
// key is another test.
func TestTestNodeRepeatWithTheSameKeyCallsOnce(t *testing.T) {
	ae := newAcmeEnv(t)
	first, second := ae.proc(), ae.proc() // two processes over one database
	g := sendGraph("acme.send", "ok", "", true)
	in := engine.TestNodeInput{Graph: json.RawMessage(g), NodeID: "send", Key: "confirm-1"}
	res1, err := first.rt.Authoring().TestNode(context.Background(), ae.human(), in)
	require.NoError(t, err)
	require.Nil(t, res1.Error)
	res2, err := second.rt.Authoring().TestNode(context.Background(), ae.human(), in)
	require.NoError(t, err)
	assert.Equal(t, res1.Outcome, res2.Outcome)
	assert.JSONEq(t, string(res1.Output), string(res2.Output), "the stored answer")
	require.Len(t, ae.acme.callsOf("acme.send"), 1, "the repeat on the other process called nothing")
	assert.Equal(t, "test:confirm-1", ae.acme.callsOf("acme.send")[0].Key)

	// The key names its node: another node with it is refused.
	_, err = second.rt.Authoring().TestNode(context.Background(), ae.human(), engine.TestNodeInput{Graph: json.RawMessage(g), NodeID: "ok", Key: "confirm-1"})
	require.ErrorIs(t, err, engine.ErrInvalidInput)

	// A claim without an answer: refused, not run.
	defer engine.SetTestWait(300 * time.Millisecond)()
	_, err = ae.db.Exec(context.Background(), `INSERT INTO automation.node_tests (tenant_id, user_id, key, node_id) VALUES ($1, $2, 'lost', 'send')`, ae.tenant, ae.human().UserID)
	require.NoError(t, err)
	in.Key = "lost"
	res3, err := second.rt.Authoring().TestNode(context.Background(), ae.human(), in)
	require.NoError(t, err)
	require.NotNil(t, res3.Error)
	assert.Equal(t, engine.CodeOutcomeUnknown, res3.Error.Code)
	assert.Len(t, ae.acme.callsOf("acme.send"), 1)

	// Past the retention the answer is gone but the claim stays: the key is refused, the send not repeated.
	_, err = ae.db.Exec(context.Background(), `UPDATE automation.node_tests SET created_at = now() - interval '25 hours' WHERE key IN ('confirm-1', 'lost')`)
	require.NoError(t, err)
	for _, key := range []string{"confirm-1", "lost"} {
		in.Key = key
		old, err := first.rt.Authoring().TestNode(context.Background(), ae.human(), in)
		require.NoError(t, err)
		require.NotNil(t, old.Error, key)
		assert.Equal(t, engine.CodeOutcomeUnknown, old.Error.Code, key)
	}
	assert.Len(t, ae.acme.callsOf("acme.send"), 1)

	in.Key = "confirm-2"
	_, err = first.rt.Authoring().TestNode(context.Background(), ae.human(), in)
	require.NoError(t, err)
	assert.Len(t, ae.acme.callsOf("acme.send"), 2)
	in.Key = strings.Repeat("k", engine.MaxTestKey+1)
	_, err = first.rt.Authoring().TestNode(context.Background(), ae.human(), in)
	require.ErrorIs(t, err, engine.ErrInvalidInput)
}

// The failure of a call recovered from its attempt's journal keeps what the node logged.
func TestRecoveredFailureKeepsTheLogs(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	wr, ok := p.rt.RecordedAnswer([]byte(`{"kind":"failed","code":"rejected","message":"refused","logs":["GET /me with Bearer ***"]}`), "acme.send")
	require.True(t, ok)
	require.NotNil(t, wr.Error)
	assert.Equal(t, []string{"GET /me with Bearer ***"}, wr.Logs)
}

// Testing an HTTP node runs it once, as the author: no run, no steps, no outbox events; the secret in
// its output is masked.
func TestTestNodeRunsOnceWithoutARun(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	g := graphJSON("HTTP", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "get", "type": "acme.http", "params": {}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "get"}, {"from": {"node": "get", "outcome": "ok"}, "to": "fin"}, {"from": {"node": "get", "outcome": "failed"}, "to": "fin"}`)
	before := ae.count(`SELECT count(*) FROM extruntime.outbox_events`)
	res, err := p.rt.Authoring().TestNode(context.Background(), ae.human(), engine.TestNodeInput{Graph: json.RawMessage(g), NodeID: "get"})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	assert.Equal(t, "ok", res.Outcome)
	assert.JSONEq(t, `{"headers": {"Authorization": "Bearer ***"}}`, string(res.Output))
	assert.NotContains(t, string(res.Output), acmeToken)
	assert.Equal(t, []string{"GET /me with Bearer ***"}, res.Logs, "what the node logged is masked")
	assert.Len(t, ae.acme.callsOf("acme.http"), 1)
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.runs`))
	assert.Zero(t, ae.count(`SELECT count(*) FROM automation.steps`))
	assert.Equal(t, before, ae.count(`SELECT count(*) FROM extruntime.outbox_events`))

	// An effect node is applied in a transaction that is rolled back.
	g2 := graphJSON("Emit", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "emit", "type": "acme.emit", "params": {}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "emit"}, {"from": {"node": "emit", "outcome": "done"}, "to": "fin"}`)
	res, err = p.rt.Authoring().TestNode(context.Background(), ae.human(), engine.TestNodeInput{Graph: json.RawMessage(g2), NodeID: "emit",
		Subject: &engine.SubjectRef{Entity: "acme.order", Key: "o-1"}})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	assert.Equal(t, before, ae.count(`SELECT count(*) FROM extruntime.outbox_events`), "the effect's event was rolled back")

	// A human node cannot be tested; testing needs Automations.Author.
	g3 := humanGraph(`{"title": "x", "assignees": [{"kind": "user", "user_id": 1}], `+humanButtons+`}`, false)
	res, err = p.rt.Authoring().TestNode(context.Background(), ae.human(), engine.TestNodeInput{Graph: json.RawMessage(g3), NodeID: "ok"})
	require.NoError(t, err)
	require.NotNil(t, res.Error)
	assert.Equal(t, engine.CodeInvalidInput, res.Error.Code)
	_, err = p.rt.Authoring().TestNode(context.Background(), ae.actor(ae.tenant, 1, engine.OriginHuman, engine.PermRead), engine.TestNodeInput{Graph: json.RawMessage(g), NodeID: "get"})
	assert.ErrorIs(t, err, engine.ErrForbidden)
}

// A tested call whose executor names no outcome takes it from its answer rule, as in a run.
func TestTestNodeTakesTheOutcomeFromTheAnswer(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	g := graphJSON("Classify", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "cls", "type": "acme.pick", "params": {"text": "sales"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "cls"}, {"from": {"node": "cls", "outcome": "sales"}, "to": "fin"},
		 {"from": {"node": "cls", "outcome": "support"}, "to": "fin"}, {"from": {"node": "cls", "outcome": "failed"}, "to": "fin"}`)
	res, err := p.rt.Authoring().TestNode(context.Background(), ae.human(), engine.TestNodeInput{Graph: json.RawMessage(g), NodeID: "cls"})
	require.NoError(t, err)
	require.Nil(t, res.Error)
	assert.Equal(t, "sales", res.Outcome)
}

// A direct execution (TestNode, an executed node of a live dry run) records a connector's error code
// only as an identifier: provider text in it (here the credentials) is effect_failed in both answers.
func TestDirectCallErrorCodeIsAnIdentifier(t *testing.T) {
	ae := newAcmeEnv(t)
	p := ae.proc()
	g := graphJSON("HTTP", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "get", "type": "acme.http", "params": {"mode": "badcode"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "get"}, {"from": {"node": "get", "outcome": "ok"}, "to": "fin"}, {"from": {"node": "get", "outcome": "failed"}, "to": "fin"}`)
	ctx := context.Background()
	res, err := p.rt.Authoring().TestNode(ctx, ae.human(), engine.TestNodeInput{Graph: json.RawMessage(g), NodeID: "get"})
	require.NoError(t, err)
	require.NotNil(t, res.Error)
	assert.Equal(t, engine.CodeEffectFailed, res.Error.Code)
	assert.NotContains(t, res.Error.Message+string(res.Output)+strings.Join(res.Logs, "\n"), acmeToken)

	dr, err := p.rt.Authoring().DryRun(ctx, ae.human(), engine.DryRunInput{Graph: json.RawMessage(g), Decisions: engine.DecisionsLive,
		Subject: engine.SubjectRef{Entity: "acme.order", Key: "o-1"}, Events: []json.RawMessage{json.RawMessage(`{"kind": "acme.order.created.v1"}`)}})
	require.NoError(t, err)
	var seen strings.Builder
	for _, st := range dr.Trace {
		seen.WriteString(st.Node + " " + st.Outcome + " " + st.Path + " " + st.Message + " " + string(st.Output) + "\n")
		for _, e := range st.Effects {
			seen.Write(e)
		}
	}
	assert.Contains(t, seen.String(), engine.CodeEffectFailed, "the executed node failed with the fallback code")
	assert.NotContains(t, seen.String(), acmeToken)
	assert.Len(t, ae.acme.callsOf("acme.http"), 2, "both executed the node")
}

// A node type with an effect must declare what a dry run does with it.
func TestCatalogRefusesAnUndeclaredDryRun(t *testing.T) {
	spec := graph.NodeSpec{Type: "acme.mystery", Title: "Mystery", Behavior: graph.BehaviorCall, Outcomes: []string{"ok"},
		Params: graph.MustSchemaParams(`{"type":"object","properties":{},"additionalProperties":false}`)}
	reg := catalog.NewRegistry(nil)
	err := reg.RegisterNode(specNode{spec})
	require.Error(t, err)
	var ce *graph.CatalogError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, graph.CodeDryRunUndeclared, ce.Code)
	errs := graph.ValidateCatalog(graph.Catalog{Nodes: map[string]graph.NodeSpec{spec.Type: spec}}, nil)
	require.Len(t, errs, 1)
	assert.Equal(t, graph.CodeDryRunUndeclared, errs[0].Code)
	spec.DryRun = graph.DryRunCapture
	assert.Empty(t, graph.ValidateCatalog(graph.Catalog{Nodes: map[string]graph.NodeSpec{spec.Type: spec}}, nil))
}

// subject.url and run.url are set in the context of a live run.
func TestLiveRunContextHasLinks(t *testing.T) {
	ae := newAcmeEnv(t)
	links := engine.NewLinks("https://erp.test", "")
	require.NoError(t, links.Register("acme.order", func(_ context.Context, _ uuid.UUID, key string) (string, error) {
		return "/orders/" + key, nil
	}))
	p := ae.proc(func(c *engine.Config) { c.Links = links })
	am := ae.create(p, ae.human(), graphJSON("Links", `{"id": "created", "event": "acme.order.created.v1"}`,
		`{"id": "note", "type": "acme.note", "params": {"text": "{{ subject.url }} {{ run.url }} {{ subject.key }}"}}, {"id": "fin", "type": "end", "params": {}}`,
		`{"from": {"node": "created", "outcome": "fired"}, "to": "note"}, {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}`))
	ae.publish(ae.tenant, orderCreated, "o-9", order{Amount: 1}, 0)
	p.until(func() bool { return len(noteTexts(t, ae.env)) == 1 }, "the run notes its links")
	run := runOf(t, ae.env, p)
	assert.Equal(t, []string{fmt.Sprintf("https://erp.test/orders/o-9 https://erp.test/automations/%s/runs/%s o-9", am.ID, run.RunID)}, noteTexts(t, ae.env))
}

// The contract of the context: for every event of the catalog that starts a graph, the fields its
// schema declares required are set on the live path.
func TestRunContextContractOverTheCatalog(t *testing.T) {
	ae := newAcmeEnv(t)
	links := engine.NewLinks("https://erp.test", "")
	p := ae.proc(func(c *engine.Config) { c.Links = links })
	var mu sync.Mutex
	seen := map[string]graph.Context{}
	p.rt.SetContextHook(func(event string, gctx graph.Context) {
		mu.Lock()
		defer mu.Unlock()
		seen[event] = gctx
	})
	view, err := ae.reg.ForTenant(context.Background(), ae.tenant)
	require.NoError(t, err)
	h := ae.handler(p, nil)
	started := 0
	for key, spec := range view.Catalog.Events {
		if spec.NoTrigger || key == engine.EventSchedule || key == engine.EventManual {
			continue
		}
		name := strings.ReplaceAll(key, ".", "_")
		ae.create(p, ae.human(), graphJSON("Contract "+name, `{"id": "t", "event": "`+key+`"}`,
			`{"id": "note", "type": "acme.note", "params": {"text": "x"}}, {"id": "fin", "type": "end", "params": {}}`,
			`{"from": {"node": "t", "outcome": "fired"}, "to": "note"}, {"from": {"node": "note", "outcome": "done"}, "to": "fin"}, {"from": {"node": "note", "outcome": "failed"}, "to": "fin"}`))
		switch key {
		case evCreated:
			ae.publish(ae.tenant, orderCreated, "o-1", order{Amount: 1, Note: "n"}, 0)
		case evPaid:
			ae.publish(ae.tenant, orderPaid, "o-1", order{Amount: 2}, 0)
		case evMessage:
			ae.hook(h, ae.hookPath(), acmeSig, acmeUpdate{ID: "c-1", Text: "hello", From: "ann"})
		default:
			t.Fatalf("the contract test does not publish %s yet", key)
		}
		started++
	}
	p.until(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(seen) == started
	}, "every event ran its graph")
	for key, gctx := range seen {
		assert.Empty(t, engine.CheckRunContext(view.Catalog, key, gctx, true), "the live context of %s", key)
	}
	// The check fails on an event without a required field.
	bad := seen[evMessage]
	bad.Event = map[string]any{"kind": evMessage, "text": "hello"}
	assert.Equal(t, []string{"event.from"}, engine.CheckRunContext(view.Catalog, evMessage, bad, true))
	bad = seen[evMessage]
	bad.Run = map[string]any{"id": "x"}
	assert.Equal(t, []string{"run.url"}, engine.CheckRunContext(view.Catalog, evMessage, bad, true))
}

func TestCheckRunContextReadsNestedFields(t *testing.T) {
	c := graph.Catalog{Events: map[string]graph.EventSpec{"e": {Key: "e", Subject: graph.SubjectNone, Payload: expr.Fields{
		{Name: "a", Field: expr.ObjectOf(expr.Fields{{Name: "b", Field: expr.StringField}, {Name: "c", Field: expr.OptionalOf(expr.StringField)}})},
	}}}}
	ctx := graph.Context{Event: map[string]any{"kind": "e", "a": map[string]any{}}, Subject: map[string]any{"entity": "none", "key": "", "url": ""}, Run: map[string]any{"id": "", "url": ""}}
	assert.Equal(t, []string{"event.a.b"}, engine.CheckRunContext(c, "e", ctx, false))
}
