// Package googleoauthconfig provides typed configuration for Google OAuth.
// It is a stdconfig package intended to be registered via config.Register[googleoauthconfig.Config].
package googleoauthconfig

// Config holds all Google OAuth settings.
// Env prefix: "googleoauth" (e.g. GOOGLE_REDIRECT_URL → googleoauth.redirecturl).
type Config struct {
	RedirectURL  string `koanf:"redirecturl"`
	ClientID     string `koanf:"clientid"`
	ClientSecret string `koanf:"clientsecret" secret:"true"`
}

// ConfigPrefix returns the koanf prefix for googleoauthconfig ("googleoauth").
// Legacy env aliases (GOOGLE_*) still map to the "google.*" path via the env
// provider's alias table; the canonical prefix is "googleoauth" to avoid
// collisions with future Google sub-services.
func (Config) ConfigPrefix() string { return "googleoauth" }

// IsConfigured returns true when both ClientID and ClientSecret are set.
// Google OAuth enablement is implicit — no explicit flag needed.
func (c *Config) IsConfigured() bool {
	return c.ClientID != "" && c.ClientSecret != ""
}
