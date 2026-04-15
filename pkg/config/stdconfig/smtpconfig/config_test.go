package smtpconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/smtpconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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

func TestConfig_SetDefaults(t *testing.T) {
	t.Parallel()

	cfg := &smtpconfig.Config{}
	cfg.SetDefaults()
	if cfg.Port != 587 {
		t.Errorf("SetDefaults Port: want 587, got %d", cfg.Port)
	}

	// Should not overwrite an explicit port.
	cfg2 := &smtpconfig.Config{Port: 25}
	cfg2.SetDefaults()
	if cfg2.Port != 25 {
		t.Errorf("SetDefaults must not overwrite explicit port, got %d", cfg2.Port)
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{}
	legacy.SMTP.Host = "smtp.example.com"
	legacy.SMTP.Port = 587
	legacy.SMTP.Username = "sender"
	legacy.SMTP.Password = "pw"
	legacy.SMTP.From = "sender@example.com"

	got := smtpconfig.FromLegacy(legacy)
	if got.Host != "smtp.example.com" {
		t.Errorf("Host mismatch")
	}
	if got.Port != 587 {
		t.Errorf("Port: want 587, got %d", got.Port)
	}
}

func TestFromLegacy_DefaultPort(t *testing.T) {
	t.Parallel()

	// Legacy port zero → default 587.
	legacy := &configuration.Configuration{}
	got := smtpconfig.FromLegacy(legacy)
	if got.Port != 587 {
		t.Errorf("default port: want 587, got %d", got.Port)
	}
}
