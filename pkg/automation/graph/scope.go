package graph

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// exprs checks and caches the expressions of one graph: an environment per scope (the event read, with or
// without node outputs) and a program per source.
type exprs struct {
	view    *GraphView
	limits  expr.Limits
	outputs expr.Fields // nodes.<id>.output by node, in graph order
	foreach expr.Fields // foreach.<id> by foreach node, in graph order
	event   string      // the event kind all triggers share; "" when they differ
	// run: the programs are for running a graph, not for checking it.
	run bool
	// triggerAt is the index of each trigger in the graph.
	triggerAt map[string]int

	mu    sync.Mutex
	envs  map[string]*expr.Env
	progs map[string]progResult
	tmpls map[string]tmplResult
}

type progResult struct {
	p      *expr.Program
	issues []expr.Issue
}

type tmplResult struct {
	t      *expr.TemplateProgram
	issues []expr.Issue
}

func newExprs(v *GraphView, limits expr.Limits) *exprs {
	x := &exprs{view: v, limits: limits, envs: map[string]*expr.Env{}, progs: map[string]progResult{}, tmpls: map[string]tmplResult{}, triggerAt: map[string]int{}}
	kinds := map[string]bool{}
	for i, t := range v.graph.Triggers {
		kinds[t.Event] = true
		if _, dup := x.triggerAt[t.ID]; !dup {
			x.triggerAt[t.ID] = i
		}
	}
	if len(kinds) == 1 {
		x.event = v.graph.Triggers[0].Event
	}
	seen := map[string]bool{}
	for _, n := range v.graph.Nodes {
		spec, ok := v.catalog.Nodes[n.Type]
		p, okp := v.params[n.ID]
		// A duplicate id or one CEL cannot select has its own error at nodes[i].id; keep it out of scope.
		if !ok || !okp || seen[n.ID] || !expr.IsName(n.ID) {
			seen[n.ID] = true
			continue
		}
		seen[n.ID] = true
		if spec.Behavior == BehaviorForeach {
			x.foreach = append(x.foreach, expr.Prop{Name: n.ID, Field: expr.ObjectOf(expr.Fields{
				{Name: "item", Field: expr.DynField},
				{Name: "index", Field: expr.IntField},
			})})
		}
		if f, ok, _ := nodeOutput(v.catalog, spec, p); ok {
			x.outputs = append(x.outputs, expr.Prop{Name: n.ID, Field: expr.ObjectOf(expr.Fields{{Name: "output", Field: f}})})
		}
	}
	return x
}

// eventField is what `event` is in a scope: the kind's payload with its kind, or a map of dyn.
func eventField(c Catalog, kind string) expr.Field {
	e, ok := c.Events[kind]
	if !ok {
		return expr.MapOf(expr.DynField)
	}
	props := expr.Fields{{Name: "kind", Field: expr.StringField}}
	props = append(props, e.Payload...)
	return expr.ObjectOf(props)
}

// eventFieldOf is what `event` is for a node several kinds of event can lead to: the kind, and the payload
// fields every kind has with the same type (a field typed differently is dyn). One kind is eventField; a
// kind the catalog lacks makes it a map of dyn.
func eventFieldOf(c Catalog, kinds []string) expr.Field {
	if len(kinds) == 0 {
		return expr.MapOf(expr.DynField)
	}
	if len(kinds) == 1 {
		return eventField(c, kinds[0])
	}
	var common expr.Fields
	for i, kind := range kinds {
		e, ok := c.Events[kind]
		if !ok {
			return expr.MapOf(expr.DynField)
		}
		if i == 0 {
			common = e.Payload.Clone()
			continue
		}
		common = intersectFields(common, e.Payload)
	}
	return expr.ObjectOf(append(expr.Fields{{Name: "kind", Field: expr.StringField}}, common...))
}

// intersectFields keeps the fields of a that b has too, each as what both declare: a nested object is
// intersected in turn, scalars of different types are dyn, and a field that is a list, a map or an object in
// one and something else in the other is not kept (reading into it would be reading what one does not have).
func intersectFields(a, b expr.Fields) expr.Fields {
	var out expr.Fields
	for _, p := range a {
		other, ok := b.Get(p.Name)
		if !ok {
			continue
		}
		if f, ok := intersectField(p.Field, other); ok {
			p.Field = f
			out = append(out, p)
		}
	}
	return out
}

func composite(f expr.Field) bool {
	switch f.Unwrap().Kind { //nolint:exhaustive // the scalars and dyn are not composite
	case expr.Object, expr.List, expr.Map:
		return true
	}
	return false
}

func intersectField(a, b expr.Field) (expr.Field, bool) {
	if a.String() == b.String() {
		return a, true
	}
	optional := a.Kind == expr.Optional || b.Kind == expr.Optional
	ua, ub := a.Unwrap(), b.Unwrap()
	var out expr.Field
	switch {
	case ua.String() == ub.String():
		out = ua // the same type, optional in one: absent where that one has it absent
	case ua.Kind == expr.Object && ub.Kind == expr.Object:
		out = expr.ObjectOf(intersectFields(ua.Props, ub.Props))
	case ua.Kind == expr.List && ub.Kind == expr.List:
		elem, ok := intersectField(*ua.Elem, *ub.Elem)
		if !ok {
			return expr.Field{}, false
		}
		out = expr.ListOf(elem)
	case ua.Kind == expr.Map && ub.Kind == expr.Map:
		elem, ok := intersectField(*ua.Elem, *ub.Elem)
		if !ok {
			return expr.Field{}, false
		}
		out = expr.MapOf(elem)
	case ua.Kind == expr.Dyn || ub.Kind == expr.Dyn:
		return expr.DynField, true
	case composite(ua) || composite(ub):
		return expr.Field{}, false
	default:
		return expr.DynField, true
	}
	if optional {
		return expr.OptionalOf(out), true
	}
	return out, true
}

// subjectVars are the variables a subject declares in expressions; a subject the catalog lacks (conversation,
// none) declares none.
func subjectVars(c Catalog, subject string) expr.Fields {
	if s, ok := c.Subjects[subject]; ok {
		return s.Context
	}
	return nil
}

// ExprScope is what an expression of a graph about subject reads: the subject's variables, event (the
// kind's payload with its kind; any other kind, or "", a map of dyn), nodes (each node's output), tenant
// (a map of dyn) and now (the step's time).
func ExprScope(c Catalog, subject, event string, nodes expr.Fields) expr.Scope {
	vars := subjectVars(c, subject)
	return scopeOf(vars, eventField(c, event), expr.ListOf(eventField(c, event)), nodes, declaredNames(vars))
}

// ExprScopeOf is the scope of an expression that runs after any of several triggers: the variables of the
// subjects and the payload of the events that all of them have, with the same type.
func ExprScopeOf(c Catalog, subjects, events []string, nodes expr.Fields) expr.Scope {
	var vars expr.Fields
	for i, subject := range subjects {
		if i == 0 {
			vars = subjectVars(c, subject).Clone()
			continue
		}
		vars = intersectFields(vars, subjectVars(c, subject))
	}
	// subject and run are the run's own only where no reaching subject declares a variable of that name:
	// a run from such a subject has the override, which the intersection may have dropped.
	declared := map[string]bool{}
	for _, subject := range subjects {
		for name := range declaredNames(subjectVars(c, subject)) {
			declared[name] = true
		}
	}
	ev := eventFieldOf(c, events)
	return scopeOf(vars, ev, expr.ListOf(ev), nodes, declared)
}

// declaredNames are the variable names of a subject's context.
func declaredNames(vars expr.Fields) map[string]bool {
	out := make(map[string]bool, len(vars))
	for _, p := range vars {
		out[p.Name] = true
	}
	return out
}

func scopeOf(vars expr.Fields, event, events expr.Field, nodes expr.Fields, declared map[string]bool) expr.Scope {
	// subject and run are the run's own unless a subject's context declares a variable of that name.
	out := map[string]expr.Field{}
	if !declared["subject"] {
		out["subject"] = SubjectInfoField
	}
	if !declared["run"] {
		out["run"] = RunInfoField
	}
	for _, p := range vars {
		out[p.Name] = p.Field
	}
	out["event"] = event
	out["events"] = events
	out["nodes"] = expr.ObjectOf(nodes)
	out["tenant"] = expr.MapOf(expr.DynField)
	out["now"] = expr.TimestampField
	return expr.Scope{Vars: out}
}

// SubjectInfoField is `subject` in every scope (unless the subject's context declares its own): what the run is about and the absolute link to its
// record ("" when its module registers no link).
var SubjectInfoField = expr.ObjectOf(expr.Fields{
	{Name: "entity", Field: expr.StringField},
	{Name: "key", Field: expr.StringField},
	{Name: "url", Field: expr.StringField},
})

// RunInfoField is `run` in every scope: the run and the link to it in the editor.
var RunInfoField = expr.ObjectOf(expr.Fields{
	{Name: "id", Field: expr.StringField},
	{Name: "url", Field: expr.StringField},
})

// allForeach is the scope of every foreach of the graph (a step reads only what its token has).
const allForeach = "*"

// triggersOf are the triggers an owner (a node, or a trigger itself) is reached from; nil: the graph as a
// whole (no owner, or a node no trigger reaches).
func (x *exprs) triggersOf(owner string) []string {
	if owner == "" {
		return nil
	}
	return x.view.reaching[owner]
}

// kindsAndSubjects are the distinct event kinds and subjects of triggers, in order.
func (x *exprs) kindsAndSubjects(triggers []string) ([]string, []string) {
	var kinds, subjects []string
	for _, id := range triggers {
		i, ok := x.triggerAt[id]
		if !ok {
			continue
		}
		kind := x.view.graph.Triggers[i].Event
		e, known := x.view.catalog.Events[kind]
		if !contains(kinds, kind) {
			kinds = append(kinds, kind)
		}
		if known && !contains(subjects, e.Subject) {
			subjects = append(subjects, e.Subject)
		}
	}
	return kinds, subjects
}

// env is the environment of a scope: the owner whose triggers' schemas are read ("": the graph's), the
// event kind read ("*": the owner's), with or without node outputs, and the foreach items in scope:
// allForeach, or the comma-joined ids of the foreachs around a node.
func (x *exprs) env(owner, event string, withNodes bool, each string) (*expr.Env, error) {
	triggers := x.triggersOf(owner)
	key := fmt.Sprintf("%s|%s|%t|%s", strings.Join(triggers, ","), event, withNodes, each)
	x.mu.Lock()
	defer x.mu.Unlock()
	if e, ok := x.envs[key]; ok {
		return e, nil
	}
	nodes := expr.Fields{}
	if withNodes {
		nodes = x.outputs
	}
	var scope expr.Scope
	if len(triggers) == 0 {
		kind := event
		if event == "*" {
			kind = x.event
		}
		scope = ExprScope(x.view.catalog, x.view.subject, kind, nodes)
		// events are the run's trigger events, whatever event a wait's filter reads.
		scope.Vars["events"] = expr.ListOf(eventField(x.view.catalog, x.event))
	} else {
		kinds, subjects := x.kindsAndSubjects(triggers)
		triggerKinds := kinds
		if event != "*" {
			kinds = []string{event}
		}
		scope = ExprScopeOf(x.view.catalog, subjects, kinds, nodes)
		scope.Vars["events"] = expr.ListOf(eventFieldOf(x.view.catalog, triggerKinds))
		if x.run && len(triggers) > 1 {
			// A run passes the event and the subject it has, whole: the fields all of them share, typed as the
			// check types them, are what an expression may read.
			for name, f := range scope.Vars {
				if name != "nodes" && name != "tenant" && name != "now" && name != "subject" && name != "run" {
					scope.Vars[name] = f.OpenObjects()
				}
			}
		}
	}
	if withNodes {
		// foreach.<id>.item and .index: read inside the foreach's body.
		in := map[string]bool{}
		for _, id := range strings.Split(each, ",") {
			in[id] = true
		}
		var fields expr.Fields
		for _, f := range x.foreach {
			if each == allForeach || in[f.Name] {
				fields = append(fields, f)
			}
		}
		if len(fields) > 0 {
			scope.Vars["foreach"] = expr.ObjectOf(fields)
		}
	}
	e, err := expr.NewEnv(scope, x.limits)
	if err != nil {
		return nil, err
	}
	x.envs[key] = e
	return e, nil
}

func (x *exprs) program(owner, event string, withNodes bool, src string, want expr.Type) (*expr.Program, []expr.Issue) {
	return x.programIn(owner, event, withNodes, allForeach, src, want)
}

// programIn checks a source in the scope of an owner with only the given foreach items (see env).
func (x *exprs) programIn(owner, event string, withNodes bool, each, src string, want expr.Type) (*expr.Program, []expr.Issue) {
	key := fmt.Sprintf("%s|%s|%t|%s|%s|%s", strings.Join(x.triggersOf(owner), ","), event, withNodes, each, want, src)
	x.mu.Lock()
	if r, ok := x.progs[key]; ok {
		x.mu.Unlock()
		return r.p, r.issues
	}
	x.mu.Unlock()
	e, err := x.env(owner, event, withNodes, each)
	if err != nil {
		return nil, []expr.Issue{{Code: expr.IssueType, Message: err.Error()}}
	}
	p, issues := e.Check(src, want)
	x.mu.Lock()
	x.progs[key] = progResult{p, issues}
	x.mu.Unlock()
	return p, issues
}

func (x *exprs) template(owner, event string, text string) (*expr.TemplateProgram, []expr.Issue) {
	return x.templateIn(owner, event, allForeach, text)
}

func (x *exprs) templateIn(owner, event, each, text string) (*expr.TemplateProgram, []expr.Issue) {
	key := strings.Join(x.triggersOf(owner), ",") + "|" + event + "|" + each + "|" + text
	x.mu.Lock()
	if r, ok := x.tmpls[key]; ok {
		x.mu.Unlock()
		return r.t, r.issues
	}
	x.mu.Unlock()
	e, err := x.env(owner, event, true, each)
	if err != nil {
		return nil, []expr.Issue{{Code: expr.IssueType, Message: err.Error()}}
	}
	t, issues := expr.Template(e, text)
	x.mu.Lock()
	x.tmpls[key] = tmplResult{t, issues}
	x.mu.Unlock()
	return t, issues
}

// nodeOutput is what a node puts in nodes.<id>.output: a wait its woken event, a call or effect its
// declared output, or the JSON Schema in its OutputFrom param. ok=false: the node has no output.
func nodeOutput(c Catalog, spec NodeSpec, params map[string]any) (expr.Field, bool, error) {
	f, ok, err := declaredOutput(c, spec, params)
	if !spec.StructuredFailure() {
		return f, ok, err
	}
	// A failed call or effect puts output.error {code, message, retryable}: templates of the failed
	// branch read it.
	errField := expr.Prop{Name: "error", Field: expr.OptionalOf(ErrorField)}
	// The failure's error is the engine's object whatever an output schema in a param declares (a static
	// Output that declares error keeps its own: StructuredFailure is false for it).
	withError := func(obj expr.Field) expr.Field {
		obj = obj.Clone()
		kept := obj.Props[:0]
		for _, p := range obj.Props {
			if p.Name != "error" {
				kept = append(kept, p)
			}
		}
		obj.Props = append(kept, errField)
		return obj
	}
	switch {
	case !ok:
		return expr.ObjectOf(expr.Fields{errField}), true, err
	case f.Kind == expr.Object:
		f = withError(f)
	case f.Kind == expr.Optional && f.Elem != nil && f.Elem.Kind == expr.Object:
		f = expr.OptionalOf(withError(*f.Elem))
	}
	return f, ok, err
}

// declaredOutput is the output a node declares, without the failure of a call.
func declaredOutput(c Catalog, spec NodeSpec, params map[string]any) (expr.Field, bool, error) {
	if spec.Behavior == BehaviorWait {
		if spec.Wait == nil {
			return expr.Field{}, false, nil
		}
		kind := spec.Wait.Event.Of(params)
		e, ok := c.Events[kind]
		if !ok {
			return expr.Field{}, false, nil
		}
		return expr.ObjectOf(expr.Fields{{Name: "event", Field: expr.ObjectOf(e.Payload)}}), true, nil
	}
	if spec.OutputFrom != nil {
		raw, ok := params[spec.OutputFrom.Param]
		if !ok || raw == nil {
			return expr.DynField, true, nil
		}
		f, err := expr.FieldFromSchemaValue(raw)
		if err != nil {
			return expr.DynField, true, err
		}
		// The node may leave its output out, so the declared output is optional.
		return expr.OptionalOf(f), true, nil
	}
	if len(spec.Output) == 0 {
		return expr.Field{}, false, nil
	}
	props := spec.Output.Clone()
	if spec.Answer != nil {
		if choices := fromValues(spec.OutcomesFrom, params); len(choices) > 0 {
			for i, p := range props {
				if p.Name == spec.Answer.Choice {
					props[i].Field = expr.EnumOf(choices...)
				}
			}
		}
	}
	return expr.ObjectOf(props), true, nil
}

// paramAt reads a dotted path inside params. A segment that is a number reads that item of a list.
func paramAt(params map[string]any, path string) (any, bool) {
	var cur any = params
	for _, k := range strings.Split(path, ".") {
		switch m := cur.(type) {
		case map[string]any:
			v, ok := m[k]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(k)
			if err != nil || i < 0 || i >= len(m) {
				return nil, false
			}
			cur = m[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// displayPath writes a concrete path the way an editor names a field of a list item: items[1].label.
func displayPath(path string) string {
	var b strings.Builder
	for i, seg := range strings.Split(path, ".") {
		if _, err := strconv.Atoi(seg); err == nil && i > 0 {
			b.WriteString("[" + seg + "]")
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(seg)
	}
	return b.String()
}

// expandPaths turns a path with "[]" (each item of a list: "options[].label") into the concrete paths
// of the params ("options.0.label", "options.1.label"); a path without it is itself. A list that is
// not there has no items.
func expandPaths(params map[string]any, path string) []string {
	head, rest, wild := strings.Cut(path, "[]")
	if !wild {
		return []string{path}
	}
	list, ok := paramAt(params, head)
	if !ok {
		return nil
	}
	items, ok := list.([]any)
	if !ok {
		return nil
	}
	var out []string
	for i := range items {
		out = append(out, expandPaths(params, head+"."+strconv.Itoa(i)+rest)...)
	}
	return out
}

// setAt returns a copy of params with a dotted path set; a numeric segment sets that item of a list.
func setAt(params map[string]any, path string, v any) map[string]any {
	out, _ := setIn(params, path, v).(map[string]any)
	return out
}

func setIn(cur any, path string, v any) any {
	head, rest, nested := strings.Cut(path, ".")
	if list, ok := cur.([]any); ok {
		i, err := strconv.Atoi(head)
		if err != nil || i < 0 || i >= len(list) {
			return cur
		}
		out := append([]any(nil), list...)
		if nested {
			out[i] = setIn(list[i], rest, v)
		} else {
			out[i] = v
		}
		return out
	}
	params, _ := cur.(map[string]any)
	out := make(map[string]any, len(params)+1)
	for k, x := range params {
		out[k] = x
	}
	if !nested {
		out[head] = v
		return out
	}
	out[head] = setIn(params[head], rest, v)
	return out
}

// nodeExpression is a CEL source inside a node's params with the scope it reads.
type nodeExpression struct {
	path     string // params.<dotted path>
	source   string
	want     expr.Type
	event    string // "*" the graph's event, or the kind a wait's filter reads
	template bool
}

func nodeExpressions(c Catalog, spec NodeSpec, params map[string]any) []nodeExpression {
	out := make([]nodeExpression, 0, len(spec.Expressions)+len(spec.Values)+len(spec.Templates))
	waited := ""
	if spec.Wait != nil {
		waited = spec.Wait.Event.Of(params)
	}
	for _, e := range spec.Expressions {
		src, ok := paramAt(params, e.Param)
		s, isStr := src.(string)
		if !ok || !isStr {
			continue
		}
		event := "*"
		want := e.Type
		if want == "" {
			want = expr.TypeAny
		}
		if spec.Wait != nil && spec.Wait.Filter.Param == e.Param && waited != "" {
			event, want = waited, expr.TypeBool // a wait's filter is a bool, whatever Expressions says
		}
		out = append(out, nodeExpression{path: "params." + e.Param, source: s, want: want, event: event})
	}
	// A wait's filter param is a bool over the waited event even when Expressions does not list it.
	if w := spec.Wait; w != nil && w.Filter.Param != "" && waited != "" {
		listed := false
		for _, e := range spec.Expressions {
			listed = listed || e.Param == w.Filter.Param
		}
		if src, ok := paramAt(params, w.Filter.Param); ok && !listed {
			if s, ok := src.(string); ok {
				out = append(out, nodeExpression{path: "params." + w.Filter.Param, source: s, want: expr.TypeBool, event: waited})
			}
		}
	}
	for _, v := range spec.Values {
		for _, path := range expandPaths(params, v) {
			if src, ok := paramAt(params, path); ok {
				if s, ok := src.(string); ok {
					out = append(out, nodeExpression{path: "params." + displayPath(path), source: s, want: expr.TypeAny, event: "*"})
				}
			}
		}
	}
	for _, t := range spec.Templates {
		for _, path := range expandPaths(params, t) {
			if text, ok := paramAt(params, path); ok {
				if s, ok := text.(string); ok {
					out = append(out, nodeExpression{path: "params." + displayPath(path), source: s, event: "*", template: true})
				}
			}
		}
	}
	return out
}
