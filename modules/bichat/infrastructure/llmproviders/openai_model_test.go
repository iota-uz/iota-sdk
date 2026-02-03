package llmproviders

import (
	"context"
	"os"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/sashabaranov/go-openai"
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
			types.SystemMessage("You are a helpful assistant"),
			types.UserMessage("Hello"),
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

func TestOpenAIModel_BuildRequest_WebSearchTool(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	oaiModel := model.(*OpenAIModel)

	// Build request with web_search tool
	req := agents.Request{
		Messages: []types.Message{
			types.UserMessage("Search for current weather"),
		},
		Tools: []agents.Tool{
			agents.NewTool(
				"web_search",
				"Search the web",
				map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
				func(ctx context.Context, input string) (string, error) {
					return "search results", nil
				},
			),
		},
	}

	config := agents.GenerateConfig{}
	oaiReq := oaiModel.buildChatCompletionRequest(req, config)

	// Verify web_search tool is included (not skipped)
	assert.Len(t, oaiReq.Tools, 1, "web_search tool should be included in request")
	assert.Equal(t, "web_search", oaiReq.Tools[0].Function.Name)
	assert.Equal(t, "Search the web", oaiReq.Tools[0].Function.Description)
}

func TestOpenAIModel_BuildRequest_CodeInterpreterSkipped(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	oaiModel := model.(*OpenAIModel)

	// Build request with code_interpreter tool (should be skipped)
	req := agents.Request{
		Messages: []types.Message{
			types.UserMessage("Run some code"),
		},
		Tools: []agents.Tool{
			agents.NewTool(
				"code_interpreter",
				"Execute code",
				map[string]any{"type": "object"},
				func(ctx context.Context, input string) (string, error) {
					return "code result", nil
				},
			),
		},
	}

	config := agents.GenerateConfig{}
	oaiReq := oaiModel.buildChatCompletionRequest(req, config)

	// Verify code_interpreter tool is skipped
	assert.Len(t, oaiReq.Tools, 0, "code_interpreter tool should be skipped")
}

func TestExtractCitationMarkersFromContent_NoCitations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		content string
	}{
		{
			name:    "Empty string",
			content: "",
		},
		{
			name:    "No citation markers",
			content: "This is a simple response without any citations.",
		},
		{
			name:    "Square brackets but not citations",
			content: "Arrays are defined with [index] notation in many languages.",
		},
		{
			name:    "Non-numeric citations",
			content: "See reference [a] and [abc] for details.",
		},
		{
			name:    "Malformed citations",
			content: "Invalid formats: [1,2] [1-3] [1a] [a1]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			citations := extractCitationMarkersFromContent(tc.content)
			assert.Empty(t, citations, "Should not extract citations from: %s", tc.content)
		})
	}
}

func TestExtractCitationMarkersFromContent_WithCitations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		content        string
		expectedCount  int
		expectedCites  []string // Expected citation titles
		expectedStarts []int    // Expected start indices
		expectedEnds   []int    // Expected end indices
	}{
		{
			name:           "Single citation",
			content:        "This is a fact[1] that needs citation.",
			expectedCount:  1,
			expectedCites:  []string{"Citation [1]"},
			expectedStarts: []int{14},
			expectedEnds:   []int{17},
		},
		{
			name:           "Multiple citations",
			content:        "First fact[1] and second fact[2] with third[3].",
			expectedCount:  3,
			expectedCites:  []string{"Citation [1]", "Citation [2]", "Citation [3]"},
			expectedStarts: []int{10, 29, 43},
			expectedEnds:   []int{13, 32, 46},
		},
		{
			name:           "Multi-digit citation",
			content:        "Reference to citation[123] in text.",
			expectedCount:  1,
			expectedCites:  []string{"Citation [123]"},
			expectedStarts: []int{21},
			expectedEnds:   []int{26},
		},
		{
			name:           "Citations at start and end",
			content:        "[1]Start and end[2]",
			expectedCount:  2,
			expectedCites:  []string{"Citation [1]", "Citation [2]"},
			expectedStarts: []int{0, 16},
			expectedEnds:   []int{3, 19},
		},
		{
			name:           "Duplicate citation numbers",
			content:        "First[1] reference and second[1] reference.",
			expectedCount:  1, // Should deduplicate
			expectedCites:  []string{"Citation [1]"},
			expectedStarts: []int{5}, // Only first occurrence
			expectedEnds:   []int{8},
		},
		{
			name:           "Mixed with non-citation brackets",
			content:        "Array[index] and citation[1] and more[text].",
			expectedCount:  1,
			expectedCites:  []string{"Citation [1]"},
			expectedStarts: []int{25},
			expectedEnds:   []int{28},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			citations := extractCitationMarkersFromContent(tc.content)

			assert.Len(t, citations, tc.expectedCount, "Wrong number of citations")

			for i, citation := range citations {
				// Check type
				assert.Equal(t, "web", citation.Type, "Citation %d should have type 'web'", i)

				// Check title
				assert.Equal(t, tc.expectedCites[i], citation.Title, "Citation %d has wrong title", i)

				// Check URL and Excerpt are empty (no metadata available)
				assert.Empty(t, citation.URL, "Citation %d should have empty URL", i)
				assert.Empty(t, citation.Excerpt, "Citation %d should have empty Excerpt", i)

				// Check position
				assert.Equal(t, tc.expectedStarts[i], citation.StartIndex, "Citation %d has wrong start index", i)
				assert.Equal(t, tc.expectedEnds[i], citation.EndIndex, "Citation %d has wrong end index", i)
			}
		})
	}
}

func TestExtractCitationMarkersFromContent_RealWorldExample(t *testing.T) {
	t.Parallel()

	content := `According to recent research[1], the global temperature has increased by 1.1°C since pre-industrial times.
This trend is expected to continue[2], with projections showing a potential increase of up to 3.2°C by 2100[3]
if current emission trends persist. Several mitigation strategies have been proposed[4] to address this challenge.`

	citations := extractCitationMarkersFromContent(content)

	assert.Len(t, citations, 4, "Should extract 4 citations from real-world example")

	// Verify all citations are type "web"
	for i, citation := range citations {
		assert.Equal(t, "web", citation.Type, "Citation %d should be type web", i)
		assert.NotEmpty(t, citation.Title, "Citation %d should have title", i)
		assert.True(t, citation.StartIndex >= 0, "Citation %d should have valid start index", i)
		assert.True(t, citation.EndIndex > citation.StartIndex, "Citation %d should have valid end index", i)
	}

	// Verify positions are in order
	for i := 1; i < len(citations); i++ {
		assert.True(t, citations[i].StartIndex > citations[i-1].StartIndex,
			"Citations should be in document order")
	}
}

func TestExtractCitationsFromResponse_EmptyResponse(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	// Import openai to access types
	var resp openai.ChatCompletionResponse

	// Empty response
	citations := extractCitationsFromResponse(&resp)
	assert.Nil(t, citations, "Empty response should return nil")
}

func TestExtractCitationsFromResponse_NoChoices(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	resp := openai.ChatCompletionResponse{
		ID:      "test-id",
		Choices: []openai.ChatCompletionChoice{},
	}

	citations := extractCitationsFromResponse(&resp)
	assert.Nil(t, citations, "Response without choices should return nil")
}

func TestExtractCitationsFromResponse_WithCitations(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	resp := openai.ChatCompletionResponse{
		ID: "test-id",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "The sky is blue[1] and grass is green[2]. This is well documented.",
				},
				FinishReason: "stop",
			},
		},
	}

	citations := extractCitationsFromResponse(&resp)
	assert.Len(t, citations, 2, "Should extract 2 citations from response")

	assert.Equal(t, "Citation [1]", citations[0].Title)
	assert.Equal(t, "Citation [2]", citations[1].Title)
	assert.Equal(t, "web", citations[0].Type)
	assert.Equal(t, "web", citations[1].Type)
}

func TestExtractCitationsFromResponse_NoCitationMarkers(t *testing.T) {
	t.Parallel()

	os.Setenv("OPENAI_API_KEY", "sk-test-key")
	defer os.Unsetenv("OPENAI_API_KEY")

	resp := openai.ChatCompletionResponse{
		ID: "test-id",
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Role:    "assistant",
					Content: "This is a plain response without any citations.",
				},
				FinishReason: "stop",
			},
		},
	}

	citations := extractCitationsFromResponse(&resp)
	assert.Nil(t, citations, "Response without citation markers should return nil")
}
