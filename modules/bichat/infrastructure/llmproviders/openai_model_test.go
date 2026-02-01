package llmproviders

import (
	"context"
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIModel_MissingAPIKey(t *testing.T) {
	t.Parallel()

	// Clear API key
	os.Unsetenv("OPENAI_API_KEY")

	_, err := NewOpenAIModel()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestNewOpenAIModel_WithAPIKey(t *testing.T) {
	t.Parallel()

	// Set fake API key
	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	assert.NotNil(t, model)
}

func TestNewOpenAIModel_DefaultModel(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// No OPENAI_MODEL set
	os.Unsetenv("OPENAI_MODEL")

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	oaiModel := model.(*OpenAIModel)
	assert.Equal(t, "gpt-4", oaiModel.modelName)
}

func TestNewOpenAIModel_CustomModel(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	os.Setenv("OPENAI_MODEL", "gpt-4-turbo")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("OPENAI_MODEL")

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	oaiModel := model.(*OpenAIModel)
	assert.Equal(t, "gpt-4-turbo", oaiModel.modelName)
}

func TestOpenAIModel_Info(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	os.Setenv("OPENAI_MODEL", "gpt-4")
	defer os.Unsetenv("OPENAI_API_KEY")
	defer os.Unsetenv("OPENAI_MODEL")

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	info := model.Info()
	assert.Equal(t, "gpt-4", info.Name)
	assert.Equal(t, "openai", info.Provider)
	assert.Contains(t, info.Capabilities, agents.CapabilityStreaming)
	assert.Contains(t, info.Capabilities, agents.CapabilityTools)
	assert.Contains(t, info.Capabilities, agents.CapabilityJSONMode)
}

func TestOpenAIModel_HasCapability(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	assert.True(t, model.HasCapability(agents.CapabilityStreaming))
	assert.True(t, model.HasCapability(agents.CapabilityTools))
	assert.True(t, model.HasCapability(agents.CapabilityJSONMode))
	assert.False(t, model.HasCapability(agents.CapabilityThinking))
}

func TestOpenAIModel_BuildRequest(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	oaiModel := model.(*OpenAIModel)

	// Build request with messages and tools
	req := agents.Request{
		Messages: []types.Message{
			{
				Role:    types.RoleSystem,
				Content: "You are a helpful assistant",
			},
			{
				Role:    types.RoleUser,
				Content: "Hello",
			},
		},
		Tools: []agents.Tool{
			agents.NewTool(
				"test_tool",
				"A test tool",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
				func(ctx context.Context, input string) (string, error) {
					return "test result", nil
				},
			),
		},
	}

	maxTokens := 100
	temperature := 0.7
	config := agents.GenerateConfig{
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
		JSONMode:    true,
	}

	oaiReq := oaiModel.buildChatCompletionRequest(req, config)

	assert.Equal(t, "gpt-4", oaiReq.Model)
	assert.Len(t, oaiReq.Messages, 2)
	assert.Equal(t, "system", oaiReq.Messages[0].Role)
	assert.Equal(t, "user", oaiReq.Messages[1].Role)
	assert.Len(t, oaiReq.Tools, 1)
	assert.Equal(t, "test_tool", oaiReq.Tools[0].Function.Name)
	assert.Equal(t, 100, oaiReq.MaxTokens)
	assert.Equal(t, float32(0.7), oaiReq.Temperature)
	assert.NotNil(t, oaiReq.ResponseFormat)
}
