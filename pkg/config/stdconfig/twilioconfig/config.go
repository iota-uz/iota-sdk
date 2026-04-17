// Package twilioconfig provides typed configuration for Twilio SMS/voice integration.
// It is a stdconfig package intended to be registered via config.Register[twilioconfig.Config].
package twilioconfig

// Config holds Twilio API credentials and webhook settings.
//
// Env prefix: "twilio" (e.g. TWILIO_ACCOUNT_SID → twilio.accountsid).
type Config struct {
	WebhookURL  string `koanf:"webhookurl"`
	AccountSID  string `koanf:"accountsid"`
	AuthToken   string `koanf:"authtoken" secret:"true"`
	PhoneNumber string `koanf:"phonenumber"`
}

// ConfigPrefix returns the koanf prefix for twilioconfig ("twilio").
func (Config) ConfigPrefix() string { return "twilio" }

// IsConfigured reports whether Twilio is usable — both AccountSID and AuthToken must be set.
func (c *Config) IsConfigured() bool {
	return c.AccountSID != "" && c.AuthToken != ""
}
