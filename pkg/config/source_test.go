package config

import (
	"testing"
)

// staticTestProvider is a minimal in-package Provider for source tests.
type staticTestProvider struct {
	data map[string]any
}

func (p *staticTestProvider) Load() (map[string]any, error) {
	out := make(map[string]any, len(p.data))
	for k, v := range p.data {
		out[k] = v
	}
	return out, nil
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

	if !src.Has("only.in.p1") {
		t.Error("Has: key only.in.p1 should be present")
	}
	if src.Has("does.not.exist") {
		t.Error("Has: non-existent key should return false")
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
	if src.Has("new.key") {
		t.Error("post-Build mutation should not be visible in Source")
	}
}

func TestBuild_EmptyProviders(t *testing.T) {
	t.Parallel()

	src, err := Build()
	if err != nil {
		t.Fatalf("Build with no providers: %v", err)
	}
	if src.Has("anything") {
		t.Error("empty source should have no keys")
	}
}
