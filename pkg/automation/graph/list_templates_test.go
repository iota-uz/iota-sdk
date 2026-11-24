package graph_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// A template path with [] reaches the items of a list: each is checked when a graph is saved and rendered
// when the node starts.
func TestTemplatesAndValuesInsideAList(t *testing.T) {
	c := testCatalog(t)
	params := graph.MustSchemaParams(`{"type":"object","additionalProperties":false,"required":["items"],"properties":{
	  "items":{"type":"array","items":{"type":"object","additionalProperties":false,"properties":{"label":{"type":"string"},"n":{"type":"string"}}}}}}`)
	c.Nodes["lister"] = graph.NodeSpec{
		Type: "lister", Title: "Lister", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunCapture, Params: params,
		Templates: []string{"items[].label"}, Values: []string{"items[].n"}, Outcomes: []string{"done", "failed"}, DoneOutcome: "done", FailedOutcome: "failed",
		Output: expr.Fields{},
	}
	if err := c.Check(nil); err != nil {
		t.Fatal(err)
	}
	const raw = `{"name":"l","triggers":[{"id":"created","event":"order.created"}],
	 "nodes":[{"id":"l","type":"lister","params":{"items":[{"label":"A {{ event.total }}","n":"1 + 2"},{"label":"B {{ event.total }}","n":"event.total"}]}}],
	 "edges":[{"from":{"node":"created","outcome":"fired"},"to":"l"},{"from":{"node":"l","outcome":"done"},"to":"ignore"},{"from":{"node":"l","outcome":"failed"},"to":"ignore"}]}`
	g := mustDecode(t, raw)
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("validate: %v", errs)
	}
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Now: time.Now(), Event: graph.Event{"kind": "order.created", "total": 7}}
	st := in.Start("l", &graph.StepContext{Ctx: ctx})
	if st.Kind != graph.StepOutcome || len(st.Effects) != 1 {
		t.Fatalf("step: %+v", st)
	}
	items, _ := st.Effects[0].Params["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items: %+v", st.Effects[0].Params)
	}
	first, second := items[0].(map[string]any), items[1].(map[string]any)
	if first["label"] != "A 7" || second["label"] != "B 7" {
		t.Fatalf("labels: %v %v", first, second)
	}
	if first["n"] != int64(3) && first["n"] != 3 && first["n"] != float64(3) {
		t.Fatalf("a value inside a list: %#v", first["n"])
	}

	// A broken template inside an item is reported at that item.
	bad := mustDecode(t, `{"name":"l","triggers":[{"id":"created","event":"order.created"}],
	 "nodes":[{"id":"l","type":"lister","params":{"items":[{"label":"ok"},{"label":"{{ event.nope }}"}]}}],
	 "edges":[{"from":{"node":"created","outcome":"fired"},"to":"l"},{"from":{"node":"l","outcome":"done"},"to":"ignore"},{"from":{"node":"l","outcome":"failed"},"to":"ignore"}]}`)
	errs := graph.Validate(c, bad, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[0].params.items[1].label" {
		t.Fatalf("errors: %v", errs)
	}
}
