package expr_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

func evalIn(t *testing.T, env *expr.Env, src string, vars map[string]any) (any, error) {
	t.Helper()
	p, issues := env.Check(src, expr.TypeAny)
	if len(issues) > 0 {
		t.Fatalf("%s: %v", src, issues)
	}
	return p.Eval(vars)
}

func missingAt(t *testing.T, err error) string {
	t.Helper()
	var mc *expr.MissingContextError
	if !errors.As(err, &mc) {
		t.Fatalf("want missing context, got %v", err)
	}
	return mc.Path
}

// Review round 2 of #1358.
func TestReviewRoundTwo(t *testing.T) {
	obj := expr.ObjectOf(expr.Fields{{Name: "a", Field: expr.IntField}})
	scope := expr.Scope{Vars: map[string]expr.Field{
		"o":      obj,
		"tenant": expr.MapOf(expr.DynField),
		"flag":   expr.BoolField,
		"m":      expr.OptionalOf(expr.MapOf(expr.IntField)),
		"x":      expr.DoubleField,
	}}
	env, err := expr.NewEnv(scope, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	// A nested declaration changed after NewEnv changes nothing.
	scope.Vars["o"].Props[0].Name = "renamed"
	if v, err := evalIn(t, env, "o.a + 1", map[string]any{"o": map[string]any{"a": 1}}); err != nil || v != int64(2) {
		t.Fatalf("o.a after the caller renamed it: %v %v", v, err)
	}

	// Equality keeps key types; map literals iterate in key order.
	if v, _ := evalIn(t, env, `dyn({1: "x"}) == dyn({"1": "x"})`, nil); v != false {
		t.Errorf(`{1: "x"} == {"1": "x"} is %v`, v)
	}
	if v, _ := evalIn(t, env, `tenant == {"k": 1}`, map[string]any{"tenant": map[string]any{"k": 1}}); v != true {
		t.Errorf("tenant == literal is %v", v)
	}
	for range 20 {
		v, err := evalIn(t, env, `{"b": 2, "a": 1, "c": 3}.map(k, k)`, nil)
		if err != nil || strings.Join(toStrings(v), ",") != "a,b,c" {
			t.Fatalf("map literal keys: %v %v", v, err)
		}
	}

	// The unknown that counts is the read the evaluation took; a literal key keeps its brackets.
	_, err = evalIn(t, env, `[flag ? tenant.z : tenant.a, tenant.z]`, map[string]any{"flag": false, "tenant": map[string]any{}})
	if got := missingAt(t, err); got != "tenant.a" {
		t.Errorf("first needed read: %s", got)
	}
	_, err = evalIn(t, env, `tenant["a.b"] == 1`, map[string]any{"tenant": map[string]any{}})
	if got := missingAt(t, err); got != `tenant["a.b"]` {
		t.Errorf("a literal key: %s", got)
	}

	// An absent optional map reads like a given empty one.
	_, err1 := evalIn(t, env, `m["x"] == 1`, map[string]any{})
	_, err2 := evalIn(t, env, `m["x"] == 1`, map[string]any{"m": map[string]any{}})
	if missingAt(t, err1) != missingAt(t, err2) {
		t.Errorf("omitted %v, empty %v", err1, err2)
	}

	// NaN is not a number of the input.
	_, err = evalIn(t, env, "x > 0.0", map[string]any{"x": math.NaN()})
	var ee *expr.EvalError
	if !errors.As(err, &ee) || ee.Code != expr.CodeInvalidInput {
		t.Errorf("NaN: %v", err)
	}
	if _, ok := expr.Number(math.Inf(1)); ok {
		t.Error("Number accepts +Inf")
	}

	// Completion offers no conversion the check refuses.
	for _, c := range env.Complete("ui") {
		if c.Label == "uint" {
			t.Error("uint is suggested")
		}
	}

	// A schema that declares a property twice is refused.
	if _, err := expr.FieldFromJSONSchema([]byte(`{"type":"object","properties":{"x":{"type":"string"},"x":{"type":"integer"}}}`)); err == nil {
		t.Error("a duplicate property is accepted")
	}

	// Parentheses do not hide the end of an expression; positions survive a NBSP after {{.
	if _, issues := expr.Template(env, "Total: {{ (1 + 2 }}"); len(issues) == 0 {
		t.Error("an unbalanced parenthesis renders literally")
	}
	_, issues := expr.Template(env, "ab {{ o.nope }}")
	if len(issues) != 1 || issues[0].Line != 1 || issues[0].Column != 8 { // the dot of o.nope; a NBSP-blind offset gives 7
		t.Errorf("the issue's position: %+v", issues)
	}
}

func toStrings(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, x := range list {
		s, _ := x.(string)
		out = append(out, s)
	}
	return out
}

// Review round 4: equality keeps CEL's cost accounting and its error precedence.
func TestEqualityCostAndErrors(t *testing.T) {
	env, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{"x": expr.IntField}}, expr.Limits{CostLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	p, issues := env.Check("(1 == 1) == true", expr.TypeBool)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	if _, err := p.Eval(nil); err == nil {
		t.Error("two comparisons fit a cost limit of 1")
	}
	env, err = expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{"x": expr.IntField}}, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	p, issues = env.Check("x == 1 / 0", expr.TypeBool)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	_, err = p.Eval(map[string]any{})
	var ee *expr.EvalError
	if !errors.As(err, &ee) || ee.Code != expr.CodeExpressionFailed {
		t.Errorf("a missing x beside a division by zero: %v", err)
	}
}
