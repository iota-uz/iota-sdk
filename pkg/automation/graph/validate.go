package graph

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

const maxErrors = 100

// buildView parses the params of every node and indexes the edges.
func buildView(c Catalog, g Graph, o Options) *GraphView {
	v := newView(c, g, o.behaviors())
	g = v.graph
	for _, n := range g.Nodes {
		spec, ok := v.catalog.Nodes[n.Type]
		if !ok || spec.Params == nil {
			continue
		}
		if _, ok := v.behavior.impl(spec.Behavior); !ok {
			continue // its own error at nodes[i].type; params of a kind this build does not run are not read
		}
		if p, issues := spec.Params.Parse(n.Params); len(issues) == 0 {
			v.params[n.ID] = p
		}
	}
	for _, t := range g.Triggers {
		if takes := triggerParamsSpec(v.catalog.Events[t.Event]); takes != nil {
			if p, issues := takes.Parse(t.Params); len(issues) == 0 {
				v.triggerParams[t.ID] = p
			}
		}
	}
	for _, e := range g.Edges {
		v.out[e.From.Node] = append(v.out[e.From.Node], e)
		v.in[e.To] = append(v.in[e.To], e)
		if v.next[e.From.Node] == nil {
			v.next[e.From.Node] = map[string]string{}
		}
		if _, dup := v.next[e.From.Node][e.From.Outcome]; !dup {
			v.next[e.From.Node][e.From.Outcome] = e.To
		}
	}
	return v
}

// triggerParamsSpec is what a trigger on an event takes: its own params, else its match fields, else nil.
func triggerParamsSpec(e EventSpec) ParamsSpec {
	if e.TriggerParams != nil {
		return e.TriggerParams
	}
	if len(e.Match) > 0 {
		if p, err := MatchParams(e.Match); err == nil {
			return p
		}
	}
	return nil
}

type errorList struct{ list []GraphError }

func (l *errorList) add(path, code, format string, args ...any) {
	if len(l.list) < maxErrors {
		l.list = append(l.list, GraphError{Path: path, Code: code, Message: fmt.Sprintf(format, args...)})
	}
}

func (l *errorList) push(e GraphError) {
	if len(l.list) < maxErrors {
		l.list = append(l.list, e)
	}
}

func issuePath(prefix string, path []any) string {
	return pathOf(prefix, path)
}

// Validate checks a graph against a catalog: the format, then ids, triggers, node types and params, edges,
// reachability, cycles and every CEL expression and template, then Options.Rules. Empty: the graph is
// valid. Errors are capped at 100.
func Validate(c Catalog, g Graph, o Options) []GraphError {
	// The structure, also for a graph built in Go rather than decoded.
	var doc any
	shape := g
	if shape.RunAs == "" {
		shape.RunAs = RunAsOwner // the default, as Decode gives it
	}
	shape.Nodes = make([]Node, len(g.Nodes))
	for i, n := range g.Nodes {
		if n.Params == nil {
			n.Params = map[string]any{} // a node built in Go without params
		}
		shape.Nodes[i] = n
	}
	raw, err := json.Marshal(shape)
	if err != nil {
		return []GraphError{{Path: "graph", Code: CodeInvalid, Message: err.Error()}}
	}
	_ = expr.UnmarshalJSON(raw, &doc)
	if errs := structureErrors(doc); len(errs) > 0 {
		return errs
	}
	if errs := nameErrors(g.Name); len(errs) > 0 {
		return errs
	}

	errs := &errorList{}
	b := o.behaviors()
	v := buildView(c, g, o)
	subject := v.subject
	mixed := c.MixedSubject != "" && subject == c.MixedSubject

	// Ids.
	ids := map[string]bool{}
	for i, t := range g.Triggers {
		if t.ID == Ignore {
			errs.add(fmt.Sprintf("triggers[%d].id", i), CodeInvalid, "%s is reserved for an edge that ends the run", Ignore)
		}
		if ids[t.ID] {
			errs.add(fmt.Sprintf("triggers[%d].id", i), CodeDuplicateID, "id %s is used twice", t.ID)
		}
		ids[t.ID] = true
	}
	for i, t := range g.Triggers {
		if t.ID != Ignore && !expr.IsName(t.ID) {
			errs.add(fmt.Sprintf("triggers[%d].id", i), CodeInvalid, "%s is a CEL keyword", t.ID)
		}
	}
	for i, n := range g.Nodes {
		if n.ID != Ignore && !expr.IsName(n.ID) {
			errs.add(fmt.Sprintf("nodes[%d].id", i), CodeInvalid, "%s is a CEL keyword: nodes.%s cannot be read", n.ID, n.ID)
		}
		if n.ID == Ignore {
			errs.add(fmt.Sprintf("nodes[%d].id", i), CodeInvalid, "%s is reserved for an edge that ends the run", Ignore)
		}
		if ids[n.ID] {
			errs.add(fmt.Sprintf("nodes[%d].id", i), CodeDuplicateID, "id %s is used twice", n.ID)
		}
		ids[n.ID] = true
	}

	// Triggers.
	for i, t := range g.Triggers {
		at := fmt.Sprintf("triggers[%d]", i)
		event, ok := c.Events[t.Event]
		if !ok {
			errs.add(at+".event", CodeUnknownEvent, "%s is not an event of the catalog", t.Event)
			continue
		}
		if event.NoTrigger {
			errs.add(at+".event", CodeInvalid, "%s starts no graph: it is a signal for nodes that wait", t.Event)
		} else if event.Subject != subject && !mixed {
			errs.add(at+".event", CodeInvalid, "%s is about a %s, the graph's first trigger about a %s", t.Event, event.Subject, subject)
		}
		takes := event.TriggerParams
		if takes == nil && len(event.Match) > 0 {
			takes, err = MatchParams(event.Match)
			if err != nil {
				errs.add(at+".event", CodeInvalid, "%v", err)
				continue
			}
		}
		if takes != nil {
			if _, issues := takes.Parse(t.Params); len(issues) > 0 {
				for k, issue := range issues {
					if k == 10 {
						break
					}
					errs.add(issuePath(at+".params", issue.Path), CodeParams, "%s", issue.Message)
				}
			}
		} else if len(t.Params) > 0 {
			errs.add(at+".params", CodeParams, "a trigger on %s takes no params", t.Event)
		}
		if event.MatchHook != "" && o.MatchHooks[event.MatchHook] == nil {
			errs.add(at+".event", CodeInvalid, "%s matches by %s, a rule this product does not have", t.Event, event.MatchHook)
		}
	}

	// Node types, params, subjects, behaviors.
	type ruled struct {
		node Node
		spec NodeSpec
		impl BehaviorImpl
	}
	behaviorRules := make([]ruled, 0, len(g.Nodes))
	for i, n := range g.Nodes {
		at := fmt.Sprintf("nodes[%d]", i)
		spec, ok := c.Nodes[n.Type]
		if !ok {
			errs.add(at+".type", CodeUnknownNodeType, "%s is not a node type of the catalog", n.Type)
			continue
		}
		impl, ok := b.impl(spec.Behavior)
		if !ok {
			errs.add(at+".type", CodeUnsupportedBehavior, "%s is a %s node, which this interpreter does not run", n.Type, spec.Behavior)
			continue
		}
		if spec.Params == nil {
			errs.add(at+".type", CodeInvalid, "%s declares no params spec", n.Type)
			continue
		}
		if _, issues := spec.Params.Parse(n.Params); len(issues) > 0 {
			for k, issue := range issues {
				if k == 10 {
					break
				}
				errs.add(issuePath(at+".params", issue.Path), CodeParams, "%s", issue.Message)
			}
			continue
		}
		if len(spec.Subjects) > 0 && !contains(spec.Subjects, subject) {
			errs.add(at+".type", CodeInvalid, "%s is not available in a graph about a %s", n.Type, subject)
		}
		if spec.OutputFrom != nil {
			params, _ := v.Params(n.ID)
			if _, _, err := nodeOutput(c, spec, params); err != nil {
				errs.add(at+".params."+spec.OutputFrom.Param, CodeParams, "%v", err)
			}
		}
		node := n
		node.Params = copyMap(n.Params)
		behaviorRules = append(behaviorRules, ruled{node, copyNodeSpec(spec), impl})
	}
	for _, r := range behaviorRules {
		for _, e := range r.impl.Validate(v, r.node, r.spec) {
			errs.push(e)
		}
	}

	// Edges: every outcome connected once, nothing else.
	connected := map[string]map[string]bool{}
	for i, e := range g.Edges {
		at := fmt.Sprintf("edges[%d]", i)
		where, ok := v.where[e.From.Node]
		if !ok {
			errs.add(at+".from.node", CodeInvalid, "no trigger or node %s", e.From.Node)
			continue
		}
		if outcomes := v.Outcomes(e.From.Node); outcomes != nil && !contains(outcomes, e.From.Outcome) {
			if len(outcomes) == 0 {
				errs.add(at+".from.outcome", CodeUnknownOutcome, "%s ends the run and has no outcomes", e.From.Node)
			} else {
				errs.add(at+".from.outcome", CodeUnknownOutcome, "%s has no outcome %s; its outcomes are %s", e.From.Node, e.From.Outcome, strings.Join(outcomes, ", "))
			}
			continue
		}
		_ = where
		if e.To != Ignore {
			target, ok := v.where[e.To]
			if !ok {
				errs.add(at+".to", CodeInvalid, "no node %s", e.To)
				continue
			}
			if strings.HasPrefix(target, "triggers") {
				errs.add(at+".to", CodeInvalid, "%s is a trigger; an edge goes to a node", e.To)
				continue
			}
		}
		if connected[e.From.Node] == nil {
			connected[e.From.Node] = map[string]bool{}
		}
		if connected[e.From.Node][e.From.Outcome] {
			errs.add(at, CodeInvalid, "outcome %s of %s is connected twice", e.From.Outcome, e.From.Node)
			continue
		}
		connected[e.From.Node][e.From.Outcome] = true
	}
	order := make([]string, 0, len(g.Triggers)+len(g.Nodes))
	for _, t := range g.Triggers {
		order = append(order, t.ID)
	}
	for _, n := range g.Nodes {
		order = append(order, n.ID)
	}
	for _, id := range order {
		for _, outcome := range v.Outcomes(id) {
			if !connected[id][outcome] {
				errs.add(v.where[id], CodeUnconnected, "outcome %s of %s is not connected (connect it, or to %s)", outcome, id, Ignore)
			}
		}
	}

	// Reachability, and an end for every trigger.
	reached := map[string]bool{}
	for _, t := range g.Triggers {
		for id := range v.Reachable(t.ID) {
			reached[id] = true
		}
	}
	for i, n := range g.Nodes {
		if !reached[n.ID] {
			errs.add(fmt.Sprintf("nodes[%d]", i), CodeUnreachable, "%s is not reachable from a trigger", n.ID)
		}
	}
	ends := map[string]bool{}
	waits := map[string]bool{}
	for _, n := range g.Nodes {
		switch c.Nodes[n.Type].Behavior {
		case BehaviorEnd:
			ends[n.ID] = true
		case BehaviorWait:
			waits[n.ID] = true
		case BehaviorBranch, BehaviorSwitch, BehaviorEffect, BehaviorCall:
		}
	}
	for i, t := range g.Triggers {
		if !reachesEnd(v, t.ID, ends, map[string]bool{}) {
			errs.add(fmt.Sprintf("triggers[%d]", i), CodeUnconnected, "no path from %s reaches an end or %s", t.ID, Ignore)
		}
	}

	// A cycle passes a node the run sleeps on.
	state := map[string]int{}
	var cycle func(id string, path []string) []string
	cycle = func(id string, path []string) []string {
		if waits[id] || id == Ignore || state[id] == 2 {
			return nil
		}
		if state[id] == 1 {
			for k, p := range path {
				if p == id {
					return append(append([]string{}, path[k:]...), id)
				}
			}
			return []string{id}
		}
		state[id] = 1
		for _, outcome := range sortedOutcomes(v.next[id]) {
			if c := cycle(v.next[id][outcome], append(path, id)); c != nil {
				return c
			}
		}
		state[id] = 2
		return nil
	}
	for _, n := range g.Nodes {
		if c := cycle(n.ID, nil); c != nil {
			errs.add(v.where[c[0]], CodeCycleWithoutWait, "cycle %s has no node that waits: it would loop without end", strings.Join(c, " → "))
			break
		}
	}

	// CEL.
	x := newExprs(v, o.Limits.Expr)
	check := func(path, event string, withNodes bool, src string, want expr.Type) {
		if _, issues := x.program(event, withNodes, src, want); len(issues) > 0 {
			errs.push(exprError(path, issues[0]))
		}
	}
	for i, t := range g.Triggers {
		if t.Filter != "" {
			if _, ok := c.Events[t.Event]; ok {
				check(fmt.Sprintf("triggers[%d].filter", i), t.Event, false, t.Filter, expr.TypeBool)
			}
		}
	}
	for i, n := range g.Nodes {
		spec, ok := c.Nodes[n.Type]
		params, okp := v.Params(n.ID)
		if !ok || !okp {
			continue
		}
		if _, ok := b.impl(spec.Behavior); !ok {
			continue
		}
		at := fmt.Sprintf("nodes[%d]", i)
		for _, e := range nodeExpressions(c, spec, params) {
			if e.template {
				if _, issues := x.template(e.event, e.source); len(issues) > 0 {
					errs.push(exprError(at+"."+e.path, issues[0]))
				}
				continue
			}
			check(at+"."+e.path, e.event, true, e.source, e.want)
		}
		// A filter fixed by the node type reads the event it waits for.
		if spec.Wait != nil && spec.Wait.Filter.Value != "" {
			if kind := spec.Wait.Event.Of(params); kind != "" {
				check(at+".type", kind, true, spec.Wait.Filter.Value, expr.TypeBool)
			}
		}
	}

	if o.Rules != nil {
		for _, e := range o.Rules(g) {
			errs.push(e)
		}
	}
	return errs.list
}

func reachesEnd(v *GraphView, from string, ends map[string]bool, seen map[string]bool) bool {
	if from == Ignore || ends[from] {
		return true
	}
	if seen[from] {
		return false
	}
	seen[from] = true
	if _, isNode := v.Node(from); isNode && v.Outcomes(from) == nil {
		return true // its outcomes are unknown (an invalid node): its own error says so
	}
	for _, outcome := range sortedOutcomes(v.next[from]) {
		if reachesEnd(v, v.next[from][outcome], ends, seen) {
			return true
		}
	}
	return false
}

func exprError(path string, i expr.Issue) GraphError {
	code := CodeExpressionType
	switch i.Code {
	case expr.IssueSyntax:
		code = CodeExpressionSyntax
	case expr.IssueLimit:
		code = CodeLimit
	}
	return GraphError{Path: path, Code: code, Message: i.Message, Line: i.Line, Column: i.Column}
}
