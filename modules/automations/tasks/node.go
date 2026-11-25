package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// HumanNode is the human node type of the engine (engine.NewHumanNode) whose params also check the form
// annotation: a form that names a kind no TaskForm provides is an issue of the graph, so a graph with it
// is not saved.
func HumanNode(forms FormLookup) catalog.NodeType {
	return humanNode{NodeType: engine.NewHumanNode(), forms: forms}
}

type humanNode struct {
	catalog.NodeType
	forms FormLookup
}

var _ engine.HumanNode = humanNode{}

func (h humanNode) Human() bool { return true }

func (h humanNode) Spec() graph.NodeSpec {
	spec := h.NodeType.Spec()
	spec.Params = formParams{ParamsSpec: spec.Params, forms: h.forms}
	return spec
}

// formParams adds the check of the form annotation to the params of the human node.
type formParams struct {
	graph.ParamsSpec
	forms FormLookup
}

func (p formParams) Parse(raw map[string]any) (map[string]any, []graph.Issue) {
	out, issues := p.ParamsSpec.Parse(raw)
	form, present := raw["form"]
	if !present || form == nil {
		return out, issues
	}
	b, err := json.Marshal(form)
	if err != nil {
		return out, issues
	}
	kind, ok := FormKindOf(b)
	path := []any{"form", FormAnnotation}
	switch {
	case !ok:
		issues = append(issues, graph.Issue{Path: path, Message: FormAnnotation + " names a task form: a non-empty string"})
	case kind == "":
	case p.forms == nil:
		issues = append(issues, graph.Issue{Path: path, Message: fmt.Sprintf("task form %q is not registered in this application", kind)})
	default:
		_, found, err := p.forms.Form(kind)
		if err != nil {
			issues = append(issues, graph.Issue{Path: path, Message: "the task forms of this application cannot be read: " + err.Error()})
		} else if !found {
			issues = append(issues, graph.Issue{Path: path, Message: fmt.Sprintf("task form %q is not registered in this application", kind)})
		}
	}
	return out, issues
}
