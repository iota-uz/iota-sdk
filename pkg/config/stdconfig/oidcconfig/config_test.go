package oidcconfig_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/oidcconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
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

func TestSetDefaults(t *testing.T) {
	t.Parallel()

	cfg := &oidcconfig.Config{}
	cfg.SetDefaults()

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

func TestSetDefaults_NoOverwrite(t *testing.T) {
	t.Parallel()

	cfg := &oidcconfig.Config{
		AccessTokenLifetime:  2 * time.Hour,
		RefreshTokenLifetime: 48 * time.Hour,
		IDTokenLifetime:      15 * time.Minute,
	}
	cfg.SetDefaults()

	if cfg.AccessTokenLifetime != 2*time.Hour {
		t.Errorf("SetDefaults must not overwrite AccessTokenLifetime")
	}
	if cfg.RefreshTokenLifetime != 48*time.Hour {
		t.Errorf("SetDefaults must not overwrite RefreshTokenLifetime")
	}
	if cfg.IDTokenLifetime != 15*time.Minute {
		t.Errorf("SetDefaults must not overwrite IDTokenLifetime")
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{}
	legacy.OIDC.IssuerURL = "https://issuer.example.com"
	legacy.OIDC.CryptoKey = "mykey"
	legacy.OIDC.AccessTokenLifetime = 30 * time.Minute
	legacy.OIDC.RefreshTokenLifetime = 24 * time.Hour
	legacy.OIDC.IDTokenLifetime = 15 * time.Minute

	got := oidcconfig.FromLegacy(legacy)
	if got.IssuerURL != "https://issuer.example.com" {
		t.Errorf("IssuerURL mismatch")
	}
	if got.AccessTokenLifetime != 30*time.Minute {
		t.Errorf("AccessTokenLifetime: want 30m, got %s", got.AccessTokenLifetime)
	}
	if !got.IsConfigured() {
		t.Error("FromLegacy result should be IsConfigured")
	}
}

func TestFromLegacy_Defaults(t *testing.T) {
	t.Parallel()

	// Zero durations in legacy → defaults applied.
	legacy := &configuration.Configuration{}
	got := oidcconfig.FromLegacy(legacy)
	if got.AccessTokenLifetime != time.Hour {
		t.Errorf("default AccessTokenLifetime: want 1h, got %s", got.AccessTokenLifetime)
	}
	if got.RefreshTokenLifetime != 720*time.Hour {
		t.Errorf("default RefreshTokenLifetime: want 720h, got %s", got.RefreshTokenLifetime)
	}
}
