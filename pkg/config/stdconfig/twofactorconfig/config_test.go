package twofactorconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/twofactorconfig"
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

func TestDefaults_OTPFields(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[twofactorconfig.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
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
}
