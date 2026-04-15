package redisconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/redisconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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

func TestSetDefaults_ZeroURL(t *testing.T) {
	t.Parallel()

	cfg := redisconfig.Config{}
	cfg.SetDefaults()

	if cfg.URL != "localhost:6379" {
		t.Errorf("URL default: got %q, want %q", cfg.URL, "localhost:6379")
	}
}

func TestSetDefaults_NonZeroURLUnchanged(t *testing.T) {
	t.Parallel()

	cfg := redisconfig.Config{URL: "custom:6380"}
	cfg.SetDefaults()

	if cfg.URL != "custom:6380" {
		t.Errorf("URL should be unchanged: got %q", cfg.URL)
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{
		RedisURL: "redis.prod.internal:6379",
	}

	got := redisconfig.FromLegacy(legacy)

	if got.URL != "redis.prod.internal:6379" {
		t.Errorf("URL: got %q, want %q", got.URL, "redis.prod.internal:6379")
	}
}

func TestFromLegacy_DefaultWhenEmpty(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{}
	got := redisconfig.FromLegacy(legacy)

	if got.URL != "localhost:6379" {
		t.Errorf("URL default via FromLegacy: got %q, want %q", got.URL, "localhost:6379")
	}
}
