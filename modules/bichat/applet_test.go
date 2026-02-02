package bichat

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/applet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBiChatApplet_Name(t *testing.T) {
	t.Parallel()

	bichatApplet := NewBiChatApplet(nil)
	assert.Equal(t, "bichat", bichatApplet.Name())
}

func TestBiChatApplet_BasePath(t *testing.T) {
	t.Parallel()

	bichatApplet := NewBiChatApplet(nil)
	assert.Equal(t, "/bichat", bichatApplet.BasePath())
}

func TestBiChatApplet_Config(t *testing.T) {
	t.Parallel()

	bichatApplet := NewBiChatApplet(nil)
	config := bichatApplet.Config()

	// Verify window global
	assert.Equal(t, "__BICHAT_CONTEXT__", config.WindowGlobal)

	// Verify endpoints
	assert.Equal(t, "/bichat/graphql", config.Endpoints.GraphQL)
	assert.Equal(t, "/bichat/stream", config.Endpoints.Stream)

	// Verify assets
	assert.NotNil(t, config.Assets.FS)
	assert.Equal(t, "/bichat/assets", config.Assets.BasePath)
	assert.Equal(t, "/bichat/assets/main.css", config.Assets.CSSPath)

	// Verify router
	assert.NotNil(t, config.Router)

	// Verify custom context is set
	assert.NotNil(t, config.CustomContext)

	// Verify middleware
	assert.NotNil(t, config.Middleware)
	assert.Empty(t, config.Middleware) // BiChat uses SDK defaults
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

func TestBiChatApplet_ImplementsAppletInterface(t *testing.T) {
	t.Parallel()

	// Verify BiChatApplet implements applet.Applet interface
	var _ applet.Applet = (*BiChatApplet)(nil)
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
