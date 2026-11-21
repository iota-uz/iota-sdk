package graph

import (
	"fmt"
	"sort"
)

// BehaviorImpl is how a kind of node runs and is checked. Implementations beyond the defaults plug in
// through NewBehaviors.
type BehaviorImpl interface {
	Behavior() Behavior
	// OutcomeRule gives the kind's outcomes as data, so an editor builds a node's outputs without Go code.
	OutcomeRule(spec NodeSpec) OutcomeRule
	// Validate adds the kind's structural rules on top of the common ones (edges, reachability, cycles).
	Validate(v *GraphView, node Node, spec NodeSpec) []GraphError
	// Start is the step on entering the node for the step context's token.
	Start(s *StepContext, node Node, spec NodeSpec) Step
}

// Behaviors is an immutable set of behavior implementations.
type Behaviors struct {
	impls map[Behavior]BehaviorImpl
	order []Behavior
}

// NewBehaviors builds a set; a behavior given twice is an error.
func NewBehaviors(impls ...BehaviorImpl) (*Behaviors, error) {
	b := &Behaviors{impls: map[Behavior]BehaviorImpl{}}
	for _, impl := range impls {
		if impl == nil {
			return nil, fmt.Errorf("a nil behavior")
		}
		k := impl.Behavior()
		if _, dup := b.impls[k]; dup {
			return nil, fmt.Errorf("behavior %s is given twice", k)
		}
		b.impls[k] = impl
		b.order = append(b.order, k)
	}
	return b, nil
}

var defaultBehaviors = func() *Behaviors {
	b, err := NewBehaviors(DefaultBehaviorImpls()...)
	if err != nil {
		panic(err)
	}
	return b
}()

// DefaultBehaviors are branch, switch, wait, effect, call and end, and the flow behaviors parallel, join,
// foreach and collect.
func DefaultBehaviors() *Behaviors { return defaultBehaviors }

// DefaultBehaviorImpls are the implementations of DefaultBehaviors, to extend a set with NewBehaviors.
func DefaultBehaviorImpls() []BehaviorImpl {
	return append([]BehaviorImpl{branchBehavior{}, switchBehavior{}, waitBehavior{}, effectBehavior{}, callBehavior{}, endBehavior{}}, FlowBehaviors()...)
}

// Supported lists the behaviors of the set, in the order given.
func (b *Behaviors) Supported() []Behavior {
	return append([]Behavior(nil), b.order...)
}

func (b *Behaviors) impl(k Behavior) (BehaviorImpl, bool) {
	if b == nil {
		b = defaultBehaviors
	}
	impl, ok := b.impls[k]
	return impl, ok
}

// OutcomeRule is a node's outcomes as data: the values of From (a list param, or a field of each of its
// items), then Fixed, then Optional.Outcome when Optional.Param is set. Order is kept, duplicates dropped.
type OutcomeRule struct {
	From     *ParamRef        `json:"from,omitempty"`
	Fixed    []string         `json:"fixed"`
	Optional *OptionalOutcome `json:"optional,omitempty"`
	// Unconnected are outcomes that may be left without an edge: a run that would take one fails instead
	// (a join's timeout).
	Unconnected []string `json:"unconnected,omitempty"`
}

// OptionalOutcome is an outcome that exists when a param is set (a wait's timeout).
type OptionalOutcome struct {
	Param   string `json:"param"`
	Outcome string `json:"outcome"`
}

// OutcomesOf applies a rule to a node's params.
func OutcomesOf(r OutcomeRule, params map[string]any) []string {
	var out []string
	seen := map[string]bool{}
	add := func(o string) {
		if !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	for _, v := range fromValues(r.From, params) {
		add(v)
	}
	for _, o := range r.Fixed {
		add(o)
	}
	if r.Optional != nil {
		if v, ok := params[r.Optional.Param]; ok && v != nil {
			add(r.Optional.Outcome)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// fromValues are the string values of a param (or of a field of each of its items).
func fromValues(ref *ParamRef, params map[string]any) []string {
	if ref == nil {
		return nil
	}
	list, ok := params[ref.Param].([]any)
	if !ok {
		// A list built in Go ([]string, []map[string]any) is read through its JSON form.
		owned, err := jsonValue(params[ref.Param])
		if list, ok = owned.([]any); err != nil || !ok {
			return nil
		}
	}
	var out []string
	for _, item := range list {
		v := item
		if ref.Field != "" {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			v = m[ref.Field]
		}
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// GraphView is a read-only view of a graph against its catalog for structural rules: params as parsed,
// edges out of and into each node, reachability.
type GraphView struct {
	catalog  Catalog
	graph    Graph
	behavior *Behaviors
	limits   Limits
	subject  string
	where    map[string]string // id → "triggers[i]" or "nodes[i]" (the last one of a duplicate)
	nodes    map[string]int
	params   map[string]map[string]any
	// triggerParams are each trigger's params as its event parsed them (defaults applied).
	triggerParams map[string]map[string]any
	out           map[string][]Edge
	in            map[string][]Edge
	next          map[string]map[string]string // node → outcome → target (first edge)
}

// Catalog is the catalog the graph is checked against.
func (v *GraphView) Catalog() Catalog { return copyCatalog(v.catalog) }

// Graph is the graph.
func (v *GraphView) Graph() Graph { return copyGraph(v.graph) }

// Limits are the limits the graph is checked and run with.
func (v *GraphView) Limits() Limits { return v.limits }

// Subject is the graph's subject.
func (v *GraphView) Subject() string { return v.subject }

// Out lists the edges leaving a node or trigger.
func (v *GraphView) Out(node string) []Edge { return append([]Edge(nil), v.out[node]...) }

// In lists the edges entering a node.
func (v *GraphView) In(node string) []Edge { return append([]Edge(nil), v.in[node]...) }

// Params are a node's params as its type parsed them (defaults applied), a copy the caller may change;
// false when they are invalid.
func (v *GraphView) Params(node string) (map[string]any, bool) {
	p, ok := v.params[node]
	if !ok {
		return nil, false
	}
	return deepCopy(p).(map[string]any), true
}

// deepCopy copies JSON values: maps and lists are not shared.
func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[k] = deepCopy(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = deepCopy(e)
		}
		return out
	}
	return v
}

// Node returns a node by id.
func (v *GraphView) Node(id string) (Node, bool) {
	i, ok := v.nodes[id]
	if !ok {
		return Node{}, false
	}
	n := v.graph.Nodes[i]
	n.Params = copyMap(n.Params)
	return n, true
}

// Path is where a node or trigger is in the graph: "nodes[2]", "triggers[0]".
func (v *GraphView) Path(id string) string { return v.where[id] }

// Outcomes are a node's outcomes for its params, or a trigger's; nil when unknown.
func (v *GraphView) Outcomes(id string) []string {
	where, ok := v.where[id]
	if !ok {
		return nil
	}
	if where[0] == 't' {
		return []string{TriggerOutcome}
	}
	n, _ := v.Node(id)
	spec, ok := v.catalog.Nodes[n.Type]
	if !ok {
		return nil
	}
	impl, ok := v.behavior.impl(spec.Behavior)
	if !ok {
		return nil
	}
	p, ok := v.params[id]
	if !ok {
		return nil
	}
	return OutcomesOf(impl.OutcomeRule(copyNodeSpec(spec)), p)
}

// MayLeaveUnconnected reports whether an outcome of a node may have no edge (OutcomeRule.Unconnected).
func (v *GraphView) MayLeaveUnconnected(id, outcome string) bool {
	n, ok := v.Node(id)
	if !ok {
		return false
	}
	spec := v.catalog.Nodes[n.Type]
	impl, ok := v.behavior.impl(spec.Behavior)
	return ok && contains(impl.OutcomeRule(copyNodeSpec(spec)).Unconnected, outcome)
}

// Reachable is every node reachable from a node or trigger along edges (from itself included).
func (v *GraphView) Reachable(from string) map[string]bool {
	seen := map[string]bool{}
	var walk func(id string)
	walk = func(id string) {
		if seen[id] || id == Ignore {
			return
		}
		seen[id] = true
		for _, e := range v.out[id] {
			walk(e.To)
		}
	}
	walk(from)
	return seen
}

// newView snapshots the catalog and the graph: neither the caller nor a behavior can change them later.
func newView(c Catalog, g Graph, b *Behaviors) *GraphView {
	c, g = copyCatalog(c), copyGraph(g)
	v := &GraphView{
		catalog: c, graph: g, behavior: b,
		where: map[string]string{}, nodes: map[string]int{}, params: map[string]map[string]any{},
		triggerParams: map[string]map[string]any{},
		out:           map[string][]Edge{}, in: map[string][]Edge{}, next: map[string]map[string]string{},
	}
	for i, t := range g.Triggers {
		v.where[t.ID] = fmt.Sprintf("triggers[%d]", i)
	}
	for i, n := range g.Nodes {
		v.where[n.ID] = fmt.Sprintf("nodes[%d]", i)
		v.nodes[n.ID] = i
	}
	v.subject = graphSubject(c, g)
	return v
}

// graphSubject is what a graph is about: the subject of its triggers' events, or the catalog's mixed
// subject when they differ.
func graphSubject(c Catalog, g Graph) string {
	var subjects []string
	seen := map[string]bool{}
	for _, t := range g.Triggers {
		if e, ok := c.Events[t.Event]; ok && !seen[e.Subject] {
			seen[e.Subject] = true
			subjects = append(subjects, e.Subject)
		}
	}
	if len(subjects) > 1 && c.MixedSubject != "" {
		return c.MixedSubject
	}
	if len(subjects) > 0 {
		return subjects[0]
	}
	keys := sortedKeys(c.Subjects)
	if len(keys) > 0 {
		return keys[0]
	}
	return SubjectNone
}

func sortedOutcomes(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// copyMap copies params. Values built in Go ([]string, map[string]int, structs) become owned JSON values,
// so no container is shared with the caller.
func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	if owned, err := jsonValue(m); err == nil {
		if out, ok := owned.(map[string]any); ok {
			return out
		}
	}
	return deepCopy(m).(map[string]any)
}

func copyGraph(g Graph) Graph {
	out := g
	out.Triggers = make([]Trigger, len(g.Triggers))
	for i, t := range g.Triggers {
		t.Params = copyMap(t.Params)
		out.Triggers[i] = t
	}
	out.Nodes = make([]Node, len(g.Nodes))
	for i, n := range g.Nodes {
		n.Params = copyMap(n.Params)
		out.Nodes[i] = n
	}
	out.Edges = append([]Edge(nil), g.Edges...)
	return out
}

func copyCatalog(c Catalog) Catalog {
	out := c
	out.Events = make(map[string]EventSpec, len(c.Events))
	for k, e := range c.Events {
		e.Payload = e.Payload.Clone()
		if e.Match != nil {
			m := make(map[string]MatchField, len(e.Match))
			for name, f := range e.Match {
				m[name] = f
			}
			e.Match = m
		}
		out.Events[k] = e
	}
	out.Nodes = make(map[string]NodeSpec, len(c.Nodes))
	for k, n := range c.Nodes {
		out.Nodes[k] = copyNodeSpec(n)
	}
	out.Subjects = make(map[string]SubjectSpec, len(c.Subjects))
	for k, s := range c.Subjects {
		out.Subjects[k] = SubjectSpec{Context: s.Context.Clone()}
	}
	return out
}

// copyNodeSpec copies a node type's slices and pointers, so a behavior cannot change the catalog.
func copyNodeSpec(n NodeSpec) NodeSpec {
	n.Subjects = append([]string(nil), n.Subjects...)
	n.Outcomes = append([]string(nil), n.Outcomes...)
	n.Expressions = append([]ExprParam(nil), n.Expressions...)
	n.Templates = append([]string(nil), n.Templates...)
	n.Values = append([]string(nil), n.Values...)
	n.Output = n.Output.Clone()
	if n.OutcomesFrom != nil {
		r := *n.OutcomesFrom
		n.OutcomesFrom = &r
	}
	if n.OutputFrom != nil {
		r := *n.OutputFrom
		n.OutputFrom = &r
	}
	if n.Answer != nil {
		a := *n.Answer
		if a.Confidence != nil {
			f := *a.Confidence
			a.Confidence = &f
		}
		n.Answer = &a
	}
	if n.Wait != nil {
		w := *n.Wait
		n.Wait = &w
	}
	return n
}
