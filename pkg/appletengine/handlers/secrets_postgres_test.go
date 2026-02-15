package handlers

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPostgresSecretsStore_RequiresDependencies(t *testing.T) {
	t.Parallel()

	store, err := NewPostgresSecretsStore(nil, "")
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "postgres pool is required")
}

func TestEncryptSecretValue_Roundtrip(t *testing.T) {
	t.Parallel()

	masterKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	cipherText, err := EncryptSecretValue(masterKey, "secret-value")
	require.NoError(t, err)
	require.NotEmpty(t, cipherText)

	key, err := decodeMasterKey(masterKey)
	require.NoError(t, err)
	plain, err := decryptString(key, cipherText)
	require.NoError(t, err)
	assert.Equal(t, "secret-value", plain)
}
