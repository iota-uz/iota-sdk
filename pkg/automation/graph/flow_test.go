package graph_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// flowCatalog is the conformance catalog with the four flow node types.
func flowCatalog(t *testing.T) graph.Catalog {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "conformance", "plot", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	nodes := doc["nodes"].(map[string]any)
	for _, b := range []string{"parallel", "join", "foreach", "collect"} {
		nodes[b] = map[string]any{"behavior": b}
	}
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	c, err := graph.CatalogFromJSON(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func flowGraph(t *testing.T, nodes, edges string) graph.Graph {
	t.Helper()
	return mustDecode(t, `{"name": "flow", "triggers": [{"id": "labeled", "event": "issue.labeled"}], "nodes": [`+nodes+`], "edges": [`+edges+`]}`)
}

func edge(from, outcome, to string) string {
	return fmt.Sprintf(`{"from": {"node": %q, "outcome": %q}, "to": %q}`, from, outcome, to)
}

func dryRun(t *testing.T, c graph.Catalog, g graph.Graph, o graph.Options, vars map[string]any, inputs ...graph.DryRunInput) []graph.TraceStep {
	t.Helper()
	in, err := graph.NewInterpreter(c, g, o)
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Vars: vars}
	all := append([]graph.DryRunInput{{Event: graph.Event{"kind": "issue.labeled", "label": "x"}}}, inputs...)
	return in.DryRun(graph.Subject{}, ctx, all, graph.DryRunOptions{})
}

func tokensOf(trace []graph.TraceStep) []string {
	var out []string
	for _, s := range trace {
		if s.Token != "" {
			out = append(out, s.Token)
		}
	}
	return out
}

// The same graph with concurrency 1 and 20 gives the same trace: items run in their order.
func TestForeachConcurrencyTrace(t *testing.T) {
	c := flowCatalog(t)
	build := func(concurrency int) graph.Graph {
		return flowGraph(t,
			fmt.Sprintf(`{"id": "each", "type": "foreach", "params": {"items": "[1, 2, 3]", "concurrency": %d}},
				{"id": "hold", "type": "wait", "params": {"event": "pull_request.opened", "filter": "event.number == foreach.each.item"}},
				{"id": "c", "type": "collect", "params": {"of": "each", "value": "foreach.each.index"}}`, concurrency),
			strings.Join([]string{edge("labeled", "fired", "each"), edge("each", "item", "hold"), edge("hold", "matched", "c"),
				edge("each", "done", "ignore"), edge("each", "partial_failed", "ignore"), edge("each", "failed", "ignore")}, ","))
	}
	events := []graph.DryRunInput{
		{Event: graph.Event{"kind": "pull_request.opened", "number": 3, "repository": "a"}},
		{Event: graph.Event{"kind": "pull_request.opened", "number": 1, "repository": "a"}},
		{Event: graph.Event{"kind": "pull_request.opened", "number": 2, "repository": "a"}},
	}
	one := dryRun(t, c, build(1), graph.Options{}, nil, events...)
	twenty := dryRun(t, c, build(20), graph.Options{}, nil, events...)
	if !reflect.DeepEqual(one, twenty) {
		t.Fatalf("concurrency changes the trace:\n 1: %+v\n20: %+v", one, twenty)
	}
	if last := one[len(one)-2]; last.Node != "each" || last.Outcome != graph.OutcomeDone {
		t.Fatalf("trace: %+v", one)
	}
}

// 21 branches are refused at validation; 1001 items fail the foreach at run time with code LIMIT and
// no item token; a fork beyond MaxTokens fails the run before any token exists.
func TestFlowLimits(t *testing.T) {
	c := flowCatalog(t)
	branches := make([]string, 21)
	edges := []string{edge("labeled", "fired", "split")}
	for k := range branches {
		branches[k] = fmt.Sprintf("%q", fmt.Sprintf("b%d", k))
		edges = append(edges, edge("split", fmt.Sprintf("b%d", k), "ignore"))
	}
	wide := flowGraph(t, `{"id": "split", "type": "parallel", "params": {"branches": [`+strings.Join(branches, ",")+`]}}`, strings.Join(edges, ","))
	errs := graph.Validate(c, wide, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[0].params.branches" || errs[0].Code != graph.CodeLimit {
		t.Fatalf("21 branches: %v", errs)
	}

	// A lower limit of the options holds too.
	three := flowGraph(t, `{"id": "split", "type": "parallel", "params": {"branches": ["a", "b", "c"]}}`,
		strings.Join([]string{edge("labeled", "fired", "split"), edge("split", "a", "ignore"), edge("split", "b", "ignore"), edge("split", "c", "ignore")}, ","))
	errs = graph.Validate(c, three, graph.Options{Limits: graph.Limits{MaxParallelBranches: 2}})
	if len(errs) != 1 || errs[0].Code != graph.CodeLimit {
		t.Fatalf("a lower branch limit: %v", errs)
	}

	labels := make([]any, 1001)
	for k := range labels {
		labels[k] = fmt.Sprintf("l%d", k)
	}
	vars := map[string]any{"issue": map[string]any{"repository": "a", "number": 1, "title": "t", "labels": labels, "state": "open"}}
	each := flowGraph(t, `{"id": "each", "type": "foreach", "params": {"items": "issue.labels"}}, {"id": "c", "type": "collect", "params": {"of": "each"}}`,
		strings.Join([]string{edge("labeled", "fired", "each"), edge("each", "item", "c"), edge("each", "done", "ignore"),
			edge("each", "partial_failed", "ignore"), edge("each", "failed", "ignore")}, ","))
	trace := dryRun(t, c, each, graph.Options{}, vars)
	if len(trace) != 3 || trace[1].Outcome != graph.OutcomeFailed || len(tokensOf(trace)) != 0 {
		t.Fatalf("1001 items: %+v", trace)
	}
	if out, _ := trace[1].ContextPatch["output"].(map[string]any); out["code"] != graph.FailLimit {
		t.Fatalf("1001 items: %+v", trace[1])
	}
	vars["issue"].(map[string]any)["labels"] = labels[:1000]
	if trace := dryRun(t, c, each, graph.Options{}, vars); trace[len(trace)-2].Outcome != graph.OutcomeDone || len(tokensOf(trace)) != 1000 {
		t.Fatalf("1000 items: %d steps", len(trace))
	}

	trace = dryRun(t, c, three, graph.Options{Limits: graph.Limits{MaxTokens: 2}}, nil)
	if len(trace) != 2 || trace[1].Outcome != graph.TraceFailed || trace[1].ContextPatch["code"] != graph.FailLimit || len(tokensOf(trace)) != 0 {
		t.Fatalf("MaxTokens: %+v", trace)
	}
	in, err := graph.NewInterpreter(c, three, graph.Options{Limits: graph.Limits{MaxTokens: 4}})
	if err != nil {
		t.Fatal(err)
	}
	if st := in.Start("split", &graph.StepContext{Tokens: 2}); st.Kind != graph.StepFork || len(st.Fork) != 3 {
		t.Fatalf("2 active, 3 more of 4: %+v", st)
	}
	if st := in.Start("split", &graph.StepContext{Tokens: 3}); st.Kind != graph.StepFailed || st.Code != graph.FailLimit {
		t.Fatalf("3 active, 3 more of 4: %+v", st)
	}
}

func joinGraph(t *testing.T, mode, extra string) graph.Graph {
	t.Helper()
	edges := []string{edge("labeled", "fired", "split"), edge("split", "a", "j"), edge("split", "b", "j"), edge("split", "c", "j"), edge("j", "joined", "ignore")}
	if strings.Contains(extra, "timeout") {
		edges = append(edges, edge("j", "timeout", "ignore"))
	}
	return flowGraph(t, `{"id": "split", "type": "parallel", "params": {"branches": ["a", "b", "c"]}},
		{"id": "j", "type": "join", "params": {"of": "split", "mode": "`+mode+`"`+extra+`}}`, strings.Join(edges, ","))
}

// Join decides from its state alone, for the engine as for dry run.
func TestJoinDecision(t *testing.T) {
	c := flowCatalog(t)
	arrived := func(bs ...string) map[string]graph.Arrival {
		m := map[string]graph.Arrival{}
		for _, b := range bs {
			m[b] = graph.Arrival{Of: "split", Token: "x/split." + b, Value: map[string]any{"b": b}}
		}
		return m
	}
	interp := func(g graph.Graph) *graph.Interpreter {
		in, err := graph.NewInterpreter(c, g, graph.Options{})
		if err != nil {
			t.Fatal(err)
		}
		return in
	}
	all := interp(joinGraph(t, "all", ""))
	if st := all.Join("j", "x", graph.JoinState{Arrived: arrived("a", "b")}); st.Kind != graph.StepWait {
		t.Fatalf("all, 2 of 3: %+v", st)
	}
	st := all.Join("j", "x", graph.JoinState{Arrived: arrived("a", "b", "c")})
	if st.Outcome != graph.OutcomeJoined || len(st.Cancel) != 0 {
		t.Fatalf("all, 3 of 3: %+v", st)
	}
	if b := st.Output["branches"].(map[string]any)["b"].(map[string]any); b["arrived"] != true || !reflect.DeepEqual(b["output"], map[string]any{"b": "b"}) {
		t.Fatalf("output: %+v", st.Output)
	}
	if st := all.Join("j", "x", graph.JoinState{Arrived: arrived("a"), Gone: map[string]bool{"b": true}}); st.Kind != graph.StepFailed || st.Code != graph.FailJoinIncomplete || !reflect.DeepEqual(st.Cancel, []string{"x/split.c"}) {
		t.Fatalf("all, a branch gone, no timeout: %+v", st)
	}
	anyJoin := interp(joinGraph(t, "any", ""))
	if st := anyJoin.Join("j", "x", graph.JoinState{Arrived: arrived("b")}); st.Outcome != graph.OutcomeJoined || !reflect.DeepEqual(st.Cancel, []string{"x/split.a", "x/split.c"}) {
		t.Fatalf("any: %+v", st)
	}
	nOf := interp(joinGraph(t, "n_of", `, "n": 2, "timeout": "1h"`))
	if st := nOf.Join("j", "x", graph.JoinState{Arrived: arrived("c"), Gone: map[string]bool{"a": true}}); st.Kind != graph.StepWait {
		t.Fatalf("n_of, 1 of 2, one may still: %+v", st)
	}
	if st := nOf.Join("j", "x", graph.JoinState{Arrived: arrived("c"), Gone: map[string]bool{"a": true, "b": true}}); st.Outcome != graph.OutcomeTimeout || len(st.Cancel) != 0 {
		t.Fatalf("n_of, cannot reach 2: %+v", st)
	}
	if st := nOf.Join("j", "x", graph.JoinState{TimedOut: true}); st.Outcome != graph.OutcomeTimeout || len(st.Cancel) != 3 {
		t.Fatalf("n_of, timed out with none arrived: %+v", st)
	}
	// The fork carries its join's timeout, counted from the fork.
	if st := nOf.Start("split", &graph.StepContext{Token: "x"}); st.Kind != graph.StepFork || st.Wait == nil || st.Wait.Timeout.Hours() != 1 || st.Fork[2].Token != "x/split.c" || st.Fork[2].Node != "j" {
		t.Fatalf("fork: %+v", st)
	}
	// A join without timeout whose branches cannot all arrive fails the run in a dry run.
	g := flowGraph(t, `{"id": "split", "type": "parallel", "params": {"branches": ["a", "b"]}},
		{"id": "hold", "type": "wait", "params": {"event": "manual"}},
		{"id": "j", "type": "join", "params": {"of": "split", "mode": "all"}}`,
		strings.Join([]string{edge("labeled", "fired", "split"), edge("split", "a", "hold"), edge("split", "b", "ignore"),
			edge("hold", "matched", "j"), edge("j", "joined", "ignore")}, ","))
	trace := dryRun(t, c, g, graph.Options{}, nil)
	last := trace[len(trace)-1]
	if last.Node != "j" || last.Outcome != graph.TraceFailed || last.ContextPatch["code"] != graph.FailJoinIncomplete || trace[len(trace)-2].Outcome != graph.TraceCancelled {
		t.Fatalf("incomplete: %+v", trace)
	}
}

// Constructs nest: a parallel inside a foreach's body names its tokens under the item's, and the join
// continues the item token.
func TestNestedFlow(t *testing.T) {
	c := flowCatalog(t)
	g := flowGraph(t, `{"id": "each", "type": "foreach", "params": {"items": "[\"x\", \"y\"]"}},
		{"id": "split", "type": "parallel", "params": {"branches": ["a", "b"]}},
		{"id": "note", "type": "action", "params": {"action": "notify", "text": "{{ foreach.each.item }}"}},
		{"id": "j", "type": "join", "params": {"of": "split", "mode": "all"}},
		{"id": "c", "type": "collect", "params": {"of": "each", "value": "nodes.j.output.branches.a.arrived"}}`,
		strings.Join([]string{edge("labeled", "fired", "each"), edge("each", "item", "split"), edge("split", "a", "note"), edge("split", "b", "j"),
			edge("note", "done", "j"), edge("note", "failed", "ignore"), edge("j", "joined", "c"),
			edge("each", "done", "ignore"), edge("each", "partial_failed", "ignore"), edge("each", "failed", "ignore")}, ","))
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("nested: %v", errs)
	}
	trace := dryRun(t, c, g, graph.Options{}, nil)
	want := []string{"each[0]/split.a", "each[0]/split.a", "each[0]/split.b", "each[0]", "each[0]",
		"each[1]/split.a", "each[1]/split.a", "each[1]/split.b", "each[1]", "each[1]"}
	if got := tokensOf(trace); !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens %v\ntrace %+v", got, trace)
	}
	done := trace[len(trace)-2]
	results, _ := done.ContextPatch["output"].(map[string]any)["results"].([]any)
	if done.Outcome != graph.OutcomeDone || len(results) != 2 || results[1].(map[string]any)["value"] != true {
		t.Fatalf("done: %+v", done)
	}
	if got := graph.BranchToken(graph.ItemToken("", "each", 3), "split", "legal"); got != "each[3]/split.legal" || graph.ParentToken(got) != "each[3]" {
		t.Fatalf("token names: %s", got)
	}

	// A body that leaves through a nested parallel's branch into the collect escapes the foreach.
	bad := flowGraph(t, `{"id": "each", "type": "foreach", "params": {"items": "[1]"}},
		{"id": "split", "type": "parallel", "params": {"branches": ["a", "b"]}},
		{"id": "c", "type": "collect", "params": {"of": "each"}}`,
		strings.Join([]string{edge("labeled", "fired", "each"), edge("each", "item", "split"), edge("split", "a", "c"), edge("split", "b", "ignore"),
			edge("each", "done", "ignore"), edge("each", "partial_failed", "ignore"), edge("each", "failed", "ignore")}, ","))
	errs := graph.Validate(c, bad, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "edges[2]" || errs[0].Code != graph.CodeForeachEscape {
		t.Fatalf("escape through a branch: %v", errs)
	}
}

// The flow node types: params as data for clients, outcome rules, a JSON catalog takes them by behavior,
// and join, foreach and collect check their params.
func TestFlowNodes(t *testing.T) {
	var schemas map[string]json.RawMessage
	if err := json.Unmarshal(graph.BuiltinsJSONSchema(), &schemas); err != nil {
		t.Fatal(err)
	}
	for _, n := range graph.FlowNodes(nil) {
		if _, ok := schemas[n.Type]; !ok {
			t.Errorf("BuiltinsJSONSchema has no %s", n.Type)
		}
	}
	c := flowCatalog(t)
	view := graph.View(c, nil)
	rules := map[string]graph.OutcomeRule{}
	for _, n := range view.Nodes {
		rules[n.Type] = n.OutcomeRule
	}
	if r := rules["foreach"]; !reflect.DeepEqual(r.Fixed, []string{"item", "done", "partial_failed", "failed"}) {
		t.Errorf("foreach outcomes: %+v", r)
	}
	if r := rules["join"]; r.Optional == nil || r.Optional.Outcome != "timeout" {
		t.Errorf("join outcomes: %+v", r)
	}
	if r := rules["parallel"]; r.From == nil || r.From.Param != "branches" {
		t.Errorf("parallel outcomes: %+v", r)
	}

	g := flowGraph(t, `{"id": "split", "type": "parallel", "params": {"branches": ["a", "b"]}},
		{"id": "j", "type": "join", "params": {"of": "split", "mode": "n_of", "n": 3}},
		{"id": "k", "type": "join", "params": {"of": "split", "mode": "all", "n": 1}},
		{"id": "each", "type": "foreach", "params": {"items": "issue.title"}},
		{"id": "c", "type": "collect", "params": {"of": "split"}}`,
		strings.Join([]string{edge("labeled", "fired", "split"), edge("split", "a", "j"), edge("split", "b", "k"),
			edge("j", "joined", "each"), edge("k", "joined", "ignore"), edge("each", "item", "c"),
			edge("each", "done", "ignore"), edge("each", "partial_failed", "ignore"), edge("each", "failed", "ignore")}, ","))
	errs := graph.Validate(c, g, graph.Options{})
	got := make([]string, 0, len(errs))
	for _, e := range errs {
		got = append(got, e.Path+" "+e.Code)
	}
	want := []string{
		"nodes[1].params.n params",              // n_of of 3 out of 2 branches
		"nodes[2].params.of params",             // a second join of split
		"nodes[2].params.n params",              // n without n_of
		"nodes[3] invalid",                      // a foreach without a collect
		"nodes[4].params.of params",             // collect of a parallel
		"nodes[3].params.items expression_type", // items is a string, not a list
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("errors\n got %v\nwant %v", got, want)
	}
}
