// Package set is the built-in node type set: named values computed by CEL expressions in the step, put in
// nodes.<id>.output for the nodes after it.
package set

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/engine"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// Type is the node type.
const Type = "set"

const paramsSchema = `{"type": "object", "properties": {
	"values": {"type": "object", "minProperties": 1, "maxProperties": 50,
		"propertyNames": {"pattern": "^[a-z_][a-zA-Z0-9_]{0,63}$"},
		"additionalProperties": {"type": "string", "minLength": 1, "maxLength": 2000, "description": "CEL"},
		"description": "name → CEL expression; the output has each name with its value"}
}, "required": ["values"], "additionalProperties": false}`

// Node is the set node type.
type Node struct{}

var (
	_ catalog.NodeType      = Node{}
	_ engine.EffectNode     = Node{}
	_ catalog.LocalizedNode = Node{}
)

// New is the set node type.
func New() Node { return Node{} }

func (Node) Spec() graph.NodeSpec {
	return graph.NodeSpec{
		Type: Type, Title: "Set values", Group: "data", Behavior: graph.BehaviorEffect,
		Description: "Computes named values with CEL; the next nodes read them as nodes.<id>.output.<name>.",
		Params:      graph.MustSchemaParams(paramsSchema),
		Outcomes:    []string{"done"}, DoneOutcome: "done",
		Values:     []string{"values.*"},
		OutputKeys: &graph.ParamRef{Param: "values"},
		DryRun:     graph.DryRunExecute,
	}
}

func (Node) Requires() []catalog.ConnectorKind { return nil }

func (Node) Localize(spec graph.NodeSpec, locale string) graph.NodeSpec {
	switch locale {
	case "ru":
		spec.Title, spec.Description = "Задать значения", "Вычисляет именованные значения выражениями CEL; следующие ноды читают их как nodes.<id>.output.<имя>."
	case "uz":
		spec.Title, spec.Description = "Qiymatlarni belgilash", "CEL ifodalari bilan nomli qiymatlarni hisoblaydi; keyingi tugunlar ularni nodes.<id>.output.<nom> sifatida o‘qiydi."
	case "uz-Cyrl":
		spec.Title, spec.Description = "Қийматларни белгилаш", "CEL ифодалари билан номли қийматларни ҳисоблайди; кейинги тугунлар уларни nodes.<id>.output.<ном> сифатида ўқийди."
	}
	return spec
}

// ApplyEffect returns the evaluated values as the node's output; it writes nothing.
func (Node) ApplyEffect(_ context.Context, _ pgx.Tx, _ engine.StepContext, _ graph.Node, effect graph.Effect) (map[string]any, error) {
	values, _ := effect.Params["values"].(map[string]any)
	out := make(map[string]any, len(values))
	for k, v := range values {
		out[k] = v
	}
	return out, nil
}
