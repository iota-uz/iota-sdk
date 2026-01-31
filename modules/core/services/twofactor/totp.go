package twofactor

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/pquerna/otp/totp"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

const (
	// defaultTOTPSkew is the default time skew in steps (1 step = 30 seconds)
	defaultTOTPSkew = uint(1)
	// defaultQRCodeSize is the default QR code image size in pixels
	defaultQRCodeSize = 256
)

// nopCloser wraps a writer to implement io.WriteCloser
type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// TOTPService handles TOTP (Time-based One-Time Password) operations (internal helper)
type TOTPService struct {
	encryptor  pkgtf.SecretEncryptor
	issuer     string
	skew       uint
	qrCodeSize int
}

// NewTOTPService creates a new TOTPService
// Returns an error if the encryptor is nil (encryption is mandatory for OWASP compliance)
func NewTOTPService(
	encryptor pkgtf.SecretEncryptor,
	issuer string,
	skew uint,
	qrCodeSize int,
) (*TOTPService, error) {
	const op serrors.Op = "NewTOTPService"

	// CRITICAL: Encryptor is required for OWASP compliance - TOTP secrets must never be stored in plaintext
	if encryptor == nil {
		return nil, serrors.E(op, serrors.Invalid, errors.New("encryptor is required for TOTP secret encryption"))
	}

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
	}, nil
}

// GenerateSecret generates a new TOTP secret key
func (s *TOTPService) GenerateSecret() (string, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: "placeholder", // Placeholder - actual account name set in QR URL generation
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

	// Generate QR code using yeqown/go-qrcode (actively maintained replacement for skip2/go-qrcode)
	qrc, err := qrcode.NewWith(otpauthURL,
		qrcode.WithEncodingMode(qrcode.EncModeByte),
		qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create QR code: %w", err)
	}

	// Write QR code to buffer as PNG
	buf := new(bytes.Buffer)
	// Calculate module width to approximate requested size (size is total pixels, divide by typical module count ~25-30)
	moduleWidth := uint8(size / 25)
	if moduleWidth < 4 {
		moduleWidth = 4 // Minimum readable size
	}
	// Wrap buffer with nopCloser to satisfy io.WriteCloser interface
	writer := standard.NewWithWriter(nopCloser{buf}, standard.WithQRWidth(moduleWidth))
	if err := qrc.Save(writer); err != nil {
		return "", fmt.Errorf("failed to generate QR code PNG: %w", err)
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
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
	const op serrors.Op = "TOTPService.EncryptSecret"

	// CRITICAL: Fail fast - encryption is required for OWASP compliance
	if s.encryptor == nil {
		return "", serrors.E(op, serrors.Invalid, errors.New("encryptor is required for TOTP secret encryption"))
	}

	encrypted, err := s.encryptor.Encrypt(ctx, secret)
	if err != nil {
		return "", serrors.E(op, pkgtf.ErrEncryptionFailed, err)
	}

	return encrypted, nil
}

// DecryptSecret decrypts a TOTP secret using the configured encryptor
func (s *TOTPService) DecryptSecret(ctx context.Context, encrypted string) (string, error) {
	const op serrors.Op = "TOTPService.DecryptSecret"

	// CRITICAL: Fail fast - encryption is required for OWASP compliance
	if s.encryptor == nil {
		return "", serrors.E(op, serrors.Invalid, errors.New("encryptor is required for TOTP secret encryption"))
	}

	plaintext, err := s.encryptor.Decrypt(ctx, encrypted)
	if err != nil {
		return "", serrors.E(op, pkgtf.ErrDecryptionFailed, err)
	}

	return plaintext, nil
}
