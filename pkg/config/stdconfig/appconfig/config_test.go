package appconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
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
		"app.environment":         "production",
		"app.telegrambottoken":    "bot-token-123",
		"app.enabletestendpoints": true,
	})

	var cfg appconfig.Config
	if err := src.Unmarshal("app", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Environment != "production" {
		t.Errorf("Environment: got %q, want %q", cfg.Environment, "production")
	}
	if cfg.TelegramBotToken != "bot-token-123" {
		t.Errorf("TelegramBotToken: got %q, want %q", cfg.TelegramBotToken, "bot-token-123")
	}
	if !cfg.EnableTestEndpoints {
		t.Error("EnableTestEndpoints: expected true")
	}
}

func TestSetDefaults_ZeroEnvironment(t *testing.T) {
	t.Parallel()

	cfg := appconfig.Config{}
	cfg.SetDefaults()

	if cfg.Environment != "development" {
		t.Errorf("Environment default: got %q, want %q", cfg.Environment, "development")
	}
	if cfg.EnableTestEndpoints {
		t.Error("EnableTestEndpoints default: expected false")
	}
}

func TestSetDefaults_NonZeroEnvironmentUnchanged(t *testing.T) {
	t.Parallel()

	cfg := appconfig.Config{Environment: "staging"}
	cfg.SetDefaults()

	if cfg.Environment != "staging" {
		t.Errorf("Environment should be unchanged: got %q", cfg.Environment)
	}
}

func TestIsProduction_True(t *testing.T) {
	t.Parallel()

	cfg := appconfig.Config{Environment: "production"}
	if !cfg.IsProduction() {
		t.Error("IsProduction: expected true for environment=production")
	}
}

func TestIsProduction_False(t *testing.T) {
	t.Parallel()

	for _, env := range []string{"development", "staging", ""} {
		t.Run(env, func(t *testing.T) {
			t.Parallel()
			cfg := appconfig.Config{Environment: env}
			if cfg.IsProduction() {
				t.Errorf("IsProduction: expected false for environment=%q", env)
			}
		})
	}
}

func TestIsDev_True(t *testing.T) {
	t.Parallel()

	for _, env := range []string{"development", "staging", "test", ""} {
		t.Run(env, func(t *testing.T) {
			t.Parallel()
			cfg := appconfig.Config{Environment: env}
			if !cfg.IsDev() {
				t.Errorf("IsDev: expected true for environment=%q", env)
			}
		})
	}
}

func TestIsDev_False(t *testing.T) {
	t.Parallel()

	cfg := appconfig.Config{Environment: "production"}
	if cfg.IsDev() {
		t.Error("IsDev: expected false for environment=production")
	}
}
