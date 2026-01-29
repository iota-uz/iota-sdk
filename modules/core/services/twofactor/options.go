package twofactor

import (
	"time"

	"github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// ServiceOption is a functional option for configuring TwoFactorService
type ServiceOption func(*TwoFactorService)

// WithIssuer sets the issuer name for TOTP QR codes (e.g., "MyApp")
func WithIssuer(issuer string) ServiceOption {
	return func(s *TwoFactorService) {
		s.issuer = issuer
	}
}

// WithOTPLength sets the length of generated OTP codes (default: 6)
func WithOTPLength(length int) ServiceOption {
	return func(s *TwoFactorService) {
		s.otpLength = length
	}
}

// WithOTPExpiry sets the expiration duration for OTP codes (default: 10 minutes)
func WithOTPExpiry(duration time.Duration) ServiceOption {
	return func(s *TwoFactorService) {
		s.otpExpiry = duration
	}
}

// WithOTPMaxAttempts sets the maximum number of OTP verification attempts (default: 3)
func WithOTPMaxAttempts(attempts int) ServiceOption {
	return func(s *TwoFactorService) {
		s.otpMaxAttempts = attempts
	}
}

// WithTOTPSkew sets the time skew for TOTP validation in steps (default: 1 = ±30 seconds)
func WithTOTPSkew(skew uint) ServiceOption {
	return func(s *TwoFactorService) {
		s.totpSkew = skew
	}
}

// WithRecoveryCodeCount sets the number of recovery codes to generate (default: 8)
func WithRecoveryCodeCount(count int) ServiceOption {
	return func(s *TwoFactorService) {
		s.recoveryCodeCount = count
	}
}

// WithSetupExpiry sets the expiration duration for setup challenges (default: 15 minutes)
func WithSetupExpiry(duration time.Duration) ServiceOption {
	return func(s *TwoFactorService) {
		s.setupExpiry = duration
	}
}

// WithQRCodeSize sets the size of generated QR code images in pixels (default: 256)
func WithQRCodeSize(size int) ServiceOption {
	return func(s *TwoFactorService) {
		s.qrCodeSize = size
	}
}

// WithOTPSender sets the OTP sender for sending OTP codes
func WithOTPSender(sender twofactor.OTPSender) ServiceOption {
	return func(s *TwoFactorService) {
		s.otpSender = sender
	}
}

// WithSecretEncryptor sets the encryptor for TOTP secrets
func WithSecretEncryptor(encryptor twofactor.SecretEncryptor) ServiceOption {
	return func(s *TwoFactorService) {
		s.encryptor = encryptor
	}
}
