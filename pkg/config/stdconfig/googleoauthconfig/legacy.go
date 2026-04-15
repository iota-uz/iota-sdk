package googleoauthconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// Pure field-for-field mapping — no validation, no derivation.
func FromLegacy(c *configuration.Configuration) Config {
	g := c.Google
	return Config{
		RedirectURL:  g.RedirectURL,
		ClientID:     g.ClientID,
		ClientSecret: g.ClientSecret,
	}
}
