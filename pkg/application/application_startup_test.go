package application

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/stretchr/testify/require"
)

func TestNew_Scenarios(t *testing.T) {
	cfg := configuration.Use()
	originalURL := cfg.MeiliURL
	originalAPIKey := cfg.MeiliAPIKey
	t.Cleanup(func() {
		cfg.MeiliURL = originalURL
		cfg.MeiliAPIKey = originalAPIKey
	})

	tests := []struct {
		name     string
		meiliURL string
		apiKey   string
	}{
		{name: "allows default construction"},
		{name: "skips meili preflight until explicit startup", meiliURL: "http://127.0.0.1:1", apiKey: "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg.MeiliURL = tt.meiliURL
			cfg.MeiliAPIKey = tt.apiKey

			app, err := New(&ApplicationOptions{
				Bundle:             LoadBundle(),
				SupportedLanguages: []string{"en"},
			})
			require.NoError(t, err)
			require.NotNil(t, app)
		})
	}
}
