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
	"github.com/pquerna/otp"
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

// GenerateSecret generates a new TOTP secret key.
// The secret is a base32-encoded string that can be used to generate time-based OTP codes.
// Returns the secret string and an error if generation fails.
func (s *TOTPService) GenerateSecret() (string, error) {
	const op serrors.Op = "TOTPService.GenerateSecret"

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.issuer,
		AccountName: "placeholder", // Placeholder - actual account name set in QR URL generation
	})
	if err != nil {
		return "", serrors.E(op, err)
	}
	return key.Secret(), nil
}

// GenerateQRCodeURL generates an otpauth:// URL for TOTP.
// The URL can be used to generate a QR code that users scan with authenticator apps (Google Authenticator, Authy, etc.).
// Parameters:
//   - accountName: The user's identifier (typically email address)
//   - secret: The TOTP secret key from GenerateSecret()
//
// Returns the otpauth:// URL string and an error if generation fails.
func (s *TOTPService) GenerateQRCodeURL(accountName, secret string) (string, error) {
	const op serrors.Op = "TOTPService.GenerateQRCodeURL"

	if accountName == "" {
		return "", serrors.E(op, serrors.Invalid, errors.New("account name cannot be empty"))
	}
	if secret == "" {
		return "", serrors.E(op, pkgtf.ErrInvalidSecret)
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

// GenerateQRCodePNG generates a QR code image as a base64-encoded PNG.
// The generated QR code can be displayed in web pages via data URL (data:image/png;base64,...).
// Parameters:
//   - accountName: The user's identifier (typically email address)
//   - secret: The TOTP secret key from GenerateSecret()
//   - size: The desired QR code size in pixels (uses default if <= 0)
//
// Returns the base64-encoded PNG string and an error if generation fails.
func (s *TOTPService) GenerateQRCodePNG(accountName, secret string, size int) (string, error) {
	const op serrors.Op = "TOTPService.GenerateQRCodePNG"

	if size <= 0 {
		size = s.qrCodeSize
	}

	// Generate otpauth URL
	otpauthURL, err := s.GenerateQRCodeURL(accountName, secret)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Generate QR code using yeqown/go-qrcode (actively maintained replacement for skip2/go-qrcode)
	qrc, err := qrcode.NewWith(otpauthURL,
		qrcode.WithEncodingMode(qrcode.EncModeByte),
		qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium),
	)
	if err != nil {
		return "", serrors.E(op, err)
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
		return "", serrors.E(op, err)
	}

	// Encode to base64
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return encoded, nil
}

// Validate validates a TOTP code against a secret.
// Uses the current time window without time skew tolerance.
// For production use, prefer ValidateWithSkew() to handle clock drift.
// Parameters:
//   - secret: The TOTP secret key
//   - code: The 6-digit code entered by the user
//
// Returns true if the code is valid, false otherwise.
func (s *TOTPService) Validate(secret, code string) bool {
	return totp.Validate(code, secret)
}

// ValidateWithSkew validates a TOTP code with time tolerance.
// Allows validation within a time window to handle clock drift between server and client.
// Each skew step represents 30 seconds (1 skew = ±30s, 2 skew = ±60s, etc.).
// Parameters:
//   - secret: The TOTP secret key
//   - code: The 6-digit code entered by the user
//   - skew: Number of time steps to accept (0 uses configured default)
//
// Returns true if the code is valid, false otherwise, or an error if validation fails.
func (s *TOTPService) ValidateWithSkew(secret, code string, skew uint) (bool, error) {
	const op serrors.Op = "TOTPService.ValidateWithSkew"

	if secret == "" {
		return false, serrors.E(op, pkgtf.ErrInvalidSecret)
	}
	if code == "" {
		return false, serrors.E(op, pkgtf.ErrInvalidCode)
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
			Period:    30,
			Skew:      skew,
			Digits:    otp.DigitsSix,
			Algorithm: otp.AlgorithmSHA1,
		},
	)
	if err != nil {
		return false, serrors.E(op, err)
	}

	return valid, nil
}

// EncryptSecret encrypts a TOTP secret using the configured encryptor.
// CRITICAL: Encryption is required for OWASP compliance - TOTP secrets must never be stored in plaintext.
// The encrypted secret can be safely stored in the database.
// Parameters:
//   - ctx: Request context for cancellation and tracing
//   - secret: The plaintext TOTP secret to encrypt
//
// Returns the encrypted secret string and an error if encryption fails.
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

// DecryptSecret decrypts a TOTP secret using the configured encryptor.
// Used to retrieve the plaintext secret from storage for verification operations.
// Parameters:
//   - ctx: Request context for cancellation and tracing
//   - encrypted: The encrypted TOTP secret from database
//
// Returns the plaintext secret string and an error if decryption fails.
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
