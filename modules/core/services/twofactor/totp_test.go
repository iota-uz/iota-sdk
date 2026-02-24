package twofactor

import (
	"context"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	pkgtf "github.com/iota-uz/iota-sdk/pkg/twofactor"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewTOTPService_RequiresEncryptor verifies that NewTOTPService rejects nil encryptor
func TestNewTOTPService_RequiresEncryptor(t *testing.T) {
	t.Parallel()

	// Attempt to create TOTPService with nil encryptor (MUST fail for OWASP compliance)
	svc, err := NewTOTPService(
		nil, // nil encryptor - CRITICAL security issue
		"IOTA",
		1,
		256,
	)

	require.Error(t, err, "NewTOTPService must reject nil encryptor")
	assert.Nil(t, svc, "TOTPService should be nil when encryptor is nil")
	assert.Contains(t, err.Error(), "encryptor is required", "Error message should explain requirement")
}

// TestNewTOTPService_ValidEncryptor verifies successful creation with valid encryptor
func TestNewTOTPService_ValidEncryptor(t *testing.T) {
	t.Parallel()

	encryptor := pkgtf.NewNoopEncryptor()

	svc, err := NewTOTPService(encryptor, "IOTA", 1, 256)

	require.NoError(t, err, "NewTOTPService should succeed with valid encryptor")
	require.NotNil(t, svc, "TOTPService should be created")
	assert.Equal(t, "IOTA", svc.issuer)
	assert.Equal(t, uint(1), svc.skew)
	assert.Equal(t, 256, svc.qrCodeSize)
}

// TestNewTOTPService_DefaultValues verifies default value application
func TestNewTOTPService_DefaultValues(t *testing.T) {
	t.Parallel()

	encryptor := pkgtf.NewNoopEncryptor()

	svc, err := NewTOTPService(
		encryptor,
		"", // Empty issuer - should default to "IOTA"
		0,  // Zero skew - should default to 1
		0,  // Zero size - should default to 256
	)

	require.NoError(t, err)
	require.NotNil(t, svc)
	assert.Equal(t, "IOTA", svc.issuer, "Empty issuer should default to IOTA")
	assert.Equal(t, defaultTOTPSkew, svc.skew, "Zero skew should default")
	assert.Equal(t, defaultQRCodeSize, svc.qrCodeSize, "Zero size should default")
}

// TestEncryptSecret_RequiresEncryptor verifies EncryptSecret fails fast with nil encryptor
func TestEncryptSecret_RequiresEncryptor(t *testing.T) {
	t.Parallel()

	// Create service with bypassed validation (for testing error path)
	svc := &TOTPService{
		encryptor:  nil, // Simulate nil encryptor
		issuer:     "IOTA",
		skew:       1,
		qrCodeSize: 256,
	}

	ctx := context.Background()
	secret := "JBSWY3DPEHPK3PXP"

	// Attempt to encrypt (MUST fail - no plaintext storage allowed)
	encrypted, err := svc.EncryptSecret(ctx, secret)

	require.Error(t, err, "EncryptSecret must fail when encryptor is nil")
	assert.Empty(t, encrypted, "Encrypted value should be empty on error")
	assert.Contains(t, err.Error(), "encryptor is required", "Error should explain requirement")

	// Verify serrors.Op is present
	var serrOp serrors.Op
	if serr, ok := err.(*serrors.Error); ok {
		serrOp = serr.Op
	}
	assert.Equal(t, serrors.Op("TOTPService.EncryptSecret"), serrOp, "Operation should be tracked")
}

// TestDecryptSecret_RequiresEncryptor verifies DecryptSecret fails fast with nil encryptor
func TestDecryptSecret_RequiresEncryptor(t *testing.T) {
	t.Parallel()

	// Create service with bypassed validation (for testing error path)
	svc := &TOTPService{
		encryptor:  nil, // Simulate nil encryptor
		issuer:     "IOTA",
		skew:       1,
		qrCodeSize: 256,
	}

	ctx := context.Background()
	encrypted := "fake-encrypted-value"

	// Attempt to decrypt (MUST fail - no plaintext storage allowed)
	plaintext, err := svc.DecryptSecret(ctx, encrypted)

	require.Error(t, err, "DecryptSecret must fail when encryptor is nil")
	assert.Empty(t, plaintext, "Plaintext should be empty on error")
	assert.Contains(t, err.Error(), "encryptor is required", "Error should explain requirement")

	// Verify serrors.Op is present
	var serrOp serrors.Op
	if serr, ok := err.(*serrors.Error); ok {
		serrOp = serr.Op
	}
	assert.Equal(t, serrors.Op("TOTPService.DecryptSecret"), serrOp, "Operation should be tracked")
}

// TestEncryptDecryptSecret_RoundTrip verifies encryption/decryption cycle
func TestEncryptDecryptSecret_RoundTrip(t *testing.T) {
	t.Parallel()

	encryptor := pkgtf.NewNoopEncryptor()
	svc, err := NewTOTPService(encryptor, "IOTA", 1, 256)
	require.NoError(t, err)

	ctx := context.Background()
	originalSecret := "JBSWY3DPEHPK3PXP"

	// Encrypt
	encrypted, err := svc.EncryptSecret(ctx, originalSecret)
	require.NoError(t, err, "Encryption should succeed")
	assert.NotEmpty(t, encrypted, "Encrypted value should not be empty")

	// Decrypt
	decrypted, err := svc.DecryptSecret(ctx, encrypted)
	require.NoError(t, err, "Decryption should succeed")
	assert.Equal(t, originalSecret, decrypted, "Decrypted value should match original")
}

// TestGenerateSecret verifies TOTP secret generation
// Note: GenerateSecret() is a low-level helper that doesn't set AccountName
// In practice, this is called during BeginSetup which handles AccountName separately
func TestGenerateSecret(t *testing.T) {
	t.Parallel()

	encryptor := pkgtf.NewNoopEncryptor()
	svc, err := NewTOTPService(encryptor, "IOTA", 1, 256)
	require.NoError(t, err)

	// GenerateSecret generates the raw secret but doesn't create a full TOTP key
	// The AccountName is set later in the QR URL generation phase
	secret, err := svc.GenerateSecret()
	require.NoError(t, err, "Secret generation should succeed")
	assert.NotEmpty(t, secret, "Generated secret should not be empty")
	assert.GreaterOrEqual(t, len(secret), 16, "Secret should be at least 16 characters")
}

// TestGenerateQRCodeURL verifies QR code URL generation
func TestGenerateQRCodeURL(t *testing.T) {
	t.Parallel()

	encryptor := pkgtf.NewNoopEncryptor()
	svc, err := NewTOTPService(encryptor, "IOTA", 1, 256)
	require.NoError(t, err)

	accountName := "user@example.com"
	secret := "JBSWY3DPEHPK3PXP"

	url, err := svc.GenerateQRCodeURL(accountName, secret)
	require.NoError(t, err, "QR URL generation should succeed")
	assert.Contains(t, url, "otpauth://totp/", "URL should start with otpauth://totp/")
	assert.Contains(t, url, accountName, "URL should contain account name")
	assert.Contains(t, url, "secret="+secret, "URL should contain secret")
	assert.Contains(t, url, "issuer=IOTA", "URL should contain issuer")
}

// TestGenerateQRCodeURL_Validation verifies input validation
func TestGenerateQRCodeURL_Validation(t *testing.T) {
	t.Parallel()

	encryptor := pkgtf.NewNoopEncryptor()
	svc, err := NewTOTPService(encryptor, "IOTA", 1, 256)
	require.NoError(t, err)

	tests := []struct {
		name        string
		accountName string
		secret      string
		wantErr     bool
		errContains string
	}{
		{
			name:        "Empty account name",
			accountName: "",
			secret:      "JBSWY3DPEHPK3PXP",
			wantErr:     true,
			errContains: "account name cannot be empty",
		},
		{
			name:        "Empty secret",
			accountName: "user@example.com",
			secret:      "",
			wantErr:     true,
			errContains: "invalid TOTP secret",
		},
		{
			name:        "Valid inputs",
			accountName: "user@example.com",
			secret:      "JBSWY3DPEHPK3PXP",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := svc.GenerateQRCodeURL(tt.accountName, tt.secret)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Empty(t, url)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, url)
			}
		})
	}
}

// TestValidateWithSkew verifies TOTP code validation
func TestValidateWithSkew(t *testing.T) {
	t.Parallel()

	encryptor := pkgtf.NewNoopEncryptor()
	svc, err := NewTOTPService(encryptor, "IOTA", 1, 256)
	require.NoError(t, err)

	secret := "JBSWY3DPEHPK3PXP"
	validCode, err := totp.GenerateCodeCustom(secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	require.NoError(t, err)

	valid, err := svc.ValidateWithSkew(secret, validCode, 1)
	require.NoError(t, err)
	assert.True(t, valid)

	tests := []struct {
		name    string
		secret  string
		code    string
		wantErr bool
	}{
		{
			name:    "Empty secret",
			secret:  "",
			code:    "123456",
			wantErr: true,
		},
		{
			name:    "Empty code",
			secret:  secret,
			code:    "",
			wantErr: true,
		},
		{
			name:    "Invalid code format",
			secret:  secret,
			code:    "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := svc.ValidateWithSkew(tt.secret, tt.code, 1)

			if tt.wantErr {
				require.Error(t, err)
				assert.False(t, valid)
			}
		})
	}
}
