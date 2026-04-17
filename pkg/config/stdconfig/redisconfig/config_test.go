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

func TestDefaults_ZeroURL(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[redisconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.URL != "localhost:6379" {
		t.Errorf("URL default: got %q, want %q", cfg.URL, "localhost:6379")
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
