package config

import (
	"testing"
)

// staticTestProvider is a minimal in-package Provider for source tests.
type staticTestProvider struct {
	data map[string]any
	name string
}

func (p *staticTestProvider) Load() (map[string]any, error) {
	out := make(map[string]any, len(p.data))
	for k, v := range p.data {
		out[k] = v
	}
	return out, nil
}

func (p *staticTestProvider) Name() string {
	if p.name != "" {
		return p.name
	}
	return "static-test"
}

func TestBuild_ProviderCompositionOrder(t *testing.T) {
	t.Parallel()

	p1 := &staticTestProvider{data: map[string]any{
		"key":        "first",
		"only.in.p1": "p1-value",
	}}
	p2 := &staticTestProvider{data: map[string]any{
		"key": "second", // overrides p1
	}}

	src, err := Build(p1, p2)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var out struct {
		Key string `koanf:"key"`
	}
	if err := src.Unmarshal("", &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Key != "second" {
		t.Errorf("later provider should win: got %q, want %q", out.Key, "second")
	}

	if _, ok := src.Get("only.in.p1"); !ok {
		t.Error("Get: key only.in.p1 should be present")
	}
	if _, ok := src.Get("does.not.exist"); ok {
		t.Error("Get: non-existent key should return false")
	}
}

func TestBuild_Immutability(t *testing.T) {
	t.Parallel()

	p1 := &staticTestProvider{data: map[string]any{"x": "original"}}
	src, err := Build(p1)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Mutating the original provider's map after Build must not affect src.
	p1.data["x"] = "mutated"
	p1.data["new.key"] = "injected"

	// src cannot absorb the new key because it has no Load method.
	// Verify the frozen state.
	var out struct {
		X string `koanf:"x"`
	}
	if err := src.Unmarshal("", &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.X != "original" {
		t.Errorf("source should be frozen: got %q, want %q", out.X, "original")
	}
	if _, ok := src.Get("new.key"); ok {
		t.Error("post-Build mutation should not be visible in Source")
	}
}

func TestBuild_EmptyProviders(t *testing.T) {
	t.Parallel()

	src, err := Build()
	if err != nil {
		t.Fatalf("Build with no providers: %v", err)
	}
	if _, ok := src.Get("anything"); ok {
		t.Error("empty source should have no keys")
	}
}

func TestOrigin_TwoProviders(t *testing.T) {
	t.Parallel()

	p1 := &staticTestProvider{
		data: map[string]any{"shared": "from-p1", "only-p1": "val1"},
		name: "provider-one",
	}
	p2 := &staticTestProvider{
		data: map[string]any{"shared": "from-p2", "only-p2": "val2"},
		name: "provider-two",
	}

	src, err := Build(p1, p2)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// p2 (later) wins "shared"
	if origin, ok := src.Origin("shared"); !ok || origin != "provider-two" {
		t.Errorf("Origin(shared): got (%q, %v), want (\"provider-two\", true)", origin, ok)
	}
	// p1 owns "only-p1"
	if origin, ok := src.Origin("only-p1"); !ok || origin != "provider-one" {
		t.Errorf("Origin(only-p1): got (%q, %v), want (\"provider-one\", true)", origin, ok)
	}
	// p2 owns "only-p2"
	if origin, ok := src.Origin("only-p2"); !ok || origin != "provider-two" {
		t.Errorf("Origin(only-p2): got (%q, %v), want (\"provider-two\", true)", origin, ok)
	}
}

func TestKeys_SortedDedup(t *testing.T) {
	t.Parallel()

	p := &staticTestProvider{data: map[string]any{
		"z.key": "1",
		"a.key": "2",
		"m.key": "3",
	}}
	src, err := Build(p)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	keys := src.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d: %v", len(keys), keys)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] <= keys[i-1] {
			t.Errorf("Keys() not sorted at index %d: %v", i, keys)
		}
	}
}

func TestGet_Missing(t *testing.T) {
	t.Parallel()

	src, err := Build(&staticTestProvider{data: map[string]any{"existing": "v"}})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	val, ok := src.Get("missing.key")
	if ok {
		t.Errorf("Get missing key: expected ok=false, got ok=true with val=%v", val)
	}
	if val != nil {
		t.Errorf("Get missing key: expected nil value, got %v", val)
	}
}
