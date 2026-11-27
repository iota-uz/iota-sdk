package backtest_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// provider is a paid decision provider. With dedupe it answers a repeated idempotency key with the first
// call's answer and charges once; hold, when set, keeps the holdAt-th call in flight until it is closed.
type provider struct {
	dedupe bool
	mu     sync.Mutex
	keys   []string // every key the provider was called with, in order
	seen   map[string]bool
	charge int // the calls charged
	hold   chan struct{}
	holdAt int           // the call (1-based) hold keeps in flight
	inside chan struct{} // closed when that call is in flight
}

func newProvider(dedupe bool) *provider {
	return &provider{dedupe: dedupe, seen: map[string]bool{}, inside: make(chan struct{})}
}

func (p *provider) call(key string) {
	p.mu.Lock()
	p.keys = append(p.keys, key)
	held := len(p.keys) == p.holdAt
	if !p.dedupe || !p.seen[key] {
		p.charge++
	}
	p.seen[key] = true
	hold := p.hold
	p.mu.Unlock()
	if held && hold != nil {
		close(p.inside)
		// A provider that took the request finishes it whether or not the caller still waits.
		<-hold
	}
}

func (p *provider) calls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.keys...)
}

func (p *provider) charged() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.charge
}

func (p *provider) setHold(h chan struct{}, at int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hold, p.holdAt = h, at
}

// decide is a decision node of provider; keyed reports provider-side deduplication
// (catalog.IdempotencyKeyed).
type decide struct {
	p     *provider
	keyed bool
}

func (decide) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.decide", Title: "Decide", Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute,
		Params: graph.MustSchemaParams(decideParams), Templates: []string{"text", "note"}, Outcomes: []string{"ok", "failed"}, FailedOutcome: "failed",
		Output: expr.Fields{{Name: "label", Field: expr.OptionalOf(expr.StringField)}}}
}
func (decide) Requires() []catalog.ConnectorKind { return nil }
func (decide) Idempotent() bool                  { return true }
func (d decide) SupportsIdempotencyKey() bool    { return d.keyed }
func (d decide) Execute(_ context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	d.p.call(in.IdempotencyKey)
	return catalog.CallResult{Outcome: "ok", Output: map[string]any{"label": "ok"}}, nil
}

// DecisionKey is the decision of the text; the note is not part of it (the provider ignores it). A text
// that says nokey has no stable form.
func (decide) DecisionKey(in catalog.CallInput) (string, bool) {
	text, _ := in.Params["text"].(string)
	if strings.Contains(text, "nokey") {
		return "", false
	}
	return engine.DecisionKey("decide", text, "acme-1", nil)
}

const decideParams = `{"type":"object","properties":{"text":{"type":"string"},"note":{"type":"string"}},"additionalProperties":false}`

// graphTwoDecisions decides twice in a row on every message.
const graphTwoDecisions = `{"name": "Twice", "triggers": [{"id": "m", "event": "acme.message.v1"}],
 "nodes": [
  {"id": "d1", "type": "acme.decide", "params": {"text": "{{ event.text }}"}},
  {"id": "d2", "type": "acme.decide", "params": {"text": "again {{ event.text }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "m", "outcome": "fired"}, "to": "d1"},
  {"from": {"node": "d1", "outcome": "ok"}, "to": "d2"}, {"from": {"node": "d1", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "d2", "outcome": "ok"}, "to": "fin"}, {"from": {"node": "d2", "outcome": "failed"}, "to": "fin"}
 ]}`

func newFenceEnv(t *testing.T, p *provider, keyed, strict bool) *env {
	t.Helper()
	return newEnvWith(t, func(_ *env, reg catalog.Registry, cfg *backtest.Config) {
		require.NoError(t, reg.RegisterNode(decide{p: p, keyed: keyed}))
		cfg.StrictFencing = strict
	})
}

// datasetOfOne is a dataset of graphTwoDecisions with one example.
func datasetOfOne(t *testing.T, e *env) backtest.DatasetID {
	t.Helper()
	_, ds := datasetOf(t, e, graphTwoDecisions, "hello")
	return ds
}

// datasetOf is an automation of the graph and a dataset of it with one message.
func datasetOf(t *testing.T, e *env, g, text string) (uuid.UUID, backtest.DatasetID) {
	t.Helper()
	ctx := context.Background()
	a := e.human()
	am, _, err := e.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(g)})
	require.NoError(t, err)
	ds, err := e.svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "d"})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]string{"text": text})
	require.NoError(t, err)
	_, err = e.svc.AddSynthetic(ctx, a, ds, backtest.SyntheticEvent{Kind: evMessage, Payload: payload})
	require.NoError(t, err)
	return am.ID, ds
}

// runLive queues a live backtest of the dataset.
func runLive(t *testing.T, e *env, ds backtest.DatasetID) (backtest.BacktestID, error) {
	t.Helper()
	ctx := context.Background()
	est, err := e.svc.Estimate(ctx, e.human(), backtest.RunInput{DatasetID: ds, Decisions: backtest.DecisionsLive})
	require.NoError(t, err)
	return e.svc.Run(ctx, e.human(), backtest.RunInput{DatasetID: ds, Decisions: backtest.DecisionsLive, Revision: est.Revision})
}

// supersede takes the backtest from a worker that is still running it: the lease another worker's claim
// gives.
func supersede(t *testing.T, e *env, id backtest.BacktestID) {
	t.Helper()
	_, err := e.db.Exec(context.Background(), `UPDATE automation.backtests SET lease = gen_random_uuid() WHERE id = $1`, id)
	require.NoError(t, err)
}

// expire lets the next worker claim the backtest, as when the one that holds it stalled past LeaseTTL.
func expire(t *testing.T, e *env, id backtest.BacktestID) {
	t.Helper()
	_, err := e.db.Exec(context.Background(), `UPDATE automation.backtests SET heartbeat_at = now() - interval '1 hour' WHERE id = $1`, id)
	require.NoError(t, err)
}

func resultCount(t *testing.T, e *env, id backtest.BacktestID) int {
	t.Helper()
	var n int
	require.NoError(t, e.db.QueryRow(context.Background(), `SELECT count(*) FROM automation.backtest_results WHERE backtest_id = $1`, id).Scan(&n))
	return n
}

// stallMidCall runs one worker until its at-th provider call is in flight, takes the lease from it while
// the call is still being made, then lets the call finish and waits for that worker to stop.
func stallMidCall(t *testing.T, e *env, p *provider, id backtest.BacktestID, at int) {
	t.Helper()
	hold := make(chan struct{})
	p.setHold(hold, at)
	done := make(chan error, 1)
	go func() {
		_, err := e.svc.RunOnce(context.Background())
		done <- err
	}()
	select {
	case <-p.inside:
	case <-time.After(30 * time.Second):
		t.Fatal("the held call was not made")
	}
	supersede(t, e, id)
	close(hold)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("the superseded worker did not stop")
	}
	p.setHold(nil, 0)
}

// The lease passes while a worker is inside a live call of the example: after d1's call (at 1), or
// after d1 completed and while d2's call is in flight (at 2). The superseded worker makes no call after
// the one in flight and keeps no result; the next worker replays the example from its trigger, every
// call under the key of the superseded one's same call.
func TestSupersededLeaseMidCall(t *testing.T) {
	for _, tc := range []struct {
		name   string
		at     int // the call in flight when the lease passes
		dedupe bool
		paid   int // the calls charged in the end
	}{
		// A provider that deduplicates by key is paid once per decision, whatever was in flight.
		{name: "first call, dedupe", at: 1, dedupe: true, paid: 2},
		{name: "second call, dedupe", at: 2, dedupe: true, paid: 2},
		// The residual risk of one that cannot (fencing not strict): every call the superseded worker
		// made in the example is paid again, the completed prefix included; nothing after them.
		{name: "first call, no dedupe", at: 1, dedupe: false, paid: 3},
		{name: "second call, no dedupe", at: 2, dedupe: false, paid: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := newProvider(tc.dedupe)
			e := newFenceEnv(t, p, tc.dedupe, tc.dedupe)
			id, err := runLive(t, e, datasetOfOne(t, e))
			require.NoError(t, err)

			stallMidCall(t, e, p, id, tc.at)
			stalled := p.calls()
			require.Len(t, stalled, tc.at, "the superseded worker makes no call after the one in flight")
			assert.Contains(t, stalled[0], "backtest:"+id.String()+":")
			assert.Contains(t, stalled[0], ":d1@#1:")
			assert.Zero(t, resultCount(t, e, id), "the superseded worker keeps no result")

			// The next worker claims the backtest and runs the example again from its trigger.
			expire(t, e, id)
			e.drain()
			keys := p.calls()
			require.Len(t, keys, tc.at+2, "d1 and d2 again")
			assert.Equal(t, stalled, keys[tc.at:tc.at+len(stalled)], "each repeat carries the key of the superseded call")
			assert.Contains(t, keys[len(keys)-1], ":d2@#1:")
			assert.Equal(t, tc.paid, p.charged())
			rep, err := e.svc.Report(context.Background(), e.human(), id)
			require.NoError(t, err)
			assert.Equal(t, backtest.StatusDone, rep.Backtest.Status, rep.Backtest.Error)
			assert.Equal(t, 1, resultCount(t, e, id))
		})
	}
}

// Strict fencing refuses up front a backtest that would pay a provider that cannot deduplicate.
func TestStrictFencingRefusesUnfencedProviders(t *testing.T) {
	p := newProvider(false)
	e := newFenceEnv(t, p, false, true)
	ds := datasetOfOne(t, e)
	_, err := runLive(t, e, ds)
	require.ErrorIs(t, err, engine.ErrUnfenced)
	_, err = e.svc.Run(context.Background(), e.human(), backtest.RunInput{DatasetID: ds, Decisions: backtest.DecisionsCached})
	require.ErrorIs(t, err, engine.ErrUnfenced, "a person's cached backtest pays its misses")
	assert.Empty(t, p.calls())

	// An agent's cached backtest reads the cache only: nothing to fence.
	_, err = e.svc.Run(context.Background(), e.actor(2, engine.OriginAgent), backtest.RunInput{DatasetID: ds, Decisions: backtest.DecisionsCached})
	require.NoError(t, err)
	e.drain()
	assert.Empty(t, p.calls())
}

// graphWithMemory is graphTwoDecisions whose d1 reads the automation's memory: in its note (which the
// provider ignores: not part of the decision) or in its text (the prompt itself).
func graphWithMemory(field string) string {
	return strings.Replace(graphTwoDecisions, `"params": {"text": "{{ event.text }}"}`,
		map[string]string{
			"note": `"params": {"text": "{{ event.text }}", "note": "{{ state.get('tag') }}"}`,
			"text": `"params": {"text": "{{ event.text }} {{ state.get('tag') }}"}`,
		}[field], 1)
}

// takeOverAfterMemoryChange stalls the first worker in d1's call, changes the memory tag, and lets the
// next worker replay the example; it returns the keys the provider got.
func takeOverAfterMemoryChange(t *testing.T, field string) (*provider, []string) {
	t.Helper()
	p := newProvider(true)
	e := newFenceEnv(t, p, true, true)
	ctx := context.Background()
	am, ds := datasetOf(t, e, graphWithMemory(field), "hello")
	_, err := e.rt.Memory().Set(ctx, e.human(), am, "tag", json.RawMessage(`"a"`), 0)
	require.NoError(t, err)
	id, err := runLive(t, e, ds)
	require.NoError(t, err)
	stallMidCall(t, e, p, id, 1)
	_, err = e.rt.Memory().Set(ctx, e.human(), am, "tag", json.RawMessage(`"b"`), 0)
	require.NoError(t, err)
	expire(t, e, id)
	e.drain()
	rep, err := e.svc.Report(ctx, e.human(), id)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusDone, rep.Backtest.Status, rep.Backtest.Error)
	keys := p.calls()
	require.Len(t, keys, 3)
	return p, keys
}

// What changes between two holders but is not part of the request the provider gets (a note the decision
// ignores) keeps the key: the repeat is paid once.
func TestFencedRepeatIgnoresWhatTheRequestIgnores(t *testing.T) {
	p, keys := takeOverAfterMemoryChange(t, "note")
	assert.Equal(t, keys[0], keys[1])
	assert.Equal(t, 2, p.charged())
}

// A request that changed between two holders (the prompt reads the memory) gets another key: the provider
// answers it, not with the first answer.
func TestFencedRepeatOfAChangedRequestIsANewCall(t *testing.T) {
	p, keys := takeOverAfterMemoryChange(t, "text")
	assert.NotEqual(t, keys[0], keys[1])
	assert.Equal(t, 3, p.charged(), "two different d1 requests and one d2")
}

// Strict fencing makes no call whose request has no stable form: a key of its own would not match the
// next holder's repeat. The backtest fails before the provider is called.
func TestStrictFencingRefusesARequestWithoutStableForm(t *testing.T) {
	p := newProvider(true)
	e := newFenceEnv(t, p, true, true)
	_, ds := datasetOf(t, e, graphTwoDecisions, "nokey")
	id, err := runLive(t, e, ds)
	require.NoError(t, err)
	e.drain()
	rep, err := e.svc.Report(context.Background(), e.human(), id)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusFailed, rep.Backtest.Status)
	assert.Contains(t, rep.Backtest.Error, "stable form")
	assert.Empty(t, p.calls())
}

// graphForeach decides once per item of a foreach, with the text given (a template over the item, or a
// constant); item 0 skips the decision when the memory says skip.
func graphForeach(text string) string {
	return `{"name": "Each", "triggers": [{"id": "m", "event": "acme.message.v1"}],
 "nodes": [
  {"id": "each", "type": "foreach", "params": {"items": "[\"x\", \"y\"]"}},
  {"id": "skip", "type": "branch", "params": {"expression": "foreach.each.index == 0 && state.get('skip') == true"}},
  {"id": "d", "type": "acme.decide", "params": {"text": "` + text + `"}},
  {"id": "take", "type": "collect", "params": {"of": "each"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "m", "outcome": "fired"}, "to": "each"},
  {"from": {"node": "each", "outcome": "item"}, "to": "skip"},
  {"from": {"node": "skip", "outcome": "true"}, "to": "take"},
  {"from": {"node": "skip", "outcome": "false"}, "to": "d"},
  {"from": {"node": "d", "outcome": "ok"}, "to": "take"}, {"from": {"node": "d", "outcome": "failed"}, "to": "take"},
  {"from": {"node": "each", "outcome": "done"}, "to": "fin"},
  {"from": {"node": "each", "outcome": "partial_failed"}, "to": "fin"},
  {"from": {"node": "each", "outcome": "failed"}, "to": "fin"}
 ]}`
}

// A replay that skips an earlier item's call keeps the key of a later item's unchanged call: the key
// names the item (its token), not how many calls of the node came before it.
func TestFencedRepeatKeepsItsKeyWhenAnEarlierCallIsSkipped(t *testing.T) {
	p := newProvider(true)
	e := newFenceEnv(t, p, true, true)
	ctx := context.Background()
	am, ds := datasetOf(t, e, graphForeach("{{ foreach.each.item }}"), "hello")
	id, err := runLive(t, e, ds)
	require.NoError(t, err)
	// The first worker decided item 0 and is inside item 1's call when the lease passes.
	stallMidCall(t, e, p, id, 2)
	stalled := p.calls()
	require.Len(t, stalled, 2)
	assert.Contains(t, stalled[0], ":d@each[0]#1:")
	assert.Contains(t, stalled[1], ":d@each[1]#1:")

	// Before the next worker replays, item 0 stops reaching the decision.
	_, err = e.rt.Memory().Set(ctx, e.human(), am, "skip", json.RawMessage(`true`), 0)
	require.NoError(t, err)
	expire(t, e, id)
	e.drain()
	keys := p.calls()
	require.Len(t, keys, 3, "the replay decides item 1 only")
	assert.Equal(t, stalled[1], keys[2], "item 1's repeat carries its first key")
	assert.Equal(t, 2, p.charged(), "item 1 is paid once")
	rep, err := e.svc.Report(ctx, e.human(), id)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusDone, rep.Backtest.Status, rep.Backtest.Error)
}

// Two occurrences of a node with the same request (two items, one prompt) are two calls: their keys
// differ by their item, so a deduplicating provider answers and charges each.
func TestFencedOccurrencesWithTheSamePayloadAreDistinctCalls(t *testing.T) {
	p := newProvider(true)
	e := newFenceEnv(t, p, true, true)
	_, ds := datasetOf(t, e, graphForeach("same"), "hello")
	id, err := runLive(t, e, ds)
	require.NoError(t, err)
	e.drain()
	keys := p.calls()
	require.Len(t, keys, 2)
	assert.NotEqual(t, keys[0], keys[1])
	assert.Equal(t, 2, p.charged())
	rep, err := e.svc.Report(context.Background(), e.human(), id)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusDone, rep.Backtest.Status, rep.Backtest.Error)
}
