// Package smtpconfig provides typed configuration for SMTP email delivery.
// It is a stdconfig package intended to be registered via config.Register[smtpconfig.Config].
package smtpconfig

// Config holds SMTP connection settings.
//
// Env prefix: "smtp" (e.g. SMTP_HOST → smtp.host, SMTP_PASSWORD → smtp.password).
type Config struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"     default:"587"`
	Username string `koanf:"username"`
	Password string `koanf:"password" secret:"true"`
	// From is the sender email address.
	From string `koanf:"from"`
}

// ConfigPrefix returns the koanf prefix for smtpconfig ("smtp").
func (Config) ConfigPrefix() string { return "smtp" }

// IsConfigured reports whether SMTP is usable — Host is the one field the
// feature cannot synthesise a default for. Port, Username, Password and From
// all have practical defaults or are optional.
func (c *Config) IsConfigured() bool { return c.Host != "" }

// DisabledReason explains why SMTP is off when IsConfigured returns false.
func (c *Config) DisabledReason() string { return "SMTP_HOST required" }
