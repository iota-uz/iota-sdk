package jevconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/jevconfig"
)

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"automation.jev.provider":     "openai",
		"automation.jev.apikey":       "sk-test",
		"automation.jev.defaultmodel": "gpt-6-luna",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	r := config.NewRegistry(src)
	cfg, err := config.Register[jevconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if cfg.Provider != "openai" || cfg.APIKey != "sk-test" || cfg.DefaultModel != "gpt-6-luna" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestValidate_Provider(t *testing.T) {
	t.Parallel()

	for p, ok := range map[string]bool{"": true, "openrouter": true, "openai": true, "anthropic": false} {
		err := (&jevconfig.Config{Provider: p}).Validate()
		if (err == nil) != ok {
			t.Errorf("Provider %q: err = %v, want ok=%v", p, err, ok)
		}
	}
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	if (&jevconfig.Config{Provider: "openai"}).IsConfigured() {
		t.Error("a config without a key is configured")
	}
	if !(&jevconfig.Config{APIKey: "k"}).IsConfigured() {
		t.Error("a config with a key is not configured")
	}
}
