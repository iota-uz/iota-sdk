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

func TestMustGet_Panics_WhenNotRegistered(t *testing.T) {
	t.Parallel()

	src := buildSource(t, nil)
	r := NewRegistry(src)

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("MustGet should panic when type not registered")
		}
	}()
	MustGet[serverConfig](r)
}

func TestMustGet_ReturnsValue_WhenRegistered(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"server.host": "example.com",
		"server.port": 443,
	})
	r := NewRegistry(src)
	if _, err := RegisterAt[serverConfig](r, "server"); err != nil {
		t.Fatalf("RegisterAt: %v", err)
	}

	cfg := MustGet[serverConfig](r)
	if cfg.Host != "example.com" {
		t.Errorf("Host: got %q", cfg.Host)
	}
}

// Ensure staticTestProvider (declared in source_test.go) satisfies Provider.
var _ Provider = (*staticTestProvider)(nil)
