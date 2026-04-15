package twilioconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy constructs a Config from the legacy *configuration.Configuration.
func FromLegacy(c *configuration.Configuration) Config {
	return Config{
		WebhookURL:  c.Twilio.WebhookURL,
		AccountSID:  c.Twilio.AccountSID,
		AuthToken:   c.Twilio.AuthToken,
		PhoneNumber: c.Twilio.PhoneNumber,
	}
}
