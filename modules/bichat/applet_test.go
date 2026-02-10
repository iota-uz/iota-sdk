package bichat

import (
	"context"
	"testing"

	"github.com/iota-uz/applets/pkg/applet"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	bichatcontext "github.com/iota-uz/iota-sdk/pkg/bichat/context"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type appletTestModel struct{}

func (m *appletTestModel) Generate(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (*agents.Response, error) {
	return &agents.Response{Message: types.AssistantMessage("ok")}, nil
}

func (m *appletTestModel) Stream(ctx context.Context, req agents.Request, opts ...agents.GenerateOption) (types.Generator[agents.Chunk], error) {
	return types.NewGenerator(ctx, func(ctx context.Context, yield func(agents.Chunk) bool) error {
		return nil
	}), nil
}

func (m *appletTestModel) Info() agents.ModelInfo {
	return agents.ModelInfo{
		Name:          "gpt-5.2-2025-12-11",
		Provider:      "openai",
		ContextWindow: 272000,
	}
}

func (m *appletTestModel) HasCapability(capability agents.Capability) bool {
	return false
}

func (m *appletTestModel) Pricing() agents.ModelPricing {
	return agents.ModelPricing{}
}

func TestBiChatApplet_Config(t *testing.T) {
	t.Parallel()

	bichatApplet := NewBiChatApplet(nil)
	config := bichatApplet.Config()

	t.Run("WindowGlobal", func(t *testing.T) {
		assert.Equal(t, "__BICHAT_CONTEXT__", config.WindowGlobal)
	})

	t.Run("Endpoints", func(t *testing.T) {
		assert.Equal(t, "/bi-chat/stream", config.Endpoints.Stream)
	})

	t.Run("Assets", func(t *testing.T) {
		assert.NotNil(t, config.Assets.FS)
		assert.Equal(t, "/assets", config.Assets.BasePath)
		assert.Equal(t, ".vite/manifest.json", config.Assets.ManifestPath)
		assert.Equal(t, "index.html", config.Assets.Entrypoint)

		require.NotNil(t, config.Assets.Dev)
		assert.False(t, config.Assets.Dev.Enabled)
		assert.Equal(t, "http://localhost:5173", config.Assets.Dev.TargetURL)
		assert.Equal(t, "/src/main.tsx", config.Assets.Dev.EntryModule)
		assert.Equal(t, "/@vite/client", config.Assets.Dev.ClientModule)
	})

	t.Run("Router", func(t *testing.T) {
		assert.NotNil(t, config.Router)
	})

	t.Run("CustomContext", func(t *testing.T) {
		assert.NotNil(t, config.CustomContext)
	})

	t.Run("Middleware", func(t *testing.T) {
		assert.NotNil(t, config.Middleware)
		assert.NotEmpty(t, config.Middleware)
	})

	t.Run("Shell", func(t *testing.T) {
		assert.Equal(t, applet.ShellModeEmbedded, config.Shell.Mode)
		assert.NotNil(t, config.Shell.Layout)
		assert.Equal(t, "BiChat", config.Shell.Title)
	})

	t.Run("RPC", func(t *testing.T) {
		assert.Nil(t, config.RPC)
	})
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

	llm, ok := custom["llm"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, llm["provider"])
	assert.Equal(t, false, llm["apiKeyConfigured"])

	debug, ok := custom["debug"].(map[string]interface{})
	require.True(t, ok)
	limits, ok := debug["limits"].(map[string]int)
	require.True(t, ok)
	assert.Equal(t, 0, limits["policyMaxTokens"])
	assert.Equal(t, 0, limits["modelMaxTokens"])
	assert.Equal(t, 0, limits["effectiveMaxTokens"])
	assert.Equal(t, 0, limits["completionReserveTokens"])
}

func TestBiChatApplet_buildCustomContext_WithConfig(t *testing.T) {
	t.Parallel()

	// Create config with some features enabled
	config := &ModuleConfig{
		EnableVision:          true,
		EnableWebSearch:       false,
		EnableCodeInterpreter: true,
		EnableMultiAgent:      false,
		ContextPolicy: bichatcontext.ContextPolicy{
			ContextWindow:     180000,
			CompletionReserve: 8000,
		},
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

	llm, ok := custom["llm"].(map[string]interface{})
	require.True(t, ok)
	assert.Empty(t, llm["provider"])
	assert.Equal(t, true, llm["apiKeyConfigured"])

	debug, ok := custom["debug"].(map[string]interface{})
	require.True(t, ok)
	limits, ok := debug["limits"].(map[string]int)
	require.True(t, ok)
	assert.Equal(t, 180000, limits["policyMaxTokens"])
	assert.Equal(t, 0, limits["modelMaxTokens"])
	assert.Equal(t, 180000, limits["effectiveMaxTokens"])
	assert.Equal(t, 8000, limits["completionReserveTokens"])
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

func TestBiChatApplet_buildCustomContext_UsesEffectiveContextWindow(t *testing.T) {
	t.Parallel()

	config := &ModuleConfig{
		Model: &appletTestModel{},
		ContextPolicy: bichatcontext.ContextPolicy{
			ContextWindow: 180000,
		},
	}

	bichatApplet := NewBiChatApplet(config)
	custom, err := bichatApplet.buildCustomContext(context.Background())
	require.NoError(t, err)

	debug, ok := custom["debug"].(map[string]interface{})
	require.True(t, ok)
	limits, ok := debug["limits"].(map[string]int)
	require.True(t, ok)
	assert.Equal(t, 180000, limits["effectiveMaxTokens"])
	assert.Equal(t, 180000, limits["policyMaxTokens"])
	assert.Equal(t, 272000, limits["modelMaxTokens"])
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
