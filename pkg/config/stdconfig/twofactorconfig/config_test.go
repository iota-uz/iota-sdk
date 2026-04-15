package twofactorconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twofactorconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"twofactor.enabled":         true,
		"twofactor.totpissuer":      "MyApp",
		"twofactor.encryptionkey":   "super-secret-key",
		"twofactor.otp.codelength":  6,
		"twofactor.otp.ttlseconds":  300,
		"twofactor.otp.maxattempts": 3,
		"twofactor.otp.enableemail": true,
		"twofactor.otp.enablesms":   false,
	})

	var cfg twofactorconfig.Config
	if err := src.Unmarshal("twofactor", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !cfg.Enabled {
		t.Error("Enabled: got false, want true")
	}
	if cfg.TOTPIssuer != "MyApp" {
		t.Errorf("TOTPIssuer: got %q, want %q", cfg.TOTPIssuer, "MyApp")
	}
	if cfg.EncryptionKey != "super-secret-key" {
		t.Errorf("EncryptionKey: got %q, want %q", cfg.EncryptionKey, "super-secret-key")
	}
	if cfg.OTP.CodeLength != 6 {
		t.Errorf("OTP.CodeLength: got %d, want 6", cfg.OTP.CodeLength)
	}
	if cfg.OTP.TTLSeconds != 300 {
		t.Errorf("OTP.TTLSeconds: got %d, want 300", cfg.OTP.TTLSeconds)
	}
	if cfg.OTP.MaxAttempts != 3 {
		t.Errorf("OTP.MaxAttempts: got %d, want 3", cfg.OTP.MaxAttempts)
	}
	if !cfg.OTP.EnableEmail {
		t.Error("OTP.EnableEmail: got false, want true")
	}
	if cfg.OTP.EnableSMS {
		t.Error("OTP.EnableSMS: got true, want false")
	}
}

func TestValidate_DisabledSkipsValidation(t *testing.T) {
	t.Parallel()

	// Even with invalid fields, disabled 2FA should not error.
	cfg := twofactorconfig.Config{
		Enabled: false,
		OTP:     twofactorconfig.OTPConfig{CodeLength: 0, TTLSeconds: 0, MaxAttempts: 0},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected nil for disabled 2FA, got: %v", err)
	}
}

func TestValidate_HappyPath(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled:    true,
		TOTPIssuer: "MyApp",
		OTP: twofactorconfig.OTPConfig{
			CodeLength:  6,
			TTLSeconds:  300,
			MaxAttempts: 3,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_MissingTOTPIssuer(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled: true,
		OTP:     twofactorconfig.OTPConfig{CodeLength: 6, TTLSeconds: 300, MaxAttempts: 3},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing TOTPIssuer, got nil")
	}
}

func TestValidate_CodeLengthTooShort(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled:    true,
		TOTPIssuer: "App",
		OTP:        twofactorconfig.OTPConfig{CodeLength: 3, TTLSeconds: 300, MaxAttempts: 3},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for CodeLength=3, got nil")
	}
}

func TestValidate_CodeLengthTooLong(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled:    true,
		TOTPIssuer: "App",
		OTP:        twofactorconfig.OTPConfig{CodeLength: 11, TTLSeconds: 300, MaxAttempts: 3},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for CodeLength=11, got nil")
	}
}

func TestValidate_TTLTooShort(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled:    true,
		TOTPIssuer: "App",
		OTP:        twofactorconfig.OTPConfig{CodeLength: 6, TTLSeconds: 30, MaxAttempts: 3},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for TTLSeconds=30, got nil")
	}
}

func TestValidate_TTLTooLong(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled:    true,
		TOTPIssuer: "App",
		OTP:        twofactorconfig.OTPConfig{CodeLength: 6, TTLSeconds: 901, MaxAttempts: 3},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for TTLSeconds=901, got nil")
	}
}

func TestValidate_MaxAttemptsZero(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled:    true,
		TOTPIssuer: "App",
		OTP:        twofactorconfig.OTPConfig{CodeLength: 6, TTLSeconds: 300, MaxAttempts: 0},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxAttempts=0, got nil")
	}
}

func TestValidate_MaxAttemptsTooHigh(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{
		Enabled:    true,
		TOTPIssuer: "App",
		OTP:        twofactorconfig.OTPConfig{CodeLength: 6, TTLSeconds: 300, MaxAttempts: 11},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxAttempts=11, got nil")
	}
}

func TestSetDefaults(t *testing.T) {
	t.Parallel()

	cfg := twofactorconfig.Config{} // all zero
	cfg.SetDefaults()

	if cfg.OTP.CodeLength != 6 {
		t.Errorf("OTP.CodeLength: got %d after SetDefaults, want 6", cfg.OTP.CodeLength)
	}
	if cfg.OTP.TTLSeconds != 300 {
		t.Errorf("OTP.TTLSeconds: got %d after SetDefaults, want 300", cfg.OTP.TTLSeconds)
	}
	if cfg.OTP.MaxAttempts != 3 {
		t.Errorf("OTP.MaxAttempts: got %d after SetDefaults, want 3", cfg.OTP.MaxAttempts)
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{
		TwoFactorAuth: configuration.TwoFactorAuthOptions{
			Enabled:        true,
			TOTPIssuer:     "LegacyApp",
			EncryptionKey:  "enc-key-abc",
			OTPCodeLength:  8,
			OTPTTLSeconds:  600,
			OTPMaxAttempts: 5,
		},
		OTPDelivery: configuration.OTPDeliveryOptions{
			EnableEmail: true,
			EnableSMS:   true,
		},
	}

	got := twofactorconfig.FromLegacy(legacy)

	if got.Enabled != legacy.TwoFactorAuth.Enabled {
		t.Errorf("Enabled: got %v, want %v", got.Enabled, legacy.TwoFactorAuth.Enabled)
	}
	if got.TOTPIssuer != legacy.TwoFactorAuth.TOTPIssuer {
		t.Errorf("TOTPIssuer: got %q, want %q", got.TOTPIssuer, legacy.TwoFactorAuth.TOTPIssuer)
	}
	if got.EncryptionKey != legacy.TwoFactorAuth.EncryptionKey {
		t.Errorf("EncryptionKey: got %q, want %q", got.EncryptionKey, legacy.TwoFactorAuth.EncryptionKey)
	}
	if got.OTP.CodeLength != legacy.TwoFactorAuth.OTPCodeLength {
		t.Errorf("OTP.CodeLength: got %d, want %d", got.OTP.CodeLength, legacy.TwoFactorAuth.OTPCodeLength)
	}
	if got.OTP.TTLSeconds != legacy.TwoFactorAuth.OTPTTLSeconds {
		t.Errorf("OTP.TTLSeconds: got %d, want %d", got.OTP.TTLSeconds, legacy.TwoFactorAuth.OTPTTLSeconds)
	}
	if got.OTP.MaxAttempts != legacy.TwoFactorAuth.OTPMaxAttempts {
		t.Errorf("OTP.MaxAttempts: got %d, want %d", got.OTP.MaxAttempts, legacy.TwoFactorAuth.OTPMaxAttempts)
	}
	if got.OTP.EnableEmail != legacy.OTPDelivery.EnableEmail {
		t.Errorf("OTP.EnableEmail: got %v, want %v", got.OTP.EnableEmail, legacy.OTPDelivery.EnableEmail)
	}
	if got.OTP.EnableSMS != legacy.OTPDelivery.EnableSMS {
		t.Errorf("OTP.EnableSMS: got %v, want %v", got.OTP.EnableSMS, legacy.OTPDelivery.EnableSMS)
	}
}
