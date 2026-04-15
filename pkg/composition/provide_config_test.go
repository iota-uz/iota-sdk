package composition_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
)

// testCfg is a minimal typed config used only in these tests.
type testCfg struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
}

// noopComponent is a composition.Component that calls ProvideConfig[testCfg]
// inside its Build method.
type noopComponent struct{}

func (noopComponent) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "noop"}
}

func (noopComponent) Build(b *composition.Builder) error {
	return composition.ProvideConfig[testCfg](b, "svc")
}

func buildTestSource(t *testing.T) config.Source {
	t.Helper()
	src, err := config.Build(static.New(map[string]any{
		"svc.host": "testhost",
		"svc.port": 9999,
	}))
	if err != nil {
		t.Fatalf("build source: %v", err)
	}
	return src
}

func TestProvideConfig_ResolvesTypedConfig(t *testing.T) {
	t.Parallel()

	src := buildTestSource(t)

	// Build a minimal BuildContext with no app (nil) and our source.
	ctx := composition.NewBuildContext(nil, nil, src)

	engine := composition.NewEngine()
	if err := engine.Register(noopComponent{}); err != nil {
		t.Fatalf("engine.Register: %v", err)
	}

	container, err := engine.Compile(ctx)
	if err != nil {
		t.Fatalf("engine.Compile: %v", err)
	}

	ptr, err := composition.Resolve[*testCfg](container)
	if err != nil {
		t.Fatalf("Resolve[*testCfg]: %v", err)
	}
	if ptr == nil {
		t.Fatal("Resolve returned nil *testCfg")
	}
	if ptr.Host != "testhost" {
		t.Errorf("expected host=testhost, got %q", ptr.Host)
	}
	if ptr.Port != 9999 {
		t.Errorf("expected port=9999, got %d", ptr.Port)
	}
}

func TestProvideConfig_NoSource_ReturnsError(t *testing.T) {
	t.Parallel()

	// BuildContext with no Source → ProvideConfig must return an error.
	ctx := composition.NewBuildContext(nil, nil)

	engine := composition.NewEngine()
	if err := engine.Register(noopComponent{}); err != nil {
		t.Fatalf("engine.Register: %v", err)
	}

	_, err := engine.Compile(ctx)
	if err == nil {
		t.Fatal("expected error when no Source is attached, got nil")
	}
}
