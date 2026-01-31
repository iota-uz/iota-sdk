package twofactor_test

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// TestAESEncryptorIntegration verifies the AES encryptor works correctly
// in an integration scenario simulating production usage.
func TestAESEncryptorIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Simulate production scenario: environment variable with strong key
	productionKey := "production-encryption-key-with-strong-entropy-32bytes!"
	encryptor := twofactor.NewAESEncryptor(productionKey)

	// Simulate TOTP secret storage workflow
	totpSecret := "JBSWY3DPEHPK3PXP"

	// 1. Encrypt secret before storing in database
	encryptedSecret, err := encryptor.Encrypt(ctx, totpSecret)
	if err != nil {
		t.Fatalf("Failed to encrypt TOTP secret: %v", err)
	}

	// Verify encrypted secret is different from plaintext
	if encryptedSecret == totpSecret {
		t.Error("Encrypted secret should differ from plaintext")
	}

	// 2. Simulate storing encrypted secret in database
	// (In real scenario, this would be saved to users table)
	storedEncryptedSecret := encryptedSecret

	// 3. Simulate retrieving encrypted secret from database
	retrievedEncryptedSecret := storedEncryptedSecret

	// 4. Decrypt secret when validating TOTP code
	decryptedSecret, err := encryptor.Decrypt(ctx, retrievedEncryptedSecret)
	if err != nil {
		t.Fatalf("Failed to decrypt TOTP secret: %v", err)
	}

	// 5. Verify decrypted secret matches original
	if decryptedSecret != totpSecret {
		t.Errorf("Decrypted secret mismatch: got %q, want %q", decryptedSecret, totpSecret)
	}
}

// TestNoopEncryptorIntegration verifies the Noop encryptor works correctly
// for development environments where encryption is not required.
func TestNoopEncryptorIntegration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	encryptor := twofactor.NewNoopEncryptor()

	totpSecret := "JBSWY3DPEHPK3PXP"

	// Encrypt (should return plaintext)
	encryptedSecret, err := encryptor.Encrypt(ctx, totpSecret)
	if err != nil {
		t.Fatalf("Noop encryptor should not fail: %v", err)
	}

	if encryptedSecret != totpSecret {
		t.Errorf("Noop encryptor should return plaintext: got %q, want %q", encryptedSecret, totpSecret)
	}

	// Decrypt (should return plaintext)
	decryptedSecret, err := encryptor.Decrypt(ctx, encryptedSecret)
	if err != nil {
		t.Fatalf("Noop decryptor should not fail: %v", err)
	}

	if decryptedSecret != totpSecret {
		t.Errorf("Noop decryptor should return plaintext: got %q, want %q", decryptedSecret, totpSecret)
	}
}

// TestEncryptorSelection verifies the correct encryptor is selected based on configuration
func TestEncryptorSelection(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	testCases := []struct {
		name          string
		encryptionKey string
		expectNoop    bool
	}{
		{
			name:          "production with encryption key",
			encryptionKey: "strong-production-key-with-entropy",
			expectNoop:    false,
		},
		{
			name:          "development without encryption key",
			encryptionKey: "",
			expectNoop:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var encryptor twofactor.SecretEncryptor
			if tc.encryptionKey != "" {
				encryptor = twofactor.NewAESEncryptor(tc.encryptionKey)
			} else {
				encryptor = twofactor.NewNoopEncryptor()
			}

			totpSecret := "JBSWY3DPEHPK3PXP"
			encrypted, err := encryptor.Encrypt(ctx, totpSecret)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			if tc.expectNoop {
				// Noop should return plaintext
				if encrypted != totpSecret {
					t.Error("Expected plaintext for Noop encryptor")
				}
			} else {
				// AES should return ciphertext
				if encrypted == totpSecret {
					t.Error("Expected ciphertext for AES encryptor")
				}
			}

			// Both should decrypt successfully
			decrypted, err := encryptor.Decrypt(ctx, encrypted)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != totpSecret {
				t.Errorf("Decryption mismatch: got %q, want %q", decrypted, totpSecret)
			}
		})
	}
}
