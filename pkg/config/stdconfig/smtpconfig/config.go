// Package smtpconfig provides typed configuration for SMTP email delivery.
// It is a stdconfig package intended to be registered via config.Register[smtpconfig.Config].
package smtpconfig

// Config holds SMTP connection settings.
//
// Env prefix: "smtp" (e.g. SMTP_HOST → smtp.host, SMTP_PASSWORD → smtp.password).
type Config struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Username string `koanf:"username"`
	Password string `koanf:"password" secret:"true"`
	// From is the sender email address.
	From string `koanf:"from"`
}

// SetDefaults applies default values for fields that are zero-valued.
// Port defaults to 587 (SMTP submission with STARTTLS).
// Call this after populating Config from a source that may omit defaults.
func (c *Config) SetDefaults() {
	if c.Port == 0 {
		c.Port = 587
	}
}
