package graph

import (
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

type branchBehavior struct{}

func (branchBehavior) Behavior() Behavior { return BehaviorBranch }
func (branchBehavior) OutcomeRule(NodeSpec) OutcomeRule {
	return OutcomeRule{Fixed: []string{"true", "false"}}
}
func (branchBehavior) Validate(*GraphView, Node, NodeSpec) []GraphError { return nil }
func (branchBehavior) Start(s *StepContext, node Node, _ NodeSpec) Step {
	src, _ := s.Params(node)["expression"].(string)
	v, st := s.Eval(src, expr.TypeBool)
	if st != nil {
		return *st
	}
	b, ok := v.(bool)
	if !ok {
		return failed(FailExpression, "the expression is not a bool")
	}
	if b {
		return Step{Kind: StepOutcome, Outcome: "true"}
	}
	return Step{Kind: StepOutcome, Outcome: "false"}
}

type switchBehavior struct{}

func (switchBehavior) Behavior() Behavior { return BehaviorSwitch }
func (switchBehavior) OutcomeRule(NodeSpec) OutcomeRule {
	return OutcomeRule{From: &ParamRef{Param: "cases", Field: "outcome"}, Fixed: []string{"default"}}
}
func (switchBehavior) Validate(*GraphView, Node, NodeSpec) []GraphError { return nil }
func (switchBehavior) Start(s *StepContext, node Node, _ NodeSpec) Step {
	params := s.Params(node)
	src, _ := params["expression"].(string)
	v, st := s.Eval(src, expr.TypeString)
	if st != nil {
		return *st
	}
	value, err := expr.RenderValue(v, expr.DynField)
	if err != nil {
		return failed(FailExpression, "%v", err)
	}
	cases, _ := params["cases"].([]any)
	for _, c := range cases {
		m, _ := c.(map[string]any)
		if m["value"] == value {
			outcome, _ := m["outcome"].(string)
			return Step{Kind: StepOutcome, Outcome: outcome}
		}
	}
	return Step{Kind: StepOutcome, Outcome: "default"}
}

type waitBehavior struct{}

func (waitBehavior) Behavior() Behavior { return BehaviorWait }
func (waitBehavior) OutcomeRule(spec NodeSpec) OutcomeRule {
	w := spec.Wait
	if w == nil {
		return OutcomeRule{Fixed: []string{}}
	}
	if w.Event.IsZero() {
		return OutcomeRule{Fixed: []string{w.TimeoutOutcome}}
	}
	r := OutcomeRule{Fixed: []string{w.Matched}}
	if w.TimeoutParam != "" {
		r.Optional = &OptionalOutcome{Param: w.TimeoutParam, Outcome: w.TimeoutOutcome}
	}
	return r
}
func (waitBehavior) Validate(v *GraphView, node Node, spec NodeSpec) []GraphError {
	if spec.Wait == nil {
		return []GraphError{{Path: v.Path(node.ID) + ".type", Code: CodeInvalid, Message: node.Type + " declares no wait"}}
	}
	params, _ := v.Params(node.ID)
	w := waitIntent(spec, params)
	if spec.Wait.Event.IsZero() {
		if w.Timeout == 0 {
			return []GraphError{{Path: v.Path(node.ID) + ".params." + spec.Wait.TimeoutParam, Code: CodeParams, Message: "a delay needs a duration"}}
		}
		return nil
	}
	if _, ok := v.catalog.Events[w.Event]; !ok {
		path := v.Path(node.ID) + ".type"
		if spec.Wait.Event.Param != "" {
			path = v.Path(node.ID) + ".params." + spec.Wait.Event.Param
		}
		return []GraphError{{Path: path, Code: CodeUnknownEvent, Message: fmt.Sprintf("%s is not an event of the catalog", w.Event)}}
	}
	return nil
}
func (waitBehavior) Start(s *StepContext, node Node, spec NodeSpec) Step {
	w := waitIntent(spec, s.Params(node))
	if w.Event == "" && w.Timeout == 0 {
		return failed(FailInvalidGraph, "node %s waits for nothing", node.ID)
	}
	st := Step{Kind: StepWait, Wait: &w}
	if spec.Wait.Effect {
		e, stop := s.Effect(node, spec)
		if stop != nil {
			return *stop
		}
		st.Effects = []Effect{e}
	}
	return st
}

type effectBehavior struct{}

func (effectBehavior) Behavior() Behavior { return BehaviorEffect }
func (effectBehavior) OutcomeRule(spec NodeSpec) OutcomeRule {
	if len(spec.Outcomes) > 0 {
		return OutcomeRule{Fixed: append([]string(nil), spec.Outcomes...)}
	}
	return OutcomeRule{Fixed: []string{doneOutcome(spec)}}
}

// Validate: an effect's done and failed outcomes are among its declared outcomes, or a run would take an
// outcome that has no edge.
func (effectBehavior) Validate(v *GraphView, node Node, spec NodeSpec) []GraphError {
	if err := effectOutcomesError(spec); err != "" {
		return []GraphError{{Path: v.Path(node.ID) + ".type", Code: CodeInvalid, Message: node.Type + ": " + err}}
	}
	return nil
}

func effectOutcomesError(spec NodeSpec) string {
	if len(spec.Outcomes) == 0 {
		if spec.FailedOutcome != "" && spec.FailedOutcome != doneOutcome(spec) {
			return "failed_outcome " + spec.FailedOutcome + " is not one of its outcomes"
		}
		return ""
	}
	if !contains(spec.Outcomes, doneOutcome(spec)) {
		return "done outcome " + doneOutcome(spec) + " is not one of its outcomes"
	}
	if spec.FailedOutcome != "" && !contains(spec.Outcomes, spec.FailedOutcome) {
		return "failed_outcome " + spec.FailedOutcome + " is not one of its outcomes"
	}
	return ""
}
func (effectBehavior) Start(s *StepContext, node Node, spec NodeSpec) Step {
	e, stop := s.Effect(node, spec)
	if stop != nil {
		return *stop
	}
	return Step{Kind: StepOutcome, Outcome: doneOutcome(spec), Output: map[string]any{}, Effects: []Effect{e}}
}

func doneOutcome(spec NodeSpec) string {
	if spec.DoneOutcome != "" {
		return spec.DoneOutcome
	}
	return "done"
}

type callBehavior struct{}

func (callBehavior) Behavior() Behavior { return BehaviorCall }
func (callBehavior) OutcomeRule(spec NodeSpec) OutcomeRule {
	r := OutcomeRule{Fixed: append([]string{}, spec.Outcomes...)}
	if spec.OutcomesFrom != nil {
		from := *spec.OutcomesFrom
		r.From = &from
	}
	return r
}

// Validate: each value a call takes its outcomes from is one outcome: an id, listed once, not one of the
// node's fixed outcomes.
func (callBehavior) Validate(v *GraphView, node Node, spec NodeSpec) []GraphError {
	if spec.OutcomesFrom == nil {
		return nil
	}
	params, _ := v.Params(node.ID)
	list, _ := params[spec.OutcomesFrom.Param].([]any)
	var out []GraphError
	seen := map[string]bool{}
	for k, item := range list {
		val := item
		if spec.OutcomesFrom.Field != "" {
			m, _ := item.(map[string]any)
			val = m[spec.OutcomesFrom.Field]
		}
		o, ok := val.(string)
		if !ok {
			continue
		}
		at := fmt.Sprintf("%s.params.%s[%d]", v.Path(node.ID), spec.OutcomesFrom.Param, k)
		if spec.OutcomesFrom.Field != "" {
			at += "." + spec.OutcomesFrom.Field
		}
		switch {
		case !IsID(o):
			out = append(out, GraphError{Path: at, Code: CodeParams, Message: fmt.Sprintf("%q cannot be an outcome: lowercase letters, digits and _", o)})
		case seen[o]:
			out = append(out, GraphError{Path: at, Code: CodeParams, Message: fmt.Sprintf("%q is listed twice", o)})
		case contains(spec.Outcomes, o):
			out = append(out, GraphError{Path: at, Code: CodeParams, Message: fmt.Sprintf("%q is reserved: it is already an outcome of %s", o, node.Type)})
		}
		seen[o] = true
	}
	return out
}
func (callBehavior) Start(s *StepContext, node Node, spec NodeSpec) Step {
	e, stop := s.Effect(node, spec)
	if stop != nil {
		return *stop
	}
	return Step{Kind: StepCall, Effects: []Effect{e}}
}

type endBehavior struct{}

func (endBehavior) Behavior() Behavior                               { return BehaviorEnd }
func (endBehavior) OutcomeRule(NodeSpec) OutcomeRule                 { return OutcomeRule{Fixed: []string{}} }
func (endBehavior) Validate(*GraphView, Node, NodeSpec) []GraphError { return nil }
func (endBehavior) Start(s *StepContext, node Node, _ NodeSpec) Step {
	params := s.Params(node)
	if params == nil {
		params = map[string]any{}
	}
	return Step{Kind: StepEnd, End: params}
}
