package oidc

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	rsaKeySize = 2048
	aesKeySize = 32 // AES-256

	// SQL queries for key management
	checkActiveKeysQuery  = `SELECT COUNT(*) FROM oidc.signing_keys WHERE is_active = true`
	insertSigningKeyQuery = `
		INSERT INTO oidc.signing_keys (key_id, algorithm, private_key, public_key, is_active)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key_id) DO NOTHING`
	getActiveSigningKeyQuery = `SELECT key_id, private_key FROM oidc.signing_keys
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT 1`
	getPublicKeysQuery = `SELECT public_key FROM oidc.signing_keys
		WHERE is_active = true
		ORDER BY created_at DESC`
	getPublicKeysWithIDQuery = `SELECT key_id, public_key FROM oidc.signing_keys
		WHERE is_active = true
		ORDER BY created_at DESC`
)

// BootstrapKeys generates RS256 keypair if oidc.signing_keys is empty.
// Uses advisory lock to prevent race conditions when multiple processes start simultaneously.
func BootstrapKeys(ctx context.Context, db *pgxpool.Pool, cryptoKey string) error {
	const op serrors.Op = "oidc.BootstrapKeys"

	// Acquire advisory lock to prevent race condition when multiple processes bootstrap concurrently
	// Lock ID 1 is reserved for OIDC key bootstrap
	const advisoryLockID = 1
	_, err := db.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockID)
	if err != nil {
		return serrors.E(op, fmt.Errorf("failed to acquire advisory lock: %w", err))
	}
	defer func() {
		_, _ = db.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockID)
	}()

	// Check if keys already exist (inside the lock)
	var count int
	err = db.QueryRow(ctx, checkActiveKeysQuery).Scan(&count)
	if err != nil {
		return serrors.E(op, err)
	}

	if count > 0 {
		// Keys already exist, no need to bootstrap
		return nil
	}

	// Generate new RSA keypair
	privateKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return serrors.E(op, fmt.Errorf("failed to generate RSA key: %w", err))
	}

	// Marshal private key to PKCS8 format
	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return serrors.E(op, fmt.Errorf("failed to marshal private key: %w", err))
	}

	// PEM encode private key
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encrypt private key with AES-256
	encryptedPrivateKey, err := encryptAES256(privateKeyPEM, cryptoKey)
	if err != nil {
		return serrors.E(op, fmt.Errorf("failed to encrypt private key: %w", err))
	}

	// Marshal public key to PKIX format
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return serrors.E(op, fmt.Errorf("failed to marshal public key: %w", err))
	}

	// PEM encode public key
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Generate unique key ID
	keyID := uuid.New().String()

	// Store in database
	_, err = db.Exec(
		ctx,
		insertSigningKeyQuery,
		keyID,
		"RS256",
		encryptedPrivateKey,
		publicKeyPEM,
		true,
	)
	if err != nil {
		return serrors.E(op, fmt.Errorf("failed to store signing key: %w", err))
	}

	return nil
}

// GetActiveSigningKey returns the active signing key (decrypted) and its key ID
func GetActiveSigningKey(ctx context.Context, db *pgxpool.Pool, cryptoKey string) (*rsa.PrivateKey, string, error) {
	const op serrors.Op = "oidc.GetActiveSigningKey"

	var keyID string
	var encryptedPrivateKey []byte

	err := db.QueryRow(ctx, getActiveSigningKeyQuery).Scan(&keyID, &encryptedPrivateKey)

	if err != nil {
		return nil, "", serrors.E(op, fmt.Errorf("failed to query signing key: %w", err))
	}

	// Decrypt private key
	decryptedPEM, err := decryptAES256(encryptedPrivateKey, cryptoKey)
	if err != nil {
		return nil, "", serrors.E(op, fmt.Errorf("failed to decrypt private key: %w", err))
	}

	// Decode PEM
	block, _ := pem.Decode(decryptedPEM)
	if block == nil {
		return nil, "", serrors.E(op, fmt.Errorf("failed to decode PEM block"))
	}

	// Parse private key
	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, "", serrors.E(op, fmt.Errorf("failed to parse private key: %w", err))
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, "", serrors.E(op, fmt.Errorf("private key is not RSA"))
	}

	return rsaKey, keyID, nil
}

// GetPublicKeys returns all active public keys for JWKS endpoint
func GetPublicKeys(ctx context.Context, db *pgxpool.Pool) ([]*rsa.PublicKey, error) {
	const op serrors.Op = "oidc.GetPublicKeys"

	rows, err := db.Query(ctx, getPublicKeysQuery)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var publicKeys []*rsa.PublicKey
	for rows.Next() {
		var publicKeyPEM []byte
		if err := rows.Scan(&publicKeyPEM); err != nil {
			return nil, serrors.E(op, err)
		}

		// Decode PEM
		block, _ := pem.Decode(publicKeyPEM)
		if block == nil {
			continue // Skip invalid PEM
		}

		// Parse public key
		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			continue // Skip invalid key
		}

		rsaKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			continue // Skip non-RSA keys
		}

		publicKeys = append(publicKeys, rsaKey)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return publicKeys, nil
}

// PublicKeyWithID represents a public key with its database key_id
type PublicKeyWithID struct {
	KeyID     string
	PublicKey *rsa.PublicKey
}

// GetPublicKeysWithIDs retrieves all active public keys with their key IDs from the database
func GetPublicKeysWithIDs(ctx context.Context, db *pgxpool.Pool) ([]PublicKeyWithID, error) {
	const op serrors.Op = "oidc.GetPublicKeysWithIDs"

	rows, err := db.Query(ctx, getPublicKeysWithIDQuery)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	var keys []PublicKeyWithID
	for rows.Next() {
		var keyID string
		var publicKeyPEM []byte
		if err := rows.Scan(&keyID, &publicKeyPEM); err != nil {
			return nil, serrors.E(op, err)
		}

		// Decode PEM
		block, _ := pem.Decode(publicKeyPEM)
		if block == nil {
			continue // Skip invalid PEM
		}

		// Parse public key
		publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			continue // Skip invalid key
		}

		rsaKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			continue // Skip non-RSA keys
		}

		keys = append(keys, PublicKeyWithID{
			KeyID:     keyID,
			PublicKey: rsaKey,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return keys, nil
}

// Encryption helpers

// encryptAES256 encrypts data using AES-256-GCM
func encryptAES256(plaintext []byte, keyString string) ([]byte, error) {
	// Derive a 32-byte key from the provided string using SHA-256
	// SECURITY: Never truncate user input directly - use cryptographic hash
	hash := sha256.Sum256([]byte(keyString))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// decryptAES256 decrypts data using AES-256-GCM
func decryptAES256(ciphertext []byte, keyString string) ([]byte, error) {
	// Derive a 32-byte key from the provided string using SHA-256
	// SECURITY: Never truncate user input directly - use cryptographic hash
	hash := sha256.Sum256([]byte(keyString))
	key := hash[:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}
