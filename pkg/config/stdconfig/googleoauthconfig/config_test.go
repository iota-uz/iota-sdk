package googleoauthconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/googleoauthconfig"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	src := buildSource(t, map[string]any{
		"googleoauth.redirecturl":  "https://example.com/callback",
		"googleoauth.clientid":     "my-client-id",
		"googleoauth.clientsecret": "my-client-secret",
	})

	var cfg googleoauthconfig.Config
	if err := src.Unmarshal("googleoauth", &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if cfg.RedirectURL != "https://example.com/callback" {
		t.Errorf("RedirectURL: got %q, want %q", cfg.RedirectURL, "https://example.com/callback")
	}
	if cfg.ClientID != "my-client-id" {
		t.Errorf("ClientID: got %q, want %q", cfg.ClientID, "my-client-id")
	}
	if cfg.ClientSecret != "my-client-secret" {
		t.Errorf("ClientSecret: got %q, want %q", cfg.ClientSecret, "my-client-secret")
	}
}

func TestIsConfigured(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		cfg      googleoauthconfig.Config
		expected bool
	}{
		{
			name:     "both set",
			cfg:      googleoauthconfig.Config{ClientID: "id", ClientSecret: "secret"},
			expected: true,
		},
		{
			name:     "missing ClientSecret",
			cfg:      googleoauthconfig.Config{ClientID: "id"},
			expected: false,
		},
		{
			name:     "missing ClientID",
			cfg:      googleoauthconfig.Config{ClientSecret: "secret"},
			expected: false,
		},
		{
			name:     "both empty",
			cfg:      googleoauthconfig.Config{},
			expected: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.cfg.IsConfigured(); got != tc.expected {
				t.Errorf("IsConfigured: got %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestFromLegacy(t *testing.T) {
	t.Parallel()

	legacy := &configuration.Configuration{
		Google: configuration.GoogleOptions{
			RedirectURL:  "https://legacy.example.com/cb",
			ClientID:     "legacy-client-id",
			ClientSecret: "legacy-client-secret",
		},
	}

	got := googleoauthconfig.FromLegacy(legacy)

	if got.RedirectURL != legacy.Google.RedirectURL {
		t.Errorf("RedirectURL: got %q, want %q", got.RedirectURL, legacy.Google.RedirectURL)
	}
	if got.ClientID != legacy.Google.ClientID {
		t.Errorf("ClientID: got %q, want %q", got.ClientID, legacy.Google.ClientID)
	}
	if got.ClientSecret != legacy.Google.ClientSecret {
		t.Errorf("ClientSecret: got %q, want %q", got.ClientSecret, legacy.Google.ClientSecret)
	}
}
