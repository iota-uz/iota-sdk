package backtest_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/automation/backtest"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// provider is a paid decision provider. With dedupe it answers a repeated idempotency key with the first
// call's answer and charges once; hold, when set, keeps the first call in flight until it is closed.
type provider struct {
	dedupe bool
	mu     sync.Mutex
	keys   []string // every key the provider was called with, in order
	seen   map[string]bool
	charge int // the calls charged
	hold   chan struct{}
	inside chan struct{} // closed when the first call is in flight
}

func newProvider(dedupe bool) *provider {
	return &provider{dedupe: dedupe, seen: map[string]bool{}, inside: make(chan struct{})}
}

func (p *provider) call(key string) {
	p.mu.Lock()
	first := len(p.keys) == 0
	p.keys = append(p.keys, key)
	if !p.dedupe || !p.seen[key] {
		p.charge++
	}
	p.seen[key] = true
	hold := p.hold
	p.mu.Unlock()
	if first && hold != nil {
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

func (p *provider) setHold(h chan struct{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hold = h
}

// decide is a decision node of provider; keyed reports provider-side deduplication
// (catalog.IdempotencyKeyed).
type decide struct {
	p     *provider
	keyed bool
}

func (decide) Spec() graph.NodeSpec {
	return graph.NodeSpec{Type: "acme.decide", Title: "Decide", Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute,
		Params: graph.MustSchemaParams(textParams), Templates: []string{"text"}, Outcomes: []string{"ok", "failed"}, FailedOutcome: "failed",
		Output: expr.Fields{{Name: "label", Field: expr.OptionalOf(expr.StringField)}}}
}
func (decide) Requires() []catalog.ConnectorKind { return nil }
func (decide) Idempotent() bool                  { return true }
func (d decide) SupportsIdempotencyKey() bool    { return d.keyed }
func (d decide) Execute(_ context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	d.p.call(in.IdempotencyKey)
	return catalog.CallResult{Outcome: "ok", Output: map[string]any{"label": "ok"}}, nil
}
func (decide) DecisionKey(in catalog.CallInput) (string, bool) {
	text, _ := in.Params["text"].(string)
	return engine.DecisionKey("decide", text, "acme-1", nil)
}

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
	ctx := context.Background()
	a := e.human()
	am, _, err := e.rt.Service().Create(ctx, a, engine.CreateInput{Graph: json.RawMessage(graphTwoDecisions)})
	require.NoError(t, err)
	ds, err := e.svc.CreateDataset(ctx, a, backtest.CreateDatasetInput{AutomationID: am.ID, Name: "d"})
	require.NoError(t, err)
	_, err = e.svc.AddSynthetic(ctx, a, ds, backtest.SyntheticEvent{Kind: evMessage, Payload: json.RawMessage(`{"text": "hello"}`)})
	require.NoError(t, err)
	return ds
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

// stallMidCall runs one worker until its first provider call is in flight, takes the lease from it while
// the call is still being made, then lets the call finish and waits for that worker to stop.
func stallMidCall(t *testing.T, e *env, p *provider, id backtest.BacktestID) {
	t.Helper()
	hold := make(chan struct{})
	p.setHold(hold)
	done := make(chan error, 1)
	go func() {
		_, err := e.svc.RunOnce(context.Background())
		done <- err
	}()
	select {
	case <-p.inside:
	case <-time.After(30 * time.Second):
		t.Fatal("the first call was not made")
	}
	supersede(t, e, id)
	close(hold)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("the superseded worker did not stop")
	}
	p.setHold(nil)
}

// A worker whose lease is taken while a live call is in flight makes that call (it cannot be recalled)
// but no further one, and keeps no result. The worker that takes the backtest over repeats the call
// under the same idempotency key, so a provider that deduplicates by key is paid once for it.
func TestSupersededLeaseMidCallIsPaidOnce(t *testing.T) {
	p := newProvider(true)
	e := newFenceEnv(t, p, true, true)
	id, err := runLive(t, e, datasetOfOne(t, e))
	require.NoError(t, err)

	stallMidCall(t, e, p, id)
	first := p.calls()
	require.Len(t, first, 1, "the superseded worker makes no call after the one in flight")
	assert.Contains(t, first[0], "backtest:"+id.String()+":")
	assert.Contains(t, first[0], ":d1:1")
	assert.Zero(t, resultCount(t, e, id), "the superseded worker keeps no result")

	// The next worker claims the backtest and runs the example again.
	expire(t, e, id)
	e.drain()
	keys := p.calls()
	require.Len(t, keys, 3, "d1 repeated, then d2")
	assert.Equal(t, first[0], keys[1], "the repeat carries the key of the superseded call")
	assert.Contains(t, keys[2], ":d2:1")
	assert.Equal(t, 2, p.charged(), "every decision is paid once")
	rep, err := e.svc.Report(context.Background(), e.human(), id)
	require.NoError(t, err)
	assert.Equal(t, backtest.StatusDone, rep.Backtest.Status, rep.Backtest.Error)
	assert.Equal(t, 1, resultCount(t, e, id))
}

// The residual risk of a provider that cannot deduplicate (fencing not strict): the call in flight when
// the lease passed is paid again by the next worker; the rest of the example is not.
func TestSupersededLeaseWithoutDedupePaysOnlyTheCallInFlightTwice(t *testing.T) {
	p := newProvider(false)
	e := newFenceEnv(t, p, false, false)
	id, err := runLive(t, e, datasetOfOne(t, e))
	require.NoError(t, err)

	stallMidCall(t, e, p, id)
	require.Len(t, p.calls(), 1)
	expire(t, e, id)
	e.drain()
	keys := p.calls()
	require.Len(t, keys, 3)
	assert.Equal(t, keys[0], keys[1], "the same key; the provider ignores it")
	assert.Equal(t, 3, p.charged(), "the call in flight is paid twice, nothing else")
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
