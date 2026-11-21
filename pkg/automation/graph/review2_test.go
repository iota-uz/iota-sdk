package graph_test

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// nanParams parses any params into values that are not JSON.
type nanParams struct{}

func (nanParams) JSONSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (nanParams) Parse(map[string]any) (map[string]any, []graph.Issue) {
	return map[string]any{"choices": []string{"approve"}, "x": math.NaN()}, nil
}

type pollBehavior struct{}

func (pollBehavior) Behavior() graph.Behavior { return "poll" }
func (pollBehavior) OutcomeRule(graph.NodeSpec) graph.OutcomeRule {
	return graph.OutcomeRule{Fixed: []string{"matched", "timeout"}}
}
func (pollBehavior) Validate(*graph.GraphView, graph.Node, graph.NodeSpec) []graph.GraphError {
	return nil
}
func (pollBehavior) Start(*graph.StepContext, graph.Node, graph.NodeSpec) graph.Step {
	return graph.Step{Kind: graph.StepWait, Wait: &graph.WaitIntent{Event: "order.paid"}}
}

type finishBehavior struct{}

func (finishBehavior) Behavior() graph.Behavior { return "finish" }
func (finishBehavior) OutcomeRule(graph.NodeSpec) graph.OutcomeRule {
	return graph.OutcomeRule{Fixed: []string{}}
}
func (finishBehavior) Validate(*graph.GraphView, graph.Node, graph.NodeSpec) []graph.GraphError {
	return nil
}
func (finishBehavior) Start(*graph.StepContext, graph.Node, graph.NodeSpec) graph.Step {
	return graph.Step{Kind: graph.StepEnd, End: map[string]any{}}
}

// Review round 2 of #1358.
func TestReviewRoundTwoGraph(t *testing.T) {
	one := func(node string) graph.Graph {
		return mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
			"nodes": [{"id": "n", "type": "`+node+`", "params": {}}],
			"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "n"}]}`)
	}

	// Params that are not JSON are refused, not kept with shared containers.
	c := testCatalog(t)
	c.Nodes["odd"] = graph.NodeSpec{Type: "odd", Behavior: graph.BehaviorEnd, Params: nanParams{}}
	errs := graph.Validate(c, one("odd"), graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[0].params" || errs[0].Code != graph.CodeParams {
		t.Errorf("non-JSON params: %v", errs)
	}

	// Registered behaviors: one that waits allows a cycle, one with no outcomes ends a path.
	b, err := graph.NewBehaviors(append(graph.DefaultBehaviorImpls(), pollBehavior{}, finishBehavior{})...)
	if err != nil {
		t.Fatal(err)
	}
	c = testCatalog(t)
	empty := graph.MustSchemaParams(`{"type": "object"}`)
	c.Nodes["poll"] = graph.NodeSpec{Type: "poll", Behavior: "poll", Params: empty, Wait: &graph.WaitSpec{Event: graph.ValueOrParam{Value: "order.paid"}, Matched: "matched", TimeoutOutcome: "timeout"}}
	c.Nodes["finish"] = graph.NodeSpec{Type: "finish", Behavior: "finish", Params: empty}
	if errs := graph.Validate(c, one("finish"), graph.Options{Behaviors: b}); len(errs) > 0 {
		t.Errorf("a terminal behavior: %v", errs)
	}
	poll := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "p", "type": "poll", "params": {}}],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "p"}, {"from": {"node": "p", "outcome": "matched"}, "to": "p"},
			{"from": {"node": "p", "outcome": "timeout"}, "to": "ignore"}]}`)
	if errs := graph.Validate(c, poll, graph.Options{Behaviors: b}); len(errs) > 0 {
		t.Errorf("a waiting behavior in a cycle: %v", errs)
	}

	// An event wait with a timeout its schema let through, but that is no duration, is refused.
	c = testCatalog(t)
	w := c.Nodes["wait"]
	w.Params = graph.MustSchemaParams(`{"type": "object", "properties": {"event": {"type": "string"}, "timeout": {"type": "string"}}, "required": ["event"]}`)
	c.Nodes["wait"] = w
	tw := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "w", "type": "wait", "params": {"event": "order.paid", "timeout": "tomorrow"}}],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "w"}, {"from": {"node": "w", "outcome": "matched"}, "to": "ignore"},
			{"from": {"node": "w", "outcome": "timeout"}, "to": "ignore"}]}`)
	if errs := graph.Validate(c, tw, graph.Options{}); len(errs) != 1 || errs[0].Path != "nodes[0].params.timeout" {
		t.Errorf("an invalid timeout: %v", errs)
	}

	// A filter listed as a string expression is still a bool.
	c = testCatalog(t)
	w = c.Nodes["wait"]
	w.Expressions = []graph.ExprParam{{Param: "filter", Type: expr.TypeString}}
	c.Nodes["wait"] = w
	sg := strings.Replace(orderGraph, `"event.amount >= order.total"`, `"'yes'"`, 1)
	if errs := graph.Validate(c, mustDecode(t, sg), graph.Options{}); len(errs) != 1 || errs[0].Path != "nodes[1].params.filter" {
		t.Errorf("a string filter: %v", errs)
	}

	// FreeID never gives ignore; defaults apply inside array items; match values are trimmed of NBSP.
	if id := graph.FreeID(graph.Graph{}, graph.Ignore); id == graph.Ignore {
		t.Error("FreeID returned ignore")
	}
	p := graph.MustSchemaParams(`{"type": "object", "properties": {"rules": {"type": "array", "items": {"type": "object",
		"properties": {"enabled": {"type": "boolean", "default": true}}, "required": ["enabled"]}}}}`)
	got, issues := p.Parse(map[string]any{"rules": []any{map[string]any{}}})
	if len(issues) > 0 || got["rules"].([]any)[0].(map[string]any)["enabled"] != true {
		t.Errorf("defaults in items: %v %v", got, issues)
	}
	mp, err := graph.MatchParams(map[string]graph.MatchField{"tags": {Payload: "tag", Type: graph.MatchStrings}})
	if err != nil {
		t.Fatal(err)
	}
	if _, issues := mp.Parse(map[string]any{"tags": []any{" vip "}}); len(issues) != 1 {
		t.Errorf("NBSP-padded match value: %v", issues)
	}

	// A generated failure output fits the output limit.
	c = testCatalog(t)
	ask, err := graph.FieldsParams(expr.Fields{{Name: "q", Field: expr.StringField}})
	if err != nil {
		t.Fatal(err)
	}
	c.Nodes["ask"] = graph.NodeSpec{Type: "ask", Behavior: graph.BehaviorCall, Params: ask, Outcomes: []string{"ok", "failed"}, FailedOutcome: "failed"}
	cg := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "a", "type": "ask", "params": {"q": "?"}}],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "a"}, {"from": {"node": "a", "outcome": "ok"}, "to": "ignore"},
			{"from": {"node": "a", "outcome": "failed"}, "to": "ignore"}]}`)
	in, err := graph.NewInterpreter(c, cg, graph.Options{Limits: graph.Limits{MaxValueBytes: 64}})
	if err != nil {
		t.Fatal(err)
	}
	trace := in.DryRun(graph.Subject{}, graph.Context{}, []graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 1}}},
		graph.DryRunOptions{CallError: func(graph.Effect) string { return strings.Repeat("x", 100) }})
	if len(trace) < 2 || trace[1].Outcome != "failed" {
		t.Fatalf("trace: %+v", trace)
	}
	b2, err := json.Marshal(trace[1].ContextPatch["output"])
	if err != nil {
		t.Fatal(err)
	}
	if b := b2; len(b) > 64 {
		t.Errorf("failed output has %d bytes: %s", len(b), b)
	}
}

// Review round 4: a built-in call needs outcomes, and only a waiting kind may carry a WaitSpec.
func TestBuiltinKindsDeclareNoFakeEnds(t *testing.T) {
	c := testCatalog(t)
	empty := graph.MustSchemaParams(`{"type": "object"}`)
	c.Nodes["ask"] = graph.NodeSpec{Type: "ask", Behavior: graph.BehaviorCall, Params: empty}
	if err := c.Check(nil); err == nil {
		t.Error("Check accepts a call with no outcomes")
	}
	g := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "a", "type": "ask", "params": {}}], "edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "a"}]}`)
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) == 0 {
		t.Error("a call with no outcomes validates as an end")
	}
	c = testCatalog(t)
	n := c.Nodes["notify"]
	n.Wait = &graph.WaitSpec{Event: graph.ValueOrParam{Value: "order.paid"}, Matched: "done"}
	c.Nodes["notify"] = n
	if err := c.Check(nil); err == nil {
		t.Error("Check accepts an effect with a WaitSpec")
	}
	loop := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "e", "type": "notify", "params": {"text": "hi"}}],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "e"}, {"from": {"node": "e", "outcome": "done"}, "to": "e"},
			{"from": {"node": "e", "outcome": "failed"}, "to": "ignore"}]}`)
	errs := graph.Validate(c, loop, graph.Options{})
	var cycle bool
	for _, e := range errs {
		cycle = cycle || e.Code == graph.CodeCycleWithoutWait
	}
	if !cycle {
		t.Errorf("an effect loop with a WaitSpec is not a cycle without wait: %v", errs)
	}
}
