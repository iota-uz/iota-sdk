package expr_test

import (
	"errors"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/automation/expr"
)

type fakeLookup struct {
	m     map[string]any
	reads []string
	err   error
}

func (f *fakeLookup) Lookup(key string) (any, bool, error) {
	f.reads = append(f.reads, key)
	if f.err != nil {
		return nil, false, f.err
	}
	v, ok := f.m[key]
	return v, ok, nil
}

func lookupEnv(t *testing.T) *expr.Env {
	t.Helper()
	env, err := expr.NewEnv(expr.Scope{Vars: map[string]expr.Field{
		"state": expr.MapOf(expr.DynField),
		"event": expr.ObjectOf(expr.Fields{{Name: "chat", Field: expr.StringField}}),
	}}, expr.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func evalLookup(t *testing.T, env *expr.Env, src string, lk expr.Lookup) (any, error) {
	t.Helper()
	p, issues := env.Check(src, expr.TypeAny)
	if len(issues) > 0 {
		t.Fatalf("%s: %v", src, issues)
	}
	return p.Eval(map[string]any{"state": lk, "event": map[string]any{"chat": "42"}})
}

func TestStateLookupGetAndHas(t *testing.T) {
	env := lookupEnv(t)
	lk := &fakeLookup{m: map[string]any{"chat:42": map[string]any{"task": "T-1", "n": int64(3)}, "flag": true}}
	for src, want := range map[string]any{
		`state.has("chat:42")`:                   true,
		`state.has("chat:43")`:                   false,
		`state.has("chat:" + event.chat)`:        true,
		`state.get("chat:43") == null`:           true,
		`state.get("flag")`:                      true,
		`state.get("chat:42").task`:              "T-1",
		`state.get("chat:42").n + 1`:             int64(4),
		`state.has("a") ? "yes" : "no"`:          "no",
		`"flag" in state`:                        true,
		`state.has("flag") && state.get("flag")`: true,
	} {
		got, err := evalLookup(t, env, src, lk)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got != want {
			t.Errorf("%s = %#v, want %#v", src, got, want)
		}
	}
}

func TestStateLookupReadsOnlyWhatIsAsked(t *testing.T) {
	env := lookupEnv(t)
	lk := &fakeLookup{m: map[string]any{"a": 1}}
	if _, err := evalLookup(t, env, `state.has("a") || state.has("b")`, lk); err != nil {
		t.Fatal(err)
	}
	if len(lk.reads) != 1 || lk.reads[0] != "a" {
		t.Fatalf("reads %v", lk.reads)
	}
}

func TestStateLookupError(t *testing.T) {
	env := lookupEnv(t)
	lk := &fakeLookup{err: errors.New("db down")}
	if _, err := evalLookup(t, env, `state.get("a")`, lk); err == nil {
		t.Fatal("a failed lookup is not an error")
	}
}

func TestMapGetAndHasOnPlainMaps(t *testing.T) {
	env := lookupEnv(t)
	got, err := evalLookup(t, env, `{"a": 1}.has("a") && {"a": 1}.get("b") == null`, &fakeLookup{})
	if err != nil || got != true {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestStateEmptyMapIsNoKey(t *testing.T) {
	env := lookupEnv(t)
	p, issues := env.Check(`state.has("a")`, expr.TypeBool)
	if len(issues) > 0 {
		t.Fatal(issues)
	}
	got, err := p.Eval(map[string]any{"state": map[string]any{}, "event": map[string]any{"chat": "1"}})
	if err != nil || got != false {
		t.Fatalf("got %v, %v", got, err)
	}
}
