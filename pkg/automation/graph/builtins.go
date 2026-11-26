package graph

import (
	"encoding/json"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// MaxWaitMs is the longest a wait or a delay may sleep: 30 days.
const MaxWaitMs = 30 * 86_400_000

const (
	expressionSchema = `{"type": "string", "minLength": 1, "maxLength": 2000, "pattern": "^\\S([\\s\\S]*\\S)?$", "description": "CEL; it may span lines"}`
	outcomeSchema    = `{"type": "string", "pattern": "^[a-z][a-z0-9_]{0,39}$"}`
	durationSchema   = `{"type": "string", "pattern": "^[1-9][0-9]{0,5}(s|m|h|d)$", "x-max-ms": 2592000000, "description": "30s, 20m, 2h or 7d; at most 30 days"}`
)

func branchSchema() string {
	return `{"type": "object", "properties": {"expression": ` + expressionSchema + `}, "required": ["expression"], "additionalProperties": false}`
}

func switchSchema() string {
	return `{"type": "object", "properties": {
		"expression": ` + expressionSchema + `,
		"cases": {"type": "array", "minItems": 1, "maxItems": 20, "x-unique": "value", "items": {
			"type": "object",
			"properties": {"value": {"type": "string", "maxLength": 200}, "outcome": ` + outcomeSchema + `},
			"required": ["value", "outcome"], "additionalProperties": false}}
	}, "required": ["expression", "cases"], "additionalProperties": false}`
}

func waitSchema(events []string) string {
	if len(events) == 0 {
		events = []string{""}
	}
	ev, err := json.Marshal(events)
	if err != nil {
		panic(err)
	}
	return `{"type": "object", "properties": {
		"event": {"enum": ` + string(ev) + `},
		"filter": ` + expressionSchema + `,
		"timeout": ` + durationSchema + `
	}, "required": ["event"], "additionalProperties": false}`
}

func delaySchema() string {
	return `{"type": "object", "properties": {"duration": ` + durationSchema + `}, "required": ["duration"], "additionalProperties": false}`
}

func endSchema() string {
	return `{"type": "object", "properties": {
		"reason": {"type": "string", "maxLength": 500},
		"output": {"type": "object", "maxProperties": 50,
			"propertyNames": {"pattern": "^[a-z_][a-zA-Z0-9_]{0,63}$"},
			"additionalProperties": {"type": "string", "minLength": 1, "maxLength": 2000, "description": "CEL"},
			"description": "the run's output: name → CEL; by the graph's output_schema when it has one"}
	}, "additionalProperties": false}`
}

func branchNode(subjects []string) NodeSpec {
	return NodeSpec{
		Type: "branch", Title: "Branch", Behavior: BehaviorBranch,
		Description: "A CEL expression of type bool: outcome true or false.",
		Params:      MustSchemaParams(branchSchema()), Subjects: subjects, DryRun: DryRunExecute,
		Expressions: []ExprParam{{Param: "expression", Type: expr.TypeBool}},
	}
}

func switchNode(subjects []string) NodeSpec {
	return NodeSpec{
		Type: "switch", Title: "Switch", Behavior: BehaviorSwitch,
		Description: "A CEL expression of type string: the outcome of the first case with that value, else default.",
		Params:      MustSchemaParams(switchSchema()), Subjects: subjects, DryRun: DryRunExecute,
		Expressions: []ExprParam{{Param: "expression", Type: expr.TypeString}},
	}
}

func waitNode(subjects, events []string) NodeSpec {
	return NodeSpec{
		Type: "wait", Title: "Wait", Behavior: BehaviorWait,
		Description: "Sleeps until an event of this kind about the run's subject passes the filter, or the timeout.",
		Params:      MustSchemaParams(waitSchema(events)), Subjects: subjects, DryRun: DryRunExecute,
		Expressions: []ExprParam{{Param: "filter", Type: expr.TypeBool}},
		Wait: &WaitSpec{
			Event: ValueOrParam{Param: "event"}, Filter: ValueOrParam{Param: "filter"}, TimeoutParam: "timeout",
			Matched: "matched", TimeoutOutcome: "timeout",
		},
	}
}

func delayNode(subjects []string) NodeSpec {
	return NodeSpec{
		Type: "delay", Title: "Delay", Behavior: BehaviorWait,
		Description: "Sleeps for a duration, then continues with done.",
		Params:      MustSchemaParams(delaySchema()), Subjects: subjects, DryRun: DryRunExecute,
		Wait: &WaitSpec{TimeoutParam: "duration", TimeoutOutcome: "done"},
	}
}

func endNode(subjects []string) NodeSpec {
	return NodeSpec{
		Type: "end", Title: "End", Behavior: BehaviorEnd,
		Description: "Ends the run; its output, when set, is the run's output (a run_automation reads it).",
		Params:      MustSchemaParams(endSchema()), Subjects: subjects, DryRun: DryRunExecute,
		Values: []string{"output.*"},
	}
}

// BuiltinNodes are the node types every catalog may take: branch, switch, wait (on one of waitEvents),
// delay (a wait without an event) and end, available with these subjects (empty: any).
func BuiltinNodes(subjects []string, waitEvents []string) []NodeSpec {
	return []NodeSpec{
		branchNode(subjects),
		switchNode(subjects),
		waitNode(subjects, waitEvents),
		delayNode(subjects),
		endNode(subjects),
	}
}

// BuiltinsJSONSchema is the params schema of each built-in node type (BuiltinNodes and FlowNodes), by
// type; wait's event is any key.
func BuiltinsJSONSchema() json.RawMessage {
	wait := `{"type": "object", "properties": {
		"event": {"type": "string", "minLength": 1, "description": "an event key of the catalog"},
		"filter": ` + expressionSchema + `,
		"timeout": ` + durationSchema + `
	}, "required": ["event"], "additionalProperties": false}`
	out := fmt.Sprintf(`{"branch": %s, "switch": %s, "wait": %s, "delay": %s, "end": %s, "parallel": %s, "join": %s, "foreach": %s, "collect": %s}`,
		branchSchema(), switchSchema(), wait, delaySchema(), endSchema(), parallelSchema(), joinSchema(), foreachSchema(), collectSchema())
	var compact json.RawMessage
	if err := json.Unmarshal([]byte(out), &compact); err != nil {
		panic(err)
	}
	b, err := json.Marshal(compact)
	if err != nil {
		panic(err)
	}
	return b
}
