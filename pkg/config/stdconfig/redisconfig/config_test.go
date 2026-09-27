package redisconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/redisconfig"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"redis.url": "redis.example.com:6380",
	})

	var cfg redisconfig.Config
	if err := src.Unmarshal("redis", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.URL != "redis.example.com:6380" {
		t.Errorf("URL: got %q, want %q", cfg.URL, "redis.example.com:6380")
	}
}

func TestImplicitDisable_WhenURLUnset(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[redisconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Unset URL means Redis is off. Previously a localhost:6379 tag default
	// masked the unset state; removing it lets gate helpers detect disabled.
	if cfg.URL != "" {
		t.Errorf("URL should be empty when unset (no default); got %q", cfg.URL)
	}
	if cfg.IsConfigured() {
		t.Error("IsConfigured should be false when URL is empty")
	}
	if got := cfg.DisabledReason(); got != "REDIS_URL not set" {
		t.Errorf("DisabledReason: got %q, want %q", got, "REDIS_URL not set")
	}
}

func TestIsConfigured_TrueWhenURLSet(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{"redis.url": "redis:6379"}))
	cfg, err := config.Register[redisconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !cfg.IsConfigured() {
		t.Error("IsConfigured should be true when URL is set")
	}
}

func TestDefaults_NonZeroURLUnchanged(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{"redis.url": "custom:6380"}))
	cfg, err := config.Register[redisconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.URL != "custom:6380" {
		t.Errorf("URL should be unchanged: got %q", cfg.URL)
	}
}
