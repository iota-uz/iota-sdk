package oidcconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy constructs a Config from the legacy *configuration.Configuration.
// SetDefaults is called to ensure token lifetime defaults are applied.
func FromLegacy(c *configuration.Configuration) Config {
	cfg := Config{
		IssuerURL:            c.OIDC.IssuerURL,
		CryptoKey:            c.OIDC.CryptoKey,
		AccessTokenLifetime:  c.OIDC.AccessTokenLifetime,
		RefreshTokenLifetime: c.OIDC.RefreshTokenLifetime,
		IDTokenLifetime:      c.OIDC.IDTokenLifetime,
	}
	cfg.SetDefaults()
	return cfg
}
