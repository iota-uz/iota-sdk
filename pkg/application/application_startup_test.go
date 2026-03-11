package application

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_DisableBackgroundWorkersLogsDisabledMessage(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := logrus.New()
	logger.SetOutput(buf)
	logger.SetLevel(logrus.InfoLevel)

	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		Logger:             logger,
		SupportedLanguages: []string{"en"},
		RuntimeProfile:     RuntimeProfileCLI,
	})
	require.NoError(t, err)
	require.NotNil(t, app)

	assert.Contains(t, buf.String(), "background workers disabled")
}

func TestNew_CLIRuntimeSkipsMeiliPreflight(t *testing.T) {
	t.Setenv("MEILI_URL", "http://127.0.0.1:1")
	t.Setenv("MEILI_API_KEY", "test")

	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
		RuntimeProfile:     RuntimeProfileCLI,
	})
	require.NoError(t, err)
	require.NotNil(t, app)
}
