package cookies_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestDefaults_Cookies(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[cookies.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.SID != "sid" {
		t.Errorf("SID default: got %q, want \"sid\"", cfg.SID)
	}
	if cfg.OAuthState != "oauthState" {
		t.Errorf("OAuthState default: got %q, want \"oauthState\"", cfg.OAuthState)
	}
}

func TestRoundTrip_Cookies(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"http.cookies.sid":        "session_id",
		"http.cookies.oauthstate": "oauth_state",
	}))
	cfg, err := config.Register[cookies.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.SID != "session_id" {
		t.Errorf("SID: got %q", cfg.SID)
	}
	if cfg.OAuthState != "oauth_state" {
		t.Errorf("OAuthState: got %q", cfg.OAuthState)
	}
}
