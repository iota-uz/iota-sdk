package bichat

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBiChatApplet_Config(t *testing.T) {
	t.Parallel()

	bichatApplet := NewBiChatApplet(nil)
	config := bichatApplet.Config()

	// Verify window global
	assert.Equal(t, "__BICHAT_CONTEXT__", config.WindowGlobal)

	// Verify endpoints
	assert.Equal(t, "/query/bichat", config.Endpoints.GraphQL)
	assert.Equal(t, "/bi-chat/stream", config.Endpoints.Stream)

	// Verify assets
	assert.NotNil(t, config.Assets.FS)
	assert.Equal(t, "/assets", config.Assets.BasePath)
	assert.Equal(t, ".vite/manifest.json", config.Assets.ManifestPath)
	assert.Equal(t, "index.html", config.Assets.Entrypoint)

	// Verify router
	assert.NotNil(t, config.Router)

	// Verify custom context is set
	assert.NotNil(t, config.CustomContext)

	// Verify middleware
	assert.NotNil(t, config.Middleware)
	assert.NotEmpty(t, config.Middleware) // BiChat requires authentication middleware
}

func TestBiChatApplet_Config_BasePathDerivedValues(t *testing.T) {
	tests := []struct {
		name               string
		moduleConfig       *ModuleConfig
		expectedBasePath   string
		expectedStreamPath string
	}{
		{
			name:               "nil config derives base path correctly",
			moduleConfig:       nil,
			expectedBasePath:   "/bi-chat",
			expectedStreamPath: "/bi-chat/stream",
		},
		{
			name:               "config with features derives base path correctly",
			moduleConfig:       &ModuleConfig{EnableVision: true},
			expectedBasePath:   "/bi-chat",
			expectedStreamPath: "/bi-chat/stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bichatApplet := NewBiChatApplet(tt.moduleConfig)
			basePath := bichatApplet.BasePath()
			config := bichatApplet.Config()

			require.Equal(t, tt.expectedBasePath, basePath, "BasePath() should return expected value")
			assert.Equal(t, basePath, config.Mount.Attributes["base-path"], "config.Mount.Attributes[base-path] should match BasePath()")
			assert.Equal(t, tt.expectedStreamPath, config.Endpoints.Stream, "config.Endpoints.Stream should be basePath+/stream")
		})
	}
}

func TestBiChatApplet_buildCustomContext_NoConfig(t *testing.T) {
	t.Parallel()

	bichatApplet := NewBiChatApplet(nil)
	ctx := context.Background()

	custom, err := bichatApplet.buildCustomContext(ctx)
	require.NoError(t, err)
	require.NotNil(t, custom)

	// Verify features exist
	features, ok := custom["features"].(map[string]bool)
	require.True(t, ok)

	// Verify all features are disabled by default
	assert.False(t, features["vision"])
	assert.False(t, features["webSearch"])
	assert.False(t, features["codeInterpreter"])
	assert.False(t, features["multiAgent"])
}

func TestBiChatApplet_buildCustomContext_WithConfig(t *testing.T) {
	t.Parallel()

	// Create config with some features enabled
	config := &ModuleConfig{
		EnableVision:          true,
		EnableWebSearch:       false,
		EnableCodeInterpreter: true,
		EnableMultiAgent:      false,
	}

	bichatApplet := NewBiChatApplet(config)
	ctx := context.Background()

	custom, err := bichatApplet.buildCustomContext(ctx)
	require.NoError(t, err)
	require.NotNil(t, custom)

	// Verify features exist
	features, ok := custom["features"].(map[string]bool)
	require.True(t, ok)

	// Verify features match config
	assert.True(t, features["vision"])
	assert.False(t, features["webSearch"])
	assert.True(t, features["codeInterpreter"])
	assert.False(t, features["multiAgent"])
}

func TestBiChatApplet_SetConfig(t *testing.T) {
	t.Parallel()

	// Create applet without config
	bichatApplet := NewBiChatApplet(nil)
	assert.Nil(t, bichatApplet.config)

	// Set config
	config := &ModuleConfig{
		EnableVision: true,
	}
	bichatApplet.SetConfig(config)
	assert.NotNil(t, bichatApplet.config)
	assert.True(t, bichatApplet.config.EnableVision)

	// Verify custom context reflects new config
	ctx := context.Background()
	custom, err := bichatApplet.buildCustomContext(ctx)
	require.NoError(t, err)

	features := custom["features"].(map[string]bool)
	assert.True(t, features["vision"])
}

func TestModuleConfig_FeatureFlagOptions(t *testing.T) {
	t.Parallel()

	// Create base config (no options would normally be used with NewModuleConfig, but this tests the options)
	config := &ModuleConfig{}

	// Apply feature flag options
	opts := []ConfigOption{
		WithVision(true),
		WithWebSearch(true),
		WithCodeInterpreter(false),
		WithMultiAgent(true),
	}

	for _, opt := range opts {
		opt(config)
	}

	// Verify flags are set correctly
	assert.True(t, config.EnableVision)
	assert.True(t, config.EnableWebSearch)
	assert.False(t, config.EnableCodeInterpreter)
	assert.True(t, config.EnableMultiAgent)
}
