// Package oidcconfig provides typed configuration for the OIDC provider.
// It is a stdconfig package intended to be registered via config.Register[oidcconfig.Config].
package oidcconfig

import "time"

// Config holds OIDC issuer and token lifetime settings.
//
// Env prefix: "oidc" (e.g. OIDC_ISSUER_URL → oidc.issuerurl, OIDC_CRYPTO_KEY → oidc.cryptokey).
type Config struct {
	IssuerURL            string        `koanf:"issuerurl"`
	CryptoKey            string        `koanf:"cryptokey"            secret:"true"`
	AccessTokenLifetime  time.Duration `koanf:"accesstokenlifetime"  default:"1h"`
	RefreshTokenLifetime time.Duration `koanf:"refreshtokenlifetime" default:"720h"`
	IDTokenLifetime      time.Duration `koanf:"idtokenlifetime"      default:"1h"`
}

// ConfigPrefix returns the koanf prefix for oidcconfig ("oidc").
func (Config) ConfigPrefix() string { return "oidc" }

// IsConfigured reports whether OIDC is usable — both IssuerURL and CryptoKey must be set.
func (c *Config) IsConfigured() bool {
	return c.IssuerURL != "" && c.CryptoKey != ""
}
