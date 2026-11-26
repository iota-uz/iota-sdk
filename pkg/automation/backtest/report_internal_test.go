package backtest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
)

// A branch that still waits at the end of the trace makes the run waiting, whatever another branch
// ended; a wait that was woken does not.
func TestAnalyzeWaitsByToken(t *testing.T) {
	split := []engine.TraceStep{
		{Node: "m", Kind: engine.TraceTriggered, Outcome: "fired"},
		{Node: "w", Token: "split.a", Kind: engine.TraceWait},
		{Node: "fin", Token: "split.b", Kind: engine.TraceEnded},
	}
	assert.Equal(t, EndWaiting, analyze(split, nil).end)
	woken := []engine.TraceStep{
		{Node: "m", Kind: engine.TraceTriggered, Outcome: "fired"},
		{Node: "w", Kind: engine.TraceWait},
		{Node: "w", Kind: engine.TraceWoke, Outcome: "matched"},
		{Node: "fin", Kind: engine.TraceEnded},
	}
	assert.Equal(t, EndEnded, analyze(woken, nil).end)
	failed := []engine.TraceStep{{Node: "m", Kind: engine.TraceTriggered}, {Node: "x", Kind: engine.TraceError, Message: "boom"}}
	a := analyze(failed, nil)
	assert.Equal(t, EndError, a.end)
	assert.True(t, a.triggered)
}

// Two runs that send from the same node with another text differ; the same effects with their keys in
// another order do not.
func TestDiffComparesEffectParams(t *testing.T) {
	x := scored{end: EndEnded, outcomes: map[string]string{"send": "sent"}, effects: []Effect{{Node: "send", Params: json.RawMessage(`{"text": "hi", "to": "a"}`)}}}
	y := x
	y.effects = []Effect{{Node: "send", Params: json.RawMessage(`{"to": "a", "text": "hi"}`)}}
	_, differ := diffOf(x, y)
	assert.False(t, differ)
	y.effects = []Effect{{Node: "send", Params: json.RawMessage(`{"to": "a", "text": "hello"}`)}}
	d, differ := diffOf(x, y)
	assert.True(t, differ)
	assert.True(t, d.ParamsChanged)
	assert.Equal(t, []string{"send"}, d.EffectsB)
}

// A module's source is offered for an automation its events start, or for any when it names none.
func TestOfferedFor(t *testing.T) {
	assert.True(t, offeredFor(HistorySource{Name: "crm"}, []string{"email.received.v1"}))
	assert.False(t, offeredFor(HistorySource{Name: "crm", Events: []string{"crm.client.created.v1"}}, []string{"email.received.v1"}))
	assert.True(t, offeredFor(HistorySource{Name: "crm", Events: []string{"crm.client.created.v1"}}, []string{"email.received.v1", "crm.client.created.v1"}))
}

// Numbers compare exactly: two integers past float64's precision differ, 1 and 1.0 do not.
func TestSameJSONComparesNumbersExactly(t *testing.T) {
	assert.False(t, sameJSON(json.RawMessage(`{"n": 9007199254740992}`), json.RawMessage(`{"n": 9007199254740993}`)))
	assert.True(t, sameJSON(json.RawMessage(`{"n": 1, "a": [true, null]}`), json.RawMessage(`{"a": [true, null], "n": 1.0}`)))
}

// Precision and recall from one pass match the counts of the pairs.
func TestMetricsCountsInOnePass(t *testing.T) {
	rows := []scored{
		{outcomes: map[string]string{"c": "a"}, labels: map[string]Label{"c": {Outcome: "a"}}},
		{outcomes: map[string]string{"c": "a"}, labels: map[string]Label{"c": {Outcome: "b"}}},
		{outcomes: map[string]string{"c": "b"}, labels: map[string]Label{"c": {Outcome: "b"}}},
		{outcomes: map[string]string{}, labels: map[string]Label{"c": {Outcome: "b"}}},
	}
	m := metricsOf("c", false, rows, nil)
	byOutcome := map[string]OutcomeMetrics{}
	for _, o := range m.Outcomes {
		byOutcome[o.Outcome] = o
	}
	assert.Equal(t, OutcomeMetrics{Outcome: "a", TruePositives: 1, FalsePositives: 1, Support: 1}, OutcomeMetrics{Outcome: "a", TruePositives: byOutcome["a"].TruePositives, FalsePositives: byOutcome["a"].FalsePositives, FalseNegatives: byOutcome["a"].FalseNegatives, Support: byOutcome["a"].Support})
	assert.Equal(t, 1, byOutcome["b"].TruePositives)
	assert.Equal(t, 2, byOutcome["b"].FalseNegatives)
	assert.Equal(t, 3, byOutcome["b"].Support)
}
