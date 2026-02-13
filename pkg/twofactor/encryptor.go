package twofactor

import "context"

// SecretEncryptor provides pluggable encryption for TOTP secrets.
//
// TOTP secrets are sensitive cryptographic keys that must be protected at rest.
// This interface allows the application to:
//   - Use different encryption strategies (AES, KMS, HSM, etc.)
//   - Implement key rotation and versioning
//   - Meet compliance requirements (PCI-DSS, HIPAA, etc.)
//   - Use cloud provider key management services (AWS KMS, Google Cloud KMS, etc.)
type SecretEncryptor interface {
	// Encrypt encrypts a plaintext TOTP secret.
	// Returns the encrypted secret and an error if encryption fails.
	Encrypt(ctx context.Context, plaintext string) (string, error)

	// Decrypt decrypts an encrypted TOTP secret.
	// Returns the plaintext secret and an error if decryption fails.
	Decrypt(ctx context.Context, ciphertext string) (string, error)
}

// NoopEncryptor is a no-op implementation that stores secrets in plaintext.
//
// WARNING: This implementation provides NO SECURITY and should ONLY be used for:
//   - Local development and testing
//   - Prototyping and demonstrations
//   - Non-production environments
//
// NEVER use NoopEncryptor in production environments. Always use a proper encryption
// implementation (AES, KMS, etc.) to protect TOTP secrets at rest.
type NoopEncryptor struct{}

// NewNoopEncryptor creates a new NoopEncryptor instance.
func NewNoopEncryptor() *NoopEncryptor {
	return &NoopEncryptor{}
}

// Encrypt returns the plaintext secret unchanged.
func (n *NoopEncryptor) Encrypt(ctx context.Context, plaintext string) (string, error) {
	return plaintext, nil
}

// Decrypt returns the ciphertext unchanged (which is actually plaintext).
func (n *NoopEncryptor) Decrypt(ctx context.Context, ciphertext string) (string, error) {
	return ciphertext, nil
}
