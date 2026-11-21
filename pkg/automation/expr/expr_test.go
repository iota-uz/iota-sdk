package expr_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

func TestFieldJSONKeepsOrder(t *testing.T) {
	src := `{"object":{"z":"string","a":{"optional":"int"},"m":{"map":"dyn"},"l":{"list":{"enum":["x","y"]}},"t":"timestamp"}}`
	var f expr.Field
	if err := json.Unmarshal([]byte(src), &f); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != src {
		t.Fatalf("round trip\n got %s\nwant %s", out, src)
	}
	schema := string(f.JSONSchema())
	if !strings.Contains(schema, `"properties":{"z":`) || !strings.Contains(schema, `"required":["z","m","l","t"]`) {
		t.Fatalf("schema: %s", schema)
	}
	for _, bad := range []string{`"uint"`, `"bytes"`, `{"enum":[]}`, `{"object":{"in":"string"}}`, `{"object":{"a b":"string"}}`, `{"list":"int","map":"int"}`} {
		if err := json.Unmarshal([]byte(bad), &f); err == nil {
			t.Errorf("%s is accepted", bad)
		}
	}
}

func TestFieldFromJSONSchema(t *testing.T) {
	f, err := expr.FieldFromJSONSchema(json.RawMessage(`{"type":"object","description":"x","properties":{
		"ok":{"type":"boolean"},"count":{"type":"integer"},"items":{"type":"array","items":{"type":"string","enum":["a"]}},"ratio":{"type":"number"}},
		"required":["ok","count"]}`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"object":{"ok":"bool","count":"int","items":{"optional":{"list":{"enum":["a"]}}},"ratio":{"optional":"double"}}}`
	if f.String() != want {
		t.Fatalf("got %s\nwant %s", f, want)
	}
	for _, bad := range []string{`{"type":"null"}`, `{"type":"object"}`, `{"type":"string","format":"date"}`, `{"type":"object","properties":{},"required":["x"]}`} {
		if _, err := expr.FieldFromJSONSchema(json.RawMessage(bad)); err == nil {
			t.Errorf("%s is accepted", bad)
		}
	}
}

func TestFormatDouble(t *testing.T) {
	for in, want := range map[float64]string{
		0: "0", 2: "2", 0.75: "0.75", -1.5: "-1.5", 1e21: "1e+21", 1e20: "100000000000000000000",
		1e-7: "1e-7", 0.000001: "0.000001", 123456.789: "123456.789", 1.5e300: "1.5e+300",
	} {
		if got := expr.FormatDouble(in); got != want {
			t.Errorf("FormatDouble(%v) = %s, want %s", in, got, want)
		}
	}
}

func env(t *testing.T) *expr.Env {
	t.Helper()
	var issue expr.Field
	if err := json.Unmarshal([]byte(`{"object":{"number":"int","title":"string","meta":"dyn"}}`), &issue); err != nil {
		t.Fatal(err)
	}
	e, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{"issue": issue, "now": expr.TimestampField}}, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestValues(t *testing.T) {
	e := env(t)
	p, issues := e.Check("issue.number + 1", expr.TypeAny)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	for _, in := range []any{json.Number("7"), 7.0, 7, int64(7)} {
		v, err := p.Eval(map[string]any{"issue": map[string]any{"number": in, "title": "x"}})
		if err != nil || v != int64(8) {
			t.Fatalf("%T: %v %v", in, v, err)
		}
	}
	_, err := p.Eval(map[string]any{"issue": map[string]any{"number": json.Number("9007199254740993"), "title": "x"}})
	var ee *expr.EvalError
	if !errors.As(err, &ee) || ee.Code != expr.CodeInvalidInput {
		t.Fatalf("an unsafe int: %v", err)
	}
	// A dyn object drops null keys and reads integers as int; a missing key is missing context cut at it.
	p, _ = e.Check("issue.meta.a.b == 1", expr.TypeBool)
	_, err = p.Eval(map[string]any{"issue": map[string]any{"number": 1, "title": "x", "meta": map[string]any{"a": nil}}})
	var mc *expr.MissingContextError
	if !errors.As(err, &mc) || mc.Path != "issue.meta.a" {
		t.Fatalf("missing: %v", err)
	}
	p, _ = e.Check("now > timestamp(\"2026-01-01T00:00:00Z\")", expr.TypeBool)
	if v, err := p.Eval(map[string]any{"now": time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)}); err != nil || v != true {
		t.Fatalf("now: %v %v", v, err)
	}
}

func TestCheckPositions(t *testing.T) {
	_, issues := env(t).Check("issue.number == \n  issue.nope", expr.TypeBool)
	if len(issues) != 1 || issues[0].Line != 2 || issues[0].Code != expr.IssueType {
		t.Fatalf("issues: %+v", issues)
	}
	_, issues = expr.Template(env(t), "Hello\n#{{ issue.nope }}")
	if len(issues) != 1 || issues[0].Line != 2 {
		t.Fatalf("template issues: %+v", issues)
	}
}

func TestComplete(t *testing.T) {
	e := env(t)
	labels := func(cs []expr.Completion) string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Kind+":"+c.Label)
		}
		return strings.Join(out, ",")
	}
	if got := labels(e.Complete("issue.ti")); got != "field:title" {
		t.Fatalf("fields: %s", got)
	}
	got := labels(e.Complete("is"))
	if !strings.Contains(got, "variable:issue") {
		t.Fatalf("variables: %s", got)
	}
	got = labels(e.Complete("issue.title.low"))
	if !strings.Contains(got, "function:lowerAscii") {
		t.Fatalf("member functions: %s", got)
	}
}
