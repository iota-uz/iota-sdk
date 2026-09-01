package services

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBrowserSessionCookieLegacyMigration(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.September, 1, 12, 0, 0, 0, time.UTC)
	state, migrated := decodeBrowserSessionState("legacy-opaque-token", now)

	require.True(t, migrated)
	require.Equal(t, browserSessionCookieVersion, state.Version)
	require.Equal(t, "legacy-opaque-token", state.Active)
	require.Equal(t, []browserSessionEntry{{Token: "legacy-opaque-token", LastActive: now.UnixNano()}}, state.Entries)

	encoded, err := encodeBrowserSessionState(state)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(encoded, "v1."))
	decoded, migrated := decodeBrowserSessionState(encoded, now.Add(time.Hour))
	require.False(t, migrated)
	require.Equal(t, state, decoded)
}

func TestBrowserSessionCookieRejectsMalformedVersionedValue(t *testing.T) {
	t.Parallel()
	state, migrated := decodeBrowserSessionState("v1.not-valid-base64", time.Now())

	require.True(t, migrated)
	require.Empty(t, state.Active)
	require.Empty(t, state.Entries)
}

func TestBrowserSessionReferenceDoesNotExposeToken(t *testing.T) {
	t.Parallel()
	token := "server-side-secret-session-token"
	reference := BrowserSessionReference(token)

	require.NotEmpty(t, reference)
	require.NotContains(t, reference, token)
	require.Equal(t, reference, BrowserSessionReference(token))
	require.NotEqual(t, reference, BrowserSessionReference(token+"-other"))
}
