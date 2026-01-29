package twofactor

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
)

const (
	// defaultTOTPSkew is the default time skew in steps (1 step = 30 seconds)
	defaultTOTPSkew = uint(1)
	// defaultQRCodeSize is the default QR code image size in pixels
	defaultQRCodeSize = 256
)

// TOTPService handles TOTP (Time-based One-Time Password) operations (internal helper)
type TOTPService struct {
	encryptor   pkgtf.SecretEncryptor
	issuer      string
	skew        uint
	qrCodeSize  int
}

// NewTOTPService creates a new TOTPService
func NewTOTPService(
	encryptor pkgtf.SecretEncryptor,
	issuer string,
	skew uint,
	qrCodeSize int,
) *TOTPService {
	if skew == 0 {
		skew = defaultTOTPSkew
	}
	if qrCodeSize <= 0 {
		qrCodeSize = defaultQRCodeSize
	}
	if issuer == "" {
		issuer = "IOTA"
	}

	return &TOTPService{
		encryptor:  encryptor,
		issuer:     issuer,
		skew:       skew,
		qrCodeSize: qrCodeSize,
	}
}

// GenerateSecret generates a new TOTP secret key
func (s *TOTPService) GenerateSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: "", // Will be set per user
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate TOTP key: %w", err)
	}
	return key.Secret(), nil
}

// GenerateQRCodeURL generates an otpauth:// URL for TOTP
func (s *TOTPService) GenerateQRCodeURL(accountName, secret string) (string, error) {
	if accountName == "" {
		return "", fmt.Errorf("account name cannot be empty")
	}
	if secret == "" {
		return "", pkgtf.ErrInvalidSecret
	}

	// Build otpauth URL
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", s.issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", "6")
	params.Set("period", "30")

	otpauthURL := fmt.Sprintf(
		"otpauth://totp/%s:%s?%s",
		url.PathEscape(s.issuer),
		url.PathEscape(accountName),
		params.Encode(),
	)

	return otpauthURL, nil
}

// GenerateQRCodePNG generates a QR code image as a base64-encoded PNG
func (s *TOTPService) GenerateQRCodePNG(accountName, secret string, size int) (string, error) {
	if size <= 0 {
		size = s.qrCodeSize
	}

	// Generate otpauth URL
	otpauthURL, err := s.GenerateQRCodeURL(accountName, secret)
	if err != nil {
		return "", fmt.Errorf("failed to generate URL: %w", err)
	}

	// Generate QR code
	qrBytes, err := qrcode.Encode(otpauthURL, qrcode.Medium, size)
	if err != nil {
		return "", fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(qrBytes)
	return encoded, nil
}

// Validate validates a TOTP code against a secret
func (s *TOTPService) Validate(secret, code string) bool {
	return totp.Validate(code, secret)
}

// ValidateWithSkew validates a TOTP code with time tolerance
func (s *TOTPService) ValidateWithSkew(secret, code string, skew uint) (bool, error) {
	if secret == "" {
		return false, pkgtf.ErrInvalidSecret
	}
	if code == "" {
		return false, pkgtf.ErrInvalidCode
	}

	// Use configured skew if not specified
	if skew == 0 {
		skew = s.skew
	}

	valid, err := totp.ValidateCustom(
		code,
		secret,
		time.Now(),
		totp.ValidateOpts{
			Skew: skew,
		},
	)
	if err != nil {
		return false, fmt.Errorf("validation error: %w", err)
	}

	return valid, nil
}

// EncryptSecret encrypts a TOTP secret using the configured encryptor
func (s *TOTPService) EncryptSecret(ctx context.Context, secret string) (string, error) {
	if s.encryptor == nil {
		// No encryption configured, store plaintext (not recommended for production)
		return secret, nil
	}

	encrypted, err := s.encryptor.Encrypt(ctx, secret)
	if err != nil {
		return "", fmt.Errorf("%w: %w", pkgtf.ErrEncryptionFailed, err)
	}

	return encrypted, nil
}

// DecryptSecret decrypts a TOTP secret using the configured encryptor
func (s *TOTPService) DecryptSecret(ctx context.Context, encrypted string) (string, error) {
	if s.encryptor == nil {
		// No encryption configured, return as-is
		return encrypted, nil
	}

	plaintext, err := s.encryptor.Decrypt(ctx, encrypted)
	if err != nil {
		return "", fmt.Errorf("%w: %w", pkgtf.ErrDecryptionFailed, err)
	}

	return plaintext, nil
}
