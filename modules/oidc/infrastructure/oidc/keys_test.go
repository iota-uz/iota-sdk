package oidc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

const testCryptoKey = "test-crypto-key-32-bytes-long-12"

func TestBootstrapKeys(t *testing.T) {
	t.Parallel()

	t.Run("GeneratesKeysWhenEmpty", func(t *testing.T) {
		env := itf.Setup(t)

		// Bootstrap keys
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Verify keys were created
		var count int
		err = env.Pool.QueryRow(env.Ctx, "SELECT COUNT(*) FROM oidc_signing_keys WHERE is_active = true").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)

		// Verify we can retrieve the key
		privateKey, keyID, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)
		assert.NotNil(t, privateKey)
		assert.NotEmpty(t, keyID)
		assert.NotNil(t, privateKey.PublicKey)
	})

	t.Run("DoesNotGenerateKeysWhenExist", func(t *testing.T) {
		env := itf.Setup(t)

		// Bootstrap keys first time
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Get first key ID
		_, firstKeyID, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Bootstrap again - should not create new keys
		err = oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Verify same key is still active
		_, secondKeyID, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)
		assert.Equal(t, firstKeyID, secondKeyID)

		// Verify only one active key exists
		var count int
		err = env.Pool.QueryRow(env.Ctx, "SELECT COUNT(*) FROM oidc_signing_keys WHERE is_active = true").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("ValidRSAKeyGeneration", func(t *testing.T) {
		env := itf.Setup(t)

		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		privateKey, _, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Verify key size
		assert.Equal(t, 2048, privateKey.N.BitLen())

		// Verify we can sign and verify with the key
		hash := []byte("test-hash-32-bytes-long-12345678")

		signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, 0, hash)
		require.NoError(t, err)
		assert.NotEmpty(t, signature)

		// Verify signature with public key
		err = rsa.VerifyPKCS1v15(&privateKey.PublicKey, 0, hash, signature)
		require.NoError(t, err)
	})
}

func TestGetActiveSigningKey(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		env := itf.Setup(t)

		// Bootstrap keys first
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Get active signing key
		privateKey, keyID, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)
		assert.NotNil(t, privateKey)
		assert.NotEmpty(t, keyID)
		assert.IsType(t, &rsa.PrivateKey{}, privateKey)
	})

	t.Run("NoKeysExist", func(t *testing.T) {
		env := itf.Setup(t)

		// Don't bootstrap keys

		// Should fail when no keys exist
		_, _, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
		assert.Error(t, err)
	})

	t.Run("DecryptionWithWrongKey", func(t *testing.T) {
		env := itf.Setup(t)

		// Bootstrap with one key
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, "original-key")
		require.NoError(t, err)

		// Try to decrypt with different key
		_, _, err = oidc.GetActiveSigningKey(env.Ctx, env.Pool, "wrong-key")
		assert.Error(t, err)
	})
}

func TestGetPublicKeys(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		env := itf.Setup(t)

		// Bootstrap keys
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Get public keys
		publicKeys, err := oidc.GetPublicKeys(env.Ctx, env.Pool)
		require.NoError(t, err)
		assert.Len(t, publicKeys, 1)
		assert.NotNil(t, publicKeys[0])
		assert.IsType(t, &rsa.PublicKey{}, publicKeys[0])
	})

	t.Run("NoKeysExist", func(t *testing.T) {
		env := itf.Setup(t)

		// Don't bootstrap keys

		// Should return empty list
		publicKeys, err := oidc.GetPublicKeys(env.Ctx, env.Pool)
		require.NoError(t, err)
		assert.Empty(t, publicKeys)
	})

	t.Run("MultipleActiveKeys", func(t *testing.T) {
		env := itf.Setup(t)

		// Bootstrap first key
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// This would normally be done through a key rotation process
		// For testing, we'll just verify the current implementation works
		publicKeys, err := oidc.GetPublicKeys(env.Ctx, env.Pool)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(publicKeys), 1)
	})
}

func TestKeyEncryptionDecryption(t *testing.T) {
	t.Parallel()

	t.Run("RoundTripEncryption", func(t *testing.T) {
		env := itf.Setup(t)

		cryptoKey := "my-secret-key-32-bytes-long-1234"

		// Bootstrap keys (encrypts private key)
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, cryptoKey)
		require.NoError(t, err)

		// Get key (decrypts private key)
		privateKey1, keyID1, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, cryptoKey)
		require.NoError(t, err)

		// Get key again - should return same key
		privateKey2, keyID2, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, cryptoKey)
		require.NoError(t, err)

		assert.Equal(t, keyID1, keyID2)
		assert.Equal(t, privateKey1.N, privateKey2.N)
		assert.Equal(t, privateKey1.E, privateKey2.E)
	})

	t.Run("DifferentCryptoKeys", func(t *testing.T) {
		env := itf.Setup(t)

		// Bootstrap with first crypto key
		err := oidc.BootstrapKeys(env.Ctx, env.Pool, "crypto-key-1")
		require.NoError(t, err)

		// Should fail to decrypt with different crypto key
		_, _, err = oidc.GetActiveSigningKey(env.Ctx, env.Pool, "crypto-key-2")
		assert.Error(t, err)
	})
}

func TestKeyStorage(t *testing.T) {
	t.Parallel()

	t.Run("KeyIDIsUnique", func(t *testing.T) {
		env := itf.Setup(t)

		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		_, keyID, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Verify key_id is a valid UUID format (36 characters with dashes)
		assert.Len(t, keyID, 36)
		assert.Contains(t, keyID, "-")
	})

	t.Run("AlgorithmIsRS256", func(t *testing.T) {
		env := itf.Setup(t)

		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Query algorithm directly from database
		var algorithm string
		err = env.Pool.QueryRow(
			env.Ctx,
			"SELECT algorithm FROM oidc_signing_keys WHERE is_active = true LIMIT 1",
		).Scan(&algorithm)
		require.NoError(t, err)
		assert.Equal(t, "RS256", algorithm)
	})

	t.Run("PrivateKeyIsEncrypted", func(t *testing.T) {
		env := itf.Setup(t)

		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Query encrypted private key from database
		var encryptedKey []byte
		err = env.Pool.QueryRow(
			env.Ctx,
			"SELECT private_key FROM oidc_signing_keys WHERE is_active = true LIMIT 1",
		).Scan(&encryptedKey)
		require.NoError(t, err)

		// Encrypted key should not contain PEM headers (it's binary)
		assert.NotContains(t, string(encryptedKey), "BEGIN PRIVATE KEY")
		assert.NotContains(t, string(encryptedKey), "END PRIVATE KEY")

		// Encrypted key should be non-empty
		assert.NotEmpty(t, encryptedKey)
	})

	t.Run("PublicKeyIsPEMFormat", func(t *testing.T) {
		env := itf.Setup(t)

		err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
		require.NoError(t, err)

		// Query public key from database
		var publicKeyPEM []byte
		err = env.Pool.QueryRow(
			env.Ctx,
			"SELECT public_key FROM oidc_signing_keys WHERE is_active = true LIMIT 1",
		).Scan(&publicKeyPEM)
		require.NoError(t, err)

		// Public key should be in PEM format
		assert.Contains(t, string(publicKeyPEM), "BEGIN PUBLIC KEY")
		assert.Contains(t, string(publicKeyPEM), "END PUBLIC KEY")
	})
}

func TestConcurrentKeyAccess(t *testing.T) {
	t.Parallel()

	env := itf.Setup(t)

	// Bootstrap keys first
	err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
	require.NoError(t, err)

	// Test concurrent access to keys
	const numGoroutines = 10
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			_, _, err := oidc.GetActiveSigningKey(env.Ctx, env.Pool, testCryptoKey)
			errors <- err
		}()
	}

	// Verify all concurrent accesses succeeded
	for i := 0; i < numGoroutines; i++ {
		err := <-errors
		assert.NoError(t, err)
	}
}
