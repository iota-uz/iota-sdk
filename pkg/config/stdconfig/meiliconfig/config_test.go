package meiliconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/meiliconfig"
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
		"meili.url":    "http://meili.example.com:7700",
		"meili.apikey": "master-key-abc",
	})

	var cfg meiliconfig.Config
	if err := src.Unmarshal("meili", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.URL != "http://meili.example.com:7700" {
		t.Errorf("URL: got %q, want %q", cfg.URL, "http://meili.example.com:7700")
	}
	if cfg.APIKey != "master-key-abc" {
		t.Errorf("APIKey: got %q, want %q", cfg.APIKey, "master-key-abc")
	}
}

func TestIsConfigured_BothSet(t *testing.T) {
	t.Parallel()

	cfg := meiliconfig.Config{
		URL:    "http://localhost:7700",
		APIKey: "secret",
	}
	if !cfg.IsConfigured() {
		t.Error("IsConfigured: expected true when both URL and APIKey are set")
	}
}

func TestIsConfigured_MissingURL(t *testing.T) {
	t.Parallel()

	cfg := meiliconfig.Config{APIKey: "secret"}
	if cfg.IsConfigured() {
		t.Error("IsConfigured: expected false when URL is empty")
	}
}

func TestIsConfigured_MissingAPIKey(t *testing.T) {
	t.Parallel()

	cfg := meiliconfig.Config{URL: "http://localhost:7700"}
	if cfg.IsConfigured() {
		t.Error("IsConfigured: expected false when APIKey is empty")
	}
}

func TestIsConfigured_BothEmpty(t *testing.T) {
	t.Parallel()

	cfg := meiliconfig.Config{}
	if cfg.IsConfigured() {
		t.Error("IsConfigured: expected false when both URL and APIKey are empty")
	}
}
