package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// FailInvalidOutput is the failure code when the output an end node computed does not match the graph's output_schema.
const FailInvalidOutput = "INVALID_OUTPUT"

// outputSpec compiles a graph's output schema: a JSON Schema of type object. nil: the graph has none.
func outputSpec(schema map[string]any) (ParamsSpec, error) {
	if len(schema) == 0 {
		return nil, nil //nolint:nilnil // no schema: the output is untyped
	}
	if t, _ := schema["type"].(string); t != "object" {
		return nil, errors.New(`the output schema is a JSON Schema of "type": "object"`)
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	ps, err := SchemaParams(raw)
	if err != nil {
		return nil, fmt.Errorf("the output schema: %w", err)
	}
	return ps, nil
}

// OutputSchema is the compiled output schema of the graph (Graph.OutputSchema), nil when it has none or
// it does not compile (Validate reports it).
func (v *GraphView) OutputSchema() ParamsSpec { return v.output }

// endOutput is the output an end node gives the run: its rendered output param, by the graph's output
// schema when it has one (defaults applied). ok is false when the node gives no output: the run keeps the
// end's params as its output, as before output existed.
func (s *StepContext) endOutput(rendered map[string]any) (map[string]any, bool, *Step) {
	out, has := rendered["output"].(map[string]any)
	var ps ParamsSpec
	if s.in != nil {
		ps = s.in.view.output
	}
	if ps == nil {
		return out, has, nil
	}
	if out == nil {
		out = map[string]any{}
	}
	in, ok := ownedParams(out)
	if !ok {
		st := failed(FailInvalidOutput, "the output is not JSON")
		return nil, false, &st
	}
	parsed, issues := ps.Parse(in)
	if len(issues) > 0 {
		st := failed(FailInvalidOutput, "the output does not match the graph's output_schema: %s: %s", pathOf("output", issues[0].Path), issues[0].Message)
		return nil, false, &st
	}
	owned, ok := ownedParams(parsed)
	if !ok {
		st := failed(FailInvalidOutput, "the output parsed into values that are not JSON")
		return nil, false, &st
	}
	return owned, true, nil
}

// outputErrors checks the end nodes' output against the graph's output schema: the required names are
// there, no name the schema does not take, and each expression's type is one the property's type takes.
func outputErrors(v *GraphView, x *exprs, each map[string]string) []GraphError {
	g := v.graph
	if len(g.OutputSchema) == 0 {
		return nil
	}
	if _, err := outputSpec(g.OutputSchema); err != nil {
		return []GraphError{{Path: "output_schema", Code: CodeInvalid, Message: err.Error()}}
	}
	props, _ := g.OutputSchema["properties"].(map[string]any)
	closed := false
	if b, ok := g.OutputSchema["additionalProperties"].(bool); ok && !b {
		closed = true
	}
	var required []string
	if rs, ok := g.OutputSchema["required"].([]any); ok {
		for _, r := range rs {
			if name, ok := r.(string); ok {
				required = append(required, name)
			}
		}
	}
	var out []GraphError
	for i, n := range g.Nodes {
		spec, ok := v.catalog.Nodes[n.Type]
		if !ok || spec.Behavior != BehaviorEnd {
			continue
		}
		params, ok := v.Params(n.ID)
		if !ok {
			continue
		}
		at := fmt.Sprintf("nodes[%d].params.output", i)
		given, _ := params["output"].(map[string]any)
		var missing []string
		for _, name := range required {
			if _, ok := given[name]; !ok {
				missing = append(missing, name)
			}
		}
		if len(missing) > 0 {
			out = append(out, GraphError{Path: at, Code: CodeParams, Message: fmt.Sprintf("the graph's output_schema requires %s", strings.Join(missing, ", "))})
		}
		names := make([]string, 0, len(given))
		for name := range given {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			prop, declared := props[name].(map[string]any)
			if !declared {
				if closed {
					out = append(out, GraphError{Path: at + "." + name, Code: CodeParams, Message: fmt.Sprintf("the graph's output_schema has no %s", name)})
				}
				continue
			}
			src, ok := given[name].(string)
			if !ok {
				continue
			}
			p, issues := x.programIn(n.ID, "*", true, each[n.ID], src, expr.TypeAny)
			if len(issues) > 0 {
				continue // the expression's own error, reported with the node's expressions
			}
			if want := schemaTypes(prop["type"]); len(want) > 0 && !celFitsJSON(p.OutputType(), want) {
				out = append(out, GraphError{Path: at + "." + name, Code: CodeExpressionType,
					Message: fmt.Sprintf("%s is %s; the graph's output_schema takes %s", name, p.OutputType(), strings.Join(want, " or "))})
			}
		}
	}
	return out
}

// schemaTypes are the JSON types a schema's "type" names.
func schemaTypes(t any) []string {
	switch x := t.(type) {
	case string:
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, v := range x {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// celFitsJSON reports whether a value of a CEL type may be of one of the JSON types once it is JSON
// (a timestamp is an RFC 3339 string, a duration seconds). A type the check cannot tell (dyn) fits.
func celFitsJSON(cel string, want []string) bool {
	var as []string
	switch {
	case cel == "string", cel == "google.protobuf.Timestamp", cel == "timestamp":
		as = []string{"string"}
	case cel == "int", cel == "uint":
		as = []string{"integer", "number"}
	case cel == "double", cel == "google.protobuf.Duration", cel == "duration":
		as = []string{"number"}
	case cel == "bool":
		as = []string{"boolean"}
	case cel == "null_type":
		as = []string{"null"}
	case strings.HasPrefix(cel, "list("):
		as = []string{"array"}
	case strings.HasPrefix(cel, "map("):
		as = []string{"object"}
	default:
		return true
	}
	for _, w := range want {
		for _, a := range as {
			if w == a {
				return true
			}
		}
	}
	return false
}
