package smtpconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy constructs a Config from the legacy *configuration.Configuration.
// SetDefaults is called to ensure port defaults are applied.
func FromLegacy(c *configuration.Configuration) Config {
	cfg := Config{
		Host:     c.SMTP.Host,
		Port:     c.SMTP.Port,
		Username: c.SMTP.Username,
		Password: c.SMTP.Password,
		From:     c.SMTP.From,
	}
	cfg.SetDefaults()
	return cfg
}
