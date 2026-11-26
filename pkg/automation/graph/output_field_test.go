package graph_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// codedParams refuses its params with a code of its own.
type codedParams struct{}

func (codedParams) JSONSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (codedParams) Parse(map[string]any) (map[string]any, []graph.Issue) {
	return nil, []graph.Issue{{Path: []any{"model"}, Code: "model_no_vision", Message: "no vision"}}
}

// An issue of a ParamsSpec keeps its code; one without a code is params.
func TestIssueCodeReachesTheGraphError(t *testing.T) {
	c := testCatalog(t)
	c.Nodes["odd"] = graph.NodeSpec{Type: "odd", Behavior: graph.BehaviorEnd, Params: codedParams{}}
	g := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "n", "type": "odd", "params": {}}], "edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "n"}]}`)
	errs := graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[0].params.model" || errs[0].Code != "model_no_vision" {
		t.Errorf("a coded issue: %v", errs)
	}
}

// OutputFrom with a Field types only that field of the output from the schema in the param.
func TestOutputFromField(t *testing.T) {
	c := testCatalog(t)
	c.Nodes["gen"] = graph.NodeSpec{
		Type: "gen", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture, Outcomes: []string{"done", "failed"},
		Params: graph.MustSchemaParams(`{"type": "object", "properties": {"schema": {"type": "object"}}}`),
		Output: expr.Fields{
			{Name: "text", Field: expr.OptionalOf(expr.StringField)},
			{Name: "json", Field: expr.DynField},
			{Name: "model", Field: expr.StringField},
		},
		OutputFrom: &graph.ParamRef{Param: "schema", Field: "json"},
	}
	doc := func(schema, expression string) graph.Graph {
		return mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
			"nodes": [{"id": "g", "type": "gen", "params": {"schema": `+schema+`}},
				{"id": "b", "type": "branch", "params": {"expression": "`+expression+`"}}],
			"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "g"},
				{"from": {"node": "g", "outcome": "done"}, "to": "b"}, {"from": {"node": "g", "outcome": "failed"}, "to": "ignore"},
				{"from": {"node": "b", "outcome": "true"}, "to": "ignore"}, {"from": {"node": "b", "outcome": "false"}, "to": "ignore"}]}`)
	}
	schema := `{"type": "object", "properties": {"score": {"type": "integer"}, "tag": {"type": "string"}}, "required": ["score"]}`
	for _, ok := range []string{"nodes.g.output.json.score > 1", "nodes.g.output.model == 'x'", "nodes.g.output.json.tag == 'a'"} {
		if errs := graph.Validate(c, doc(schema, ok), graph.Options{}); len(errs) > 0 {
			t.Errorf("%s: %v", ok, errs)
		}
	}
	for _, bad := range []string{"nodes.g.output.json.missing > 1", "nodes.g.output.json.score == 'a'", "nodes.g.output.nope == 1"} {
		if errs := graph.Validate(c, doc(schema, bad), graph.Options{}); len(errs) == 0 {
			t.Errorf("%s was accepted", bad)
		}
	}
	// A schema outside the subset is refused where it is written.
	errs := graph.Validate(c, doc(`{"type": "object", "properties": {"a": {"type": "string", "pattern": "x"}}}`, "true"), graph.Options{})
	if len(errs) == 0 || !strings.HasPrefix(errs[0].Path, "nodes[0].params.schema") {
		t.Errorf("a schema outside the subset: %v", errs)
	}
	// The editor's view keeps the declared output and names the param the field is typed by.
	for _, n := range graph.View(c, nil).Nodes {
		if n.Type == "gen" && (n.OutputFrom == nil || n.OutputFrom.Field != "json" || len(n.Output) == 0) {
			t.Errorf("the view of the node: %+v", n)
		}
	}
}
