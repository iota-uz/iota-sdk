package twofactor

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AuthMethod represents the primary authentication method used (password, session cookie, etc.).
type AuthMethod string

const (
	// AuthMethodPassword indicates authentication via username/password.
	AuthMethodPassword AuthMethod = "password"

	// AuthMethodSession indicates authentication via existing session cookie.
	AuthMethodSession AuthMethod = "session"

	// AuthMethodAPI indicates authentication via API key or bearer token.
	AuthMethodAPI AuthMethod = "api"

	// AuthMethodOAuth indicates authentication via OAuth provider.
	AuthMethodOAuth AuthMethod = "oauth"
)

// Method represents a two-factor authentication method.
type Method string

const (
	// MethodTOTP represents Time-based One-Time Password (TOTP) authentication.
	MethodTOTP Method = "totp"

	// MethodSMS represents SMS-based OTP authentication.
	MethodSMS Method = "sms"

	// MethodEmail represents email-based OTP authentication.
	MethodEmail Method = "email"

	// MethodBackupCodes represents recovery/backup codes authentication.
	MethodBackupCodes Method = "backup_codes"
)

// AuthAttempt captures the context of an authentication attempt to determine if 2FA is required.
type AuthAttempt struct {
	// UserID is the unique identifier of the user attempting to authenticate.
	UserID uuid.UUID

	// Method is the primary authentication method used (password, session, etc.).
	Method AuthMethod

	// IPAddress is the IP address from which the authentication attempt originated.
	IPAddress string

	// UserAgent is the user agent string of the client making the authentication attempt.
	UserAgent string

	// Timestamp is when the authentication attempt occurred.
	Timestamp time.Time

	// SessionID is the existing session ID if authenticating with a session cookie.
	SessionID *uuid.UUID

	// DeviceFingerprint is an optional device fingerprint for device recognition.
	DeviceFingerprint string
}

// TwoFactorPolicy determines whether two-factor authentication is required for a given authentication attempt.
//
// Implementations can use various signals to make this decision:
//   - User's 2FA enrollment status and method preferences
//   - IP-based geolocation and risk scoring
//   - Device recognition and trust levels
//   - Time-based policies (e.g., require 2FA for logins outside business hours)
//   - Organization-wide security policies
//   - Regulatory compliance requirements
type TwoFactorPolicy interface {
	// Requires determines if 2FA is required for the given authentication attempt.
	// Returns true if 2FA must be enforced, false otherwise.
	// Returns an error if the policy evaluation fails.
	Requires(ctx context.Context, attempt AuthAttempt) (bool, error)
}
