package twofactor

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// AESEncryptor encrypts TOTP secrets using AES-256-GCM.
//
// AES-256-GCM provides authenticated encryption with associated data (AEAD).
// This means it provides both confidentiality (encryption) and authenticity
// (tamper detection) for TOTP secrets at rest.
//
// Key Derivation:
//   - The encryption key string is hashed with SHA-256 to derive a 32-byte key
//   - This ensures consistent key length regardless of input string length
//
// Encryption Process:
//   - Uses AES-256-GCM (Galois/Counter Mode) for authenticated encryption
//   - Generates a random 12-byte nonce for each encryption operation
//   - Prepends nonce to ciphertext for decryption
//   - Returns base64-encoded result for safe storage
//
// Security Considerations:
//   - TOTP_ENCRYPTION_KEY must be kept secret and rotated periodically
//   - Use a strong, random encryption key (at least 32 bytes of randomness)
//   - Consider key rotation strategies for compliance requirements
//   - For production, consider using a KMS (AWS KMS, Google Cloud KMS, etc.)
type AESEncryptor struct {
	key []byte
}

// NewAESEncryptor creates a new AES-256-GCM encryptor.
//
// The provided encryption key string is hashed with SHA-256 to derive a
// consistent 32-byte key suitable for AES-256.
//
// Parameters:
//   - encryptionKey: A secret key string used to encrypt TOTP secrets.
//     Should be a strong, random value stored in environment variables.
//
// Example:
//
//	encryptor := twofactor.NewAESEncryptor(os.Getenv("TOTP_ENCRYPTION_KEY"))
func NewAESEncryptor(encryptionKey string) *AESEncryptor {
	// Derive a 32-byte key from the encryption key string using SHA-256
	hash := sha256.Sum256([]byte(encryptionKey))
	return &AESEncryptor{
		key: hash[:],
	}
}

// Encrypt encrypts a plaintext TOTP secret using AES-256-GCM.
//
// The encryption process:
//  1. Create AES cipher with 32-byte key (AES-256)
//  2. Wrap cipher with GCM for authenticated encryption
//  3. Generate random 12-byte nonce (GCM standard nonce size)
//  4. Encrypt plaintext with nonce
//  5. Prepend nonce to ciphertext (needed for decryption)
//  6. Encode result as base64 for safe storage
//
// Returns the encrypted secret as a base64-encoded string, or an error if encryption fails.
func (e *AESEncryptor) Encrypt(ctx context.Context, plaintext string) (string, error) {
	const op serrors.Op = "AESEncryptor.Encrypt"

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", serrors.E(op, err)
	}

	// Encrypt plaintext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode as base64 for safe storage
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts an encrypted TOTP secret using AES-256-GCM.
//
// The decryption process:
//  1. Decode base64-encoded ciphertext
//  2. Extract nonce from beginning of ciphertext
//  3. Create AES cipher with 32-byte key
//  4. Wrap cipher with GCM for authenticated decryption
//  5. Decrypt and authenticate ciphertext
//
// Returns the decrypted plaintext secret, or an error if decryption fails.
// Errors can occur due to:
//   - Invalid base64 encoding
//   - Tampering (GCM authentication failure)
//   - Wrong encryption key
//   - Corrupted ciphertext
func (e *AESEncryptor) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	const op serrors.Op = "AESEncryptor.Decrypt"

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", serrors.E(op, err)
	}

	// Check minimum length (nonce + at least 1 byte of ciphertext)
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", serrors.E(op, serrors.Invalid, fmt.Errorf("ciphertext too short"))
	}

	// Extract nonce and ciphertext
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt and authenticate
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", serrors.E(op, err)
	}

	return string(plaintext), nil
}
