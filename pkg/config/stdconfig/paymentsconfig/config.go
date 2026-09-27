// Package paymentsconfig provides typed configuration for payment providers
// (Click, Payme, Octo, Stripe). Register via config.Register[paymentsconfig.Config].
package paymentsconfig

// ClickConfig holds Click payment gateway settings.
// Env prefix: "click" (e.g. CLICK_URL → click.url).
type ClickConfig struct {
	URL            string `koanf:"url"            default:"https://my.click.uz"`
	MerchantID     int64  `koanf:"merchantid"`
	MerchantUserID int64  `koanf:"merchantuserid"`
	ServiceID      int64  `koanf:"serviceid"`
	SecretKey      string `koanf:"secretkey"      secret:"true"`
}

// PaymeConfig holds Payme payment gateway settings.
// Env prefix: "payme" (e.g. PAYME_URL → payme.url).
type PaymeConfig struct {
	URL        string `koanf:"url"        default:"https://checkout.test.paycom.uz"`
	MerchantID string `koanf:"merchantid"`
	User       string `koanf:"user"       default:"Paycom"`
	SecretKey  string `koanf:"secretkey"  secret:"true"`
}

// OctoConfig holds Octo payment gateway settings.
// Env prefix: "octo" (e.g. OCTO_SHOP_ID → octo.shopid).
type OctoConfig struct {
	ShopID     int32  `koanf:"shopid"`
	Secret     string `koanf:"secret"     secret:"true"`
	SecretHash string `koanf:"secrethash" secret:"true"`
	NotifyURL  string `koanf:"notifyurl"`
}

// StripeConfig holds Stripe payment gateway settings.
// Env prefix: "stripe" (e.g. STRIPE_SECRET_KEY → stripe.secretkey).
type StripeConfig struct {
	SecretKey     string `koanf:"secretkey"     secret:"true"`
	SigningSecret string `koanf:"signingsecret" secret:"true"`
}

// Config bundles all four payment-provider configurations.
// Register under an empty prefix or a "payments" prefix depending on your env layout.
type Config struct {
	Click  ClickConfig  `koanf:"click"`
	Payme  PaymeConfig  `koanf:"payme"`
	Octo   OctoConfig   `koanf:"octo"`
	Stripe StripeConfig `koanf:"stripe"`
}

// ConfigPrefix returns the koanf prefix for paymentsconfig ("payments").
func (Config) ConfigPrefix() string { return "payments" }

// IsConfigured reports whether any payment provider has the minimum settings.
// Payments is a fan-out feature: the module-level gate lights up as soon as
// one provider is configured, and modules then use composition.IfConfigured
// on individual sub-structs to gate per-provider wiring.
func (c *Config) IsConfigured() bool {
	return c.Click.IsConfigured() || c.Payme.IsConfigured() ||
		c.Octo.IsConfigured() || c.Stripe.IsConfigured()
}

// DisabledReason explains why payments are off when IsConfigured returns false.
func (c *Config) DisabledReason() string {
	return "at least one payment provider's credentials required (CLICK / PAYME / OCTO / STRIPE)"
}

// IsConfigured reports whether Click has the minimum credentials to operate.
func (c ClickConfig) IsConfigured() bool {
	return c.MerchantID != 0 && c.ServiceID != 0 && c.SecretKey != ""
}

// IsConfigured reports whether Payme has the minimum credentials to operate.
func (c PaymeConfig) IsConfigured() bool {
	return c.MerchantID != "" && c.SecretKey != ""
}

// IsConfigured reports whether Octo has the minimum credentials to operate.
func (c OctoConfig) IsConfigured() bool {
	return c.ShopID != 0 && c.Secret != ""
}

// IsConfigured reports whether Stripe has the minimum credentials to operate.
func (c StripeConfig) IsConfigured() bool {
	return c.SecretKey != ""
}
