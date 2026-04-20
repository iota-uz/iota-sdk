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

// TestIntegration_CanonicalKeys verifies that canonical UPPER_SNAKE_CASE env
// vars resolve correctly via the natural transform, and that explicit values
// override tag-based defaults.
func TestIntegration_CanonicalKeys(t *testing.T) {
	t.Parallel()

	// Use keys that CI runners and developer shells are unlikely to set in
	// their process environment — otherwise the env overlay (process env > file)
	// would mask the .env file values we're testing here.
	envContent := strings.Join([]string{
		"HTTP_PORT=8080",
		"RATELIMIT_ENABLED=false",
		"APP_TELEGRAMBOTTOKEN=canonical",
	}, "\n")
	envPath := writeTempEnv(t, envContent)

	src, err := config.Build(
		envprov.New(envPath),
		static.New(map[string]any{"app.environment": "static-wins"}),
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

	// HTTP_PORT → http.port
	if bundle.HTTP.Port != 8080 {
		t.Errorf("HTTP.Port: got %d, want 8080", bundle.HTTP.Port)
	}

	// APP_TELEGRAMBOTTOKEN → app.telegrambottoken from .env file.
	if bundle.App.TelegramBotToken != "canonical" {
		t.Errorf("App.TelegramBotToken: got %q, want \"canonical\"", bundle.App.TelegramBotToken)
	}

	// app.environment from static provider (later provider wins over tag default).
	if bundle.App.Environment != "static-wins" {
		t.Errorf("App.Environment: got %q, want \"static-wins\"", bundle.App.Environment)
	}

	// RATELIMIT_ENABLED=false: *bool sentinel correctly captures explicit false.
	if bundle.RateLimit.Enabled == nil {
		t.Fatal("RateLimit.Enabled: got nil, want non-nil *bool")
	}
	if *bundle.RateLimit.Enabled != false {
		t.Errorf("RateLimit.Enabled: got %v, want false (RATELIMIT_ENABLED=false in .env must win)", *bundle.RateLimit.Enabled)
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

	// Origin of app.environment comes from static provider.
	origin, ok = src.Origin("app.environment")
	if !ok {
		t.Error("Origin(app.environment): expected ok=true")
	}
	if origin != "static" {
		t.Errorf("Origin(app.environment): got %q, want \"static\"", origin)
	}
}

// TestIntegration_RateLimitDefaultPath verifies that when RATELIMIT_ENABLED is
// absent from the environment, the tag engine allocates *bool = true.
func TestIntegration_RateLimitDefaultPath(t *testing.T) {
	t.Parallel()

	// Empty .env — no RATELIMIT_ENABLED set.
	envPath := writeTempEnv(t, "HTTP_PORT=3200\n")

	src, err := config.Build(
		envprov.New(envPath),
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
