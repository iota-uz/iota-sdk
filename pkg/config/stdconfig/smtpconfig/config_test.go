package smtpconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
)

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"smtp.host":     "mail.example.com",
		"smtp.port":     465,
		"smtp.username": "user@example.com",
		"smtp.password": "secret",
		"smtp.from":     "no-reply@example.com",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var cfg smtpconfig.Config
	if err := src.Unmarshal("smtp", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.Host != "mail.example.com" {
		t.Errorf("Host: want mail.example.com, got %s", cfg.Host)
	}
	if cfg.Port != 465 {
		t.Errorf("Port: want 465, got %d", cfg.Port)
	}
	if cfg.Password != "secret" {
		t.Errorf("Password: want secret, got %s", cfg.Password)
	}
	if cfg.From != "no-reply@example.com" {
		t.Errorf("From: want no-reply@example.com, got %s", cfg.From)
	}
}

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestConfig_Defaults(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[smtpconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if cfg.Port != 587 {
		t.Errorf("default Port: want 587, got %d", cfg.Port)
	}

	// Should not overwrite an explicit port.
	r2 := config.NewRegistry(buildSource(t, map[string]any{"smtp.port": 25}))
	cfg2, err := config.Register[smtpconfig.Config](r2)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if cfg2.Port != 25 {
		t.Errorf("explicit port must not be overwritten, got %d", cfg2.Port)
	}
}
