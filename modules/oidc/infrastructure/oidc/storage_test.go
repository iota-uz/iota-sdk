package oidc_test

import (
	"crypto/rsa"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/modules/oidc/infrastructure/oidc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageKeySet_ExposesOnlyPublicKeys(t *testing.T) {
	t.Parallel()

	env := setupTest(t)

	err := oidc.BootstrapKeys(env.Ctx, env.Pool, testCryptoKey)
	require.NoError(t, err)

	storage := oidc.NewStorage(
		nil,
		nil,
		nil,
		nil,
		env.Pool,
		testCryptoKey,
		"https://issuer.example.com/oidc",
		time.Hour,
		24*time.Hour,
	)

	keys, err := storage.KeySet(env.Ctx)
	require.NoError(t, err)
	require.NotEmpty(t, keys)

	for _, key := range keys {
		assert.IsType(t, &rsa.PublicKey{}, key.Key())
	}
}
