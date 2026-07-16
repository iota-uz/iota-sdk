package cookies_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/stretchr/testify/require"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	require.NoError(t, err)
	return src
}

func TestDefaults_Cookies(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[cookies.Config](r)
	require.NoError(t, err)
	require.Equal(t, "sid", cfg.SID)
	require.Equal(t, "oauthState", cfg.OAuthState)
	require.Empty(t, cfg.Domain)
}

func TestRoundTrip_Cookies(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"http.cookies.sid":        "session_id",
		"http.cookies.oauthstate": "oauth_state",
		"http.cookies.domain":     ".example.com",
	}))
	cfg, err := config.Register[cookies.Config](r)
	require.NoError(t, err)
	require.Equal(t, "session_id", cfg.SID)
	require.Equal(t, "oauth_state", cfg.OAuthState)
	require.Equal(t, ".example.com", cfg.Domain)
}
