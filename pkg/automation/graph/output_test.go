package graph_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// outputGraph ends a run on order.created with the end's output by an output schema.
func outputGraph(t *testing.T, schema, output string) graph.Graph {
	t.Helper()
	raw := `{"name": "Out", "triggers": [{"id": "created", "event": "order.created"}],
	  "nodes": [{"id": "fin", "type": "end", "params": {"reason": "done", "output": ` + output + `}}],
	  "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "fin"}]`
	if schema != "" {
		raw += `, "output_schema": ` + schema
	}
	return mustDecode(t, raw+`}`)
}

const totalSchema = `{"type": "object", "properties": {"total": {"type": "number", "maximum": 1000}, "label": {"type": "string"}},
  "required": ["total", "label"], "additionalProperties": false}`

func errorsAt(errs []graph.GraphError) map[string]string {
	out := map[string]string{}
	for _, e := range errs {
		out[e.Path] = e.Code
	}
	return out
}

// The output schema is part of the graph's version: decoded, kept by a round trip, and the end nodes'
// output is checked against it when the graph is saved.
func TestEndOutputIsCheckedAgainstTheOutputSchema(t *testing.T) {
	c := testCatalog(t)
	g := outputGraph(t, totalSchema, `{"total": "event.total", "label": "'big'"}`)
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("a valid output: %v", errs)
	}
	raw, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	if again := mustDecode(t, string(raw)); again.OutputSchema["type"] != "object" {
		t.Fatalf("the round trip lost the output schema: %s", raw)
	}

	cases := []struct {
		name, schema, output, path, code string
	}{
		{"a required name is missing", totalSchema, `{"total": "event.total"}`, "nodes[0].params.output", graph.CodeParams},
		{"a name the schema does not take", totalSchema, `{"total": "event.total", "label": "'x'", "extra": "1"}`, "nodes[0].params.output.extra", graph.CodeParams},
		{"a type the property does not take", totalSchema, `{"total": "'many'", "label": "'x'"}`, "nodes[0].params.output.total", graph.CodeExpressionType},
		{"a list for a string", totalSchema, `{"total": "1", "label": "[1, 2]"}`, "nodes[0].params.output.label", graph.CodeExpressionType},
		{"a schema that is not an object", `{"type": "array"}`, `{"total": "1"}`, "output_schema", graph.CodeInvalid},
		{"a schema that does not compile", `{"type": "object", "properties": {"a": {"type": 7}}}`, `{"a": "1"}`, "output_schema", graph.CodeInvalid},
		{"an expression that does not compile", "", `{"total": "event.nope +"}`, "nodes[0].params.output.total", graph.CodeExpressionSyntax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := graph.Validate(c, outputGraph(t, tc.schema, tc.output), graph.Options{})
			if got := errorsAt(errs)[tc.path]; got != tc.code {
				t.Fatalf("want %s at %s, got %v", tc.code, tc.path, errs)
			}
		})
	}
	// An int fits a number; an open schema takes other names; no schema: the output is untyped.
	open := `{"type": "object", "properties": {"total": {"type": "number"}}}`
	for _, g := range []graph.Graph{
		outputGraph(t, open, `{"total": "1", "more": "'x'"}`),
		outputGraph(t, "", `{"anything": "[event.total]"}`),
	} {
		if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
			t.Fatalf("%v", errs)
		}
	}
}

// The end node's output is evaluated when the run ends, and a value the schema refuses fails the run.
func TestEndOutputIsTheRunsOutput(t *testing.T) {
	c := testCatalog(t)
	run := func(g graph.Graph, total float64) []graph.TraceStep {
		t.Helper()
		in, err := graph.NewInterpreter(c, g, graph.Options{})
		if err != nil {
			t.Fatal(err)
		}
		return in.DryRun(graph.Subject{Entity: "order", Key: "o-1"}, graph.Context{Vars: map[string]any{"order": map[string]any{"total": total, "status": "new"}}},
			[]graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": total}}}, graph.DryRunOptions{})
	}
	last := func(trace []graph.TraceStep) graph.TraceStep {
		t.Helper()
		for i := len(trace) - 1; i >= 0; i-- {
			if trace[i].Node == "fin" {
				return trace[i]
			}
		}
		t.Fatalf("no step of fin: %+v", trace)
		return graph.TraceStep{}
	}

	end := last(run(outputGraph(t, totalSchema, `{"total": "event.total * 2.0", "label": "'big'"}`), 150))
	if end.Outcome != graph.TraceEnd {
		t.Fatalf("the run ends: %+v", end)
	}
	out, _ := end.End["output"].(map[string]any)
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"label":"big","total":300}` {
		t.Fatalf("the output: %s", b)
	}
	if end.End["reason"] != "done" {
		t.Fatalf("the reason stays: %v", end.End)
	}

	failed := last(run(outputGraph(t, totalSchema, `{"total": "event.total * 2.0", "label": "'big'"}`), 900))
	if failed.Outcome != graph.TraceFailed || !strings.Contains(mustJSON(t, failed.ContextPatch), graph.FailInvalidOutput) {
		t.Fatalf("an output over the schema's maximum fails the run: %+v", failed)
	}

	// Without an output the end gives its params, as before.
	legacy := last(run(mustDecode(t, `{"name": "Old", "triggers": [{"id": "created", "event": "order.created"}],
	  "nodes": [{"id": "fin", "type": "end", "params": {"reason": "done"}}],
	  "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "fin"}]}`), 1))
	if _, ok := legacy.End["output"]; ok || legacy.End["reason"] != "done" {
		t.Fatalf("the legacy end: %+v", legacy.End)
	}
}
