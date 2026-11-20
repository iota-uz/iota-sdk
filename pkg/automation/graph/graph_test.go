package graph_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The packages are pure: no database driver, no SDK modules, no extension runtime.
func TestPackagesArePure(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	out, err := exec.Command("go", "list", "-deps", ".", "../expr").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		for _, banned := range []string{"github.com/jackc/pgx", "github.com/iota-uz/iota-sdk/modules/", "github.com/iota-uz/iota-sdk/pkg/extruntime"} {
			if strings.HasPrefix(dep, banned) {
				t.Errorf("pkg/automation depends on %s", dep)
			}
		}
	}
}

// Neither package does IO or talks to a database itself: no direct import of os, network, exec or sql.
func TestPackagesDoNoIO(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not on PATH")
	}
	out, err := exec.Command("go", "list", "-f", `{{.ImportPath}}{{range .Imports}} {{.}}{{end}}`, ".", "../expr").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	banned := []string{"os", "os/exec", "io/fs", "io/ioutil", "net", "net/http", "database/sql", "syscall", "plugin"}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		for _, imp := range fields[1:] {
			if slices.Contains(banned, imp) || strings.HasPrefix(imp, "database/sql/") || strings.HasPrefix(imp, "net/") {
				t.Errorf("%s imports %s", fields[0], imp)
			}
		}
	}
}

// testCatalog is a small catalog built in Go: one subject, two events, the built-in nodes and an effect.
func testCatalog(t *testing.T) graph.Catalog {
	t.Helper()
	nodes := map[string]graph.NodeSpec{}
	for _, n := range graph.BuiltinNodes(nil, []string{"order.paid"}) {
		nodes[n.Type] = n
	}
	notify, err := graph.FieldsParams(expr.Fields{{Name: "text", Field: expr.StringField}})
	if err != nil {
		t.Fatal(err)
	}
	nodes["notify"] = graph.NodeSpec{
		Type: "notify", Title: "Notify", Behavior: graph.BehaviorEffect, Params: notify,
		Templates: []string{"text"}, Outcomes: []string{"done", "failed"}, DoneOutcome: "done", FailedOutcome: "failed",
	}
	order := expr.ObjectOf(expr.Fields{{Name: "total", Field: expr.DoubleField}, {Name: "status", Field: expr.StringField}})
	c := graph.Catalog{
		Events: map[string]graph.EventSpec{
			"order.created": {Key: "order.created", Subject: "order", Payload: expr.Fields{{Name: "total", Field: expr.DoubleField}},
				Match: map[string]graph.MatchField{"channels": {Payload: "channel", Type: graph.MatchStrings, Fold: graph.FoldLower}}},
			"order.paid": {Key: "order.paid", Subject: "order", Payload: expr.Fields{{Name: "amount", Field: expr.DoubleField}}, NoTrigger: true},
		},
		Nodes:    nodes,
		Subjects: map[string]graph.SubjectSpec{"order": {Context: expr.Fields{{Name: "order", Field: order}}}},
	}
	if err := c.Check(nil); err != nil {
		t.Fatal(err)
	}
	c.Revision = graph.Revision(c, nil)
	return c
}

func mustDecode(t *testing.T, raw string) graph.Graph {
	t.Helper()
	g, errs := graph.Decode([]byte(raw))
	if len(errs) > 0 {
		t.Fatalf("decode: %v", errs)
	}
	return g
}

const orderGraph = `{
  "name": "  Big orders  ",
  "triggers": [{"id": "created", "event": "order.created", "params": {"channels": ["Web"]}, "filter": "event.total > 100.0"}],
  "nodes": [
    {"id": "check", "type": "branch", "params": {"expression": "order.status == \"new\""}},
    {"id": "pay", "type": "wait", "params": {"event": "order.paid", "filter": "event.amount >= order.total", "timeout": "2d"}},
    {"id": "say", "type": "notify", "params": {"text": "Paid {{ nodes.pay.output.event.amount }} at {{ now }}"}},
    {"id": "stop", "type": "end", "params": {"reason": "done"}}
  ],
  "edges": [
    {"from": {"node": "created", "outcome": "fired"}, "to": "check"},
    {"from": {"node": "check", "outcome": "true"}, "to": "pay"},
    {"from": {"node": "check", "outcome": "false"}, "to": "ignore"},
    {"from": {"node": "pay", "outcome": "matched"}, "to": "say"},
    {"from": {"node": "pay", "outcome": "timeout"}, "to": "ignore"},
    {"from": {"node": "say", "outcome": "done"}, "to": "stop"},
    {"from": {"node": "say", "outcome": "failed"}, "to": "stop"}
  ]
}`

func TestDecodeDefaultsAndTrim(t *testing.T) {
	g := mustDecode(t, orderGraph)
	if g.Name != "Big orders" || g.RunAs != graph.RunAsOwner {
		t.Fatalf("name %q run_as %q", g.Name, g.RunAs)
	}
	if _, errs := graph.Decode([]byte(`{"name": "   ", "triggers": [{"id": "a", "event": "x"}], "nodes": [], "edges": []}`)); len(errs) != 1 || errs[0].Path != "name" {
		t.Fatalf("a blank name: %v", errs)
	}
	var schema map[string]any
	if err := json.Unmarshal(graph.JSONSchema(), &schema); err != nil || schema["type"] != "object" {
		t.Fatalf("JSONSchema: %v", err)
	}
	var builtins map[string]any
	if err := json.Unmarshal(graph.BuiltinsJSONSchema(), &builtins); err != nil || len(builtins) != 5 {
		t.Fatalf("BuiltinsJSONSchema: %v %v", err, builtins)
	}
}

func TestInterpreterSteps(t *testing.T) {
	c := testCatalog(t)
	g := mustDecode(t, orderGraph)
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("validate: %v", errs)
	}
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 150, "status": "new"}}, Now: now}
	created := graph.Event{"kind": "order.created", "total": json.Number("150.5"), "channel": "web"}
	if ok, st := in.TriggerMatches("created", created, ctx); !ok || st != nil {
		t.Fatalf("trigger: %v %v", ok, st)
	}
	if m, _ := in.TriggerMismatch("created", graph.Event{"kind": "order.created", "total": 150.5, "channel": "phone"}, ctx); m != "channels" {
		t.Fatalf("mismatch %q, want channels", m)
	}
	if m, _ := in.TriggerMismatch("created", graph.Event{"kind": "order.created", "total": 5, "channel": "web"}, ctx); m != graph.MismatchFilter {
		t.Fatalf("mismatch %q, want filter", m)
	}
	ctx.Event = created
	if st := in.Start("check", &graph.StepContext{Ctx: ctx}); st.Kind != graph.StepOutcome || st.Outcome != "true" {
		t.Fatalf("check: %+v", st)
	}
	if to, ok := in.Next("check", "true"); !ok || to != "pay" {
		t.Fatalf("next: %s %v", to, ok)
	}
	st := in.Start("pay", &graph.StepContext{Ctx: ctx})
	if st.Kind != graph.StepWait || st.Wait.Event != "order.paid" || st.Wait.Timeout != 48*time.Hour {
		t.Fatalf("pay: %+v", st)
	}
	if ok, st := in.WaitMatches("pay", graph.Event{"kind": "order.paid", "amount": 10}, ctx); ok || st != nil {
		t.Fatalf("a payment below the total: ok %v, step %+v; want the filter to evaluate to false", ok, st)
	}
	paid := graph.Event{"kind": "order.paid", "amount": 150}
	if ok, st := in.WaitMatches("pay", paid, ctx); !ok || st != nil {
		t.Fatalf("wait: %v %v", ok, st)
	}
	outcome, output := in.Wake("pay", paid)
	if outcome != "matched" {
		t.Fatalf("wake: %s", outcome)
	}
	ctx.Nodes = map[string]graph.NodeState{"pay": {Output: output}}
	st = in.Start("say", &graph.StepContext{Ctx: ctx})
	if st.Kind != graph.StepOutcome || st.Outcome != "done" || st.Effects[0].Params["text"] != "Paid 150 at 2026-11-20T10:00:00.000Z" {
		t.Fatalf("say: %+v", st)
	}
	if st := in.Start("stop", &graph.StepContext{Ctx: ctx}); st.Kind != graph.StepEnd || st.End["reason"] != "done" {
		t.Fatalf("stop: %+v", st)
	}
	if outcome, _ := in.Wake("pay", nil); outcome != "timeout" {
		t.Fatalf("timeout: %s", outcome)
	}
}

func TestMaxValueBytes(t *testing.T) {
	c := testCatalog(t)
	in, err := graph.NewInterpreter(c, mustDecode(t, orderGraph), graph.Options{Limits: graph.Limits{MaxValueBytes: 64}})
	if err != nil {
		t.Fatal(err)
	}
	big := graph.Event{"kind": "order.created", "total": 1, "channel": strings.Repeat("x", 100)}
	_, st := in.TriggerMatches("created", big, graph.Context{})
	if st == nil || st.Code != graph.FailValueTooLarge {
		t.Fatalf("a large event: %+v", st)
	}
}

// A node whose type's behavior the interpreter does not run is refused at nodes[i].type; a JSON catalog
// with that behavior is refused when it loads.
func TestUnsupportedBehavior(t *testing.T) {
	c := testCatalog(t)
	c.Nodes["split"] = graph.NodeSpec{Type: "split", Behavior: "parallel", Params: graph.MustSchemaParams(`{"type": "object"}`)}
	g := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "s", "type": "split", "params": {}}], "edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "s"}]}`)
	errs := graph.Validate(c, g, graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[0].type" || errs[0].Code != graph.CodeUnsupportedBehavior {
		t.Fatalf("errors: %v", errs)
	}
	if err := c.Check(nil); err == nil {
		t.Fatal("Check accepts a catalog with behavior parallel")
	}
	_, err := graph.CatalogFromJSON([]byte(`{"events": {}, "subjects": {}, "nodes": {"split": {"behavior": "parallel"}}}`), nil)
	if err == nil || !strings.Contains(err.Error(), "parallel") {
		t.Fatalf("CatalogFromJSON: %v", err)
	}
}

type parallelStub struct{}

func (parallelStub) Behavior() graph.Behavior { return "parallel" }
func (parallelStub) OutcomeRule(graph.NodeSpec) graph.OutcomeRule {
	return graph.OutcomeRule{From: &graph.ParamRef{Param: "branches"}, Fixed: []string{}}
}
func (parallelStub) Validate(*graph.GraphView, graph.Node, graph.NodeSpec) []graph.GraphError {
	return nil
}
func (parallelStub) Start(*graph.StepContext, graph.Node, graph.NodeSpec) graph.Step {
	return graph.Step{Kind: graph.StepOutcome, Outcome: "a"}
}

// A behavior plugs in through NewBehaviors: catalogs then load it, graphs validate with it.
func TestBehaviorsExtend(t *testing.T) {
	if _, err := graph.NewBehaviors(parallelStub{}, parallelStub{}); err == nil {
		t.Fatal("a duplicate behavior is accepted")
	}
	b, err := graph.NewBehaviors(append(graph.DefaultBehaviorImpls(), parallelStub{})...)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(b.Supported()); got != 7 {
		t.Fatalf("supported: %d", got)
	}
	c, err := graph.CatalogFromJSON([]byte(`{"events": {"e": {"subject": "none", "payload": {}}}, "subjects": {},
		"nodes": {"split": {"behavior": "parallel", "params": {"branches": {"list": "string"}}}}}`), b)
	if err != nil {
		t.Fatal(err)
	}
	g := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "e"}],
		"nodes": [{"id": "s", "type": "split", "params": {"branches": ["a", "b"]}}],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "s"}, {"from": {"node": "s", "outcome": "a"}, "to": "ignore"}]}`)
	errs := graph.Validate(c, g, graph.Options{Behaviors: b})
	if len(errs) != 1 || errs[0].Path != "nodes[0]" || errs[0].Code != graph.CodeUnconnected {
		t.Fatalf("errors: %v", errs)
	}
	v := graph.View(c, b)
	if v.Nodes[0].OutcomeRule.From == nil || v.Nodes[0].OutcomeRule.From.Param != "branches" {
		t.Fatalf("view: %+v", v.Nodes[0])
	}
}

func TestOutcomesOf(t *testing.T) {
	r := graph.OutcomeRule{
		From:     &graph.ParamRef{Param: "cases", Field: "outcome"},
		Fixed:    []string{"default", "bug"},
		Optional: &graph.OptionalOutcome{Param: "timeout", Outcome: "timeout"},
	}
	got := graph.OutcomesOf(r, map[string]any{"cases": []any{map[string]any{"outcome": "bug"}, map[string]any{"outcome": "chore"}, "x"}, "timeout": "1h"})
	if strings.Join(got, ",") != "bug,chore,default,timeout" {
		t.Fatalf("outcomes: %v", got)
	}
	if got := graph.OutcomesOf(graph.OutcomeRule{Fixed: []string{"a"}, Optional: r.Optional}, map[string]any{}); strings.Join(got, ",") != "a" {
		t.Fatalf("outcomes without the optional param: %v", got)
	}
}

func TestViewAndRevision(t *testing.T) {
	c := testCatalog(t)
	v := graph.View(c, nil)
	if v.Revision == "" || v.Revision != c.Revision {
		t.Fatalf("revision %q, catalog %q", v.Revision, c.Revision)
	}
	if v.Events[0].Key != "order.created" || len(v.Events[0].TriggerParams) == 0 {
		t.Fatalf("events: %+v", v.Events)
	}
	if _, err := json.Marshal(v); err != nil {
		t.Fatal(err)
	}
	c2 := testCatalog(t)
	c2.Events["order.created"] = graph.EventSpec{Key: "order.created", Subject: "order", Title: "Order created"}
	if graph.Revision(c2, nil) == c.Revision {
		t.Fatal("the revision does not change with the catalog")
	}
}

func TestLayout(t *testing.T) {
	g := mustDecode(t, orderGraph)
	l := graph.Layout{"check": {X: 1, Y: 2}, "gone": {X: 3, Y: 4}}
	if kept := graph.LayoutOf(g, l); len(kept) != 1 {
		t.Fatalf("layout: %v", kept)
	}
	if errs := graph.ValidateLayout(graph.Layout{"Bad": {}}); len(errs) != 1 {
		t.Fatalf("layout errors: %v", errs)
	}
	if id := graph.FreeID(g, "check"); id != "check_2" {
		t.Fatalf("free id: %s", id)
	}
}

func TestRulesOption(t *testing.T) {
	c := testCatalog(t)
	errs := graph.Validate(c, mustDecode(t, orderGraph), graph.Options{Rules: func(graph.Graph) []graph.GraphError {
		return []graph.GraphError{{Path: "nodes[2].type", Code: "connection_unavailable", Message: "no connection"}}
	}})
	if len(errs) != 1 || errs[0].Code != "connection_unavailable" {
		t.Fatalf("errors: %v", errs)
	}
}

// Params handed out (End, Effect) are copies: an engine changing them does not change the next run.
func TestParamsAreNotShared(t *testing.T) {
	c := testCatalog(t)
	in, err := graph.NewInterpreter(c, mustDecode(t, orderGraph), graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	st := in.Start("stop", nil)
	st.End["reason"] = "changed"
	if again := in.Start("stop", nil); again.End["reason"] != "done" {
		t.Fatalf("the cached params changed: %v", again.End)
	}
}

func TestDecodeErrorsAreDeterministic(t *testing.T) {
	raw := `{"name": "x", "bad": 1, "triggers": [{"id": "A", "event": ""}], "nodes": [{"id": "B", "type": 1}], "edges": [{"from": {}, "to": "C"}]}`
	_, first := graph.Decode([]byte(raw))
	for range 30 {
		_, errs := graph.Decode([]byte(raw))
		if len(errs) != len(first) {
			t.Fatalf("%v vs %v", errs, first)
		}
		for i := range errs {
			if errs[i].Path != first[i].Path || errs[i].Message != first[i].Message {
				t.Fatalf("order differs: %v vs %v", errs, first)
			}
		}
	}
}

func TestRevisionCoversBehavior(t *testing.T) {
	c := testCatalog(t)
	n := c.Nodes["notify"]
	n.FailedOutcome = "oops"
	c.Nodes["notify"] = n
	if graph.Revision(c, nil) == testCatalog(t).Revision {
		t.Fatal("a failed outcome change keeps the revision")
	}
}

func TestGoBuiltGraph(t *testing.T) {
	c := testCatalog(t)
	g := graph.Graph{
		Name:     "Go",
		Triggers: []graph.Trigger{{ID: "t", Event: "order.created"}},
		Nodes:    []graph.Node{{ID: "stop", Type: "end"}},
		Edges:    []graph.Edge{{From: graph.EdgeFrom{Node: "t", Outcome: graph.TriggerOutcome}, To: "stop"}},
	}
	if errs := graph.Validate(c, g, graph.Options{}); len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	trace := in.DryRun(graph.Subject{Entity: "customer", Key: "1"}, graph.Context{}, nil, graph.DryRunOptions{})
	if len(trace) != 1 || trace[0].ContextPatch["code"] != graph.FailInvalidSubject {
		t.Fatalf("another subject: %+v", trace)
	}
	trace = in.DryRun(graph.Subject{Entity: "order", Key: "1"}, graph.Context{},
		[]graph.DryRunInput{{Event: graph.Event{"kind": "order.created", "total": 1}}}, graph.DryRunOptions{})
	if len(trace) != 2 || trace[1].Outcome != graph.TraceEnd {
		t.Fatalf("trace: %+v", trace)
	}
}

// Review round 1: trigger defaults, snapshots, no external schemas, nil params, strict EOF, trimmed name.
func TestTriggerParamsDefaults(t *testing.T) {
	c := testCatalog(t)
	e := c.Events["order.created"]
	e.Match = nil
	e.TriggerParams = graph.MustSchemaParams(`{"type": "object", "properties": {"vip": {"type": "boolean", "default": true}}, "additionalProperties": false}`)
	e.MatchHook = "vip_only"
	c.Events["order.created"] = e
	var seen map[string]any
	opts := graph.Options{MatchHooks: map[string]func(map[string]any, graph.Event) bool{
		"vip_only": func(p map[string]any, _ graph.Event) bool { seen = p; return p["vip"] == true },
	}}
	g := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}], "nodes": [],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "ignore"}]}`)
	in, err := graph.NewInterpreter(c, g, opts)
	if err != nil {
		t.Fatal(err)
	}
	if ok, st := in.TriggerMatches("t", graph.Event{"kind": "order.created", "total": 1}, graph.Context{}); !ok || st != nil {
		t.Fatalf("the hook saw %v", seen)
	}
}

func TestViewIsASnapshot(t *testing.T) {
	c := testCatalog(t)
	g := mustDecode(t, orderGraph)
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	g.Triggers[0].Event = "order.paid" // the caller's graph
	view := in.View().Graph()
	view.Triggers[0].Event = "order.paid"
	view.Nodes[3].Params["reason"] = "x"
	in.View().Out("check")[0].To = "x"
	n, _ := in.View().Node("stop")
	n.Params["reason"] = "x"
	in.View().Catalog().Nodes["notify"].Outcomes[0] = "x"
	if got := in.Graph().Triggers[0].Event; got != "order.created" {
		t.Fatalf("trigger event %s", got)
	}
	if to, _ := in.Next("check", "true"); to != "pay" {
		t.Fatalf("edge %s", to)
	}
	if st := in.Start("stop", nil); st.End["reason"] != "done" {
		t.Fatalf("end %v", st.End)
	}
	if in.View().Catalog().Nodes["notify"].Outcomes[0] != "done" {
		t.Fatal("the catalog changed")
	}
}

func TestSchemasLoadNoFiles(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ext.json"
	if err := os.WriteFile(path, []byte(`{"type": "string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := graph.SchemaParams(json.RawMessage(`{"$ref": "file://` + path + `"}`)); err == nil {
		t.Fatal("a file-backed schema compiles")
	}
	p, err := graph.SchemaParams(json.RawMessage(`{"$defs": {"s": {"type": "string"}}, "type": "object", "properties": {"a": {"$ref": "#/$defs/s"}}}`))
	if err != nil {
		t.Fatalf("a local reference: %v", err)
	}
	if _, issues := p.Parse(map[string]any{"a": 1}); len(issues) != 1 {
		t.Fatalf("issues: %v", issues)
	}
}

func TestNilParamsSpec(t *testing.T) {
	c := testCatalog(t)
	c.Nodes["split"] = graph.NodeSpec{Type: "split", Behavior: "parallel"}
	c.Nodes["bare"] = graph.NodeSpec{Type: "bare", Behavior: graph.BehaviorEnd}
	g := mustDecode(t, `{"name": "x", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "s", "type": "split", "params": {}}, {"id": "b", "type": "bare", "params": {}}],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "s"}]}`)
	errs := graph.Validate(c, g, graph.Options{})
	paths := make([]string, 0, len(errs))
	for _, e := range errs {
		paths = append(paths, e.Path+" "+e.Code)
	}
	got := strings.Join(paths, ",")
	if !strings.Contains(got, "nodes[0].type unsupported_behavior") || !strings.Contains(got, "nodes[1].type invalid") {
		t.Fatalf("errors: %v", errs)
	}
}

func TestDecodeStrictEnd(t *testing.T) {
	for _, tail := range []string{"]", "}", " {}", "x"} {
		if _, errs := graph.Decode([]byte(orderGraph + tail)); len(errs) != 1 || errs[0].Path != "graph" {
			t.Errorf("%q after the graph: %v", tail, errs)
		}
	}
	if _, errs := graph.Decode([]byte(orderGraph + "\n  ")); len(errs) > 0 {
		t.Fatalf("trailing space: %v", errs)
	}
}

// The published schema and Decode agree on a name at both limits, on raw (untrimmed) input.
func TestNameSchemaMatchesDecode(t *testing.T) {
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(graph.JSONSchema())))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("mem://graph.json", doc); err != nil {
		t.Fatal(err)
	}
	schema, err := c.Compile("mem://graph.json")
	if err != nil {
		t.Fatal(err)
	}
	nbsp, ideographic := "\u00a0", "\u3000"
	for _, tc := range []struct {
		name  string
		valid bool
		code  string
	}{
		{" " + strings.Repeat("a", 120) + " ", true, ""},
		{"\t" + strings.Repeat("я", 120) + "\n", true, ""},
		{nbsp + strings.Repeat("a", 120) + ideographic + "\v", true, ""},
		{strings.Repeat("a", 121), false, graph.CodeLimit},
		{" " + strings.Repeat("a", 121) + " ", false, graph.CodeLimit},
		{"   ", false, graph.CodeInvalid},
		{nbsp + nbsp, false, graph.CodeInvalid},
		{"\u0085", false, graph.CodeInvalid},
		{"", false, graph.CodeInvalid},
		{"a", true, ""},
		{" a" + nbsp + "b ", true, ""},
	} {
		quoted, err := json.Marshal(tc.name)
		if err != nil {
			t.Fatal(err)
		}
		raw := strings.Replace(orderGraph, `"  Big orders  "`, string(quoted), 1)
		g, errs := graph.Decode([]byte(raw))
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		schemaOK := schema.Validate(inst) == nil
		if (len(errs) == 0) != tc.valid || schemaOK != tc.valid {
			t.Errorf("%q: decode errors %v, schema valid %v; want valid %v", tc.name, errs, schemaOK, tc.valid)
		}
		if !tc.valid && (len(errs) != 1 || errs[0].Path != "name" || errs[0].Code != tc.code) {
			t.Errorf("%q: errors %v, want one at name with code %s", tc.name, errs, tc.code)
		}
		if tc.valid && g.Name != graph.TrimName(tc.name) {
			t.Errorf("%q: kept %q", tc.name, g.Name)
		}
		if tc.valid {
			g.Name = tc.name
			if errs := graph.Validate(testCatalog(t), g, graph.Options{}); len(errs) > 0 {
				t.Errorf("%q: validate %v", tc.name, errs)
			}
		}
	}
}

// Nested field declarations are copies too: renaming a property through the view changes nothing.
func TestViewCopiesFields(t *testing.T) {
	in, err := graph.NewInterpreter(testCatalog(t), mustDecode(t, orderGraph), graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	cat := in.View().Catalog()
	cat.Subjects["order"].Context[0].Field.Props[1].Name = "renamed"
	cat.Events["order.created"].Payload[0].Field.Kind = 0
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 1, "status": "new"}}}
	if st := in.Start("check", &graph.StepContext{Ctx: ctx}); st.Kind != graph.StepOutcome || st.Outcome != "true" {
		t.Fatalf("check: %+v", st)
	}
	in.Graph().Nodes[0].Params["expression"] = "false"
	if st := in.Start("check", &graph.StepContext{Ctx: ctx}); st.Outcome != "true" {
		t.Fatalf("the graph changed: %+v", st)
	}
}

// Params built in Go with typed containers are owned by the interpreter once it is built.
func TestTypedParamsAreSnapshotted(t *testing.T) {
	c := testCatalog(t)
	channels := []string{"web"}
	cases := []map[string]any{{"value": "new", "outcome": "fresh"}}
	g := graph.Graph{
		Name:     "Typed",
		Triggers: []graph.Trigger{{ID: "t", Event: "order.created", Params: map[string]any{"channels": channels}}},
		Nodes:    []graph.Node{{ID: "route", Type: "switch", Params: map[string]any{"expression": "order.status", "cases": cases}}},
		Edges: []graph.Edge{
			{From: graph.EdgeFrom{Node: "t", Outcome: graph.TriggerOutcome}, To: "route"},
			{From: graph.EdgeFrom{Node: "route", Outcome: "fresh"}, To: graph.Ignore},
			{From: graph.EdgeFrom{Node: "route", Outcome: "default"}, To: graph.Ignore},
		},
	}
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	channels[0] = "phone" // the caller's typed slice
	cases[0]["outcome"] = "stale"
	first := in.Graph()
	if got := first.Triggers[0].Params["channels"].([]any)[0]; got != "web" {
		t.Fatalf("the caller's slice changed the interpreter: %v", got)
	}
	if got := first.Nodes[0].Params["cases"].([]any)[0].(map[string]any)["outcome"]; got != "fresh" {
		t.Fatalf("the caller's map changed the interpreter: %v", got)
	}
	first.Triggers[0].Params["channels"].([]any)[0] = "phone"
	if got := in.Graph().Triggers[0].Params["channels"].([]any)[0]; got != "web" {
		t.Fatalf("a Graph() result changed the interpreter: %v", got)
	}
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 1, "status": "new"}}}
	if m, _ := in.TriggerMismatch("t", graph.Event{"kind": "order.created", "total": 1, "channel": "web"}, ctx); m != "" {
		t.Fatalf("mismatch %q", m)
	}
	if st := in.Start("route", &graph.StepContext{Ctx: ctx}); st.Outcome != "fresh" {
		t.Fatalf("route: %+v", st)
	}
}

// Review round of the PR: what each fix guarantees.
func TestReviewRoundFixes(t *testing.T) {
	c := testCatalog(t)

	// A typed Go list in an event matches as a list.
	in, err := graph.NewInterpreter(c, mustDecode(t, orderGraph), graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := graph.Context{Vars: map[string]any{"order": map[string]any{"total": 1, "status": "new"}}}
	if m, _ := in.TriggerMismatch("created", graph.Event{"kind": "order.created", "total": 200, "channel": []string{"web"}}, ctx); m != "" {
		t.Fatalf("a []string channel: mismatch %q", m)
	}

	// An effect's done outcome is one of its outcomes.
	bad := testCatalog(t)
	n := bad.Nodes["notify"]
	n.DoneOutcome, n.Outcomes = "", []string{"sent", "failed"}
	bad.Nodes["notify"] = n
	if err := bad.Check(nil); err == nil {
		t.Error("Check accepts an effect whose done outcome has no edge")
	}

	// FreeID stays within 40 characters.
	long := strings.Repeat("a", 40)
	g := graph.Graph{Nodes: []graph.Node{{ID: long}}}
	if id := graph.FreeID(g, long); len(id) > 40 || id == long || !graph.IsID(id) {
		t.Errorf("free id %q", id)
	}

	// x-unique compares numbers by value; x-max-ms takes 1e3; an undeclared null is refused.
	p := graph.MustSchemaParams(`{"type": "object", "properties": {"n": {"type": "array", "x-unique": true}, "ms": {"type": "integer", "x-max-ms": 2000}, "r": {"type": "string"}}, "additionalProperties": false}`)
	if _, issues := p.Parse(map[string]any{"n": []any{json.Number("1"), json.Number("1.0")}}); len(issues) != 1 {
		t.Errorf("1 and 1.0: %v", issues)
	}
	if _, issues := p.Parse(map[string]any{"ms": json.Number("1e3")}); len(issues) != 0 {
		t.Errorf("1e3 ms: %v", issues)
	}
	if _, issues := p.Parse(map[string]any{"rr": nil, "r": nil}); len(issues) != 1 {
		t.Errorf("an undeclared null: %v", issues)
	}
	schema := p.JSONSchema()
	schema[0] = 'X'
	if p.JSONSchema()[0] != '{' {
		t.Error("JSONSchema returns the spec's own bytes")
	}

	// A branch expression may span lines.
	multi := strings.Replace(orderGraph, `"order.status == \"new\""`, `"order.status == \"new\" &&\n true"`, 1)
	if errs := graph.Validate(c, mustDecode(t, multi), graph.Options{}); len(errs) > 0 {
		t.Errorf("a multiline expression: %v", errs)
	}

	// The interpreter keeps the hooks it validated.
	hc := testCatalog(t)
	e := hc.Events["order.created"]
	e.MatchHook = "team"
	hc.Events["order.created"] = e
	hooks := map[string]func(map[string]any, graph.Event) bool{"team": func(map[string]any, graph.Event) bool { return true }}
	hin, err := graph.NewInterpreter(hc, mustDecode(t, orderGraph), graph.Options{MatchHooks: hooks})
	if err != nil {
		t.Fatal(err)
	}
	delete(hooks, "team")
	if m, _ := hin.TriggerMismatch("created", graph.Event{"kind": "order.created", "total": 200, "channel": "web"}, ctx); m != "" {
		t.Errorf("after the caller's map changed: %q", m)
	}

	// CatalogFromJSON and FieldFromJSONSchema refuse a second value.
	if _, err := graph.CatalogFromJSON([]byte(`{}{"nodes":{"bad":{"behavior":"unknown"}}}`), nil); err == nil {
		t.Error("a second catalog document is accepted")
	}
	if _, err := expr.FieldFromJSONSchema(json.RawMessage(`{"type":"string"} {"type":"boolean"}`)); err == nil {
		t.Error("a second schema is accepted")
	}
	var f expr.Field
	if err := json.Unmarshal([]byte(`{"enum":[null]}`), &f); err == nil {
		t.Error("a null enum member is accepted")
	}
}

// A mixed graph's dry run about one entity starts only on that entity's triggers.
func TestDryRunSubjectTriggers(t *testing.T) {
	c := testCatalog(t)
	c.Subjects["customer"] = graph.SubjectSpec{Context: expr.Fields{{Name: "customer", Field: expr.ObjectOf(expr.Fields{{Name: "name", Field: expr.StringField}})}}}
	c.Events["customer.created"] = graph.EventSpec{Key: "customer.created", Subject: "customer"}
	c.Subjects["any"] = graph.SubjectSpec{Context: expr.Fields{{Name: "run", Field: expr.ObjectOf(expr.Fields{{Name: "n", Field: expr.IntField}})}}}
	c.MixedSubject = "any"
	g := mustDecode(t, `{"name": "Mixed", "triggers": [{"id": "order", "event": "order.created"}, {"id": "customer", "event": "customer.created"}],
		"nodes": [], "edges": [{"from": {"node": "order", "outcome": "fired"}, "to": "ignore"}, {"from": {"node": "customer", "outcome": "fired"}, "to": "ignore"}]}`)
	in, err := graph.NewInterpreter(c, g, graph.Options{})
	if err != nil {
		t.Fatal(err)
	}
	trace := in.DryRun(graph.Subject{Entity: "order", Key: "1"}, graph.Context{}, []graph.DryRunInput{
		{Event: graph.Event{"kind": "customer.created"}},
		{Event: graph.Event{"kind": "order.created", "total": 1}},
	}, graph.DryRunOptions{})
	if len(trace) == 0 || trace[0].Node != "order" {
		t.Fatalf("trace: %+v", trace)
	}
}

type bigOutput struct{}

func (bigOutput) Behavior() graph.Behavior { return "big" }
func (bigOutput) OutcomeRule(graph.NodeSpec) graph.OutcomeRule {
	return graph.OutcomeRule{Fixed: []string{"done"}}
}
func (bigOutput) Validate(*graph.GraphView, graph.Node, graph.NodeSpec) []graph.GraphError {
	return nil
}
func (bigOutput) Start(*graph.StepContext, graph.Node, graph.NodeSpec) graph.Step {
	return graph.Step{Kind: graph.StepOutcome, Outcome: "done", Output: map[string]any{"s": strings.Repeat("x", 100)}}
}

// The output limit holds for registered behaviors; a wait's filter param is checked as a bool.
func TestOutputLimitAndFilterParam(t *testing.T) {
	b, err := graph.NewBehaviors(append(graph.DefaultBehaviorImpls(), bigOutput{})...)
	if err != nil {
		t.Fatal(err)
	}
	c := testCatalog(t)
	c.Nodes["big"] = graph.NodeSpec{Type: "big", Behavior: "big", Params: graph.MustSchemaParams(`{"type": "object"}`)}
	g := mustDecode(t, `{"name": "Big", "triggers": [{"id": "t", "event": "order.created"}],
		"nodes": [{"id": "b", "type": "big", "params": {}}],
		"edges": [{"from": {"node": "t", "outcome": "fired"}, "to": "b"}, {"from": {"node": "b", "outcome": "done"}, "to": "ignore"}]}`)
	in, err := graph.NewInterpreter(c, g, graph.Options{Behaviors: b, Limits: graph.Limits{MaxValueBytes: 64}})
	if err != nil {
		t.Fatal(err)
	}
	if st := in.Start("b", nil); st.Kind != graph.StepFailed || st.Code != graph.FailValueTooLarge {
		t.Fatalf("a large output: %+v", st)
	}

	w := c.Nodes["wait"]
	w.Expressions = nil // the filter param is not listed: it is still a bool over the waited event
	c.Nodes["wait"] = w
	wg := strings.Replace(orderGraph, `"event.amount >= order.total"`, `"event.amount"`, 1)
	errs := graph.Validate(c, mustDecode(t, wg), graph.Options{})
	if len(errs) != 1 || errs[0].Path != "nodes[1].params.filter" {
		t.Fatalf("errors: %v", errs)
	}
}

func TestEnvScopeAndCompletion(t *testing.T) {
	scope := expr.Scope{Vars: map[string]expr.Field{"x": expr.IntField, "flag": expr.BoolField, "s": expr.StringField}}
	env, err := expr.NewEnv(scope, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, issues := env.Check("x + 1", expr.TypeAny)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	scope.Vars["x"] = expr.StringField
	env.Scope().Vars["x"] = expr.StringField
	if v, err := p.Eval(map[string]any{"x": 7}); err != nil || v != int64(8) {
		t.Fatalf("after the caller changed the scope: %v %v", v, err)
	}
	has := func(prefix, label string) bool {
		for _, c := range env.Complete(prefix) {
			if c.Label == label {
				return true
			}
		}
		return false
	}
	if has("flag.low", "lowerAscii") || !has("s.low", "lowerAscii") {
		t.Error("member completions ignore the receiver type")
	}
}
