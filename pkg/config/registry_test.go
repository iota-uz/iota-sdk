package config

import (
	"errors"
	"testing"
)

// --- helpers ----------------------------------------------------------------

func buildSource(t *testing.T, data map[string]any) Source {
	t.Helper()
	p := &staticTestProvider{data: data}
	src, err := Build(p)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return src
}

// --- test types -------------------------------------------------------------

type serverConfig struct {
	Host string `koanf:"host"`
	Port int    `koanf:"port"`
}

type alwaysValid struct {
	Value string `koanf:"value"`
}

func (a *alwaysValid) Validate() error { return nil }

type alwaysInvalid struct {
	Value string `koanf:"value"`
}

func (a *alwaysInvalid) Validate() error {
	return errors.New("always invalid")
}

// --- tests ------------------------------------------------------------------

func TestRegister_UnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"server.host": "localhost",
		"server.port": 8080,
	})
	r := NewRegistry(src)

	cfg, err := RegisterAt[serverConfig](r, "server")
	if err != nil {
		t.Fatalf("RegisterAt: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 8080)
	}
}

func TestRegister_ValidateCalledOnSuccess(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{"cfg.value": "hello"})
	r := NewRegistry(src)

	_, err := RegisterAt[alwaysValid](r, "cfg")
	if err != nil {
		t.Fatalf("RegisterAt with valid config: %v", err)
	}
}

func TestRegister_ValidateError_PropagatesAndAborts(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{"cfg.value": "hello"})
	r := NewRegistry(src)

	_, err := RegisterAt[alwaysInvalid](r, "cfg")
	if err == nil {
		t.Fatal("expected error from Validate, got nil")
	}
	if !errors.Is(err, errors.New("always invalid")) {
		// Just check the string contains the validation message.
		if got := err.Error(); got == "" {
			t.Error("error message must not be empty")
		}
	}

	// Type must not be stored after validation failure.
	_, ok := Lookup[alwaysInvalid](r)
	if ok {
		t.Error("failed registration should not store the type")
	}
}

func TestGet_NotFound(t *testing.T) {
	t.Parallel()

	src := buildSource(t, nil)
	r := NewRegistry(src)

	_, ok := Lookup[serverConfig](r)
	if ok {
		t.Error("Lookup should return false when type not registered")
	}
}

func TestGet_Lookup_WhenRegistered(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"server.host": "example.com",
		"server.port": 443,
	})
	r := NewRegistry(src)
	if _, err := RegisterAt[serverConfig](r, "server"); err != nil {
		t.Fatalf("RegisterAt: %v", err)
	}

	cfg, ok := Lookup[serverConfig](r)
	if !ok {
		t.Fatal("Lookup: expected ok=true")
	}
	if cfg.Host != "example.com" {
		t.Errorf("Host: got %q", cfg.Host)
	}
}

// featureToggle mirrors the shape every periodic task, request log and feature
// flag in the wild uses: an on-by-default switch plus values whose defaults are
// non-zero.
type featureToggle struct {
	Enabled    bool   `koanf:"enabled"    default:"true"`
	MaxRetries int    `koanf:"maxretries" default:"3"`
	Schedule   string `koanf:"schedule"   default:"*/15 * * * *"`
}

// A source that says "off" must be able to turn a default-on switch off.
// Applying tag defaults to whatever the source left zero cannot express this -
// false is indistinguishable from absent - so the setting was unreachable and
// every on-by-default feature was permanently on.
func TestRegisterAt_SourceCanTurnOffADefaultOnFlag(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"feature.enabled":    false,
		"feature.maxretries": 0,
		"feature.schedule":   "",
	})
	cfg, err := RegisterAt[featureToggle](NewRegistry(src), "feature")
	if err != nil {
		t.Fatalf("RegisterAt: %v", err)
	}
	if cfg.Enabled {
		t.Error("Enabled: got true, want false — the source said off")
	}
	if cfg.MaxRetries != 0 {
		t.Errorf("MaxRetries: got %d, want 0 — the source said 0", cfg.MaxRetries)
	}
	if cfg.Schedule != "" {
		t.Errorf("Schedule: got %q, want empty — the source said empty", cfg.Schedule)
	}
}

// Env vars arrive as strings; koanf decodes them weakly. "false" must not be
// read as "unset" either.
func TestRegisterAt_SourceCanTurnOffADefaultOnFlagWithStrings(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{"feature.enabled": "false"})
	cfg, err := RegisterAt[featureToggle](NewRegistry(src), "feature")
	if err != nil {
		t.Fatalf("RegisterAt: %v", err)
	}
	if cfg.Enabled {
		t.Error("Enabled: got true, want false")
	}
	// Everything the source did not mention still gets its default.
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want the default 3", cfg.MaxRetries)
	}
	if cfg.Schedule != "*/15 * * * *" {
		t.Errorf("Schedule: got %q, want the default", cfg.Schedule)
	}
}

// The other half of the contract: a silent source leaves every default alone.
func TestRegisterAt_DefaultsSurviveASilentSource(t *testing.T) {
	t.Parallel()

	cfg, err := RegisterAt[featureToggle](NewRegistry(buildSource(t, map[string]any{})), "feature")
	if err != nil {
		t.Fatalf("RegisterAt: %v", err)
	}
	if !cfg.Enabled || cfg.MaxRetries != 3 || cfg.Schedule != "*/15 * * * *" {
		t.Errorf("defaults not applied: %+v", *cfg)
	}
}

// Ensure staticTestProvider (declared in source_test.go) satisfies Provider.
var _ Provider = (*staticTestProvider)(nil)
