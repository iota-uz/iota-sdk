package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	envprov "github.com/iota-uz/iota-sdk/pkg/config/providers/env"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig"
)

func writeTempEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp .env: %v", err)
	}
	return path
}

// TestIntegration_AliasAndCanonicalKeys verifies that legacy env-var aliases
// resolve correctly alongside canonical dot-delimited keys, and that explicit
// values override tag-based defaults.
func TestIntegration_AliasAndCanonicalKeys(t *testing.T) {
	t.Parallel()

	envContent := strings.Join([]string{
		"PORT=8080",
		"RATE_LIMIT_ENABLED=false",
		"db.host=canonical",
	}, "\n")
	envPath := writeTempEnv(t, envContent)

	src, err := config.Build(
		envprov.New(envPath).WithAliases(stdconfig.AllLegacyAliases()...),
		static.New(map[string]any{"db.port": "9999"}),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	registry := config.NewRegistry(src)
	bundle, err := stdconfig.RegisterAll(registry)
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}
	if err := registry.Seal(); err != nil {
		t.Fatalf("Seal: %v", err)
	}

	// PORT alias maps to http.port.
	if bundle.HTTP.Port != 8080 {
		t.Errorf("HTTP.Port: got %d, want 8080", bundle.HTTP.Port)
	}

	// db.host comes from the .env file as a canonical key.
	if bundle.DB.Host != "canonical" {
		t.Errorf("DB.Host: got %q, want \"canonical\"", bundle.DB.Host)
	}

	// db.port from static provider (later provider wins).
	if bundle.DB.Port != "9999" {
		t.Errorf("DB.Port: got %q, want \"9999\"", bundle.DB.Port)
	}

	// RATE_LIMIT_ENABLED=false in env: *bool sentinel correctly captures explicit false.
	if bundle.RateLimit.Enabled == nil {
		t.Fatal("RateLimit.Enabled: got nil, want non-nil *bool")
	}
	if *bundle.RateLimit.Enabled != false {
		t.Errorf("RateLimit.Enabled: got %v, want false (RATE_LIMIT_ENABLED=false in .env must win)", *bundle.RateLimit.Enabled)
	}
	if bundle.RateLimit.IsEnabled() {
		t.Error("RateLimit.IsEnabled(): got true, want false")
	}

	// Origin of http.port comes from the env provider.
	origin, ok := src.Origin("http.port")
	if !ok {
		t.Error("Origin(http.port): expected ok=true")
	}
	if !strings.HasPrefix(origin, "env:") {
		t.Errorf("Origin(http.port): expected env: prefix, got %q", origin)
	}

	// Origin of db.port comes from static provider.
	origin, ok = src.Origin("db.port")
	if !ok {
		t.Error("Origin(db.port): expected ok=true")
	}
	if origin != "static" {
		t.Errorf("Origin(db.port): got %q, want \"static\"", origin)
	}
}

// TestIntegration_RateLimitDefaultPath verifies that when RATE_LIMIT_ENABLED is
// absent from the environment, the tag engine allocates *bool = true.
func TestIntegration_RateLimitDefaultPath(t *testing.T) {
	t.Parallel()

	// Empty .env — no RATE_LIMIT_ENABLED set.
	envPath := writeTempEnv(t, "PORT=3200\n")

	src, err := config.Build(
		envprov.New(envPath).WithAliases(stdconfig.AllLegacyAliases()...),
	)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	registry := config.NewRegistry(src)
	bundle, err := stdconfig.RegisterAll(registry)
	if err != nil {
		t.Fatalf("RegisterAll: %v", err)
	}

	// Tag engine must allocate *bool and set to true when key is absent.
	if bundle.RateLimit.Enabled == nil {
		t.Fatal("RateLimit.Enabled: got nil, want non-nil (allocated by tag engine)")
	}
	if !*bundle.RateLimit.Enabled {
		t.Errorf("RateLimit.Enabled: got false, want true (tag default path)")
	}
	if !bundle.RateLimit.IsEnabled() {
		t.Error("RateLimit.IsEnabled(): got false, want true")
	}
}

// TestIntegration_Keys verifies that Keys() returns a sorted, deduped slice.
func TestIntegration_Keys(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"z.key": "1",
		"a.key": "2",
		"m.key": "3",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	keys := src.Keys()
	if len(keys) != 3 {
		t.Fatalf("Keys: expected 3, got %d: %v", len(keys), keys)
	}
	for i := 1; i < len(keys); i++ {
		if keys[i] <= keys[i-1] {
			t.Errorf("Keys() not sorted at index %d: %v", i, keys)
		}
	}
}
