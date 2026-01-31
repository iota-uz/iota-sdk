package twofactor_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/twofactor"
)

// generateTestKey generates a random 32-byte key for testing
func generateTestKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatalf("Failed to generate test key: %v", err)
	}
	return base64.StdEncoding.EncodeToString(key)
}

func TestAESEncryptor_EncryptDecrypt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	encryptionKey := generateTestKey(t)
	encryptor := twofactor.NewAESEncryptor(encryptionKey)

	testCases := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "standard TOTP secret",
			plaintext: "JBSWY3DPEHPK3PXP",
		},
		{
			name:      "longer TOTP secret",
			plaintext: "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
		{
			name:      "unicode characters",
			plaintext: "Hello世界🌍",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Encrypt
			encrypted, err := encryptor.Encrypt(ctx, tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Verify encrypted value is different from plaintext (unless empty)
			if tc.plaintext != "" && encrypted == tc.plaintext {
				t.Error("Encrypted value should differ from plaintext")
			}

			// Decrypt
			decrypted, err := encryptor.Decrypt(ctx, encrypted)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			// Verify round-trip
			if decrypted != tc.plaintext {
				t.Errorf("Decrypted value mismatch: got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestAESEncryptor_DifferentKeysProduceDifferentCiphertexts(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plaintext := "JBSWY3DPEHPK3PXP"

	encryptor1 := twofactor.NewAESEncryptor(generateTestKey(t))
	encryptor2 := twofactor.NewAESEncryptor(generateTestKey(t))

	encrypted1, err := encryptor1.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Encryptor1 Encrypt failed: %v", err)
	}

	encrypted2, err := encryptor2.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Encryptor2 Encrypt failed: %v", err)
	}

	// Different keys should produce different ciphertexts
	if encrypted1 == encrypted2 {
		t.Error("Different encryption keys should produce different ciphertexts")
	}

	// Verify encryptor2 cannot decrypt encryptor1's ciphertext
	_, err = encryptor2.Decrypt(ctx, encrypted1)
	if err == nil {
		t.Error("Decryption with wrong key should fail")
	}
}

func TestAESEncryptor_SameKeyDifferentNonces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	plaintext := "JBSWY3DPEHPK3PXP"
	encryptor := twofactor.NewAESEncryptor(generateTestKey(t))

	// Encrypt same plaintext multiple times
	encrypted1, err := encryptor.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("First Encrypt failed: %v", err)
	}

	encrypted2, err := encryptor.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Second Encrypt failed: %v", err)
	}

	// Same plaintext should produce different ciphertexts (due to different nonces)
	if encrypted1 == encrypted2 {
		t.Error("Same plaintext encrypted twice should produce different ciphertexts (different nonces)")
	}

	// Both should decrypt to the same plaintext
	decrypted1, err := encryptor.Decrypt(ctx, encrypted1)
	if err != nil {
		t.Fatalf("First Decrypt failed: %v", err)
	}

	decrypted2, err := encryptor.Decrypt(ctx, encrypted2)
	if err != nil {
		t.Fatalf("Second Decrypt failed: %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Both ciphertexts should decrypt to the same plaintext")
	}
}

func TestAESEncryptor_InvalidCiphertext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	encryptor := twofactor.NewAESEncryptor(generateTestKey(t))

	testCases := []struct {
		name       string
		ciphertext string
	}{
		{
			name:       "invalid base64",
			ciphertext: "not-valid-base64!@#$",
		},
		{
			name:       "empty string",
			ciphertext: "",
		},
		{
			name:       "too short ciphertext",
			ciphertext: "YWJj", // "abc" in base64, too short for nonce
		},
		{
			name:       "corrupted ciphertext",
			ciphertext: "AAAAAAAAAAAAAAAABBBBBBBBBBBBBBBB", // Valid base64 but corrupted data
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := encryptor.Decrypt(ctx, tc.ciphertext)
			if err == nil {
				t.Error("Decrypt should fail for invalid ciphertext")
			}
		})
	}
}

func TestAESEncryptor_TamperedCiphertext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	encryptor := twofactor.NewAESEncryptor(generateTestKey(t))
	plaintext := "JBSWY3DPEHPK3PXP"

	// Encrypt
	encrypted, err := encryptor.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with the ciphertext (flip a bit)
	tamperedEncrypted := encrypted[:len(encrypted)-1] + "X"

	// Attempt to decrypt tampered ciphertext
	_, err = encryptor.Decrypt(ctx, tamperedEncrypted)
	if err == nil {
		t.Error("Decrypt should fail for tampered ciphertext (GCM authentication should fail)")
	}
}

func TestAESEncryptor_KeyDerivation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Test that different key strings produce different derived keys
	testCases := []struct {
		name string
	}{
		{name: "case1"},
		{name: "case2"},
	}

	plaintext := "JBSWY3DPEHPK3PXP"

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encryptor1 := twofactor.NewAESEncryptor(generateTestKey(t))
			encryptor2 := twofactor.NewAESEncryptor(generateTestKey(t))

			encrypted1, err := encryptor1.Encrypt(ctx, plaintext)
			if err != nil {
				t.Fatalf("Encryptor1 Encrypt failed: %v", err)
			}

			encrypted2, err := encryptor2.Encrypt(ctx, plaintext)
			if err != nil {
				t.Fatalf("Encryptor2 Encrypt failed: %v", err)
			}

			// Different keys should produce different ciphertexts
			if encrypted1 == encrypted2 {
				t.Error("Different key strings should produce different ciphertexts")
			}

			// Cross-decryption should fail
			_, err = encryptor2.Decrypt(ctx, encrypted1)
			if err == nil {
				t.Error("Cross-decryption should fail with different keys")
			}
		})
	}
}

func TestAESEncryptor_ConsistentKeyDerivation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	keyString := generateTestKey(t)
	plaintext := "JBSWY3DPEHPK3PXP"

	// Create two encryptors with the same key string
	encryptor1 := twofactor.NewAESEncryptor(keyString)
	encryptor2 := twofactor.NewAESEncryptor(keyString)

	// Encrypt with first encryptor
	encrypted, err := encryptor1.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Decrypt with second encryptor (same key string, different instance)
	decrypted, err := encryptor2.Decrypt(ctx, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Key derivation should be consistent across instances: got %q, want %q", decrypted, plaintext)
	}
}

func TestAESEncryptor_Base64Encoding(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	encryptor := twofactor.NewAESEncryptor(generateTestKey(t))
	plaintext := "JBSWY3DPEHPK3PXP"

	encrypted, err := encryptor.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Verify encrypted output is valid base64
	// Base64 alphabet: A-Z, a-z, 0-9, +, /, = (padding)
	for _, char := range encrypted {
		if !isBase64Char(char) {
			t.Errorf("Encrypted output contains non-base64 character: %c", char)
		}
	}
}

func isBase64Char(c rune) bool {
	return (c >= 'A' && c <= 'Z') ||
		(c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') ||
		c == '+' || c == '/' || c == '='
}

func TestAESEncryptor_LongPlaintext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	encryptor := twofactor.NewAESEncryptor(generateTestKey(t))

	// Generate a long plaintext (1KB)
	plaintext := strings.Repeat("ABCDEFGHIJKLMNOP", 64) // 1024 bytes

	encrypted, err := encryptor.Encrypt(ctx, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed for long plaintext: %v", err)
	}

	decrypted, err := encryptor.Decrypt(ctx, encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed for long plaintext: %v", err)
	}

	if decrypted != plaintext {
		t.Error("Long plaintext round-trip failed")
	}
}
