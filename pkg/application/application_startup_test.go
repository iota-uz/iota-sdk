package application

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/stretchr/testify/require"
)

func TestNew_Scenarios(t *testing.T) {
	conf := configuration.Use()
	originalMeiliURL := conf.MeiliURL
	originalMeiliAPIKey := conf.MeiliAPIKey
	t.Cleanup(func() {
		conf.MeiliURL = originalMeiliURL
		conf.MeiliAPIKey = originalMeiliAPIKey
	})

	tests := []struct {
		name        string
		meiliURL    string
		meiliAPIKey string
	}{
		{
			name: "allows default construction",
		},
		{
			name:        "skips meili preflight until explicit startup",
			meiliURL:    "http://127.0.0.1:1",
			meiliAPIKey: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf.MeiliURL = tt.meiliURL
			conf.MeiliAPIKey = tt.meiliAPIKey

			app, err := New(&ApplicationOptions{
				Bundle:             LoadBundle(),
				SupportedLanguages: []string{"en"},
			})
			require.NoError(t, err)
			require.NotNil(t, app)
		})
	}
}
