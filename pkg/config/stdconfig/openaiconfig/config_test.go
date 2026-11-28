package openaiconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/openaiconfig"
)

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"automation.openai.apikey":       "sk-test",
		"automation.openai.baseurl":      "https://llm.example.com/v1",
		"automation.openai.defaultmodel": "gpt-6-luna",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	r := config.NewRegistry(src)
	cfg, err := config.Register[openaiconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if cfg.APIKey != "sk-test" || cfg.BaseURL != "https://llm.example.com/v1" || cfg.DefaultModel != "gpt-6-luna" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if !cfg.IsConfigured() {
		t.Fatal("IsConfigured: want true")
	}
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	if (&openaiconfig.Config{}).IsConfigured() {
		t.Error("an empty config is configured")
	}
	if !(&openaiconfig.Config{APIKey: "k"}).IsConfigured() {
		t.Error("a config with a key is not configured")
	}
	if (&openaiconfig.Config{}).DisabledReason() == "" {
		t.Error("DisabledReason is empty")
	}
}

func TestValidate_BaseURL(t *testing.T) {
	t.Parallel()

	for url, ok := range map[string]bool{
		"":                                 true,
		"https://api.example.com/v1":       true,
		"http://api.example.com/v1":        false,
		"https://user:pw@api.example.com/": false,
		"ftp://x":                          false,
		"not a url":                        false,
	} {
		err := (&openaiconfig.Config{BaseURL: url}).Validate()
		if (err == nil) != ok {
			t.Errorf("BaseURL %q: err = %v, want ok=%v", url, err, ok)
		}
	}
}
