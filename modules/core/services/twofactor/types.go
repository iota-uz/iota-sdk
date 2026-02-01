package twofactor

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// SetupChallenge represents the result of initiating a 2FA setup flow
type SetupChallenge struct {
	// ChallengeID is a unique identifier for this setup attempt
	ChallengeID string
	// Method is the 2FA method being set up
	Method twofactor.Method
	// QRCodeURL is the otpauth:// URL for TOTP method (empty for OTP methods)
	QRCodeURL string
	// QRCodePNG is the base64-encoded PNG image of the QR code (empty for OTP methods)
	QRCodePNG string
	// ExpiresAt is when this challenge expires
	ExpiresAt time.Time
	// Destination is the phone/email where OTP was sent (empty for TOTP)
	Destination string
}

// SetupResult represents the successful completion of 2FA setup
type SetupResult struct {
	// Method is the 2FA method that was enabled
	Method twofactor.Method
	// EnabledAt is when 2FA was enabled
	EnabledAt time.Time
	// RecoveryCodes is the list of recovery codes generated (plain text, show once)
	RecoveryCodes []string
}

// VerifyChallenge represents the result of initiating a 2FA verification flow
type VerifyChallenge struct {
	// ChallengeID is a unique identifier for this verification attempt
	ChallengeID string
	// Method is the 2FA method being used
	Method twofactor.Method
	// ExpiresAt is when this challenge expires (for OTP methods)
	ExpiresAt *time.Time
	// Destination is the phone/email where OTP was sent (empty for TOTP)
	Destination string
}

// Status represents the current 2FA status for a user
type Status struct {
	// Enabled indicates if 2FA is currently enabled
	Enabled bool
	// Method is the current 2FA method (empty if not enabled)
	Method twofactor.Method
	// EnabledAt is when 2FA was enabled (zero if not enabled)
	EnabledAt time.Time
	// RemainingRecoveryCodes is the count of unused recovery codes
	RemainingRecoveryCodes int
}
