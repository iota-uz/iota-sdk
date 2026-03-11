package application

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_AllowsDefaultConstruction(t *testing.T) {
	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	require.NoError(t, err)
	require.NotNil(t, app)
}

func TestNew_SkipsMeiliPreflightUntilExplicitStartup(t *testing.T) {
	t.Setenv("MEILI_URL", "http://127.0.0.1:1")
	t.Setenv("MEILI_API_KEY", "test")

	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	require.NoError(t, err)
	require.NotNil(t, app)
}
