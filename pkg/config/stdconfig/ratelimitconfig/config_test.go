package ratelimitconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/ratelimitconfig"
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
		"ratelimit.enabled":   true,
		"ratelimit.globalrps": 500,
		"ratelimit.storage":   "memory",
		"ratelimit.redisurl":  "",
	})

	var cfg ratelimitconfig.Config
	if err := src.Unmarshal("ratelimit", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}
	if cfg.GlobalRPS != 500 {
		t.Errorf("GlobalRPS: got %d, want 500", cfg.GlobalRPS)
	}
	if cfg.Storage != "memory" {
		t.Errorf("Storage: got %q, want %q", cfg.Storage, "memory")
	}
}

func TestValidate_HappyPath_Memory(t *testing.T) {
	t.Parallel()

	cfg := ratelimitconfig.Config{Enabled: true, GlobalRPS: 1000, Storage: "memory"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_HappyPath_Redis(t *testing.T) {
	t.Parallel()

	cfg := ratelimitconfig.Config{Enabled: true, GlobalRPS: 500, Storage: "redis", RedisURL: "redis://localhost:6379"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_NegativeRPS(t *testing.T) {
	t.Parallel()

	cfg := ratelimitconfig.Config{Enabled: true, GlobalRPS: -1, Storage: "memory"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for negative GlobalRPS, got nil")
	}
}

func TestValidate_RPSTooHigh(t *testing.T) {
	t.Parallel()

	cfg := ratelimitconfig.Config{Enabled: true, GlobalRPS: 2000000, Storage: "memory"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for GlobalRPS > 1000000, got nil")
	}
}

func TestValidate_InvalidStorage(t *testing.T) {
	t.Parallel()

	cfg := ratelimitconfig.Config{Enabled: true, GlobalRPS: 100, Storage: "invalid"}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid storage, got nil")
	}
}

func TestValidate_RedisWithoutURL(t *testing.T) {
	t.Parallel()

	cfg := ratelimitconfig.Config{Enabled: true, GlobalRPS: 100, Storage: "redis", RedisURL: ""}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for redis storage without RedisURL, got nil")
	}
}

func TestSetDefaults(t *testing.T) {
	t.Parallel()

	cfg := ratelimitconfig.Config{} // all zero
	cfg.SetDefaults()

	if !cfg.Enabled {
		t.Error("Enabled: got false after SetDefaults, want true")
	}
	if cfg.GlobalRPS != 1000 {
		t.Errorf("GlobalRPS: got %d after SetDefaults, want 1000", cfg.GlobalRPS)
	}
	if cfg.Storage != "memory" {
		t.Errorf("Storage: got %q after SetDefaults, want %q", cfg.Storage, "memory")
	}
}
