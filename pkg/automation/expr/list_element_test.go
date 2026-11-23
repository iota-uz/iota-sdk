package expr_test

import (
	"errors"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

type pull struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// A field of a list element (`items[0].url`, `prs[i].title`) passes the type check, and evaluating it must
// not fail however the host typed the list and its elements.
func TestListElementField(t *testing.T) {
	fields := map[string]expr.Field{
		"list<map<string>>": expr.ListOf(expr.MapOf(expr.StringField)),
		"list<map<dyn>>":    expr.ListOf(expr.MapOf(expr.DynField)),
		"list<dyn>":         expr.ListOf(expr.DynField),
		"list<object>":      expr.ListOf(expr.ObjectOf(expr.Fields{{Name: "url", Field: expr.StringField}, {Name: "title", Field: expr.OptionalOf(expr.StringField)}})),
	}
	values := map[string]any{
		"[]any of map[string]any":    []any{map[string]any{"url": "u0", "title": "t0"}, map[string]any{"url": "u1", "title": "t1"}},
		"[]map[string]any":           []map[string]any{{"url": "u0", "title": "t0"}, {"url": "u1", "title": "t1"}},
		"[]map[string]string":        []map[string]string{{"url": "u0", "title": "t0"}, {"url": "u1", "title": "t1"}},
		"[]any of map[string]string": []any{map[string]string{"url": "u0", "title": "t0"}, map[string]string{"url": "u1", "title": "t1"}},
		"[]struct":                   []pull{{URL: "u0", Title: "t0"}, {URL: "u1", Title: "t1"}},
		"[]*struct":                  []*pull{{URL: "u0", Title: "t0"}, {URL: "u1", Title: "t1"}},
	}
	for ft, f := range fields {
		env, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{"items": f, "i": expr.IntField}}, expr.Limits{})
		if err != nil {
			t.Fatal(err)
		}
		for vt, items := range values {
			for src, want := range map[string]string{
				`items[0].url`:                     "u0",
				`items[i].title`:                   "t1",
				`items[size(items) - 1].url`:       "u1",
				`items.map(p, p.url).join(",")`:    "u0,u1",
				`join(items.map(p, p.title), "+")`: "t0+t1",
			} {
				p, issues := env.Check(src, expr.TypeString)
				if len(issues) > 0 {
					t.Fatalf("%s %s: %v", ft, src, issues)
				}
				got, err := p.Eval(map[string]any{"items": items, "i": 1})
				if err != nil || got != want {
					t.Errorf("%s over %s: %s = %v, %v; want %q", ft, vt, src, got, err, want)
				}
			}
		}
	}
}

// What the list lacks is missing context or a failure, as for any other read.
func TestListElementFieldAbsent(t *testing.T) {
	env, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{"items": expr.ListOf(expr.MapOf(expr.StringField))}}, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, issues := env.Check(`items[0].url`, expr.TypeString)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	var mc *expr.MissingContextError
	if _, err := p.Eval(map[string]any{"items": []any{map[string]any{"title": "x"}}}); !errors.As(err, &mc) || mc.Path != "items[0].url" {
		t.Fatalf("a key the element lacks: %v", err)
	}
	var ee *expr.EvalError
	if _, err := p.Eval(map[string]any{"items": []any{}}); !errors.As(err, &ee) || ee.Code != expr.CodeExpressionFailed {
		t.Fatalf("an index out of bounds: %v", err)
	}
}

func TestJoin(t *testing.T) {
	env, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{"labels": expr.ListOf(expr.StringField), "d": expr.DynField, "n": expr.ListOf(expr.IntField)}}, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	vars := map[string]any{"labels": []string{"bug", "ui"}, "d": []any{"a", int64(1)}, "n": []any{int64(1)}}
	for src, want := range map[string]any{
		`join(["a", "b"], ", ") == "a, b"`: true,
		`join(labels, "\n• ")`:             "bug\n• ui",
		`join([], ",")`:                    "",
		`join(["x"], ",")`:                 "x",
		`labels.join(", ")`:                "bug, ui",
		`"[" + join(labels, "") + "]"`:     "[bugui]",
	} {
		p, issues := env.Check(src, expr.TypeAny)
		if len(issues) > 0 {
			t.Fatalf("%s: %v", src, issues)
		}
		if got, err := p.Eval(vars); err != nil || got != want {
			t.Errorf("%s = %v, %v; want %v", src, got, err, want)
		}
	}
	for _, src := range []string{`join(n, ",")`, `join(labels)`, `join(labels, 1)`, `join("a", ",")`} {
		if _, issues := env.Check(src, expr.TypeAny); len(issues) == 0 {
			t.Errorf("%s is accepted", src)
		}
	}
	p, issues := env.Check(`join(d, ",")`, expr.TypeString)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	var ee *expr.EvalError
	if _, err := p.Eval(vars); !errors.As(err, &ee) {
		t.Fatalf("a dyn list with a number: %v", err)
	}
}

func TestReferences(t *testing.T) {
	env, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{"a": expr.DynField}}, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	got := env.References(`record.title + event.mail["subject"].x + prs[0].url + (a.l.exists(x, x.k == record.n && has(event.q.w)) ? "" : "")`)
	want := [][]string{{"record", "title"}, {"event", "mail", "subject", "x"}, {"prs"}, {"a", "l"}, {"record", "n"}, {"event", "q", "w"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if len(got[i]) != len(want[i]) {
			t.Fatalf("got %v, want %v", got, want)
		}
		for j := range want[i] {
			if got[i][j] != want[i][j] {
				t.Fatalf("got %v, want %v", got, want)
			}
		}
	}
	if env.References(`(`) != nil {
		t.Fatal("a source that does not parse has references")
	}
}
