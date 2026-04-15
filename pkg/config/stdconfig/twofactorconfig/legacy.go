package twofactorconfig

import "github.com/iota-uz/iota-sdk/pkg/configuration"

// FromLegacy produces a Config from the monolithic *configuration.Configuration.
// Merges TwoFactorAuthOptions and OTPDeliveryOptions — both concern 2FA/OTP.
// SetDefaults is called after mapping to cover any zero-value fields.
func FromLegacy(c *configuration.Configuration) Config {
	tfa := c.TwoFactorAuth
	otp := c.OTPDelivery
	cfg := Config{
		Enabled:       tfa.Enabled,
		TOTPIssuer:    tfa.TOTPIssuer,
		EncryptionKey: tfa.EncryptionKey,
		OTP: OTPConfig{
			CodeLength:  tfa.OTPCodeLength,
			TTLSeconds:  tfa.OTPTTLSeconds,
			MaxAttempts: tfa.OTPMaxAttempts,
			EnableEmail: otp.EnableEmail,
			EnableSMS:   otp.EnableSMS,
		},
	}
	cfg.SetDefaults()
	return cfg
}
