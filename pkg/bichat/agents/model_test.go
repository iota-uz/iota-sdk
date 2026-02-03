package agents_test

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

func TestModelInfo_Basic(t *testing.T) {
	t.Parallel()

	info := agents.ModelInfo{
		Name:     "gpt-5.2-2025-12-11",
		Provider: "openai",
		Capabilities: []agents.Capability{
			agents.CapabilityStreaming,
			agents.CapabilityTools,
			agents.CapabilityJSONMode,
		},
	}

	// Verify fields
	if info.Name != "gpt-5.2-2025-12-11" {
		t.Errorf("Expected name 'gpt-5.2-2025-12-11', got '%s'", info.Name)
	}
	if info.Provider != "openai" {
		t.Errorf("Expected provider 'openai', got '%s'", info.Provider)
	}
	if len(info.Capabilities) != 3 {
		t.Errorf("Expected 3 capabilities, got %d", len(info.Capabilities))
	}

	// Verify String() method
	expected := "openai/gpt-5.2-2025-12-11"
	if info.String() != expected {
		t.Errorf("Expected String() '%s', got '%s'", expected, info.String())
	}
}

func TestCapability_HasCapability(t *testing.T) {
	t.Parallel()

	info := agents.ModelInfo{
		Name:     "claude-3-5-sonnet",
		Provider: "anthropic",
		Capabilities: []agents.Capability{
			agents.CapabilityStreaming,
			agents.CapabilityTools,
			agents.CapabilityThinking,
		},
	}

	tests := []struct {
		name       string
		capability agents.Capability
		expected   bool
	}{
		{
			name:       "has streaming",
			capability: agents.CapabilityStreaming,
			expected:   true,
		},
		{
			name:       "has tools",
			capability: agents.CapabilityTools,
			expected:   true,
		},
		{
			name:       "has thinking",
			capability: agents.CapabilityThinking,
			expected:   true,
		},
		{
			name:       "does not have vision",
			capability: agents.CapabilityVision,
			expected:   false,
		},
		{
			name:       "does not have json mode",
			capability: agents.CapabilityJSONMode,
			expected:   false,
		},
		{
			name:       "does not have json schema",
			capability: agents.CapabilityJSONSchema,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := info.HasCapability(tt.capability)
			if result != tt.expected {
				t.Errorf("HasCapability(%s): expected %v, got %v", tt.capability, tt.expected, result)
			}
		})
	}
}

func TestCapability_EmptyCapabilities(t *testing.T) {
	t.Parallel()

	info := agents.ModelInfo{
		Name:         "basic-model",
		Provider:     "test",
		Capabilities: []agents.Capability{},
	}

	// Should return false for all capabilities
	capabilities := []agents.Capability{
		agents.CapabilityStreaming,
		agents.CapabilityTools,
		agents.CapabilityVision,
		agents.CapabilityThinking,
		agents.CapabilityJSONMode,
		agents.CapabilityJSONSchema,
	}

	for _, cap := range capabilities {
		if info.HasCapability(cap) {
			t.Errorf("Expected HasCapability(%s) to be false for empty capabilities", cap)
		}
	}
}

func TestMessage_Creation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		role     types.Role
		content  string
		factory  func() types.Message
	}{
		{
			name:    "user message",
			role:    types.RoleUser,
			content: "Hello, how are you?",
			factory: func() types.Message {
				return types.UserMessage("Hello, how are you?")
			},
		},
		{
			name:    "assistant message",
			role:    types.RoleAssistant,
			content: "I'm doing well, thank you!",
			factory: func() types.Message {
				return types.AssistantMessage("I'm doing well, thank you!")
			},
		},
		{
			name:    "system message",
			role:    types.RoleSystem,
			content: "You are a helpful assistant.",
			factory: func() types.Message {
				return types.SystemMessage("You are a helpful assistant.")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.factory()

			if msg.Role() != tt.role {
				t.Errorf("Expected role '%s', got '%s'", tt.role, msg.Role())
			}
			if msg.Content() != tt.content {
				t.Errorf("Expected content '%s', got '%s'", tt.content, msg.Content())
			}
		})
	}
}

func TestToolCall_Basic(t *testing.T) {
	t.Parallel()

	toolCall := types.ToolCall{
		ID:        "call_123",
		Name:      "get_weather",
		Arguments: `{"location":"San Francisco","unit":"celsius"}`,
	}

	// Verify fields
	if toolCall.ID != "call_123" {
		t.Errorf("Expected ID 'call_123', got '%s'", toolCall.ID)
	}
	if toolCall.Name != "get_weather" {
		t.Errorf("Expected Name 'get_weather', got '%s'", toolCall.Name)
	}
	if toolCall.Arguments != `{"location":"San Francisco","unit":"celsius"}` {
		t.Errorf("Unexpected Arguments: %s", toolCall.Arguments)
	}
}

func TestGenerateConfig_Options(t *testing.T) {
	t.Parallel()

	t.Run("WithMaxTokens", func(t *testing.T) {
		config := agents.ApplyGenerateOptions(agents.WithMaxTokens(1000))
		if config.MaxTokens == nil {
			t.Fatal("Expected MaxTokens to be set")
		}
		if *config.MaxTokens != 1000 {
			t.Errorf("Expected MaxTokens 1000, got %d", *config.MaxTokens)
		}
	})

	t.Run("WithReasoningEffort", func(t *testing.T) {
		config := agents.ApplyGenerateOptions(agents.WithReasoningEffort(agents.ReasoningHigh))
		if config.ReasoningEffort == nil {
			t.Fatal("Expected ReasoningEffort to be set")
		}
		if *config.ReasoningEffort != agents.ReasoningHigh {
			t.Errorf("Expected ReasoningHigh, got %s", *config.ReasoningEffort)
		}
	})

	t.Run("WithJSONMode", func(t *testing.T) {
		config := agents.ApplyGenerateOptions(agents.WithJSONMode())
		if !config.JSONMode {
			t.Error("Expected JSONMode to be true")
		}
	})

	t.Run("WithJSONSchema", func(t *testing.T) {
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
		}
		config := agents.ApplyGenerateOptions(agents.WithJSONSchema(schema))
		if config.JSONSchema == nil {
			t.Fatal("Expected JSONSchema to be set")
		}
	})

	t.Run("WithTemperature", func(t *testing.T) {
		config := agents.ApplyGenerateOptions(agents.WithTemperature(0.7))
		if config.Temperature == nil {
			t.Fatal("Expected Temperature to be set")
		}
		if *config.Temperature != 0.7 {
			t.Errorf("Expected Temperature 0.7, got %f", *config.Temperature)
		}
	})

	t.Run("Multiple options", func(t *testing.T) {
		config := agents.ApplyGenerateOptions(
			agents.WithMaxTokens(500),
			agents.WithTemperature(0.5),
			agents.WithReasoningEffort(agents.ReasoningMedium),
		)

		if config.MaxTokens == nil || *config.MaxTokens != 500 {
			t.Error("MaxTokens not set correctly")
		}
		if config.Temperature == nil || *config.Temperature != 0.5 {
			t.Error("Temperature not set correctly")
		}
		if config.ReasoningEffort == nil || *config.ReasoningEffort != agents.ReasoningMedium {
			t.Error("ReasoningEffort not set correctly")
		}
	})
}

func TestReasoningEffort_Values(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		effort   agents.ReasoningEffort
		expected string
	}{
		{
			name:     "low",
			effort:   agents.ReasoningLow,
			expected: "low",
		},
		{
			name:     "medium",
			effort:   agents.ReasoningMedium,
			expected: "medium",
		},
		{
			name:     "high",
			effort:   agents.ReasoningHigh,
			expected: "high",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.effort) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.effort))
			}
		})
	}
}

func TestCapability_Values(t *testing.T) {
	t.Parallel()

	capabilities := []struct {
		cap      agents.Capability
		expected string
	}{
		{agents.CapabilityStreaming, "streaming"},
		{agents.CapabilityTools, "tools"},
		{agents.CapabilityVision, "vision"},
		{agents.CapabilityThinking, "thinking"},
		{agents.CapabilityJSONMode, "json_mode"},
		{agents.CapabilityJSONSchema, "json_schema"},
	}

	for _, tc := range capabilities {
		if string(tc.cap) != tc.expected {
			t.Errorf("Expected capability value '%s', got '%s'", tc.expected, string(tc.cap))
		}
	}
}

func TestRequest_Creation(t *testing.T) {
	t.Parallel()

	testTool := agents.NewTool(
		"test_tool",
		"A test tool",
		map[string]interface{}{},
		func(ctx context.Context, input string) (string, error) {
			return "test", nil
		},
	)

	req := agents.Request{
		Messages: []types.Message{
			types.UserMessage("Hello"),
		},
		Tools: []agents.Tool{
			testTool,
		},
	}

	if len(req.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(req.Messages))
	}
	if len(req.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(req.Tools))
	}
	if req.Messages[0].Role() != types.RoleUser {
		t.Errorf("Expected role 'user', got '%s'", req.Messages[0].Role())
	}
}

func TestResponse_Creation(t *testing.T) {
	t.Parallel()

	resp := agents.Response{
		Message: types.AssistantMessage("Hello!"),
		Usage: types.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
		FinishReason: "stop",
		Thinking:     "I should greet the user",
	}

	if resp.Message.Role() != types.RoleAssistant {
		t.Errorf("Expected role 'assistant', got '%s'", resp.Message.Role())
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("Expected 15 total tokens, got %d", resp.Usage.TotalTokens)
	}
	if resp.FinishReason != "stop" {
		t.Errorf("Expected finish reason 'stop', got '%s'", resp.FinishReason)
	}
}

func TestChunk_Creation(t *testing.T) {
	t.Parallel()

	chunk := agents.Chunk{
		Delta:        "Hello",
		ToolCalls:    []types.ToolCall{},
		Usage:        nil,
		FinishReason: "",
		Done:         false,
	}

	if chunk.Delta != "Hello" {
		t.Errorf("Expected delta 'Hello', got '%s'", chunk.Delta)
	}
	if chunk.Done {
		t.Error("Expected Done to be false")
	}

	// Test final chunk
	finalChunk := agents.Chunk{
		Delta: "",
		Usage: &types.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
		FinishReason: "stop",
		Done:         true,
	}

	if !finalChunk.Done {
		t.Error("Expected Done to be true for final chunk")
	}
	if finalChunk.Usage == nil {
		t.Error("Expected Usage to be set for final chunk")
	}
	if finalChunk.FinishReason != "stop" {
		t.Errorf("Expected finish reason 'stop', got '%s'", finalChunk.FinishReason)
	}
}

func TestModelPricing_CalculateCost(t *testing.T) {
	t.Parallel()

	// Helper function for approximate float comparison
	floatEquals := func(a, b float64) bool {
		const epsilon = 0.000001
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff < epsilon
	}

	t.Run("Basic cost calculation", func(t *testing.T) {
		pricing := agents.ModelPricing{
			Currency:        "USD",
			InputPer1M:      2.50,
			OutputPer1M:     10.00,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		}

		usage := types.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			TotalTokens:      1500,
		}

		cost := pricing.CalculateCost(usage)
		// Expected: (1000/1M * 2.50) + (500/1M * 10.00) = 0.0025 + 0.005 = 0.0075
		expected := 0.0075
		if !floatEquals(cost, expected) {
			t.Errorf("Expected cost %.6f, got %.6f", expected, cost)
		}
	})

	t.Run("Cost with cache read tokens", func(t *testing.T) {
		pricing := agents.ModelPricing{
			Currency:        "USD",
			InputPer1M:      2.50,
			OutputPer1M:     10.00,
			CacheWritePer1M: 1.25,
			CacheReadPer1M:  0.625,
		}

		usage := types.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			CacheWriteTokens: 0,
			CacheReadTokens:  2000,
			TotalTokens:      3500,
		}

		cost := pricing.CalculateCost(usage)
		// Expected:
		// Input: (1000/1M * 2.50) = 0.0025
		// Output: (500/1M * 10.00) = 0.0050
		// Cache Write: (0/1M * 1.25) = 0.0000
		// Cache Read: (2000/1M * 0.625) = 0.00125
		// Total: 0.00875
		expected := 0.00875
		if !floatEquals(cost, expected) {
			t.Errorf("Expected cost %.6f, got %.6f", expected, cost)
		}
	})

	t.Run("Cost with cache write and read tokens", func(t *testing.T) {
		pricing := agents.ModelPricing{
			Currency:        "USD",
			InputPer1M:      0.150,
			OutputPer1M:     0.600,
			CacheWritePer1M: 0.075,
			CacheReadPer1M:  0.038,
		}

		usage := types.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			CacheWriteTokens: 2000,
			CacheReadTokens:  1000,
			TotalTokens:      4500,
		}

		cost := pricing.CalculateCost(usage)
		// Expected:
		// Input: (1000/1M * 0.150) = 0.00015
		// Output: (500/1M * 0.600) = 0.00030
		// Cache Write: (2000/1M * 0.075) = 0.00015
		// Cache Read: (1000/1M * 0.038) = 0.000038
		// Total: 0.000638
		expected := 0.000638
		if !floatEquals(cost, expected) {
			t.Errorf("Expected cost %.6f, got %.6f", expected, cost)
		}
	})

	t.Run("Cost with zero cache pricing", func(t *testing.T) {
		pricing := agents.ModelPricing{
			Currency:        "USD",
			InputPer1M:      0.50,
			OutputPer1M:     1.50,
			CacheWritePer1M: 0,
			CacheReadPer1M:  0,
		}

		usage := types.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 500,
			CacheWriteTokens: 0,
			CacheReadTokens:  500,
			TotalTokens:      2000,
		}

		cost := pricing.CalculateCost(usage)
		// Expected (cache tokens ignored):
		// Input: (1000/1M * 0.50) = 0.0005
		// Output: (500/1M * 1.50) = 0.00075
		// Cache tokens: 0 (pricing is 0)
		// Total: 0.00125
		expected := 0.00125
		if !floatEquals(cost, expected) {
			t.Errorf("Expected cost %.6f, got %.6f", expected, cost)
		}
	})
}
