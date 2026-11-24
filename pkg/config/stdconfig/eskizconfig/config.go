// Package eskizconfig provides typed configuration for the platform's Eskiz SMS account.
// It is a stdconfig package intended to be registered via config.Register[eskizconfig.Config].
package eskizconfig

// DefaultURL is the Eskiz API the connector uses when URL is empty.
const DefaultURL = "https://notify.eskiz.uz"

// Config holds the Eskiz account the platform offers to tenants as a platform credential of the
// automation connector "eskiz".
//
// Env prefix: "eskiz" (e.g. ESKIZ_EMAIL → eskiz.email, ESKIZ_PASSWORD → eskiz.password).
type Config struct {
	// URL is the API base URL; empty: DefaultURL.
	URL      string `koanf:"url"`
	Email    string `koanf:"email"`
	Password string `koanf:"password" secret:"true"`
	// From is the sender id; empty: the account's default.
	From string `koanf:"from"`
}

// ConfigPrefix returns the koanf prefix for eskizconfig ("eskiz").
func (Config) ConfigPrefix() string { return "eskiz" }

// IsConfigured reports whether the platform Eskiz account is usable: Email and Password are set.
// Implements config.Configured; there is no separate enable flag.
func (c *Config) IsConfigured() bool { return c.Email != "" && c.Password != "" }

// DisabledReason explains why Eskiz is off when IsConfigured returns false.
func (c *Config) DisabledReason() string { return "ESKIZ_EMAIL and ESKIZ_PASSWORD required" }
