package graph

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

// Flow behaviors run several tokens in one run. A parallel starts a token per branch; the join of that
// parallel waits for them (all, any or n of them) and continues the forking token. A foreach runs its body
// — the part of the graph from its `item` outcome to its collect — once per item, each item a token, and
// continues with done, partial_failed or failed.
//
// Tokens are named by their path: `<parallel>.<branch>`, `<foreach>[<index>]`, nested ones joined by `/`
// (`split.legal/each[3]`). The main path is the empty token.

// Errors of the flow structure.
const (
	CodeParallelStructure   = "parallel_structure"
	CodeJoinForeignEdge     = "join_foreign_edge"
	CodeForeachEscape       = "foreach_escape"
	CodeCycleThroughForeach = "cycle_through_foreach"
)

// Outcomes of the flow behaviors.
const (
	OutcomeJoined        = "joined"
	OutcomeTimeout       = "timeout"
	OutcomeItem          = "item"
	OutcomeDone          = "done"
	OutcomePartialFailed = "partial_failed"
	OutcomeFailed        = "failed"
)

// Join modes.
const (
	JoinAll = "all"
	JoinAny = "any"
	JoinNOf = "n_of"
)

const idSchema = `{"type": "string", "pattern": "^[a-z][a-z0-9_]{0,39}$"}`

func parallelSchema() string {
	return `{"type": "object", "properties": {
		"branches": {"type": "array", "minItems": 2, "maxItems": 20, "x-unique": true, "items": ` + outcomeSchema + `,
			"description": "2 to 20 outcome names, each connected by one edge; every branch runs as its own token"}
	}, "required": ["branches"], "additionalProperties": false}`
}

func joinSchema() string {
	return `{"type": "object", "properties": {
		"of": ` + idSchema + `,
		"mode": {"enum": ["all", "any", "n_of"]},
		"n": {"type": "integer", "minimum": 1, "maximum": 20, "description": "for n_of: 1 to the parallel's number of branches"},
		"timeout": ` + durationSchema + `
	}, "required": ["of", "mode"], "additionalProperties": false}`
}

func foreachSchema() string {
	return `{"type": "object", "properties": {
		"items": ` + expressionSchema + `,
		"concurrency": {"type": "integer", "minimum": 1, "maximum": 20, "default": 1},
		"max_items": {"type": "integer", "minimum": 1, "maximum": 1000, "description": "more items take outcome failed without running the body; 1000 when left out"}
	}, "required": ["items"], "additionalProperties": false}`
}

func collectSchema() string {
	return `{"type": "object", "properties": {
		"of": ` + idSchema + `,
		"value": ` + expressionSchema + `
	}, "required": ["of"], "additionalProperties": false}`
}

var (
	joinOutput = expr.Fields{{Name: "branches", Field: expr.MapOf(expr.ObjectOf(expr.Fields{
		{Name: "output", Field: expr.DynField},
		{Name: "arrived", Field: expr.BoolField},
	}))}}
	foreachOutput = expr.Fields{
		{Name: "results", Field: expr.ListOf(expr.ObjectOf(expr.Fields{
			{Name: "index", Field: expr.IntField},
			{Name: "ok", Field: expr.BoolField},
			{Name: "value", Field: expr.DynField},
			{Name: "error", Field: expr.OptionalOf(expr.StringField)},
		}))},
		{Name: "error", Field: expr.OptionalOf(expr.StringField)},
		{Name: "code", Field: expr.OptionalOf(expr.StringField)},
	}
)

func flowNode(b Behavior, subjects []string) NodeSpec {
	switch b { //nolint:exhaustive // only the flow behaviors have flow nodes
	case BehaviorParallel:
		return NodeSpec{
			Type: "parallel", Title: "Parallel", Behavior: BehaviorParallel,
			Description: "Starts every branch at once, each as its own token.",
			Params:      MustSchemaParams(parallelSchema()), Subjects: subjects,
		}
	case BehaviorJoin:
		return NodeSpec{
			Type: "join", Title: "Join", Behavior: BehaviorJoin,
			Description: "Waits for all, any or n branches of its parallel, or its timeout counted from the fork.",
			Params:      MustSchemaParams(joinSchema()), Subjects: subjects, Output: joinOutput.Clone(),
		}
	case BehaviorForeach:
		return NodeSpec{
			Type: "foreach", Title: "For each", Behavior: BehaviorForeach,
			Description: "Runs the body from item to its collect once per item of a CEL list.",
			Params:      MustSchemaParams(foreachSchema()), Subjects: subjects,
			Expressions: []ExprParam{{Param: "items", Type: expr.TypeList}},
			Output:      foreachOutput.Clone(), FailedOutcome: OutcomeFailed,
		}
	case BehaviorCollect:
		return NodeSpec{
			Type: "collect", Title: "Collect", Behavior: BehaviorCollect,
			Description: "Ends the body of a foreach for one item, with an optional CEL value as its result.",
			Params:      MustSchemaParams(collectSchema()), Subjects: subjects,
			Expressions: []ExprParam{{Param: "value", Type: expr.TypeAny}},
		}
	}
	panic(fmt.Sprintf("graph: %s is not a flow behavior", b))
}

// FlowNodes are the node types of the flow behaviors: parallel, join, foreach and collect, available with
// these subjects (empty: any).
func FlowNodes(subjects []string) []NodeSpec {
	return []NodeSpec{
		flowNode(BehaviorParallel, subjects),
		flowNode(BehaviorJoin, subjects),
		flowNode(BehaviorForeach, subjects),
		flowNode(BehaviorCollect, subjects),
	}
}

// FlowBehaviors are the implementations of parallel, join, foreach and collect.
func FlowBehaviors() []BehaviorImpl {
	return []BehaviorImpl{parallelBehavior{}, joinBehavior{}, foreachBehavior{}, collectBehavior{}}
}

// ---------------------------------------------------------------------------------------------
// Tokens

// BranchToken is the token of a parallel's branch started by token parent.
func BranchToken(parent, parallel, branch string) string {
	return childToken(parent, parallel+"."+branch)
}

// ItemToken is the token of a foreach's item started by token parent.
func ItemToken(parent, foreach string, index int) string {
	return childToken(parent, foreach+"["+strconv.Itoa(index)+"]")
}

func childToken(parent, segment string) string {
	if parent == "" {
		return segment
	}
	return parent + "/" + segment
}

func lastSegment(token string) string { return token[strings.LastIndex(token, "/")+1:] }

// ParentToken is the token that started token: "" for a token of the main path.
func ParentToken(token string) string {
	i := strings.LastIndex(token, "/")
	if i < 0 {
		return ""
	}
	return token[:i]
}

// branchOf is the branch of a parallel token is, if it is one of that parallel's.
func branchOf(token, parallel string) (string, bool) {
	seg := lastSegment(token)
	if !strings.HasPrefix(seg, parallel+".") {
		return "", false
	}
	return seg[len(parallel)+1:], true
}

// itemOf is the index of a foreach's item token is, if it is one of that foreach's.
func itemOf(token, foreach string) (int, bool) {
	seg := lastSegment(token)
	if !strings.HasPrefix(seg, foreach+"[") || !strings.HasSuffix(seg, "]") {
		return 0, false
	}
	n, err := strconv.Atoi(seg[len(foreach)+1 : len(seg)-1])
	return n, err == nil && n >= 0
}

// ---------------------------------------------------------------------------------------------
// Params

func stringParam(p map[string]any, key string) string {
	s, _ := p[key].(string)
	return s
}

func intParam(p map[string]any, key string) (int, bool) {
	if p[key] == nil {
		return 0, false
	}
	f, ok := expr.Number(p[key])
	if !ok || f != float64(int(f)) {
		return 0, false
	}
	return int(f), true
}

func stringsParam(p map[string]any, key string) []string {
	list, _ := p[key].([]any)
	out := make([]string, 0, len(list))
	for _, x := range list {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ---------------------------------------------------------------------------------------------
// Structure

// flowGraph indexes a view's edges by position, for the structural rules of tokens.
type flowGraph struct {
	v     *GraphView
	edges []Edge
	out   map[string][]int
	in    map[string][]int
}

func newFlowGraph(v *GraphView) *flowGraph {
	f := &flowGraph{v: v, edges: v.graph.Edges, out: map[string][]int{}, in: map[string][]int{}}
	for k, e := range f.edges {
		f.out[e.From.Node] = append(f.out[e.From.Node], k)
		f.in[e.To] = append(f.in[e.To], k)
	}
	return f
}

func (f *flowGraph) behavior(id string) Behavior {
	i, ok := f.v.nodes[id]
	if !ok {
		return ""
	}
	return f.v.catalog.Nodes[f.v.graph.Nodes[i].Type].Behavior
}

// closers are the joins (of a parallel) or collects (of a foreach) of a node, in graph order.
func (f *flowGraph) closers(of string, kind Behavior) []string {
	var out []string
	for _, n := range f.v.graph.Nodes {
		if f.behavior(n.ID) == kind && stringParam(f.v.params[n.ID], "of") == of {
			out = append(out, n.ID)
		}
	}
	return out
}

// entries are the edges a construct's tokens start on: a parallel's branches, a foreach's item.
func (f *flowGraph) entries(construct string) []int {
	var out []int
	for _, k := range f.out[construct] {
		if f.behavior(construct) != BehaviorForeach || f.edges[k].From.Outcome == OutcomeItem {
			out = append(out, k)
		}
	}
	return out
}

// region is where the tokens of a construct run.
type region struct {
	members map[string]bool // nodes the tokens step on; a nested construct's own node, not its inside
	nested  map[string]bool // joins of nested parallels: their outcomes continue inside the region
	toSelf  []int           // edges back into the construct
}

// region walks from a construct's entries. A nested parallel is passed over to its join's outcomes, a
// nested foreach to its outcomes besides item; a join or collect stops the walk (its own rules read the
// edge into it).
func (f *flowGraph) region(construct string) region {
	r := region{members: map[string]bool{}, nested: map[string]bool{}}
	stack := append([]int(nil), f.entries(construct)...)
	seen := map[int]bool{}
	push := func(ks []int, skipItem bool) {
		for _, k := range ks {
			if !skipItem || f.edges[k].From.Outcome != OutcomeItem {
				stack = append(stack, k)
			}
		}
	}
	for len(stack) > 0 {
		k := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[k] {
			continue
		}
		seen[k] = true
		to := f.edges[k].To
		if to == construct {
			r.toSelf = append(r.toSelf, k)
			continue
		}
		if _, ok := f.v.nodes[to]; !ok || r.members[to] {
			continue
		}
		switch f.behavior(to) { //nolint:exhaustive // joins and collects close a region
		case BehaviorJoin, BehaviorCollect:
			continue
		}
		r.members[to] = true
		switch f.behavior(to) { //nolint:exhaustive // only constructs are passed over
		case BehaviorParallel:
			for _, j := range f.closers(to, BehaviorJoin) {
				r.nested[j] = true
				push(f.out[j], false)
			}
		case BehaviorForeach:
			push(f.out[to], true)
		default:
			push(f.out[to], false)
		}
	}
	sort.Ints(r.toSelf)
	return r
}

// inside reports whether an edge runs in the region: from a member (not into a nested construct's
// branches or body), from a nested join, or from the construct through one of its entries.
func (f *flowGraph) inside(r region, construct string, k int) bool {
	from := f.edges[k].From
	if r.members[from.Node] {
		// A nested parallel's branches and a nested foreach's body run in their own regions.
		switch f.behavior(from.Node) { //nolint:exhaustive // only constructs run their own regions
		case BehaviorParallel:
			return false
		case BehaviorForeach:
			return from.Outcome != OutcomeItem
		}
		return true
	}
	if r.nested[from.Node] {
		return true
	}
	if from.Node != construct {
		return false
	}
	return f.behavior(construct) != BehaviorForeach || from.Outcome == OutcomeItem
}

// closure reports each node of the region that is also entered from outside it.
func (f *flowGraph) closure(construct string, r region, code, what string) []GraphError {
	var out []GraphError
	for _, n := range f.v.graph.Nodes {
		if !r.members[n.ID] {
			continue
		}
		for _, k := range f.in[n.ID] {
			if !f.inside(r, construct, k) {
				out = append(out, GraphError{Path: f.v.Path(n.ID), Code: code, Message: fmt.Sprintf(
					"%s is inside %s %s and is also entered from %s, outside it", n.ID, what, construct, f.edges[k].From.Node)})
				break
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------------------------
// parallel

type parallelBehavior struct{}

func (parallelBehavior) Behavior() Behavior { return BehaviorParallel }
func (parallelBehavior) OutcomeRule(NodeSpec) OutcomeRule {
	return OutcomeRule{From: &ParamRef{Param: "branches"}, Fixed: []string{}}
}

// Validate: at most Limits.MaxParallelBranches branches; every branch ends in this parallel's join or an
// end, does not loop back into the parallel, and no node of the branches is entered from outside them.
func (parallelBehavior) Validate(v *GraphView, node Node, _ NodeSpec) []GraphError {
	params, _ := v.Params(node.ID)
	at := v.Path(node.ID)
	out := make([]GraphError, 0, 1)
	if n, most := len(stringsParam(params, "branches")), v.limits.ParallelBranches(); n > most {
		out = append(out, GraphError{Path: at + ".params.branches", Code: CodeLimit, Message: fmt.Sprintf("%d branches, at most %d", n, most)})
	}
	f := newFlowGraph(v)
	r := f.region(node.ID)
	for _, k := range r.toSelf {
		out = append(out, GraphError{Path: fmt.Sprintf("edges[%d]", k), Code: CodeParallelStructure, Message: fmt.Sprintf(
			"a branch of %s leads back into it: a branch ends in its join or an end", node.ID)})
	}
	return append(out, f.closure(node.ID, r, CodeParallelStructure, "the branches of")...)
}

func (parallelBehavior) Start(s *StepContext, node Node, _ NodeSpec) Step {
	params := s.Params(node)
	branches := stringsParam(params, "branches")
	st := Step{Kind: StepFork, Fork: make([]Fork, 0, len(branches))}
	for _, b := range branches {
		to, ok := s.Next(node.ID, b)
		if !ok {
			return failed(FailInvalidGraph, "branch %s of %s has no edge", b, node.ID)
		}
		st.Fork = append(st.Fork, Fork{Token: BranchToken(s.Token, node.ID, b), Node: to})
	}
	if v := s.View(); v != nil {
		if joins := newFlowGraph(v).closers(node.ID, BehaviorJoin); len(joins) > 0 {
			jp, _ := v.Params(joins[0])
			if ms, ok := DurationMs(jp["timeout"]); ok && jp["timeout"] != nil {
				st.Wait = &WaitIntent{Timeout: msDuration(ms)}
			}
		}
	}
	return st
}

// ---------------------------------------------------------------------------------------------
// join

type joinBehavior struct{}

func (joinBehavior) Behavior() Behavior { return BehaviorJoin }
func (joinBehavior) OutcomeRule(NodeSpec) OutcomeRule {
	return OutcomeRule{Fixed: []string{OutcomeJoined}, Optional: &OptionalOutcome{Param: "timeout", Outcome: OutcomeTimeout}}
}

// Validate: of is a parallel with no other join; n is set for n_of only, at most the parallel's branches;
// every edge into the join comes from that parallel's branches (join_foreign_edge).
func (joinBehavior) Validate(v *GraphView, node Node, _ NodeSpec) []GraphError {
	params, _ := v.Params(node.ID)
	at := v.Path(node.ID)
	f := newFlowGraph(v)
	of := stringParam(params, "of")
	if f.behavior(of) != BehaviorParallel {
		return []GraphError{{Path: at + ".params.of", Code: CodeParams, Message: fmt.Sprintf("%s is not a parallel of the graph", of)}}
	}
	var out []GraphError
	if joins := f.closers(of, BehaviorJoin); len(joins) > 0 && joins[0] != node.ID {
		out = append(out, GraphError{Path: at + ".params.of", Code: CodeParams, Message: fmt.Sprintf("%s already has join %s", of, joins[0])})
	}
	mode := stringParam(params, "mode")
	n, hasN := intParam(params, "n")
	switch {
	case mode == JoinNOf && !hasN:
		out = append(out, GraphError{Path: at + ".params.n", Code: CodeParams, Message: "n_of needs n"})
	case mode != JoinNOf && params["n"] != nil:
		out = append(out, GraphError{Path: at + ".params.n", Code: CodeParams, Message: "n is for mode n_of"})
	case hasN:
		pp, ok := v.Params(of)
		if branches := len(stringsParam(pp, "branches")); ok && n > branches {
			out = append(out, GraphError{Path: at + ".params.n", Code: CodeParams, Message: fmt.Sprintf("%s has %d branches", of, branches)})
		}
	}
	r := f.region(of)
	for _, k := range f.in[node.ID] {
		if !f.inside(r, of, k) {
			out = append(out, GraphError{Path: fmt.Sprintf("edges[%d]", k), Code: CodeJoinForeignEdge, Message: fmt.Sprintf(
				"%s joins the branches of %s; %s is not one of them", node.ID, of, f.edges[k].From.Node)})
		}
	}
	return out
}

func (joinBehavior) Start(s *StepContext, node Node, _ NodeSpec) Step {
	of := stringParam(s.Params(node), "of")
	if _, ok := branchOf(s.Token, of); !ok {
		return failed(FailInvalidGraph, "token %q is not a branch of %s", s.Token, of)
	}
	var value any
	if s.From != "" && s.From != of {
		// An empty output ({} of an effect or of a wait's timeout) is no output.
		if st, ok := s.Ctx.Nodes[s.From]; ok && len(st.Output) > 0 {
			value = deepCopy(st.Output)
		}
	}
	return Step{Kind: StepArrive, Arrive: &Arrival{Of: of, Token: s.Token, Value: value}}
}

// JoinState is what a join has seen of its parallel's branches in a run, by branch name.
type JoinState struct {
	Arrived map[string]Arrival `json:"arrived"`
	// Gone are branches whose token ended without arriving: an end, ignore, a failure.
	Gone     map[string]bool `json:"gone"`
	TimedOut bool            `json:"timed_out"` // the join's timeout fired
}

// Join decides a join for the token that forked its parallel: StepWait while it still waits; StepOutcome
// joined (or timeout) with the output and the branch tokens to cancel, in branch order; StepFailed when
// the branches left cannot give what it waits for and it has no timeout.
func (i *Interpreter) Join(joinID, token string, st JoinState) Step {
	params, ok := i.view.Params(joinID)
	node, _ := i.view.Node(joinID)
	if !ok || i.view.catalog.Nodes[node.Type].Behavior != BehaviorJoin {
		return failed(FailInvalidGraph, "%s is not a join", joinID)
	}
	of := stringParam(params, "of")
	pp, _ := i.view.Params(of)
	branches := stringsParam(pp, "branches")
	need := len(branches)
	switch stringParam(params, "mode") {
	case JoinAny:
		need = 1
	case JoinNOf:
		need, _ = intParam(params, "n")
	}
	arrived, open := 0, 0
	result := make(map[string]any, len(branches))
	var cancel []string
	for _, b := range branches {
		if a, ok := st.Arrived[b]; ok {
			arrived++
			result[b] = map[string]any{"output": a.Value, "arrived": true}
			continue
		}
		result[b] = map[string]any{"output": nil, "arrived": false}
		if !st.Gone[b] {
			open++
			cancel = append(cancel, BranchToken(token, of, b))
		}
	}
	output := map[string]any{"branches": result}
	switch {
	case arrived >= need:
		return Step{Kind: StepOutcome, Outcome: OutcomeJoined, Output: output, Cancel: cancel}
	case st.TimedOut || arrived+open < need:
		if params["timeout"] != nil {
			return Step{Kind: StepOutcome, Outcome: OutcomeTimeout, Output: output, Cancel: cancel}
		}
		s := failed(FailJoinIncomplete, "%s waits for %d branches of %s; %d arrived and %d may still", joinID, need, of, arrived, open)
		s.Cancel = cancel
		return s
	}
	return Step{Kind: StepWait}
}

// ---------------------------------------------------------------------------------------------
// foreach

type foreachBehavior struct{}

func (foreachBehavior) Behavior() Behavior { return BehaviorForeach }
func (foreachBehavior) OutcomeRule(NodeSpec) OutcomeRule {
	return OutcomeRule{Fixed: []string{OutcomeItem, OutcomeDone, OutcomePartialFailed, OutcomeFailed}}
}

// Validate: concurrency and max_items within Limits; the foreach has a collect; everything its body
// reaches stays inside it (foreach_escape); no cycle passes it (cycle_through_foreach).
func (foreachBehavior) Validate(v *GraphView, node Node, _ NodeSpec) []GraphError {
	params, _ := v.Params(node.ID)
	at := v.Path(node.ID)
	var out []GraphError
	if n, ok := intParam(params, "concurrency"); ok && n > v.limits.ForeachConcurrency() {
		out = append(out, GraphError{Path: at + ".params.concurrency", Code: CodeLimit, Message: fmt.Sprintf("at most %d", v.limits.ForeachConcurrency())})
	}
	if n, ok := intParam(params, "max_items"); ok && n > v.limits.ForeachItems() {
		out = append(out, GraphError{Path: at + ".params.max_items", Code: CodeLimit, Message: fmt.Sprintf("at most %d", v.limits.ForeachItems())})
	}
	f := newFlowGraph(v)
	if len(f.closers(node.ID, BehaviorCollect)) == 0 {
		out = append(out, GraphError{Path: at, Code: CodeInvalid, Message: fmt.Sprintf("%s has no collect: its body ends in a collect of it", node.ID)})
	}
	for _, e := range v.out[node.ID] {
		if v.Reachable(e.To)[node.ID] {
			out = append(out, GraphError{Path: at, Code: CodeCycleThroughForeach, Message: fmt.Sprintf("a cycle passes %s: a foreach runs once per entry of its run", node.ID)})
			break
		}
	}
	return append(out, f.closure(node.ID, f.region(node.ID), CodeForeachEscape, "the body of")...)
}

func (foreachBehavior) Start(s *StepContext, node Node, _ NodeSpec) Step {
	params := s.Params(node)
	v, st := s.Eval(stringParam(params, "items"), expr.TypeList)
	if st != nil {
		return *st
	}
	fail := func(code, msg string) Step {
		return Step{Kind: StepOutcome, Outcome: OutcomeFailed, Output: map[string]any{"results": []any{}, "error": msg, "code": code}}
	}
	list, ok := v.([]any)
	if !ok {
		return fail(FailInvalidInput, fmt.Sprintf("items is a %T, not a list", v))
	}
	most := Limits{}.ForeachItems()
	if view := s.View(); view != nil {
		most = view.limits.ForeachItems()
	}
	if n, ok := intParam(params, "max_items"); ok && n < most {
		most = n
	}
	if len(list) > most {
		return fail(FailLimit, fmt.Sprintf("%d items, at most %d", len(list), most))
	}
	if len(list) == 0 {
		return Step{Kind: StepOutcome, Outcome: OutcomeDone, Output: map[string]any{"results": []any{}}}
	}
	items := make([]any, len(list))
	for k, x := range list {
		j, err := jsonOut(x)
		if err != nil {
			return fail(FailInvalidInput, fmt.Sprintf("item %d: %v", k, err))
		}
		items[k] = j
	}
	concurrency, ok := intParam(params, "concurrency")
	if !ok {
		concurrency = 1
	}
	return Step{Kind: StepEach, Each: &Each{Items: items, Concurrency: concurrency}}
}

// ItemResult is how one item of a foreach ended.
type ItemResult struct {
	Index int    `json:"index"`
	OK    bool   `json:"ok"`
	Value any    `json:"value"`
	Error string `json:"error,omitempty"`
}

// ForeachDone is the step of a foreach once every item ended: done when all are ok, else
// partial_failed, with the results by index.
func (i *Interpreter) ForeachDone(foreachID string, results []ItemResult) Step {
	sorted := append([]ItemResult(nil), results...)
	sort.SliceStable(sorted, func(a, b int) bool { return sorted[a].Index < sorted[b].Index })
	outcome := OutcomeDone
	list := make([]any, len(sorted))
	for k, r := range sorted {
		m := map[string]any{"index": r.Index, "ok": r.OK, "value": r.Value}
		if !r.OK {
			outcome = OutcomePartialFailed
			m["error"] = r.Error
		}
		list[k] = m
	}
	if node, ok := i.view.Node(foreachID); !ok || i.view.catalog.Nodes[node.Type].Behavior != BehaviorForeach {
		return failed(FailInvalidGraph, "%s is not a foreach", foreachID)
	}
	return Step{Kind: StepOutcome, Outcome: outcome, Output: map[string]any{"results": list}}
}

// ---------------------------------------------------------------------------------------------
// collect

type collectBehavior struct{}

func (collectBehavior) Behavior() Behavior { return BehaviorCollect }
func (collectBehavior) OutcomeRule(NodeSpec) OutcomeRule {
	return OutcomeRule{Fixed: []string{}}
}

// Validate: of is a foreach, and every edge into the collect comes from that foreach's body.
func (collectBehavior) Validate(v *GraphView, node Node, _ NodeSpec) []GraphError {
	params, _ := v.Params(node.ID)
	f := newFlowGraph(v)
	of := stringParam(params, "of")
	if f.behavior(of) != BehaviorForeach {
		return []GraphError{{Path: v.Path(node.ID) + ".params.of", Code: CodeParams, Message: fmt.Sprintf("%s is not a foreach of the graph", of)}}
	}
	var out []GraphError
	r := f.region(of)
	for _, k := range f.in[node.ID] {
		if !f.inside(r, of, k) {
			out = append(out, GraphError{Path: fmt.Sprintf("edges[%d]", k), Code: CodeForeachEscape, Message: fmt.Sprintf(
				"%s collects the body of %s; %s is not in it", node.ID, of, f.edges[k].From.Node)})
		}
	}
	return out
}

func (collectBehavior) Start(s *StepContext, node Node, _ NodeSpec) Step {
	params := s.Params(node)
	of := stringParam(params, "of")
	if _, ok := itemOf(s.Token, of); !ok {
		return failed(FailInvalidGraph, "token %q is not an item of %s", s.Token, of)
	}
	var value any
	if src := stringParam(params, "value"); src != "" {
		v, st := s.Eval(src, expr.TypeAny)
		if st != nil {
			return *st
		}
		j, err := jsonOut(v)
		if err != nil {
			return failed(FailExpression, "value: %v", err)
		}
		value = j
	}
	return Step{Kind: StepArrive, Arrive: &Arrival{Of: of, Token: s.Token, Value: value}}
}

func msDuration(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond }

// tokenLimit fails a fork or a foreach that would make more than Limits.MaxTokens tokens active: the
// forking token waits, its new tokens run (a foreach's at most concurrency at once).
func (i *Interpreter) tokenLimit(active int, st Step) *Step {
	var n int
	switch st.Kind { //nolint:exhaustive // only forks and foreachs make tokens
	case StepFork:
		n = len(st.Fork)
	case StepEach:
		n = len(st.Each.Items)
		if c := st.Each.Concurrency; c > 0 && c < n {
			n = c
		}
	default:
		return nil
	}
	if active < 1 {
		active = 1
	}
	if most := i.view.limits.Tokens(); active-1+n > most {
		s := failed(FailLimit, "%d active tokens, at most %d", active-1+n, most)
		return &s
	}
	return nil
}
