package automations_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/automations/presentation/graphs"
	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// decideCalls counts the real calls of acme.decide, a node that decides like a model: it executes in a dry run
// and its decision is cached.
var decideCalls atomic.Int64

type decideNode struct{}

func (decideNode) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: "acme.decide", Title: "Decide", Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute,
		Outcomes: []string{"done", "failed"}, DoneOutcome: "done", FailedOutcome: "failed",
		Params:    graph.MustSchemaParams(`{"type":"object","properties":{"prompt":{"type":"string"}},"required":["prompt"],"additionalProperties":false}`),
		Templates: []string{"prompt"},
		Output:    expr.Fields{{Name: "answer", Field: expr.StringField}},
	}
}
func (decideNode) Requires() []catalog.ConnectorKind { return nil }
func (decideNode) Idempotent() bool                  { return true }
func (decideNode) Execute(_ context.Context, in catalog.CallInput) (catalog.CallResult, error) {
	decideCalls.Add(1)
	prompt, _ := in.Params["prompt"].(string)
	return catalog.CallResult{Outcome: "done", Output: map[string]any{"answer": "decided: " + prompt}}, nil
}
func (decideNode) DecisionKey(in catalog.CallInput) (string, bool) {
	prompt, _ := in.Params["prompt"].(string)
	return engine.DecisionKey(prompt, nil, "test-model", nil)
}

func decideGraph() map[string]any {
	return map[string]any{
		"name": "Decide", "run_as": "owner",
		"triggers": []any{map[string]any{"id": "start", "event": engine.EventManual}},
		"nodes": []any{
			map[string]any{"id": "ask", "type": "acme.decide", "params": map[string]any{"prompt": "classify {{ event.input.what }}"}},
			map[string]any{"id": "fin", "type": "end", "params": map[string]any{}},
		},
		"edges": []any{
			map[string]any{"from": map[string]any{"node": "start", "outcome": "fired"}, "to": "ask"},
			map[string]any{"from": map[string]any{"node": "ask", "outcome": "done"}, "to": "fin"},
			map[string]any{"from": map[string]any{"node": "ask", "outcome": "failed"}, "to": "ignore"},
		},
	}
}

// The dry run RPC selects how decisions are taken: cached reuses the decision of an earlier run; the test
// node RPC returns the request the node sent.
func TestDryRunRPCSelectsCachedDecisionsAndTestNodeShowsTheRequest(t *testing.T) {
	s, _ := app(t, allAutomations...)
	event := json.RawMessage(`{"kind": "` + engine.EventManual + `", "input": {"what": "a refund"}}`)
	dry := func(decisions graphs.Decisions) *rpcReply {
		return rpc(t, s, graphs.MethodDryRun, map[string]any{
			"graph": decideGraph(), "subject": map[string]any{"entity": graph.SubjectNone, "key": ""}, "events": []json.RawMessage{event}, "decisions": decisions,
		}, nil)
	}
	before := decideCalls.Load()
	for i := 0; i < 2; i++ {
		var out graphs.DryRunResponse
		r := rpc(t, s, graphs.MethodDryRun, map[string]any{
			"graph": decideGraph(), "subject": map[string]any{"entity": graph.SubjectNone, "key": ""}, "events": []json.RawMessage{event}, "decisions": graphs.DecisionsCached,
		}, &out)
		require.Nil(t, r.Error, "run %d", i)
		var step graphs.TraceStep
		for _, st := range out.Trace {
			if st.Node == "ask" {
				step = st
			}
		}
		assert.Equal(t, "done", step.Outcome, "run %d", i)
		assert.Contains(t, string(step.Output), "decided: classify a refund", "run %d: a cached decision keeps its output", i)
	}
	assert.EqualValues(t, 1, decideCalls.Load()-before, "the second dry run took the decision from the cache")

	assert.NotNil(t, dry("sometimes").Error, "an unknown mode is refused")

	var tested graphs.TestNodeResponse
	r := rpc(t, s, graphs.MethodTestNode, map[string]any{
		"graph": decideGraph(), "node_id": "ask", "context": map[string]any{"event": map[string]any{"input": map[string]any{"what": "an exchange"}}},
	}, &tested)
	require.Nil(t, r.Error)
	require.Nil(t, tested.Error)
	assert.Equal(t, "done", tested.Outcome)
	assert.Contains(t, string(tested.Request), "classify an exchange", "the rendered prompt")
}
