package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- DialPool (primary API) ---

func TestDialPool_EmptyDSN_ReturnsFallback(t *testing.T) {
	t.Parallel()
	fallback := poolSentinel()

	pool, active, err := DialPool(context.Background(), "", fallback)
	require.NoError(t, err)
	assert.False(t, active, "dial should not activate when DSN is empty")
	assert.Same(t, fallback, pool, "fallback pool should be returned unchanged")
}

func TestDialPool_WhitespaceOnlyDSN_UsesFallback(t *testing.T) {
	t.Parallel()
	fallback := poolSentinel()

	pool, active, err := DialPool(context.Background(), "   \t\n  ", fallback)
	require.NoError(t, err, "whitespace-only DSN is treated as unset")
	assert.False(t, active)
	assert.Same(t, fallback, pool)
}

func TestDialPool_MalformedDSN_ReturnsError(t *testing.T) {
	t.Parallel()

	pool, active, err := DialPool(context.Background(), "not-a-valid-dsn://", nil)
	require.Error(t, err, "malformed DSN should produce parse error")
	assert.False(t, active)
	assert.Nil(t, pool)
}

// --- DialNamedPool (deprecated env-var wrapper) ---

func TestDialNamedPool_UnsetEnvVar_ReturnsFallback(t *testing.T) {
	// Sentinel fallback — we only check pointer equality.
	fallback := poolSentinel()

	// Ensure env var is unset for this test.
	t.Setenv("IOTA_TEST_UNSET_DSN", "")

	pool, active, err := DialNamedPool(context.Background(), "IOTA_TEST_UNSET_DSN", fallback)
	require.NoError(t, err)
	assert.False(t, active, "dial should not activate when env var is empty")
	assert.Same(t, fallback, pool, "fallback pool should be returned unchanged")
}

func TestDialNamedPool_MalformedDSN_ReturnsError(t *testing.T) {
	t.Setenv("IOTA_TEST_BAD_DSN", "not-a-valid-dsn://")

	pool, active, err := DialNamedPool(context.Background(), "IOTA_TEST_BAD_DSN", nil)
	require.Error(t, err, "malformed DSN should produce parse error")
	assert.False(t, active)
	assert.Nil(t, pool)
}

func TestDialNamedPool_WhitespaceOnlyEnvVar_UsesFallback(t *testing.T) {
	fallback := poolSentinel()
	t.Setenv("IOTA_TEST_WS_DSN", "   \t\n  ")

	pool, active, err := DialNamedPool(context.Background(), "IOTA_TEST_WS_DSN", fallback)
	require.NoError(t, err, "whitespace-only DSN is treated as unset")
	assert.False(t, active)
	assert.Same(t, fallback, pool)
}
