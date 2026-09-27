package oidcconfig_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/oidcconfig"
)

func TestConfig_StaticRoundTrip(t *testing.T) {
	t.Parallel()

	src, err := config.Build(static.New(map[string]any{
		"oidc.issuerurl":            "https://auth.example.com",
		"oidc.cryptokey":            "base64keyhere",
		"oidc.accesstokenlifetime":  "2h",
		"oidc.refreshtokenlifetime": "336h",
		"oidc.idtokenlifetime":      "30m",
	}))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	var cfg oidcconfig.Config
	if err := src.Unmarshal("oidc", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.IssuerURL != "https://auth.example.com" {
		t.Errorf("IssuerURL: want https://auth.example.com, got %s", cfg.IssuerURL)
	}
	if cfg.CryptoKey != "base64keyhere" {
		t.Errorf("CryptoKey mismatch")
	}
	if cfg.AccessTokenLifetime != 2*time.Hour {
		t.Errorf("AccessTokenLifetime: want 2h, got %s", cfg.AccessTokenLifetime)
	}
	if cfg.RefreshTokenLifetime != 336*time.Hour {
		t.Errorf("RefreshTokenLifetime: want 336h, got %s", cfg.RefreshTokenLifetime)
	}
	if cfg.IDTokenLifetime != 30*time.Minute {
		t.Errorf("IDTokenLifetime: want 30m, got %s", cfg.IDTokenLifetime)
	}
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	full := &oidcconfig.Config{IssuerURL: "https://auth.example.com", CryptoKey: "key"}
	if !full.IsConfigured() {
		t.Error("IsConfigured: should be true when both IssuerURL and CryptoKey set")
	}

	noURL := &oidcconfig.Config{CryptoKey: "key"}
	if noURL.IsConfigured() {
		t.Error("IsConfigured: should be false when IssuerURL missing")
	}

	noKey := &oidcconfig.Config{IssuerURL: "https://auth.example.com"}
	if noKey.IsConfigured() {
		t.Error("IsConfigured: should be false when CryptoKey missing")
	}

	empty := &oidcconfig.Config{}
	if empty.IsConfigured() {
		t.Error("IsConfigured: should be false when both fields empty")
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

func TestDefaults_TokenLifetimes(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[oidcconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.AccessTokenLifetime != time.Hour {
		t.Errorf("AccessTokenLifetime default: want 1h, got %s", cfg.AccessTokenLifetime)
	}
	if cfg.RefreshTokenLifetime != 720*time.Hour {
		t.Errorf("RefreshTokenLifetime default: want 720h, got %s", cfg.RefreshTokenLifetime)
	}
	if cfg.IDTokenLifetime != time.Hour {
		t.Errorf("IDTokenLifetime default: want 1h, got %s", cfg.IDTokenLifetime)
	}
}

func TestDefaults_NoOverwrite(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"oidc.accesstokenlifetime":  "2h",
		"oidc.refreshtokenlifetime": "48h",
		"oidc.idtokenlifetime":      "15m",
	}))
	cfg, err := config.Register[oidcconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.AccessTokenLifetime != 2*time.Hour {
		t.Errorf("explicit AccessTokenLifetime must not be overwritten: got %s", cfg.AccessTokenLifetime)
	}
	if cfg.RefreshTokenLifetime != 48*time.Hour {
		t.Errorf("explicit RefreshTokenLifetime must not be overwritten: got %s", cfg.RefreshTokenLifetime)
	}
	if cfg.IDTokenLifetime != 15*time.Minute {
		t.Errorf("explicit IDTokenLifetime must not be overwritten: got %s", cfg.IDTokenLifetime)
	}
}
