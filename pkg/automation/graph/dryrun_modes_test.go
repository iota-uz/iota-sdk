package graph_test

import (
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// callCatalog is testCatalog with a call ("ask", a send with failed) and a pure lookup ("lookup", execute).
func callCatalog(t *testing.T) graph.Catalog {
	t.Helper()
	c := testCatalog(t)
	p, err := graph.FieldsParams(expr.Fields{{Name: "text", Field: expr.OptionalOf(expr.StringField)}})
	if err != nil {
		t.Fatal(err)
	}
	c.Nodes["send"] = graph.NodeSpec{Type: "send", Title: "Send", Behavior: graph.BehaviorCall, DryRun: graph.DryRunCapture, Params: p,
		Templates: []string{"text"}, Outcomes: []string{"sent", "failed"}, FailedOutcome: "failed",
		Output: expr.Fields{{Name: "id", Field: expr.OptionalOf(expr.StringField)}}}
	c.Nodes["lookup"] = graph.NodeSpec{Type: "lookup", Title: "Lookup", Behavior: graph.BehaviorCall, DryRun: graph.DryRunExecute, Params: p,
		Outcomes: []string{"found", "missing", "failed"}, FailedOutcome: "failed"}
	c.Nodes["wipe"] = graph.NodeSpec{Type: "wipe", Title: "Wipe", Behavior: graph.BehaviorEffect, DryRun: graph.DryRunForbid, Params: p,
		Outcomes: []string{"done"}}
	if err := c.Check(nil); err != nil {
		t.Fatal(err)
	}
	c.Revision = graph.Revision(c, nil)
	return c
}

const sendGraph = `{"name": "Send", "triggers": [{"id": "created", "event": "order.created", "include_self": true}],
 "nodes": [
  {"id": "find", "type": "lookup", "params": {}},
  {"id": "send", "type": "send", "params": {"text": "order {{ event.total }} {{ subject.key }} {{ run.url }}"}},
  {"id": "bad", "type": "notify", "params": {"text": "{{ nodes.send.output.error.code }}: {{ nodes.send.output.error.message }}"}},
  {"id": "fin", "type": "end", "params": {}}
 ],
 "edges": [
  {"from": {"node": "created", "outcome": "fired"}, "to": "find"},
  {"from": {"node": "find", "outcome": "found"}, "to": "send"},
  {"from": {"node": "find", "outcome": "missing"}, "to": "fin"},
  {"from": {"node": "find", "outcome": "failed"}, "to": "fin"},
  {"from": {"node": "send", "outcome": "sent"}, "to": "fin"},
  {"from": {"node": "send", "outcome": "failed"}, "to": "bad"},
  {"from": {"node": "bad", "outcome": "done"}, "to": "fin"},
  {"from": {"node": "bad", "outcome": "failed"}, "to": "fin"}
 ]}`

func TestDryRunModes(t *testing.T) {
	c := callCatalog(t)
	g := mustDecode(t, sendGraph)
	if !g.Triggers[0].IncludeSelf {
		t.Fatal("include_self is decoded")
	}
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("output.error, subject and run type-check: %v", errs)
	}
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 1.0, "status": "new"}}, Now: time.Now(),
		Subject: map[string]any{"entity": "order", "key": "o-7"}, Run: map[string]any{"id": "r", "url": "https://x/r"}}
	inputs := []graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 5.0}}}
	var executed []string
	trace := in.DryRun(graph.Subject{Entity: "order", Key: "o-7"}, ctx, inputs, graph.DryRunOptions{Modes: true,
		Execute: func(node graph.Node, _ []graph.Effect) graph.ExecAnswer {
			executed = append(executed, node.ID)
			return graph.ExecAnswer{Outcome: "found", Output: map[string]any{}}
		}})
	got := []string{}
	for _, s := range trace {
		got = append(got, s.Node+":"+s.Outcome+":"+s.Status)
	}
	want := []string{"created:fired:", "find:found:executed", "send:sent:captured", "fin:end:"}
	if len(got) != len(want) {
		t.Fatalf("trace %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("trace %v, want %v", got, want)
		}
	}
	if len(executed) != 1 || executed[0] != "find" {
		t.Fatalf("executed %v", executed)
	}
	effects, _ := trace[2].ContextPatch["effects"].([]graph.Effect)
	if len(effects) != 1 || effects[0].Params["text"] != "order 5 o-7 https://x/r" {
		t.Fatalf("the send's rendered params: %v", trace[2].ContextPatch)
	}

	// An executed call that fails takes its failed outcome with output.error.
	trace = in.DryRun(graph.Subject{Entity: "order", Key: "o-7"}, ctx, inputs, graph.DryRunOptions{Modes: true,
		Execute: func(graph.Node, []graph.Effect) graph.ExecAnswer {
			return graph.ExecAnswer{Code: "unavailable", Message: "down"}
		}})
	if trace[1].Outcome != "failed" {
		t.Fatalf("failed: %v", trace)
	}
	out, _ := trace[1].ContextPatch["output"].(map[string]any)
	e, _ := out["error"].(map[string]any)
	if e["code"] != "unavailable" || e["message"] != "down" || e["retryable"] != false {
		t.Fatalf("output.error: %v", out)
	}
}

func TestDryRunForbidStops(t *testing.T) {
	c := callCatalog(t)
	g := mustDecode(t, `{"name": "W", "triggers": [{"id": "created", "event": "order.created"}],
	 "nodes": [{"id": "w", "type": "wipe", "params": {}}, {"id": "fin", "type": "end", "params": {}}],
	 "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "w"}, {"from": {"node": "w", "outcome": "done"}, "to": "fin"}]}`)
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 1.0, "status": "new"}}}
	trace := in.DryRun(graph.Subject{Entity: "order", Key: "o"}, ctx, []graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 1.0}}}, graph.DryRunOptions{Modes: true})
	last := trace[len(trace)-1]
	if last.Outcome != graph.TraceFailed || last.ContextPatch["code"] != graph.FailDryRunForbidden {
		t.Fatalf("forbid: %v", trace)
	}
	// Without modes the format's dry run traces the effect.
	trace = in.DryRun(graph.Subject{Entity: "order", Key: "o"}, ctx, []graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 1.0}}}, graph.DryRunOptions{})
	if trace[len(trace)-1].Outcome != graph.TraceEnd {
		t.Fatalf("no modes: %v", trace)
	}
}

func TestCollectIsValidated(t *testing.T) {
	c := testCatalog(t)
	for raw, path := range map[string]string{
		`{"key": "event.total", "window": "5s"}`:              "triggers[0].collect.window",
		`{"key": "event.total", "window": "1h"}`:              "triggers[0].collect.window",
		`{"key": "event.nope + 1", "window": "1m"}`:           "triggers[0].collect.key",
		`{"key": "event.total", "window": "1m", "max": 5000}`: "graph",
	} {
		g, errs := graph.Decode([]byte(`{"name": "C", "triggers": [{"id": "created", "event": "order.created", "collect": ` + raw + `}],
		 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "fin"}]}`))
		if len(errs) == 0 {
			errs = graph.Validate(c, g, graph.Options{})
		}
		found := false
		for _, e := range errs {
			if e.Path == path || (path == "graph" && len(errs) > 0) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want an error at %s, got %v", raw, path, errs)
		}
	}
	g := mustDecode(t, `{"name": "C", "triggers": [{"id": "created", "event": "order.created", "collect": {"key": "event.total", "window": "1m", "max": 4}}],
	 "nodes": [{"id": "fin", "type": "end", "params": {}}], "edges": [{"from": {"node": "created", "outcome": "fired"}, "to": "fin"}]}`)
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("a valid collect: %v", errs)
	}
	if d, ok := g.Triggers[0].Collect.WindowDuration(); !ok || d != time.Minute || g.Triggers[0].Collect.MaxEvents() != 4 {
		t.Fatalf("collect %+v", g.Triggers[0].Collect)
	}
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	key, ok, st := in.CollectKey("created", graph.Event{"kind": "order.created", "total": 2.5}, graph.Context{})
	if st != nil || !ok || key != "2.5" {
		t.Fatalf("collect key %q %v %v", key, ok, st)
	}
}

// Every node type declares its dry run mode, a pure one too; forbid on a pure node stops the dry run on
// entry.
func TestPureNodesDeclareTheirDryRun(t *testing.T) {
	c := testCatalog(t)
	b := c.Nodes["branch"]
	b.DryRun = ""
	c.Nodes["branch"] = b
	errs := graph.ValidateCatalog(c, nil)
	if len(errs) != 1 || errs[0].Code != graph.CodeDryRunUndeclared {
		t.Fatalf("an undeclared pure node: %v", errs)
	}
	b.DryRun = graph.DryRunForbid
	c.Nodes["branch"] = b
	g := mustDecode(t, orderGraph)
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	trace := in.DryRun(graph.Subject{Entity: "order", Key: "o"}, graph.Context{},
		[]graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 500.0, "channel": "web"}}}, graph.DryRunOptions{Modes: true})
	last := trace[len(trace)-1]
	if last.Node != "check" || last.ContextPatch["code"] != graph.FailDryRunForbidden {
		t.Fatalf("forbid on entry, before the branch reads its context: %v", trace)
	}
}

// A join declared forbid stops the dry run however it is decided: a branch reaching it (walk) or its
// timeout (decided without a walk). Nothing after it runs.
func TestDryRunForbidReachesJoins(t *testing.T) {
	c := flowCatalog(t)
	j := c.Nodes["join"]
	j.DryRun = graph.DryRunForbid
	c.Nodes["join"] = j
	g := flowGraph(t, `{"id": "split", "type": "parallel", "params": {"branches": ["a", "b"]}},
		{"id": "hold", "type": "wait", "params": {"event": "manual"}},
		{"id": "j", "type": "join", "params": {"of": "split", "mode": "all", "timeout": "1h"}},
		{"id": "after", "type": "branch", "params": {"expression": "true"}}`,
		strings.Join([]string{edge("labeled", "fired", "split"), edge("split", "a", "hold"), edge("split", "b", "hold"),
			edge("hold", "matched", "j"), edge("j", "joined", "after"), edge("j", "timeout", "after"),
			edge("after", "true", "ignore"), edge("after", "false", "ignore")}, ","))
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	trace := in.DryRun(graph.Subject{}, graph.Context{}, []graph.DryRunInput{{Event: graph.Event{"kind": "issue.labeled", "label": "x"}}, {Timeout: true}},
		graph.DryRunOptions{Modes: true})
	last := trace[len(trace)-1]
	if last.Node != "j" || last.ContextPatch["code"] != graph.FailDryRunForbidden {
		t.Fatalf("a forbidden join on its timeout: %+v", trace)
	}
	for _, s := range trace {
		if s.Node == "after" {
			t.Fatalf("a node after the forbidden join ran: %+v", trace)
		}
	}
}

// A catalog written as data keeps an explicit dry run mode of a built-in node and refuses an invalid one.
func TestCatalogFromJSONKeepsBuiltinDryRun(t *testing.T) {
	c, err := graph.CatalogFromJSON([]byte(`{"name": "c", "events": {}, "subjects": {}, "nodes": {
		"branch": {"behavior": "branch", "dry_run": "forbid"}, "join": {"behavior": "join", "dry_run": "forbid"}, "end": {"behavior": "end"}}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nodes["branch"].DryRun != graph.DryRunForbid || c.Nodes["join"].DryRun != graph.DryRunForbid || c.Nodes["end"].DryRun != graph.DryRunExecute {
		t.Fatalf("modes: %v %v %v", c.Nodes["branch"].DryRun, c.Nodes["join"].DryRun, c.Nodes["end"].DryRun)
	}
	if _, err := graph.CatalogFromJSON([]byte(`{"name": "c", "events": {}, "subjects": {}, "nodes": {"branch": {"behavior": "branch", "dry_run": "maybe"}}}`), nil); err == nil {
		t.Fatal("an invalid mode is refused")
	}
}
