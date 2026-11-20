package graph

import (
	"fmt"
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
	event   string      // the event kind all triggers share; "" when they differ

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
	x := &exprs{view: v, limits: limits, envs: map[string]*expr.Env{}, progs: map[string]progResult{}, tmpls: map[string]tmplResult{}}
	kinds := map[string]bool{}
	for _, t := range v.graph.Triggers {
		kinds[t.Event] = true
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

// ExprScope is what an expression of a graph about subject reads: the subject's variables, event (the
// kind's payload with its kind; any other kind, or "", a map of dyn), nodes (each node's output), tenant
// (a map of dyn) and now (the step's time).
func ExprScope(c Catalog, subject, event string, nodes expr.Fields) expr.Scope {
	vars := map[string]expr.Field{}
	if s, ok := c.Subjects[subject]; ok {
		for _, p := range s.Context {
			vars[p.Name] = p.Field
		}
	}
	vars["event"] = eventField(c, event)
	vars["nodes"] = expr.ObjectOf(nodes)
	vars["tenant"] = expr.MapOf(expr.DynField)
	vars["now"] = expr.TimestampField
	return expr.Scope{Vars: vars}
}

// env is the environment of a scope: the event kind read ("*": the graph's), with or without node outputs. ("*": the graph's), and whether node outputs are visible.
func (x *exprs) env(event string, withNodes bool) (*expr.Env, error) {
	key := fmt.Sprintf("%s|%t", event, withNodes)
	x.mu.Lock()
	defer x.mu.Unlock()
	if e, ok := x.envs[key]; ok {
		return e, nil
	}
	kind := event
	if event == "*" {
		kind = x.event
	}
	nodes := expr.Fields{}
	if withNodes {
		nodes = x.outputs
	}
	scope := ExprScope(x.view.catalog, x.view.subject, kind, nodes)
	e, err := expr.NewEnv(scope, x.limits)
	if err != nil {
		return nil, err
	}
	x.envs[key] = e
	return e, nil
}

func (x *exprs) program(event string, withNodes bool, src string, want expr.Type) (*expr.Program, []expr.Issue) {
	key := fmt.Sprintf("%s|%t|%s|%s", event, withNodes, want, src)
	x.mu.Lock()
	if r, ok := x.progs[key]; ok {
		x.mu.Unlock()
		return r.p, r.issues
	}
	x.mu.Unlock()
	e, err := x.env(event, withNodes)
	if err != nil {
		return nil, []expr.Issue{{Code: expr.IssueType, Message: err.Error()}}
	}
	p, issues := e.Check(src, want)
	x.mu.Lock()
	x.progs[key] = progResult{p, issues}
	x.mu.Unlock()
	return p, issues
}

func (x *exprs) template(event string, text string) (*expr.TemplateProgram, []expr.Issue) {
	key := event + "|" + text
	x.mu.Lock()
	if r, ok := x.tmpls[key]; ok {
		x.mu.Unlock()
		return r.t, r.issues
	}
	x.mu.Unlock()
	e, err := x.env(event, true)
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
	props := append(expr.Fields(nil), spec.Output...)
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

// paramAt reads a dotted path inside params.
func paramAt(params map[string]any, path string) (any, bool) {
	var cur any = params
	for _, k := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[k]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// setAt returns a copy of params with a dotted path set.
func setAt(params map[string]any, path string, v any) map[string]any {
	head, rest, nested := strings.Cut(path, ".")
	out := make(map[string]any, len(params))
	for k, x := range params {
		out[k] = x
	}
	if !nested {
		out[head] = v
		return out
	}
	child, _ := params[head].(map[string]any)
	out[head] = setAt(child, rest, v)
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
		if spec.Wait != nil && spec.Wait.Filter.Param == e.Param && waited != "" {
			event = waited
		}
		want := e.Type
		if want == "" {
			want = expr.TypeAny
		}
		out = append(out, nodeExpression{path: "params." + e.Param, source: s, want: want, event: event})
	}
	for _, v := range spec.Values {
		if src, ok := paramAt(params, v); ok {
			if s, ok := src.(string); ok {
				out = append(out, nodeExpression{path: "params." + v, source: s, want: expr.TypeAny, event: "*"})
			}
		}
	}
	for _, t := range spec.Templates {
		if text, ok := paramAt(params, t); ok {
			if s, ok := text.(string); ok {
				out = append(out, nodeExpression{path: "params." + t, source: s, event: "*", template: true})
			}
		}
	}
	return out
}
