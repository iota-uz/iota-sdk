package llmproviders

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/openai/openai-go/v3/responses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOpenAIModel_MissingAPIKey(t *testing.T) {
	require.NoError(t, os.Unsetenv("OPENAI_API_KEY"))

	_, err := NewOpenAIModel()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY")
}

func TestNewOpenAIModel_WithAPIKey(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { require.NoError(t, os.Unsetenv("OPENAI_API_KEY")) }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	assert.NotNil(t, model)
}

func TestNewOpenAIModel_DefaultModel(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { require.NoError(t, os.Unsetenv("OPENAI_API_KEY")) }()
	require.NoError(t, os.Unsetenv("OPENAI_MODEL"))

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	oaiModel := model.(*OpenAIModel)
	assert.Equal(t, "gpt-5.2", oaiModel.modelName)
}

func TestNewOpenAIModel_CustomModel(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	require.NoError(t, os.Setenv("OPENAI_MODEL", "gpt-5.2"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY"); _ = os.Unsetenv("OPENAI_MODEL") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	oaiModel := model.(*OpenAIModel)
	assert.Equal(t, "gpt-5.2", oaiModel.modelName)
}

func TestOpenAIModel_Info(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	require.NoError(t, os.Setenv("OPENAI_MODEL", "gpt-5-mini"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY"); _ = os.Unsetenv("OPENAI_MODEL") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	info := model.Info()
	assert.Equal(t, "gpt-5-mini", info.Name)
	assert.Equal(t, "openai", info.Provider)
	assert.Equal(t, 400000, info.ContextWindow)
	assert.Contains(t, info.Capabilities, agents.CapabilityStreaming)
	assert.Contains(t, info.Capabilities, agents.CapabilityTools)
	assert.Contains(t, info.Capabilities, agents.CapabilityJSONMode)
}

func TestOpenAIModel_Info_DefaultGPT52ContextWindow(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	require.NoError(t, os.Unsetenv("OPENAI_MODEL"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY"); _ = os.Unsetenv("OPENAI_MODEL") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	info := model.Info()
	assert.Equal(t, "gpt-5.2", info.Name)
	assert.Equal(t, 400000, info.ContextWindow)
}

func TestOpenAIModel_Info_ContextWindowFromCatalog(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	tests := []struct {
		name       string
		modelEnv   string
		expectCtx  int
		expectName string
	}{
		{name: "canonical gpt-5.2", modelEnv: "gpt-5.2", expectCtx: 400000, expectName: "gpt-5.2"},
		{name: "versioned alias", modelEnv: "gpt-5.2-2025-12-11", expectCtx: 400000, expectName: "gpt-5.2-2025-12-11"},
		{name: "normalized alias", modelEnv: " GPT-5.2-2025-12-11 ", expectCtx: 400000, expectName: " GPT-5.2-2025-12-11 "},
		{name: "gpt-5-mini", modelEnv: "gpt-5-mini", expectCtx: 400000, expectName: "gpt-5-mini"},
		{name: "gpt-5-nano", modelEnv: "gpt-5-nano", expectCtx: 400000, expectName: "gpt-5-nano"},
		{name: "unknown falls back to default spec", modelEnv: "unknown-model", expectCtx: 400000, expectName: "unknown-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.Setenv("OPENAI_MODEL", tt.modelEnv))
			defer func() { _ = os.Unsetenv("OPENAI_MODEL") }()

			model, err := NewOpenAIModel()
			require.NoError(t, err)

			info := model.Info()
			assert.Equal(t, tt.expectName, info.Name)
			assert.Equal(t, "openai", info.Provider)
			assert.Equal(t, tt.expectCtx, info.ContextWindow)
		})
	}
}

func TestOpenAIModel_HasCapability(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	assert.True(t, model.HasCapability(agents.CapabilityStreaming))
	assert.True(t, model.HasCapability(agents.CapabilityTools))
	assert.True(t, model.HasCapability(agents.CapabilityJSONMode))
	assert.False(t, model.HasCapability(agents.CapabilityThinking))
}

func TestOpenAIModel_BuildResponseParams(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	req := agents.Request{
		Messages: []types.Message{
			types.SystemMessage("You are a helpful assistant"),
			types.UserMessage("Hello"),
		},
		PreviousResponseID: func() *string {
			id := "resp_prev_123"
			return &id
		}(),
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

	params := oaiModel.buildResponseParams(context.Background(), req, config)

	// Verify model
	assert.Equal(t, "gpt-5.2", params.Model)

	// Verify input items
	assert.NotNil(t, params.Input.OfInputItemList)
	assert.Len(t, params.Input.OfInputItemList, 2)

	// Verify tools — test_tool should be a function tool
	assert.Len(t, params.Tools, 1)
	assert.NotNil(t, params.Tools[0].OfFunction)
	assert.Equal(t, "test_tool", params.Tools[0].OfFunction.Name)

	// Verify max tokens
	assert.True(t, params.MaxOutputTokens.Valid())
	assert.Equal(t, int64(100), params.MaxOutputTokens.Value)

	// Verify temperature
	assert.True(t, params.Temperature.Valid())
	assert.InDelta(t, 0.7, params.Temperature.Value, 1e-6)

	// Verify response continuity + server-side state storage
	assert.True(t, params.Store.Valid())
	assert.True(t, params.Store.Value)
	assert.True(t, params.PreviousResponseID.Valid())
	assert.Equal(t, "resp_prev_123", params.PreviousResponseID.Value)

	// Verify JSON mode
	assert.NotNil(t, params.Text.Format.OfJSONObject)
}

func TestOpenAIModel_BuildResponseParams_NativeWebSearch(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	req := agents.Request{
		Messages: []types.Message{
			types.UserMessage("Search for current weather"),
		},
		Tools: []agents.Tool{
			agents.NewTool(
				"web_search",
				"Search the web",
				map[string]any{"type": "object"},
				func(ctx context.Context, input string) (string, error) {
					return "", nil
				},
			),
		},
	}

	config := agents.GenerateConfig{}
	params := oaiModel.buildResponseParams(context.Background(), req, config)

	// web_search should be added as native WebSearchToolParam
	require.Len(t, params.Tools, 1)
	assert.NotNil(t, params.Tools[0].OfWebSearch, "web_search should be a native WebSearchToolParam")
	assert.Equal(t, responses.WebSearchToolTypeWebSearch, params.Tools[0].OfWebSearch.Type)

	// Should request sources in include
	assert.Contains(t, params.Include, responses.ResponseIncludableWebSearchCallActionSources)
}

func TestOpenAIModel_BuildResponseParams_NativeCodeInterpreter(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

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
					return "", nil
				},
			),
		},
	}

	config := agents.GenerateConfig{}
	params := oaiModel.buildResponseParams(context.Background(), req, config)

	// code_interpreter should be added as native ToolCodeInterpreterParam
	require.Len(t, params.Tools, 1)
	assert.NotNil(t, params.Tools[0].OfCodeInterpreter, "code_interpreter should be a native ToolCodeInterpreterParam")
	assert.Equal(t, "4g", params.Tools[0].OfCodeInterpreter.Container.OfCodeInterpreterToolAuto.MemoryLimit)

	// Should request outputs in include
	assert.Contains(t, params.Include, responses.ResponseIncludableCodeInterpreterCallOutputs)
}

func TestOpenAIModel_BuildResponseParams_CodeInterpreterMemoryLimitOverride(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel(WithCodeInterpreterMemoryLimit("16g"))
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	req := agents.Request{
		Messages: []types.Message{
			types.UserMessage("Run code"),
		},
		Tools: []agents.Tool{
			agents.NewTool(
				"code_interpreter",
				"Execute code",
				map[string]any{"type": "object"},
				func(ctx context.Context, input string) (string, error) {
					return "", nil
				},
			),
		},
	}

	params := oaiModel.buildResponseParams(context.Background(), req, agents.GenerateConfig{})
	require.Len(t, params.Tools, 1)
	require.NotNil(t, params.Tools[0].OfCodeInterpreter)
	assert.Equal(t, "16g", params.Tools[0].OfCodeInterpreter.Container.OfCodeInterpreterToolAuto.MemoryLimit)
}

func TestOpenAIModel_BuildResponseParams_MixedTools(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	makeTool := func(name, desc string) agents.Tool {
		return agents.NewTool(name, desc, map[string]any{"type": "object"},
			func(ctx context.Context, input string) (string, error) { return "", nil })
	}

	req := agents.Request{
		Messages: []types.Message{types.UserMessage("test")},
		Tools: []agents.Tool{
			makeTool("sql_execute", "Execute SQL"),
			makeTool("web_search", "Search the web"),
			makeTool("code_interpreter", "Run code"),
			makeTool("schema_list", "List schemas"),
		},
	}

	config := agents.GenerateConfig{}
	params := oaiModel.buildResponseParams(context.Background(), req, config)

	// Should have 4 tools: 2 function + 1 web_search + 1 code_interpreter
	require.Len(t, params.Tools, 4)

	// Count tool types
	var funcCount, webCount, codeCount int
	for _, tool := range params.Tools {
		switch {
		case tool.OfFunction != nil:
			funcCount++
		case tool.OfWebSearch != nil:
			webCount++
		case tool.OfCodeInterpreter != nil:
			codeCount++
		}
	}

	assert.Equal(t, 2, funcCount, "should have 2 function tools")
	assert.Equal(t, 1, webCount, "should have 1 web search tool")
	assert.Equal(t, 1, codeCount, "should have 1 code interpreter tool")
}

func TestOpenAIModel_MapResponse_TextOnly(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	resp := &responses.Response{
		ID: "resp_abc123",
		Output: []responses.ResponseOutputItemUnion{
			{
				Type: "message",
				Content: []responses.ResponseOutputMessageContentUnion{
					{
						Type: "output_text",
						Text: "Hello, world!",
					},
				},
			},
		},
		Usage: responses.ResponseUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
		Status: "completed",
	}

	agentResp, err := oaiModel.mapResponse(resp)
	require.NoError(t, err)

	assert.Equal(t, "Hello, world!", agentResp.Message.Content())
	assert.Equal(t, "stop", agentResp.FinishReason)
	assert.Empty(t, agentResp.Message.ToolCalls())
	assert.Equal(t, 10, agentResp.Usage.PromptTokens)
	assert.Equal(t, 5, agentResp.Usage.CompletionTokens)
	assert.Equal(t, 15, agentResp.Usage.TotalTokens)
	assert.Equal(t, "resp_abc123", agentResp.ProviderResponseID)
}

func TestOpenAIModel_MapResponse_FunctionCalls(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	resp := &responses.Response{
		Output: []responses.ResponseOutputItemUnion{
			{
				Type:      "function_call",
				CallID:    "call_abc",
				Name:      "sql_execute",
				Arguments: `{"query":"SELECT 1"}`,
			},
		},
		Usage: responses.ResponseUsage{
			InputTokens:  20,
			OutputTokens: 10,
			TotalTokens:  30,
		},
		Status: "completed",
	}

	agentResp, err := oaiModel.mapResponse(resp)
	require.NoError(t, err)

	assert.Equal(t, "tool_calls", agentResp.FinishReason)
	require.Len(t, agentResp.Message.ToolCalls(), 1)

	tc := agentResp.Message.ToolCalls()[0]
	assert.Equal(t, "call_abc", tc.ID)
	assert.Equal(t, "sql_execute", tc.Name)
	assert.JSONEq(t, `{"query":"SELECT 1"}`, tc.Arguments)
}

func TestOpenAIModel_MapResponse_WithCitations(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	resp := &responses.Response{
		Output: []responses.ResponseOutputItemUnion{
			{
				Type: "message",
				Content: []responses.ResponseOutputMessageContentUnion{
					{
						Type: "output_text",
						Text: "The sky is blue according to science.",
						Annotations: []responses.ResponseOutputTextAnnotationUnion{
							{
								Type:       "url_citation",
								Title:      "Why is the sky blue?",
								URL:        "https://example.com/sky",
								StartIndex: 0,
								EndIndex:   18,
							},
						},
					},
				},
			},
		},
		Usage:  responses.ResponseUsage{InputTokens: 5, OutputTokens: 8, TotalTokens: 13},
		Status: "completed",
	}

	agentResp, err := oaiModel.mapResponse(resp)
	require.NoError(t, err)

	assert.True(t, agentResp.Message.HasCitations())
	require.Len(t, agentResp.Message.Citations(), 1)

	cite := agentResp.Message.Citations()[0]
	assert.Equal(t, "web", cite.Type)
	assert.Equal(t, "Why is the sky blue?", cite.Title)
	assert.Equal(t, "https://example.com/sky", cite.URL)
	assert.Equal(t, 0, cite.StartIndex)
	assert.Equal(t, 18, cite.EndIndex)
}

func TestOpenAIModel_MapResponse_CodeInterpreterCall(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	resp := &responses.Response{
		Output: []responses.ResponseOutputItemUnion{
			{
				Type:        "code_interpreter_call",
				ID:          "ci_123",
				Code:        "print('hello')",
				ContainerID: "container_abc",
				Status:      "completed",
				Outputs: []responses.ResponseCodeInterpreterToolCallOutputUnion{
					{
						Type: "logs",
						Logs: "hello\n",
					},
				},
			},
			{
				Type: "message",
				Content: []responses.ResponseOutputMessageContentUnion{
					{
						Type: "output_text",
						Text: "I executed the code.",
					},
				},
			},
		},
		Usage:  responses.ResponseUsage{InputTokens: 10, OutputTokens: 15, TotalTokens: 25},
		Status: "completed",
	}

	agentResp, err := oaiModel.mapResponse(resp)
	require.NoError(t, err)

	assert.Equal(t, "I executed the code.", agentResp.Message.Content())
	assert.Equal(t, "stop", agentResp.FinishReason)

	require.Len(t, agentResp.CodeInterpreterResults, 1)
	ciResult := agentResp.CodeInterpreterResults[0]
	assert.Equal(t, "ci_123", ciResult.ID)
	assert.Equal(t, "print('hello')", ciResult.Code)
	assert.Equal(t, "container_abc", ciResult.ContainerID)
	assert.Equal(t, "completed", ciResult.Status)
	assert.Equal(t, "hello\n", ciResult.Logs)
}

func TestOpenAIModel_BuildInputItems(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	// System + User + Assistant with tool calls + Tool response
	messages := []types.Message{
		types.SystemMessage("You are helpful"),
		types.UserMessage("What is 2+2?"),
		types.AssistantMessage("", types.WithToolCalls(types.ToolCall{
			ID:        "call_1",
			Name:      "calculate",
			Arguments: `{"expr":"2+2"}`,
		})),
		types.ToolResponse("call_1", `{"result": 4}`),
		types.AssistantMessage("The answer is 4."),
	}

	items := oaiModel.buildInputItems(messages)

	// System → developer message, User → user message, Assistant with tool calls → function_call,
	// Tool → function_call_output, Assistant → assistant message
	assert.Len(t, items, 5)
}

func TestOpenAIModel_BuildInputItems_AssistantWithContentAndToolCalls(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	// Assistant message with BOTH text content and tool calls
	messages := []types.Message{
		types.AssistantMessage("Let me check that for you.", types.WithToolCalls(types.ToolCall{
			ID:        "call_1",
			Name:      "sql_execute",
			Arguments: `{"query":"SELECT 1"}`,
		})),
	}

	items := oaiModel.buildInputItems(messages)

	// Should emit BOTH an assistant message AND a function_call item
	assert.Len(t, items, 2, "should emit both assistant message and function_call")
}

func TestOpenAIModel_BuildInputItems_SkipsInvalidToolCalls(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	messages := []types.Message{
		types.AssistantMessage("Running checks...", types.WithToolCalls(
			types.ToolCall{
				ID:        "call_valid",
				Name:      "sql_execute",
				Arguments: `{"query":"SELECT 1"}`,
			},
			types.ToolCall{
				ID:        "call_invalid_name",
				Name:      "",
				Arguments: `{"query":"SELECT 2"}`,
			},
			types.ToolCall{
				ID:        "",
				Name:      "schema_list",
				Arguments: `{}`,
			},
		)),
		types.ToolResponse("call_invalid_name", `{"rows":[]}`),
		types.ToolResponse("call_valid", `{"rows":[[1]]}`),
		types.ToolResponse("   ", `{"rows":[[2]]}`),
	}

	items := oaiModel.buildInputItems(messages)

	// assistant message + valid function_call + valid function_call_output
	assert.Len(t, items, 3)
}

func TestOpenAIModel_BuildInputItems_WebFetchCases(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	cases := []struct {
		name        string
		messages    []types.Message
		contains    []string
		notContains []string
	}{
		{
			name: "web_fetch_image_rich_output",
			messages: []types.Message{
				types.AssistantMessage("", types.WithToolCalls(types.ToolCall{
					ID:        "call_web_1",
					Name:      "web_fetch",
					Arguments: `{"url":"https://example.com/chart.png"}`,
				})),
				types.ToolResponse("call_web_1", `{"source_url":"https://example.com/chart.png","content_type":"image/png","size_bytes":1234,"injectable":true,"injection_type":"input_image","injection_url":"https://example.com/chart.png","saved":false,"filename":"chart.png"}`),
			},
			contains:    []string{`"input_text"`, `"input_image"`, "https://example.com/chart.png"},
			notContains: nil,
		},
		{
			name: "web_fetch_pdf_rich_output",
			messages: []types.Message{
				types.AssistantMessage("", types.WithToolCalls(types.ToolCall{
					ID:        "call_web_2",
					Name:      "web_fetch",
					Arguments: `{"url":"https://example.com/report.pdf"}`,
				})),
				types.ToolResponse("call_web_2", `{"source_url":"https://example.com/report.pdf","content_type":"application/pdf","size_bytes":2222,"injectable":true,"injection_type":"input_file","injection_url":"https://example.com/report.pdf","saved":false,"filename":"report.pdf"}`),
			},
			contains:    []string{`"input_text"`, `"input_file"`, `"report.pdf"`},
			notContains: nil,
		},
		{
			name: "non_web_fetch_tool_output_unchanged",
			messages: []types.Message{
				types.AssistantMessage("", types.WithToolCalls(types.ToolCall{
					ID:        "call_sql_1",
					Name:      "sql_execute",
					Arguments: `{"query":"SELECT 1"}`,
				})),
				types.ToolResponse("call_sql_1", `{"rows":[[1]]}`),
			},
			contains:    []string{`{\"rows\":[[1]]}`},
			notContains: []string{`"input_image"`, `"input_file"`},
		},
		{
			name: "web_fetch_invalid_json_falls_back_to_string",
			messages: []types.Message{
				types.AssistantMessage("", types.WithToolCalls(types.ToolCall{
					ID:        "call_web_3",
					Name:      "web_fetch",
					Arguments: `{"url":"https://example.com/file"}`,
				})),
				types.ToolResponse("call_web_3", `not-json`),
			},
			contains:    []string{"not-json"},
			notContains: []string{`"input_image"`, `"input_file"`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model, err := NewOpenAIModel()
			require.NoError(t, err)
			oaiModel := model.(*OpenAIModel)

			items := oaiModel.buildInputItems(tc.messages)
			serialized, err := json.Marshal(items)
			require.NoError(t, err)
			payload := string(serialized)

			for _, sub := range tc.contains {
				assert.Contains(t, payload, sub, "payload should contain %q", sub)
			}
			for _, sub := range tc.notContains {
				assert.NotContains(t, payload, sub, "payload should not contain %q", sub)
			}
		})
	}
}

func TestOpenAIModel_BuildInputItems_OnlyImagesBecomeInputImage(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)
	oaiModel := model.(*OpenAIModel)

	messages := []types.Message{
		types.UserMessage(
			"Analyze attached files",
			types.WithAttachments(
				types.Attachment{
					FileName: "chart.png",
					MimeType: "image/png",
					FilePath: "https://example.com/chart.png",
				},
				types.Attachment{
					FileName: "report.pdf",
					MimeType: "application/pdf",
					FilePath: "https://example.com/report.pdf",
				},
			),
		),
	}

	items := oaiModel.buildInputItems(messages)
	serialized, err := json.Marshal(items)
	require.NoError(t, err)

	payload := string(serialized)
	assert.Equal(t, 1, strings.Count(payload, `"input_image"`))
	assert.Contains(t, payload, "chart.png")
	assert.Contains(t, payload, "report.pdf")
	assert.Contains(t, payload, "artifact_reader")
}

func TestFunctionCallItemKey(t *testing.T) {
	t.Run("prefers output item id", func(t *testing.T) {
		key := functionCallItemKey(responses.ResponseOutputItemUnion{
			ID:     "fc_123",
			CallID: "call_123",
		}, "")
		assert.Equal(t, "fc_123", key)
	})

	t.Run("falls back to event item_id", func(t *testing.T) {
		key := functionCallItemKey(responses.ResponseOutputItemUnion{
			CallID: "call_123",
		}, "fc_fallback")
		assert.Equal(t, "fc_fallback", key)
	})

	t.Run("falls back to call_id when item ids missing", func(t *testing.T) {
		key := functionCallItemKey(responses.ResponseOutputItemUnion{
			CallID: "call_123",
		}, "")
		assert.Equal(t, "call_123", key)
	})
}

func TestOpenAIModel_BuildToolCallsFromAccum_DeduplicatesByCallID(t *testing.T) {
	m := &OpenAIModel{}
	accum := map[string]*toolCallAccumEntry{
		"call_abc": {
			id:   "call_abc",
			name: "sql_execute",
			args: `{"query":"SELECT 1"}`,
		},
		"fc_123": {
			id:     "fc_123",
			callID: "call_abc",
			name:   "sql_execute",
			args:   `{"query":"SELECT 2"}`,
		},
	}

	calls := m.buildToolCallsFromAccum(accum, []string{"call_abc", "fc_123"})
	require.Len(t, calls, 1)
	assert.Equal(t, "call_abc", calls[0].ID)
	assert.Equal(t, "sql_execute", calls[0].Name)
	assert.JSONEq(t, `{"query":"SELECT 2"}`, calls[0].Arguments)
}

func TestOpenAIModel_BuildReadyToolCallsFromAccum_DeduplicatesByCallID(t *testing.T) {
	m := &OpenAIModel{}
	accum := map[string]*toolCallAccumEntry{
		"fc_1": {
			id:     "fc_1",
			callID: "call_1",
			name:   "sql_execute",
			args:   `{"query":"SELECT 1"}`,
		},
		"fc_1_dup": {
			id:     "fc_1_dup",
			callID: "call_1",
			name:   "sql_execute",
			args:   `{"query":"SELECT 1"}`,
		},
		"fc_2": {
			id:     "fc_2",
			callID: "call_2",
			name:   "schema_list",
			args:   `{}`,
		},
	}

	calls := m.buildReadyToolCallsFromAccum(accum, []string{"fc_1", "fc_1_dup", "fc_2"})
	require.Len(t, calls, 2)
	assert.Equal(t, "call_1", calls[0].ID)
	assert.Equal(t, "call_2", calls[1].ID)
}

func TestOpenAIModel_Pricing(t *testing.T) {
	require.NoError(t, os.Setenv("OPENAI_API_KEY", "sk-test-key"))
	defer func() { _ = os.Unsetenv("OPENAI_API_KEY") }()

	// Pricing() comes from catalog; exact numbers are catalog data. Only assert behavior:
	// unknown model falls back to default spec and returns valid pricing usable for cost calculation.
	require.NoError(t, os.Setenv("OPENAI_MODEL", "gpt-99-future"))
	defer func() { _ = os.Unsetenv("OPENAI_MODEL") }()

	model, err := NewOpenAIModel()
	require.NoError(t, err)

	pricing := model.Pricing()
	assert.Equal(t, "USD", pricing.Currency)
	assert.Greater(t, pricing.InputPer1M, 0.)
	assert.Greater(t, pricing.OutputPer1M, 0.)
	// Smoke-check: CalculateCost doesn't panic and returns non-negative for typical usage
	cost := pricing.CalculateCost(types.TokenUsage{PromptTokens: 1000, CompletionTokens: 500})
	assert.GreaterOrEqual(t, cost, 0.)
}
