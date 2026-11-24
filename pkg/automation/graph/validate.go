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
	v.limits = o.Limits
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
			if owned, ok := ownedParams(p); ok {
				v.params[n.ID] = owned
			}
		}
	}
	for _, t := range g.Triggers {
		if takes := triggerParamsSpec(v.catalog.Events[t.Event]); takes != nil {
			if p, issues := takes.Parse(t.Params); len(issues) == 0 {
				if owned, ok := ownedParams(p); ok {
					v.triggerParams[t.ID] = owned
				}
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
	for _, t := range g.Triggers {
		for id := range v.Reachable(t.ID) {
			if r := v.reaching[id]; len(r) == 0 || r[len(r)-1] != t.ID {
				v.reaching[id] = append(r, t.ID)
			}
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

// ownedParams stores what a ParamsSpec parsed as owned JSON values: a custom spec may return typed Go
// containers, which later copies would otherwise share.
// ok=false: the parsed params are not JSON values (NaN, a channel); they are not kept, and Validate
// reports them, so nothing the caller can change is shared.
func ownedParams(p map[string]any) (map[string]any, bool) {
	owned, err := jsonValue(p)
	if err != nil {
		return nil, false
	}
	m, ok := owned.(map[string]any)
	return m, ok
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
	subject := v.subject // what a node no trigger reaches is checked against

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
			} else if _, ok := v.triggerParams[t.ID]; !ok {
				errs.add(at+".params", CodeParams, "the params of %s parsed into values that are not JSON", t.Event)
			}
		} else if len(t.Params) > 0 {
			errs.add(at+".params", CodeParams, "a trigger on %s takes no params", t.Event)
		}
		if col := t.Collect; col != nil {
			if _, ok := col.WindowDuration(); !ok {
				errs.add(at+".collect.window", CodeParams, "the window is a duration from %s to %s", CollectMinWindow, CollectMaxWindow)
			}
			if col.Max < 0 || col.Max > CollectMaxEvents {
				errs.add(at+".collect.max", CodeParams, "max is 1..%d", CollectMaxEvents)
			}
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
		if _, ok := v.params[n.ID]; !ok {
			errs.add(at+".params", CodeParams, "%s parsed its params into values that are not JSON", n.Type)
			continue
		}
		if len(spec.Subjects) > 0 {
			// A node is available when every subject its triggers are about is one it takes.
			if subjects := v.SubjectsReaching(n.ID); len(subjects) > 0 {
				for _, about := range subjects {
					if !contains(spec.Subjects, about) {
						var from []string
						for _, id := range v.reaching[n.ID] {
							if c.Events[eventOfTrigger(g, id)].Subject == about {
								from = append(from, id)
							}
						}
						if len(from) == 0 {
							from = v.reaching[n.ID] // triggers about different subjects: the run is about the mixed one
						}
						errs.add(at+".type", CodeInvalid, "%s is not available after %s, which is about a %s", n.Type, strings.Join(from, ", "), about)
						break
					}
				}
			} else if !contains(spec.Subjects, subject) {
				errs.add(at+".type", CodeInvalid, "%s is not available in a graph about a %s", n.Type, subject)
			}
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
	inside := insideConstructs(v)
	for _, id := range order {
		failedOutcome := ""
		if n, ok := v.Node(id); ok && inside[id] {
			failedOutcome = c.Nodes[n.Type].FailedOutcome
		}
		for _, outcome := range v.Outcomes(id) {
			// Inside a branch or a foreach's body a failed outcome may have no edge: the token ends there,
			// the branch does not arrive and the item is not ok.
			if outcome == failedOutcome && failedOutcome != "" {
				continue
			}
			if !connected[id][outcome] && !v.MayLeaveUnconnected(id, outcome) {
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
	// An end is a node with no outcomes (End, or a registered terminal behavior); a node with a WaitSpec
	// suspends the run (Wait, or a registered behavior that waits), so a cycle through it is allowed.
	ends := map[string]bool{}
	waits := map[string]bool{}
	for _, n := range g.Nodes {
		spec := c.Nodes[n.Type]
		// A built-in kind ends or suspends by what it is; a registered kind declares it: no outcomes ends a
		// path, a WaitSpec suspends the run.
		builtin := isBuiltinBehavior(spec.Behavior)
		if outcomes := v.Outcomes(n.ID); spec.Behavior == BehaviorEnd || spec.Behavior == BehaviorCollect || !builtin && outcomes != nil && len(outcomes) == 0 {
			ends[n.ID] = true
		}
		// A call waits for its answer (ask, a script); a cycle through a foreach is cycle_through_foreach,
		// its own error, so it is not reported twice.
		if spec.Behavior == BehaviorWait || spec.Behavior == BehaviorCall || spec.Behavior == BehaviorForeach || !builtin && spec.Wait != nil {
			waits[n.ID] = true
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
	each := foreachScopes(v)
	check := func(owner, path, event string, withNodes bool, src string, want expr.Type) {
		if _, issues := x.programIn(owner, event, withNodes, each[""], src, want); len(issues) > 0 {
			errs.push(x.exprError(owner, path, issues[0], src))
		}
	}
	checkIn := func(node, path, event string, src string, want expr.Type) {
		if _, issues := x.programIn(node, event, true, each[node], src, want); len(issues) > 0 {
			errs.push(x.exprError(node, path, issues[0], src))
		}
	}
	for i, t := range g.Triggers {
		if t.Filter != "" {
			if _, ok := c.Events[t.Event]; ok {
				check(t.ID, fmt.Sprintf("triggers[%d].filter", i), t.Event, false, t.Filter, expr.TypeBool)
			}
		}
		if t.Collect != nil && t.Collect.Key != "" {
			if _, ok := c.Events[t.Event]; ok {
				check(t.ID, fmt.Sprintf("triggers[%d].collect.key", i), t.Event, false, t.Collect.Key, expr.TypeAny)
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
				if _, issues := x.templateIn(n.ID, e.event, each[n.ID], e.source); len(issues) > 0 {
					errs.push(x.exprError(n.ID, at+"."+e.path, issues[0], e.source))
				}
				continue
			}
			checkIn(n.ID, at+"."+e.path, e.event, e.source, e.want)
		}
		// A filter fixed by the node type reads the event it waits for.
		if spec.Wait != nil && spec.Wait.Filter.Value != "" {
			if kind := spec.Wait.Event.Of(params); kind != "" {
				checkIn(n.ID, at+".type", kind, spec.Wait.Filter.Value, expr.TypeBool)
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

func isBuiltinBehavior(b Behavior) bool {
	switch b { //nolint:exhaustive // the other behaviors are flow behaviors or registered ones
	case BehaviorBranch, BehaviorSwitch, BehaviorWait, BehaviorEffect, BehaviorCall, BehaviorEnd:
		return true
	}
	return isFlowBehavior(b)
}

func isFlowBehavior(b Behavior) bool {
	switch b { //nolint:exhaustive // only the four flow behaviors
	case BehaviorParallel, BehaviorJoin, BehaviorForeach, BehaviorCollect:
		return true
	}
	return false
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

// exprError is the error of an expression (or, for a template, the template's text) of an owner: a node or
// a trigger. A name the expression reads that some other trigger of the graph has but a trigger that leads
// to the owner lacks is unknown_variable_for_trigger; anything else is the check's own error.
func eventOfTrigger(g Graph, id string) string {
	for _, t := range g.Triggers {
		if t.ID == id {
			return t.Event
		}
	}
	return ""
}

func (x *exprs) exprError(owner, path string, i expr.Issue, src string) GraphError {
	if i.Code == expr.IssueType {
		if e, ok := x.unknownVariable(owner, path, src); ok {
			e.Line, e.Column = i.Line, i.Column
			return e
		}
	}
	return exprError(path, i)
}

// unknownVariable looks for a variable of src (a `{{ }}` template or an expression) that a trigger leading to
// owner does not have although the catalog declares it for some subject or event.
func (x *exprs) unknownVariable(owner, path, src string) (GraphError, bool) {
	triggers := x.triggersOf(owner)
	if len(triggers) == 0 {
		return GraphError{}, false
	}
	var sources []string
	if strings.Contains(src, "{{") {
		for _, e := range expr.TemplateExpressions(src) {
			sources = append(sources, e.Source)
		}
	}
	if len(sources) == 0 {
		sources = []string{src}
	}
	scopes := map[string]map[string]expr.Field{}
	scopeOfTrigger := func(id string) map[string]expr.Field {
		if vars, ok := scopes[id]; ok {
			return vars
		}
		var vars map[string]expr.Field
		if e, err := x.env(id, "*", false, allForeach); err == nil {
			vars = e.Scope().Vars
		}
		scopes[id] = vars
		return vars
	}
	probe, err := x.env(owner, "*", true, allForeach)
	if err != nil {
		return GraphError{}, false
	}
	for _, source := range sources {
		for _, chain := range probe.References(source) {
			if contains([]string{"nodes", "foreach", "tenant", "now"}, chain[0]) {
				continue
			}
			for _, id := range triggers {
				at := missingAt(scopeOfTrigger(id), chain)
				if at < 0 {
					continue
				}
				name := strings.ReplaceAll(strings.Join(chain[:at+1], "."), "."+expr.IndexStep, expr.IndexStep)
				if x.catalogHas(chain[:at+1]) {
					return GraphError{Path: path, Code: CodeUnknownVariableForTrigger, Message: x.variableMessage(owner, name, id)}, true
				}
			}
		}
	}
	return GraphError{}, false
}

// catalogHas reports whether the catalog declares a chain: a variable of some subject, or a field of some
// event (`event.mail.subject`).
func (x *exprs) catalogHas(chain []string) bool {
	c := x.view.catalog
	if chain[0] == "event" {
		for kind := range c.Events {
			if missingAt(map[string]expr.Field{"event": eventField(c, kind)}, chain) < 0 {
				return true
			}
		}
		return false
	}
	for subject := range c.Subjects {
		vars := map[string]expr.Field{}
		for _, p := range subjectVars(c, subject) {
			vars[p.Name] = p.Field
		}
		if missingAt(vars, chain) < 0 {
			return true
		}
	}
	return false
}

func (x *exprs) variableMessage(owner, name, trigger string) string {
	t := x.view.graph.Triggers[x.triggerAt[trigger]]
	what := "node"
	if _, isTrigger := x.triggerAt[owner]; isTrigger {
		what = "trigger"
	}
	return fmt.Sprintf("%s %s reads %s, which trigger %s (on %s) does not have", what, owner, name, trigger, t.Event)
}

// missingAt is the index in chain of the first name vars lack, or -1 when all of them are there. Names under a
// map or a dyn value are not known and are taken as there.
func missingAt(vars map[string]expr.Field, chain []string) int {
	f, ok := vars[chain[0]]
	if !ok {
		return 0
	}
	for i := 1; i < len(chain); i++ {
		f = f.Unwrap()
		switch {
		case f.Kind == expr.Dyn:
			return -1 // what is under a dyn value is not declared
		case f.Kind == expr.Map:
			f = *f.Elem // any key (a name or an expression) reads the map's value
		case chain[i] == expr.IndexStep:
			if f.Kind != expr.List {
				return i
			}
			f = *f.Elem
		case f.Kind != expr.Object:
			return i // a scalar, a list or a timestamp has no fields
		default:
			if f, ok = f.Prop(chain[i]); !ok {
				return i
			}
		}
	}
	return -1
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
