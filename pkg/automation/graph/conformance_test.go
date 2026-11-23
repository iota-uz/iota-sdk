package graph_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
)

// The conformance runner (testdata/conformance/README.md): Plot's fixtures, copied once, and the SDK's.

const conformanceDir = "testdata/conformance"

type fixture struct {
	file string

	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	Description  string          `json:"description"`
	Changed      string          `json:"changed"`
	Plot         *bool           `json:"plot"`
	Format       int             `json:"format"`
	Requires     []string        `json:"requires"`
	Unless       []string        `json:"unless"`
	Catalog      json.RawMessage `json:"catalog"`
	CatalogPatch json.RawMessage `json:"catalog_patch"`
	Limits       *fixtureLimits  `json:"limits"`

	// trace and validate
	Graph   map[string]any   `json:"graph"`
	Subject map[string]any   `json:"subject"`
	Context map[string]any   `json:"context"`
	Inputs  []map[string]any `json:"inputs"`
	Trace   []any            `json:"trace"`
	Errors  []expectedError  `json:"errors"`
	Options *fixtureOptions  `json:"options"`

	// expressions and templates
	Cases []exprCase `json:"cases"`

	// catalog: whether loading it fails
	Error string `json:"error"`
}

type fixtureLimits struct {
	MaxNodes   int    `json:"max_nodes"`
	MaxDepth   int    `json:"max_depth"`
	MaxLiteral int    `json:"max_literal"`
	MaxArgs    int    `json:"max_args"`
	CostLimit  uint64 `json:"cost_limit"`
}

func (l *fixtureLimits) expr() expr.Limits {
	if l == nil {
		return expr.Limits{}
	}
	return expr.Limits{MaxNodes: l.MaxNodes, MaxDepth: l.MaxDepth, MaxLiteral: l.MaxLiteral, MaxArgs: l.MaxArgs, CostLimit: l.CostLimit}
}

type fixtureOptions struct {
	CallError  string   `json:"call_error"`  // every call's request is refused with this message
	MatchHooks []string `json:"match_hooks"` // hooks that take every event
	// WithoutBehaviors validates with the default behaviors minus these (the catalog loads with all).
	WithoutBehaviors []string `json:"without_behaviors"`
}

type expectedError struct {
	Path string `json:"path"`
	Code string `json:"code"`
}

type exprCase struct {
	Subject string         `json:"subject"`
	Event   *string        `json:"event"`
	Source  string         `json:"source"`
	Type    *string        `json:"type"`
	Context map[string]any `json:"context"`
	Value   any            `json:"value"`
	Error   string         `json:"error"` // type | missing_context | failed
	Path    string         `json:"path"`
	Issue   string         `json:"issue"` // with error type: syntax | type | limit
	Code    string         `json:"code"`  // with error failed: the step code
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := expr.UnmarshalJSON(raw, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func loadFixtures(t *testing.T, dir string) []fixture {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	out := make([]fixture, 0, len(files))
	for _, f := range files {
		var fx fixture
		readJSON(t, f, &fx)
		fx.file = f
		out = append(out, fx)
	}
	return out
}

// capabilities of this build: what `requires` and `unless` name.
func capabilities(b *graph.Behaviors) map[string]bool {
	caps := map[string]bool{}
	for _, k := range b.Supported() {
		caps["behavior:"+string(k)] = true
	}
	return caps
}

func plotCatalogJSON(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(conformanceDir, "plot", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestConformance(t *testing.T) {
	behaviors := graph.DefaultBehaviors()
	caps := capabilities(behaviors)
	base := plotCatalogJSON(t)
	shared, err := graph.CatalogFromJSON(base, behaviors)
	if err != nil {
		t.Fatalf("the conformance catalog: %v", err)
	}
	plot := loadFixtures(t, filepath.Join(conformanceDir, "plot", "fixtures"))
	sdk := loadFixtures(t, filepath.Join(conformanceDir, "sdk"))
	if len(plot) == 0 || len(sdk) == 0 {
		t.Fatal("no fixtures")
	}
	ran, skipped := 0, 0
	for _, f := range append(plot, sdk...) {
		if reason := skipReason(f, caps); reason != "" {
			skipped++
			t.Logf("skip %s: %s", f.Name, reason)
			continue
		}
		ran++
		t.Run(f.Name, func(t *testing.T) {
			if strings.Contains(f.file, string(filepath.Separator)+"sdk"+string(filepath.Separator)) && f.Format != 2 {
				t.Fatalf("an SDK fixture is format 2")
			}
			catalog := shared
			if len(f.Catalog) > 0 || len(f.CatalogPatch) > 0 {
				raw := base
				if len(f.Catalog) > 0 {
					raw = f.Catalog
				}
				if len(f.CatalogPatch) > 0 {
					raw = mergePatch(t, raw, f.CatalogPatch)
				}
				c, err := graph.CatalogFromJSON(raw, behaviors)
				if f.Kind == "catalog" {
					if f.Error != "" && err == nil {
						t.Fatalf("the catalog loads, want an error")
					}
					if f.Error == "" && err != nil {
						t.Fatalf("the catalog does not load: %v", err)
					}
					if err != nil && !strings.Contains(err.Error(), f.Error) {
						t.Fatalf("error %q, want one containing %q", err, f.Error)
					}
					return
				}
				if err != nil {
					t.Fatalf("the fixture's catalog: %v", err)
				}
				catalog = c
			}
			opts := graph.Options{Behaviors: behaviors, Limits: graph.Limits{Expr: f.Limits.expr()}}
			var dry graph.DryRunOptions
			if f.Options != nil {
				if f.Options.CallError != "" {
					msg := f.Options.CallError
					dry.CallError = func(graph.Effect) string { return msg }
				}
				if len(f.Options.WithoutBehaviors) > 0 {
					var keep []graph.BehaviorImpl
					for _, impl := range graph.DefaultBehaviorImpls() {
						if !slices.Contains(f.Options.WithoutBehaviors, string(impl.Behavior())) {
							keep = append(keep, impl)
						}
					}
					reduced, err := graph.NewBehaviors(keep...)
					if err != nil {
						t.Fatal(err)
					}
					opts.Behaviors = reduced
				}
				opts.MatchHooks = map[string]func(map[string]any, graph.Event) bool{}
				for _, h := range f.Options.MatchHooks {
					opts.MatchHooks[h] = func(map[string]any, graph.Event) bool { return true }
				}
			}
			switch f.Kind {
			case "trace":
				runTrace(t, catalog, opts, dry, f)
			case "validate":
				runValidate(t, catalog, opts, f)
			case "expressions":
				runExpressions(t, catalog, f, false)
			case "templates":
				runExpressions(t, catalog, f, true)
			default:
				t.Fatalf("unknown fixture kind %q", f.Kind)
			}
		})
	}
	t.Logf("conformance: %d fixtures ran, %d skipped by requires/unless", ran, skipped)
}

func skipReason(f fixture, caps map[string]bool) string {
	for _, r := range f.Requires {
		if !caps[r] {
			return "requires " + r
		}
	}
	for _, u := range f.Unless {
		if caps[u] {
			return "unless " + u
		}
	}
	return ""
}

// mergePatch applies an RFC 7386 JSON merge patch.
func mergePatch(t *testing.T, doc, patch []byte) []byte {
	t.Helper()
	var d, p any
	if err := expr.UnmarshalJSON(doc, &d); err != nil {
		t.Fatal(err)
	}
	if err := expr.UnmarshalJSON(patch, &p); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(merge(d, p))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func merge(doc, patch any) any {
	pm, ok := patch.(map[string]any)
	if !ok {
		return patch
	}
	dm, ok := doc.(map[string]any)
	if !ok {
		dm = map[string]any{}
	}
	out := map[string]any{}
	for k, v := range dm {
		out[k] = v
	}
	for k, v := range pm {
		if v == nil {
			delete(out, k)
			continue
		}
		out[k] = merge(out[k], v)
	}
	return out
}

// decodeGraph strips what Plot's fixtures carry around a version (id, version, enabled, and attach
// kind event: every SDK graph is one) and decodes the rest strictly.
func decodeGraph(t *testing.T, raw map[string]any) (graph.Graph, []graph.GraphError) {
	t.Helper()
	g := map[string]any{}
	for k, v := range raw {
		g[k] = v
	}
	delete(g, "id")
	delete(g, "version")
	delete(g, "enabled")
	if a, ok := g["attach"].(map[string]any); ok && a["kind"] == "event" && len(a) == 1 {
		delete(g, "attach")
	}
	b, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	return graph.Decode(b)
}

func runTrace(t *testing.T, c graph.Catalog, opts graph.Options, dry graph.DryRunOptions, f fixture) {
	t.Helper()
	g, errs := decodeGraph(t, f.Graph)
	if len(errs) > 0 {
		t.Fatalf("a trace fixture's graph decodes: %v", errs)
	}
	if errs := graph.Validate(c, g, opts); len(errs) > 0 {
		t.Fatalf("a trace fixture's graph is valid: %v", errs)
	}
	in, err := graph.NewInterpreter(c, g, opts)
	if err != nil {
		t.Fatal(err)
	}
	var subject graph.Subject
	if e, ok := f.Subject["entity"].(string); ok {
		subject.Entity = e
		subject.Key, _ = f.Subject["key"].(string)
	}
	ctx := contextOf(t, f.Context)
	inputs := make([]graph.DryRunInput, 0, len(f.Inputs))
	for _, i := range f.Inputs {
		if i["timeout"] == true {
			inputs = append(inputs, graph.DryRunInput{Timeout: true})
			continue
		}
		inputs = append(inputs, graph.DryRunInput{Event: graph.Event(i)})
	}
	got := in.DryRun(subject, ctx, inputs, dry)
	gotJSON := generic(t, got)
	want := generic(t, f.Trace)
	if !matches(want, gotJSON) {
		t.Fatalf("trace\n got: %s\nwant: %s", compact(t, gotJSON), compact(t, want))
	}
}

func contextOf(t *testing.T, raw map[string]any) graph.Context {
	t.Helper()
	ctx := graph.Context{Vars: map[string]any{}, Nodes: map[string]graph.NodeState{}}
	for k, v := range raw {
		switch k {
		case "nodes":
			nodes, _ := v.(map[string]any)
			for id, st := range nodes {
				m, _ := st.(map[string]any)
				ns := graph.NodeState{}
				if out, ok := m["output"].(map[string]any); ok {
					ns.Output = out
				}
				ns.Outcome, _ = m["outcome"].(string)
				ctx.Nodes[id] = ns
			}
		case "now":
			s, _ := v.(string)
			now, err := time.Parse(time.RFC3339Nano, s)
			if err != nil {
				t.Fatalf("context.now: %v", err)
			}
			ctx.Now = now
		case "tenant":
			ctx.Tenant, _ = v.(map[string]any)
		case "event":
			ctx.Event, _ = v.(map[string]any)
		default:
			ctx.Vars[k] = v
		}
	}
	return ctx
}

func generic(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func compact(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// matches compares JSON values; the expected string "*" matches any value (texts are not normative).
func matches(want, got any) bool {
	if s, ok := want.(string); ok && s == "*" {
		return true
	}
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for k, wv := range w {
			gv, ok := g[k]
			if !ok || !matches(wv, gv) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !matches(w[i], g[i]) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(want, got)
}

func runValidate(t *testing.T, c graph.Catalog, opts graph.Options, f fixture) {
	t.Helper()
	g, errs := decodeGraph(t, f.Graph)
	if len(errs) == 0 {
		errs = graph.Validate(c, g, opts)
	}
	if len(errs) == 0 && len(f.Errors) > 0 {
		t.Fatalf("the graph is valid, want errors %v", f.Errors)
	}
	var got, want []string
	codes := false
	for _, e := range f.Errors {
		if e.Code != "" {
			codes = true
		}
	}
	for _, e := range errs {
		if codes {
			got = append(got, e.Path+" "+e.Code)
		} else {
			got = append(got, e.Path)
		}
	}
	for _, e := range f.Errors {
		if codes {
			want = append(want, e.Path+" "+e.Code)
		} else {
			want = append(want, e.Path)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("error paths\n got: %v\nwant: %v\nerrors: %v", got, want, errs)
	}
}

func runExpressions(t *testing.T, c graph.Catalog, f fixture, templates bool) {
	t.Helper()
	for n, cs := range f.Cases {
		t.Run(fmt.Sprintf("%d", n), func(t *testing.T) {
			event := ""
			if cs.Event != nil {
				event = *cs.Event
			}
			env, err := expr.NewEnv(graph.ExprScope(c, cs.Subject, event, nil), f.Limits.expr())
			if err != nil {
				t.Fatal(err)
			}
			want := expr.TypeAny
			if cs.Type != nil {
				want = expr.Type(*cs.Type)
			}
			var eval func(map[string]any) (any, error)
			var issues []expr.Issue
			if templates {
				var tp *expr.TemplateProgram
				tp, issues = expr.Template(env, cs.Source)
				if tp != nil {
					eval = func(v map[string]any) (any, error) { return tp.Render(v) }
				}
			} else {
				var p *expr.Program
				p, issues = env.Check(cs.Source, want)
				if p != nil {
					eval = p.Eval
				}
			}
			if cs.Error == "type" {
				if len(issues) == 0 {
					t.Fatalf("%s: the check passes, want it refused", cs.Source)
				}
				if cs.Issue != "" && issues[0].Code != cs.Issue {
					t.Fatalf("%s: issue %s (%s), want %s", cs.Source, issues[0].Code, issues[0].Message, cs.Issue)
				}
				return
			}
			if len(issues) > 0 {
				t.Fatalf("%s: %v", cs.Source, issues)
			}
			vars := cs.Context
			if vars == nil {
				vars = map[string]any{}
			}
			v, err := eval(vars)
			switch cs.Error {
			case "missing_context":
				var mc *expr.MissingContextError
				if !errors.As(err, &mc) || mc.Path != cs.Path {
					t.Fatalf("%s: got %v, %v; want missing_context at %s", cs.Source, v, err, cs.Path)
				}
				return
			case "failed":
				var ee *expr.EvalError
				if !errors.As(err, &ee) {
					t.Fatalf("%s: got %v, %v; want a failure", cs.Source, v, err)
				}
				if cs.Code != "" && ee.Code != cs.Code {
					t.Fatalf("%s: code %s, want %s", cs.Source, ee.Code, cs.Code)
				}
				return
			case "":
			default:
				t.Fatalf("unknown error kind %q", cs.Error)
			}
			if err != nil {
				t.Fatalf("%s: %v", cs.Source, err)
			}
			if got, want := generic(t, v), generic(t, cs.Value); !reflect.DeepEqual(got, want) {
				t.Fatalf("%s: got %s, want %s", cs.Source, compact(t, got), compact(t, want))
			}
		})
	}
}

// The copied fixtures cover every node type of the catalog in a trace, and the named cases exist.
func TestConformanceCoverage(t *testing.T) {
	fixtures := loadFixtures(t, filepath.Join(conformanceDir, "plot", "fixtures"))
	c, err := graph.CatalogFromJSON(plotCatalogJSON(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	traced := map[string]bool{}
	names := map[string]bool{}
	for _, f := range fixtures {
		names[f.Name] = true
		if f.Kind != "trace" {
			continue
		}
		nodes, _ := f.Graph["nodes"].([]any)
		for _, n := range nodes {
			m, _ := n.(map[string]any)
			typ, _ := m["type"].(string)
			traced[typ] = true
		}
	}
	for typ := range c.Nodes {
		if !traced[typ] {
			t.Errorf("no trace fixture uses %s", typ)
		}
	}
	for _, name := range []string{"trace-wait-timeout", "trace-switch-default", "trace-call-choices", "validate-cel-type-error", "validate-end-in-deployment"} {
		if !names[name] {
			t.Errorf("no fixture %s", name)
		}
	}
	// A copied fixture is adapted only with its reason.
	for _, f := range fixtures {
		var keys map[string]json.RawMessage
		readJSON(t, f.file, &keys)
		if _, ok := keys["changed"]; ok && f.Changed == "" {
			t.Errorf("%s: changed is empty", f.Name)
		}
	}
}
