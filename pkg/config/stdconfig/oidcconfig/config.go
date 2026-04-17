// Package oidcconfig provides typed configuration for the OIDC provider.
// It is a stdconfig package intended to be registered via config.Register[oidcconfig.Config].
package oidcconfig

import "time"

// Config holds OIDC issuer and token lifetime settings.
//
// Env prefix: "oidc" (e.g. OIDC_ISSUER_URL → oidc.issuerurl, OIDC_CRYPTO_KEY → oidc.cryptokey).
type Config struct {
	IssuerURL            string        `koanf:"issuerurl"`
	CryptoKey            string        `koanf:"cryptokey" secret:"true"`
	AccessTokenLifetime  time.Duration `koanf:"accesstokenlifetime"`
	RefreshTokenLifetime time.Duration `koanf:"refreshtokenlifetime"`
	IDTokenLifetime      time.Duration `koanf:"idtokenlifetime"`
}

// ConfigPrefix returns the koanf prefix for oidcconfig ("oidc").
func (Config) ConfigPrefix() string { return "oidc" }

// SetDefaults applies default token lifetime values when fields are zero.
// Defaults: AccessTokenLifetime=1h, RefreshTokenLifetime=720h, IDTokenLifetime=1h.
func (c *Config) SetDefaults() {
	if c.AccessTokenLifetime == 0 {
		c.AccessTokenLifetime = time.Hour
	}
	if c.RefreshTokenLifetime == 0 {
		c.RefreshTokenLifetime = 720 * time.Hour
	}
	if c.IDTokenLifetime == 0 {
		c.IDTokenLifetime = time.Hour
	}
}

// IsConfigured reports whether OIDC is usable — both IssuerURL and CryptoKey must be set.
func (c *Config) IsConfigured() bool {
	return c.IssuerURL != "" && c.CryptoKey != ""
}
