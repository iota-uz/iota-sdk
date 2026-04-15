// Package twofactorconfig provides typed configuration for two-factor authentication
// (TOTP) and OTP delivery. It merges the legacy TwoFactorAuthOptions and OTPDeliveryOptions
// concerns into a single cohesive Config.
// Intended to be registered via config.Register[twofactorconfig.Config].
package twofactorconfig

import "fmt"

// OTPConfig groups OTP delivery and policy settings under the "otp" sub-key.
type OTPConfig struct {
	// CodeLength is the number of digits in an OTP code. Defaults to 6. Valid range: 4-10.
	CodeLength int `koanf:"codelength"`
	// TTLSeconds is the OTP validity window in seconds. Defaults to 300. Valid range: 60-900.
	TTLSeconds int `koanf:"ttlseconds"`
	// MaxAttempts is the maximum allowed verification attempts. Defaults to 3. Valid range: 1-10.
	MaxAttempts int `koanf:"maxattempts"`
	// EnableEmail controls whether OTPs are delivered via email.
	EnableEmail bool `koanf:"enableemail"`
	// EnableSMS controls whether OTPs are delivered via SMS.
	EnableSMS bool `koanf:"enablesms"`
}

// Config holds all two-factor authentication settings.
// Env prefix: "twofactor" (e.g. ENABLE_2FA → twofactor.enabled).
type Config struct {
	// Enabled controls whether 2FA is active. Defaults to false.
	Enabled bool `koanf:"enabled"`
	// TOTPIssuer is the issuer name shown in authenticator apps. Required when Enabled=true.
	TOTPIssuer string `koanf:"totpissuer"`
	// EncryptionKey is used to encrypt TOTP secrets at rest. Required for production.
	EncryptionKey string `koanf:"encryptionkey" secret:"true"`
	// OTP groups OTP delivery and policy settings.
	OTP OTPConfig `koanf:"otp"`
}

// SetDefaults applies default values for fields with zero values.
// Called from FromLegacy to cover gaps when constructing from a zero Config.
func (c *Config) SetDefaults() {
	if c.OTP.CodeLength == 0 {
		c.OTP.CodeLength = 6
	}
	if c.OTP.TTLSeconds == 0 {
		c.OTP.TTLSeconds = 300
	}
	if c.OTP.MaxAttempts == 0 {
		c.OTP.MaxAttempts = 3
	}
}

// Validate checks 2FA configuration for errors.
// Validation is skipped entirely when Enabled=false, matching legacy behaviour.
// Implements config.Validatable so config.Register invokes it automatically.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil
	}

	if c.TOTPIssuer == "" {
		return fmt.Errorf("twofactorconfig: totpissuer is required when enabled=true")
	}

	if c.OTP.CodeLength < 4 || c.OTP.CodeLength > 10 {
		return fmt.Errorf("twofactorconfig: otp.codelength must be between 4 and 10, got %d", c.OTP.CodeLength)
	}

	if c.OTP.TTLSeconds < 60 || c.OTP.TTLSeconds > 900 {
		return fmt.Errorf("twofactorconfig: otp.ttlseconds must be between 60 and 900, got %d", c.OTP.TTLSeconds)
	}

	if c.OTP.MaxAttempts < 1 || c.OTP.MaxAttempts > 10 {
		return fmt.Errorf("twofactorconfig: otp.maxattempts must be between 1 and 10, got %d", c.OTP.MaxAttempts)
	}

	return nil
}
