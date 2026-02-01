package twofactor

import "errors"

// Domain errors for two-factor authentication operations.
var (
	// ErrInvalidCode indicates the provided OTP code does not match the expected value.
	ErrInvalidCode = errors.New("invalid verification code")

	// ErrExpiredCode indicates the OTP code has exceeded its validity period.
	ErrExpiredCode = errors.New("verification code has expired")

	// ErrTooManyAttempts indicates the maximum number of verification attempts has been exceeded.
	ErrTooManyAttempts = errors.New("too many verification attempts")

	// ErrInvalidSecret indicates the TOTP secret key is malformed or invalid.
	ErrInvalidSecret = errors.New("invalid TOTP secret")

	// ErrChannelUnavailable indicates the requested OTP delivery channel is not available or configured.
	ErrChannelUnavailable = errors.New("channel unavailable")

	// ErrSendFailed indicates the OTP message could not be sent through the delivery channel.
	ErrSendFailed = errors.New("failed to send verification code")

	// ErrMethodNotSupported indicates the requested authentication method is not supported for this user or context.
	ErrMethodNotSupported = errors.New("authentication method not supported")

	// ErrEncryptionFailed indicates the secret encryption operation failed.
	ErrEncryptionFailed = errors.New("failed to encrypt secret")

	// ErrDecryptionFailed indicates the secret decryption operation failed.
	ErrDecryptionFailed = errors.New("failed to decrypt secret")
)
