package graph_test

import (
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// mapCatalog is testCatalog with nodes whose params are maps of expressions and templates (a "*" path),
// a node whose output is the keys of its values, a call that carries the step's context and a wait
// that resolves its event into several outcomes.
func mapCatalog(t *testing.T) graph.Catalog {
	t.Helper()
	c := testCatalog(t)
	c.Nodes["set"] = graph.NodeSpec{
		Type: "set", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunExecute, Outcomes: []string{"done"},
		Params:     graph.MustSchemaParams(`{"type":"object","properties":{"values":{"type":"object","additionalProperties":{"type":"string"}}},"required":["values"],"additionalProperties":false}`),
		Values:     []string{"values.*"},
		OutputKeys: &graph.ParamRef{Param: "values"},
	}
	c.Nodes["fetch"] = graph.NodeSpec{
		Type: "fetch", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture, Outcomes: []string{"ok", "failed"}, FailedOutcome: "failed",
		Params:      graph.MustSchemaParams(`{"type":"object","properties":{"headers":{"type":"object","additionalProperties":{"type":"string"}}},"additionalProperties":false}`),
		Templates:   []string{"headers.*"},
		StepContext: true,
	}
	c.Nodes["child"] = graph.NodeSpec{
		Type: "child", Behavior: graph.BehaviorWait, DryRun: graph.DryRunCapture, Outcomes: []string{"done", "failed"}, FailedOutcome: "failed",
		Params: graph.MustSchemaParams(`{"type":"object","properties":{"timeout":{"type":"string","default":"1h"}},"additionalProperties":false}`),
		Wait:   &graph.WaitSpec{Event: graph.ValueOrParam{Value: "order.paid"}, TimeoutParam: "timeout", Matched: "done", TimeoutOutcome: "timeout", Effect: true},
		Output: expr.Fields{{Name: "status", Field: expr.StringField}},
	}
	if err := c.Check(nil); err != nil {
		t.Fatal(err)
	}
	c.Revision = graph.Revision(c, nil)
	return c
}

const mapGraph = `{"name": "Maps", "triggers": [{"id": "created", "event": "order.created"}],
 "nodes": [
  {"id": "calc", "type": "set", "params": {"values": {"double": "event.total * 2.0", "label": "'order ' + order.status"}}},
  {"id": "call", "type": "fetch", "params": {"headers": {"X-Label": "{{ nodes.calc.output.label }}", "X-Total": "{{ event.total }}"}}},
  {"id": "wait", "type": "child", "params": {}},
  {"id": "say", "type": "notify", "params": {"text": "{{ nodes.wait.output.status }} {{ nodes.calc.output.double }}"}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "calc"},
  {"from": {"node": "calc", "outcome": "done"}, "to": "call"},
  {"from": {"node": "call", "outcome": "ok"}, "to": "wait"},
  {"from": {"node": "call", "outcome": "failed"}, "to": "ignore"},
  {"from": {"node": "wait", "outcome": "done"}, "to": "say"},
  {"from": {"node": "wait", "outcome": "failed"}, "to": "say"},
  {"from": {"node": "wait", "outcome": "timeout"}, "to": "ignore"},
  {"from": {"node": "say", "outcome": "done"}, "to": "ignore"},
  {"from": {"node": "say", "outcome": "failed"}, "to": "ignore"}
 ]}`

func TestMapParamsAreCheckedAndEvaluatedPerKey(t *testing.T) {
	c := mapCatalog(t)
	g := mustDecode(t, mapGraph)
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("validate: %v", errs)
	}
	// A wrong expression of one key is reported at that key.
	bad := mustDecode(t, strings.Replace(mapGraph, `"event.total * 2.0"`, `"event.nope * 2.0"`, 1))
	errs := graph.Validate(c, bad, graph.Options{})
	if len(errs) == 0 || errs[0].Path != "nodes[0].params.values.double" {
		t.Fatalf("errors: %v", errs)
	}
	// A template of a key reads only what the key's scope has.
	bad = mustDecode(t, strings.Replace(mapGraph, `{{ nodes.calc.output.label }}`, `{{ nodes.calc.output.nope }}`, 1))
	if errs := graph.Validate(c, bad, graph.Options{}); len(errs) == 0 || errs[0].Path != "nodes[1].params.headers.X-Label" {
		t.Fatalf("errors: %v", errs)
	}

	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Event: graph.Event{"kind": "order.created", "total": 21.5}, Vars: map[string]any{"order": map[string]any{"total": 21.5, "status": "new"}},
		Now: time.Date(2026, 11, 24, 9, 0, 0, 0, time.UTC)}
	st := in.Start("calc", &graph.StepContext{Ctx: ctx})
	if st.Kind != graph.StepOutcome || st.Output["double"] != 43.0 || st.Output["label"] != "order new" {
		t.Fatalf("calc: %+v", st)
	}
	ctx.Nodes = map[string]graph.NodeState{"calc": {Output: st.Output}}
	st = in.Start("call", &graph.StepContext{Ctx: ctx})
	if st.Kind != graph.StepCall {
		t.Fatalf("call: %+v", st)
	}
	headers := st.Effects[0].Params["headers"].(map[string]any)
	if headers["X-Label"] != "order new" || headers["X-Total"] != "21.5" {
		t.Fatalf("headers: %v", headers)
	}
	sc := st.Effects[0].Context
	if sc == nil || sc["now"] != "2026-11-24T09:00:00Z" || sc["event"].(map[string]any)["total"] != 21.5 {
		t.Fatalf("the step's context: %v", sc)
	}
	if outcomes := in.View().Outcomes("wait"); strings.Join(outcomes, ",") != "done,failed,timeout" {
		t.Fatalf("wait outcomes: %v", outcomes)
	}
}

func TestWaitOutcomesMustHoldMatched(t *testing.T) {
	c := mapCatalog(t)
	n := c.Nodes["child"]
	n.Outcomes = []string{"failed"}
	c.Nodes["child"] = n
	if err := c.Check(nil); err == nil || !strings.Contains(err.Error(), "matched outcome") {
		t.Fatalf("check: %v", err)
	}
	n.Outcomes = []string{"done", "failed", "timeout"}
	c.Nodes["child"] = n
	if err := c.Check(nil); err == nil || !strings.Contains(err.Error(), "timeout outcome") {
		t.Fatalf("check: %v", err)
	}
}
