package composition

import (
	"context"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/health"
	"github.com/sirupsen/logrus"
)

// --- test config fixtures ---------------------------------------------------

type gatedCfg struct {
	APIKey  string `koanf:"apikey"`
	Flavour string `koanf:"flavour"`
}

func (gatedCfg) ConfigPrefix() string      { return "feat" }
func (c *gatedCfg) IsConfigured() bool     { return c.APIKey != "" }
func (c *gatedCfg) DisabledReason() string { return "feat.apikey required" }

type alwaysOnCfg struct {
	Value string `koanf:"value"`
}

func (alwaysOnCfg) ConfigPrefix() string { return "core" }

// --- helpers ----------------------------------------------------------------

func newGateBuilder(t *testing.T, data map[string]any) (*Builder, health.CapabilityRegistry) {
	t.Helper()
	src, err := config.Build(static.New(data))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	registry := health.NewCapabilityRegistry()
	ctx := NewBuildContext(nil, src, WithLogger(logrus.New()), WithCapabilityRegistry(registry))
	return newBuilder(ctx, Descriptor{Name: "test"}), registry
}

func mustFirstCapability(t *testing.T, registry health.CapabilityRegistry, key string) health.Capability {
	t.Helper()
	for _, p := range registry.List() {
		cap := p.Probe(context.Background())
		if cap.Key == key {
			return cap
		}
	}
	t.Fatalf("capability %q not emitted; registry list: %v", key, registry.List())
	return health.Capability{}
}

// --- SkipIfDisabled ---------------------------------------------------------

func TestSkipIfDisabled_ActiveFeature_ReturnsFalse(t *testing.T) {
	t.Parallel()

	b, registry := newGateBuilder(t, map[string]any{"feat.apikey": "sk-123"})
	if SkipIfDisabled[gatedCfg](b) {
		t.Fatal("SkipIfDisabled should return false when feature is configured")
	}

	cap := mustFirstCapability(t, registry, "feat")
	if !cap.Enabled || cap.Status != health.StatusHealthy {
		t.Errorf("probe for active feature: got enabled=%v status=%s, want true/healthy", cap.Enabled, cap.Status)
	}
}

func TestSkipIfDisabled_DisabledFeature_ReturnsTrue_AndEmitsProbe(t *testing.T) {
	t.Parallel()

	// No keys under "feat" → Disabled.
	b, registry := newGateBuilder(t, map[string]any{"other.k": "v"})
	if !SkipIfDisabled[gatedCfg](b) {
		t.Fatal("SkipIfDisabled should return true when feature is disabled")
	}

	cap := mustFirstCapability(t, registry, "feat")
	if cap.Enabled {
		t.Error("disabled feature should emit Enabled=false")
	}
	if cap.Status != health.StatusDisabled {
		t.Errorf("Status: got %s, want %s", cap.Status, health.StatusDisabled)
	}
	if !strings.Contains(cap.Message, "feat.apikey required") {
		t.Errorf("Message should carry DisabledReason, got %q", cap.Message)
	}
}

func TestSkipIfDisabled_NoSource_StaysActive(t *testing.T) {
	t.Parallel()

	// No Source → treat as Active so test harnesses don't silently drop modules.
	ctx := NewBuildContext(nil, WithLogger(logrus.New()))
	b := newBuilder(ctx, Descriptor{Name: "test"})

	if SkipIfDisabled[gatedCfg](b) {
		t.Error("no Source should not mark feature disabled")
	}
}

func TestSkipIfDisabled_AlwaysOnCfg_ReturnsFalse(t *testing.T) {
	t.Parallel()

	// alwaysOnCfg does not implement Configured → Active.
	b, registry := newGateBuilder(t, nil)
	if SkipIfDisabled[alwaysOnCfg](b) {
		t.Error("always-on config (no Configured) should not be skipped")
	}

	cap := mustFirstCapability(t, registry, "core")
	if !cap.Enabled {
		t.Error("always-on config should emit Enabled=true probe")
	}
}

// --- IfConfigured -----------------------------------------------------------

type subFeature struct {
	Token string `koanf:"token"`
}

func (s subFeature) IsConfigured() bool { return s.Token != "" }

func TestIfConfigured_InvokesWhenConfigured(t *testing.T) {
	t.Parallel()

	b, registry := newGateBuilder(t, nil)
	sub := subFeature{Token: "abc"}

	called := false
	IfConfigured(b, "parent.sub", sub, func() { called = true })
	if !called {
		t.Error("fn must be invoked when sub-feature is configured")
	}

	cap := mustFirstCapability(t, registry, "parent.sub")
	if !cap.Enabled {
		t.Errorf("probe should report Enabled=true; got %+v", cap)
	}
}

func TestIfConfigured_SkipsWhenUnconfigured(t *testing.T) {
	t.Parallel()

	b, registry := newGateBuilder(t, nil)
	sub := subFeature{Token: ""}

	called := false
	IfConfigured(b, "parent.sub", sub, func() { called = true })
	if called {
		t.Error("fn must not be invoked when sub-feature is unconfigured")
	}

	cap := mustFirstCapability(t, registry, "parent.sub")
	if cap.Enabled || cap.Status != health.StatusDisabled {
		t.Errorf("probe for unconfigured sub-feature: got %+v", cap)
	}
}

// --- GatedRegister ----------------------------------------------------------

func TestGatedRegister_ActiveFeature_InvokesFn(t *testing.T) {
	t.Parallel()

	b, _ := newGateBuilder(t, map[string]any{"feat.apikey": "sk-123"})
	called := false
	err := GatedRegister[gatedCfg](b, func() error { called = true; return nil })
	if err != nil {
		t.Fatalf("GatedRegister: %v", err)
	}
	if !called {
		t.Error("fn must be invoked for Active feature")
	}
}

func TestGatedRegister_DisabledFeature_SkipsAndReturnsNil(t *testing.T) {
	t.Parallel()

	b, registry := newGateBuilder(t, nil)
	called := false
	err := GatedRegister[gatedCfg](b, func() error { called = true; return nil })
	if err != nil {
		t.Errorf("GatedRegister should return nil when disabled; got %v", err)
	}
	if called {
		t.Error("fn must not be invoked when feature is disabled")
	}

	// Probe must still be emitted.
	mustFirstCapability(t, registry, "feat")
}

// --- Engine.Compile integration --------------------------------------------

// gatedComponent is a minimal Component that uses SkipIfDisabled in Build.
// It provides one *gatedCfg-keyed value so we can assert zero providers
// land in the container when the feature is disabled.
type gatedComponent struct{}

func (gatedComponent) Descriptor() Descriptor { return Descriptor{Name: "gatedcomponent"} }
func (gatedComponent) Build(builder *Builder) error {
	if SkipIfDisabled[gatedCfg](builder) {
		return nil
	}
	Provide[string](builder, "gated-provider-value")
	return nil
}

func TestIntegration_Engine_DisabledFeature_NoProvidersWired(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(nil))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	capReg := health.NewCapabilityRegistry()
	engine := NewEngine()
	if err := engine.Register(gatedComponent{}); err != nil {
		t.Fatalf("engine.Register: %v", err)
	}

	ctx := NewBuildContext(nil, src, WithLogger(logrus.New()), WithCapabilityRegistry(capReg))
	container, err := engine.Compile(ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Disabled feature: the string provider must NOT be in the container.
	// Any resolution attempt should report NotProvided.
	if v, err := Resolve[string](container); err == nil {
		t.Errorf("disabled feature should not provide string; got %q", v)
	} else if !IsNotProvided(err) {
		t.Errorf("expected NotProvided, got %v", err)
	}

	// Capability probe still emitted by the gate helper.
	mustFirstCapability(t, capReg, "feat")
}

func TestIntegration_Engine_ActiveFeature_ProvidersWired(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{"feat.apikey": "sk-test"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	capReg := health.NewCapabilityRegistry()
	engine := NewEngine()
	if err := engine.Register(gatedComponent{}); err != nil {
		t.Fatalf("engine.Register: %v", err)
	}

	ctx := NewBuildContext(nil, src, WithLogger(logrus.New()), WithCapabilityRegistry(capReg))
	container, err := engine.Compile(ctx)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	v, err := Resolve[string](container)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if v != "gated-provider-value" {
		t.Errorf("Resolve: got %q, want %q", v, "gated-provider-value")
	}
}

// --- Partial-config gate --------------------------------------------------

func TestSkipIfDisabled_PartialConfig_StrictPanics(t *testing.T) {
	t.Parallel()

	// feat.flavour set but feat.apikey missing → partial. In strict mode
	// Register errors; the gate helper surfaces it as a panic.
	src, err := config.Build(static.New(map[string]any{"feat.flavour": "spicy"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	reg := config.NewRegistry(src)
	reg.SetStrict(config.StrictYes)

	// Build the BuildContext and swap in the strict registry by pre-populating
	// so buildCtx.Registry() returns our instance on first call.
	ctx := NewBuildContext(nil, src, WithLogger(logrus.New()), WithCapabilityRegistry(health.NewCapabilityRegistry()))
	ctx.registry = reg // internal override for the test
	b := newBuilder(ctx, Descriptor{Name: "test"})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on strict-mode partial config")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, "partially configured") {
			t.Errorf("panic message should name partial: %v", r)
		}
	}()
	SkipIfDisabled[gatedCfg](b)
}

func TestSkipIfDisabled_PartialConfig_LaxSkips(t *testing.T) {
	t.Parallel()

	// In lax mode Register stores StatePartiallyConfigured; SkipIfDisabled
	// treats that as skip and logs a warning.
	src, err := config.Build(static.New(map[string]any{"feat.flavour": "spicy"}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	reg := config.NewRegistry(src)
	reg.SetStrict(config.StrictLax)

	ctx := NewBuildContext(nil, src, WithLogger(logrus.New()), WithCapabilityRegistry(health.NewCapabilityRegistry()))
	ctx.registry = reg
	b := newBuilder(ctx, Descriptor{Name: "test"})

	if !SkipIfDisabled[gatedCfg](b) {
		t.Error("partial config in lax mode should skip")
	}

	cap := mustFirstCapability(t, ctx.CapabilityRegistry(), "feat")
	if cap.Status != health.StatusDown {
		t.Errorf("partial-config capability should report Status=down, got %s", cap.Status)
	}
}
