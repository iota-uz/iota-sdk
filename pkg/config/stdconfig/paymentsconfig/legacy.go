package paymentsconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// Pure field-for-field mapping — no validation, no derivation.
// SetDefaults is called so that envDefault-sourced values (e.g. Click.URL) are
// preserved when the legacy struct already holds the defaults from env parsing.
func FromLegacy(c *configuration.Configuration) Config {
	cfg := Config{
		Click: ClickConfig{
			URL:            c.Click.URL,
			MerchantID:     c.Click.MerchantID,
			MerchantUserID: c.Click.MerchantUserID,
			ServiceID:      c.Click.ServiceID,
			SecretKey:      c.Click.SecretKey,
		},
		Payme: PaymeConfig{
			URL:        c.Payme.URL,
			MerchantID: c.Payme.MerchantID,
			User:       c.Payme.User,
			SecretKey:  c.Payme.SecretKey,
		},
		Octo: OctoConfig{
			ShopID:     c.Octo.OctoShopID,
			Secret:     c.Octo.OctoSecret,
			SecretHash: c.Octo.OctoSecretHash,
			NotifyURL:  c.Octo.NotifyURL,
		},
		Stripe: StripeConfig{
			SecretKey:     c.Stripe.SecretKey,
			SigningSecret: c.Stripe.SigningSecret,
		},
	}
	cfg.SetDefaults()
	return cfg
}
