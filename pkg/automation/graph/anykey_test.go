package graph_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// toolCatalog is testCatalog with a call whose arguments are keyed by name (an MCP tool): templates and
// CEL values under any key, a read-only flag for the dry run and a structured output typed by a schema.
func toolCatalog(t *testing.T) graph.Catalog {
	t.Helper()
	c := testCatalog(t)
	c.Nodes["tool"] = graph.NodeSpec{Type: "tool", Title: "Tool", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture,
		Params:    graph.MustSchemaParams(`{"type":"object","properties":{"arguments":{"type":"object"},"pin":{"type":"object"}},"additionalProperties":false}`),
		Templates: []string{"arguments." + graph.AnyKey + ".template"}, Values: []string{"arguments." + graph.AnyKey + ".cel"},
		Outcomes: []string{"done", "failed"}, FailedOutcome: "failed", DoneOutcome: "done",
		Output:        expr.Fields{{Name: "structured", Field: expr.OptionalOf(expr.DynField)}, {Name: "text", Field: expr.StringField}},
		OutputSchemas: map[string]string{"structured": "pin.output_schema"}, ReadOnlyParam: "pin.read_only"}
	if err := c.Check(nil); err != nil {
		t.Fatal(err)
	}
	c.Revision = graph.Revision(c, nil)
	return c
}

func toolGraph(args, pin, notify string) string {
	return `{"name": "Tool", "triggers": [{"id": "created", "event": "order.created"}],
 "nodes": [
  {"id": "call", "type": "tool", "params": {"arguments": ` + args + `, "pin": ` + pin + `}},
  {"id": "say", "type": "notify", "params": {"text": "` + notify + `"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "call"},
  {"from": {"node": "call", "outcome": "done"}, "to": "say"},
  {"from": {"node": "call", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "say", "outcome": "done"}, "to": "fin"},
  {"from": {"node": "say", "outcome": "failed"}, "to": "fin"}
 ]}`
}

func TestAnyKeyParamsAreCheckedAndRendered(t *testing.T) {
	c := toolCatalog(t)
	pin := `{"output_schema": {"type": "object", "properties": {"temp": {"type": "number"}}}, "read_only": true}`
	g := mustDecode(t, toolGraph(`{"city": {"template": "in {{ event.total }}"}, "a.b": {"cel": "event.total + 1.0"}, "lit": {"value": "{{ x }}"}}`, pin,
		"{{ nodes.call.output.structured.temp }}"))
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("templates and CEL under any key check: %v", errs)
	}
	// A wrong expression under a key is reported at that key, a dotted key in brackets.
	bad := mustDecode(t, toolGraph(`{"a.b": {"cel": "event.nope +"}}`, pin, "x"))
	errs := graph.Validate(c, bad, graph.Options{})
	if len(errs) == 0 || errs[0].Path != `nodes[0].params.arguments["a.b"].cel` {
		t.Fatalf("errors %v", errs)
	}
	// The structured output is typed by the pinned schema: a field it lacks does not check.
	typed := mustDecode(t, toolGraph(`{}`, pin, "{{ nodes.call.output.structured.nope }}"))
	if errs := graph.Validate(c, typed, graph.Options{}); len(errs) == 0 {
		t.Fatal("structured is typed by the output schema")
	}

	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 1.0, "status": "new"}}, Now: time.Now()}
	var effects []graph.Effect
	var executed []string
	trace := in.DryRun(graph.Subject{}, ctx, []graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 5.0}}}, graph.DryRunOptions{Modes: true,
		Execute: func(node graph.Node, e []graph.Effect) graph.ExecAnswer {
			executed, effects = append(executed, node.ID), e
			return graph.ExecAnswer{Outcome: "done", Output: map[string]any{"text": "", "structured": map[string]any{"temp": 20.0}}}
		}})
	if len(executed) != 1 {
		t.Fatalf("a read-only call executes in a dry run: %v", trace)
	}
	raw, err := json.Marshal(effects[0].Params["arguments"])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"city":{"template":"in 5"}`, `"a.b":{"cel":6}`, `"lit":{"value":"{{ x }}"}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("arguments %s lack %s", raw, want)
		}
	}

	// Not read-only: captured.
	g2 := mustDecode(t, toolGraph(`{}`, `{}`, "x"))
	in2, err := graph.NewInterpreter(c, g2, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	executed = nil
	trace = in2.DryRun(graph.Subject{}, ctx, []graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 5.0}}}, graph.DryRunOptions{Modes: true,
		Execute: func(node graph.Node, _ []graph.Effect) graph.ExecAnswer {
			executed = append(executed, node.ID)
			return graph.ExecAnswer{Outcome: "done"}
		}})
	if len(executed) != 0 || trace[1].Status != graph.StatusCaptured {
		t.Fatalf("a call that is not read-only is captured: %v", trace)
	}
}
