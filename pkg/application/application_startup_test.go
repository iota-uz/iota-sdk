package application

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/meiliconfig"
	"github.com/stretchr/testify/require"
)

func TestNew_Scenarios(t *testing.T) {
	tests := []struct {
		name  string
		meili *meiliconfig.Config
	}{
		{
			name: "allows default construction",
		},
		{
			name: "skips meili preflight until explicit startup",
			meili: &meiliconfig.Config{
				URL:    "http://127.0.0.1:1",
				APIKey: "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := New(&ApplicationOptions{
				Bundle:             LoadBundle(),
				SupportedLanguages: []string{"en"},
				Meili:              tt.meili,
			})
			require.NoError(t, err)
			require.NotNil(t, app)
		})
	}
}
