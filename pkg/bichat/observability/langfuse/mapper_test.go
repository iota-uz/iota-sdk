package langfuse

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/henomis/langfuse-go/model"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/stretchr/testify/assert"
)

// 7. Mapper Tests (2 tests)

func TestMappers_AllFields(t *testing.T) {
	t.Parallel()

	t.Run("mapGenerationToLangfuse", func(t *testing.T) {
		obs := observability.GenerationObservation{
			ID:               uuid.New().String(),
			TraceID:          uuid.New().String(),
			TenantID:         uuid.New(),
			SessionID:        uuid.New(),
			Timestamp:        time.Now(),
			Model:            "claude-3-5-sonnet-20241022",
			Provider:         "anthropic",
			PromptMessages:   5,
			PromptTokens:     1000,
			Tools:            3,
			PromptContent:    "What is the revenue?",
			CompletionTokens: 500,
			TotalTokens:      1500,
			LatencyMs:        2000,
			FinishReason:     "stop",
			ToolCalls:        1,
			CompletionText:   "The revenue is $1M",
			Duration:         2 * time.Second,
			Attributes: map[string]interface{}{
				"custom_field":        "custom_value",
				"cache_read_tokens":   100,
				"cache_write_tokens":  50,
				"input_price_per_1m":  3.0,
				"output_price_per_1m": 15.0,
			},
		}

		metadata := mapGenerationToLangfuse(obs)

		// Core fields
		assert.Equal(t, "claude-3-5-sonnet-20241022", metadata["model"])
		assert.Equal(t, "anthropic", metadata["provider"])
		assert.Equal(t, "stop", metadata["finish_reason"])

		// Request details
		assert.Equal(t, 5, metadata["prompt_messages"])
		assert.Equal(t, 3, metadata["tools_count"])
		assert.Equal(t, 1, metadata["tool_calls_count"])

		// Optional content
		assert.Equal(t, "What is the revenue?", metadata["prompt_content"])
		assert.Equal(t, "The revenue is $1M", metadata["completion_text"])

		// Custom attributes (merged)
		assert.Equal(t, "custom_value", metadata["custom_field"])
		assert.Equal(t, 100, metadata["cache_read_tokens"])
		assert.Equal(t, 50, metadata["cache_write_tokens"])
		assert.Equal(t, 3.0, metadata["input_price_per_1m"])
		assert.Equal(t, 15.0, metadata["output_price_per_1m"])
	})

	t.Run("mapGenerationToLangfuse - minimal fields", func(t *testing.T) {
		obs := observability.GenerationObservation{
			ID:               uuid.New().String(),
			TraceID:          uuid.New().String(),
			TenantID:         uuid.New(),
			SessionID:        uuid.New(),
			Timestamp:        time.Now(),
			Model:            "gpt-4",
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		}

		metadata := mapGenerationToLangfuse(obs)

		// Should include model
		assert.Equal(t, "gpt-4", metadata["model"])

		// Should not include empty optional fields
		assert.NotContains(t, metadata, "provider")
		assert.NotContains(t, metadata, "finish_reason")
		assert.NotContains(t, metadata, "prompt_content")
		assert.NotContains(t, metadata, "completion_text")
		assert.NotContains(t, metadata, "tools_count")
	})

	t.Run("extractTokenUsage", func(t *testing.T) {
		obs := observability.GenerationObservation{
			PromptTokens:     500,
			CompletionTokens: 250,
			TotalTokens:      750,
			Attributes: map[string]interface{}{
				"cache_read_tokens":  100,
				"cache_write_tokens": 50,
			},
		}

		usage := extractTokenUsage(obs)

		assert.Equal(t, 500, usage["input"])
		assert.Equal(t, 250, usage["output"])
		assert.Equal(t, 750, usage["total"])
		assert.Equal(t, 100, usage["cache_read"])
		assert.Equal(t, 50, usage["cache_write"])
	})

	t.Run("extractTokenUsage - no cache tokens", func(t *testing.T) {
		obs := observability.GenerationObservation{
			PromptTokens:     200,
			CompletionTokens: 100,
			TotalTokens:      300,
		}

		usage := extractTokenUsage(obs)

		assert.Equal(t, 200, usage["input"])
		assert.Equal(t, 100, usage["output"])
		assert.Equal(t, 300, usage["total"])
		assert.NotContains(t, usage, "cache_read")
		assert.NotContains(t, usage, "cache_write")
	})

	t.Run("mapSpanToLangfuse", func(t *testing.T) {
		obs := observability.SpanObservation{
			ID:        uuid.New().String(),
			TraceID:   uuid.New().String(),
			ParentID:  "parent-span-123",
			TenantID:  uuid.New(),
			SessionID: uuid.New(),
			Timestamp: time.Now(),
			Name:      "execute_sql",
			Type:      "tool",
			Input:     `{"query": "SELECT * FROM users"}`,
			Output:    `{"rows": 10}`,
			Duration:  150 * time.Millisecond,
			Status:    "success",
			ToolName:  "sql_execute",
			CallID:    "call-456",
			Attributes: map[string]interface{}{
				"database": "postgres",
				"table":    "users",
			},
		}

		metadata := mapSpanToLangfuse(obs)

		// Core fields
		assert.Equal(t, "tool", metadata["span_type"])
		assert.Equal(t, "success", metadata["status"])

		// Tool-specific fields
		assert.Equal(t, "sql_execute", metadata["tool_name"])
		assert.Equal(t, "call-456", metadata["call_id"])

		// Input/output
		assert.Equal(t, `{"query": "SELECT * FROM users"}`, metadata["input"])
		assert.Equal(t, `{"rows": 10}`, metadata["output"])

		// Custom attributes
		assert.Equal(t, "postgres", metadata["database"])
		assert.Equal(t, "users", metadata["table"])
	})

	t.Run("mapSpanToLangfuse - minimal fields", func(t *testing.T) {
		obs := observability.SpanObservation{
			ID:        uuid.New().String(),
			TraceID:   uuid.New().String(),
			TenantID:  uuid.New(),
			SessionID: uuid.New(),
			Timestamp: time.Now(),
			Name:      "custom_operation",
		}

		metadata := mapSpanToLangfuse(obs)

		// Should not include empty optional fields
		assert.NotContains(t, metadata, "span_type")
		assert.NotContains(t, metadata, "status")
		assert.NotContains(t, metadata, "tool_name")
		assert.NotContains(t, metadata, "call_id")
		assert.NotContains(t, metadata, "input")
		assert.NotContains(t, metadata, "output")
	})

	t.Run("mapEventToLangfuse", func(t *testing.T) {
		obs := observability.EventObservation{
			ID:        uuid.New().String(),
			TraceID:   uuid.New().String(),
			TenantID:  uuid.New(),
			SessionID: uuid.New(),
			Timestamp: time.Now(),
			Name:      "interrupt",
			Type:      "session",
			Message:   "User clarification required",
			Level:     "warn",
			Attributes: map[string]interface{}{
				"question_id":    "q-789",
				"interrupt_type": "question",
			},
		}

		metadata := mapEventToLangfuse(obs)

		// Core fields
		assert.Equal(t, "session", metadata["event_type"])
		assert.Equal(t, "User clarification required", metadata["message"])
		assert.Equal(t, "warn", metadata["level"])

		// Custom attributes
		assert.Equal(t, "q-789", metadata["question_id"])
		assert.Equal(t, "question", metadata["interrupt_type"])
	})

	t.Run("mapEventToLangfuse - minimal fields", func(t *testing.T) {
		obs := observability.EventObservation{
			ID:        uuid.New().String(),
			TraceID:   uuid.New().String(),
			TenantID:  uuid.New(),
			SessionID: uuid.New(),
			Timestamp: time.Now(),
			Name:      "simple_event",
		}

		metadata := mapEventToLangfuse(obs)

		// Should not include empty optional fields
		assert.NotContains(t, metadata, "event_type")
		assert.NotContains(t, metadata, "message")
		assert.NotContains(t, metadata, "level")
	})

	t.Run("mapTraceToLangfuse", func(t *testing.T) {
		userID := uuid.New()
		obs := observability.TraceObservation{
			ID:          uuid.New().String(),
			TenantID:    uuid.New(),
			SessionID:   uuid.New(),
			Timestamp:   time.Now(),
			Name:        "BI Analysis Session",
			Duration:    10 * time.Minute,
			Status:      "completed",
			UserID:      userID,
			TotalCost:   0.05,
			TotalTokens: 2500,
			Attributes: map[string]interface{}{
				"agent":      "default_agent",
				"tool_count": 5,
			},
		}

		metadata := mapTraceToLangfuse(obs)

		// Core fields
		assert.Equal(t, "completed", metadata["status"])
		assert.Equal(t, userID.String(), metadata["user_id"])
		assert.Equal(t, 0.05, metadata["total_cost"])
		assert.Equal(t, 2500, metadata["total_tokens"])

		// Custom attributes
		assert.Equal(t, "default_agent", metadata["agent"])
		assert.Equal(t, 5, metadata["tool_count"])
	})

	t.Run("mapTraceToLangfuse - zero UUID", func(t *testing.T) {
		obs := observability.TraceObservation{
			ID:        uuid.New().String(),
			TenantID:  uuid.New(),
			SessionID: uuid.New(),
			Timestamp: time.Now(),
			Name:      "Trace",
			UserID:    uuid.UUID{}, // Zero UUID
		}

		metadata := mapTraceToLangfuse(obs)

		// Should not include zero UUID
		assert.NotContains(t, metadata, "user_id")
	})

	t.Run("mapTraceToLangfuse - zero cost and tokens", func(t *testing.T) {
		obs := observability.TraceObservation{
			ID:          uuid.New().String(),
			TenantID:    uuid.New(),
			SessionID:   uuid.New(),
			Timestamp:   time.Now(),
			Name:        "Trace",
			UserID:      uuid.New(),
			TotalCost:   0,
			TotalTokens: 0,
		}

		metadata := mapTraceToLangfuse(obs)

		// Should not include zero values
		assert.NotContains(t, metadata, "total_cost")
		assert.NotContains(t, metadata, "total_tokens")
	})

	t.Run("toJSONString - success", func(t *testing.T) {
		data := map[string]interface{}{
			"key1": "value1",
			"key2": 42,
		}

		result := toJSONString(data)
		assert.NotEmpty(t, result)
		assert.Contains(t, result, "key1")
		assert.Contains(t, result, "value1")
	})

	t.Run("toJSONString - nil input", func(t *testing.T) {
		result := toJSONString(nil)
		assert.Empty(t, result)
	})

	t.Run("toJSONString - unmarshalable type", func(t *testing.T) {
		// Channels cannot be marshaled to JSON
		result := toJSONString(make(chan int))
		assert.Empty(t, result)
	})
}

func TestMapLevel_AllLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		level    string
		expected model.ObservationLevel
	}{
		{
			name:     "info level",
			level:    "info",
			expected: model.ObservationLevelDefault,
		},
		{
			name:     "warn level",
			level:    "warn",
			expected: model.ObservationLevelWarning,
		},
		{
			name:     "warning level (alias)",
			level:    "warning",
			expected: model.ObservationLevelWarning,
		},
		{
			name:     "error level",
			level:    "error",
			expected: model.ObservationLevelError,
		},
		{
			name:     "debug level",
			level:    "debug",
			expected: model.ObservationLevelDebug,
		},
		{
			name:     "unknown level defaults to default",
			level:    "unknown",
			expected: model.ObservationLevelDefault,
		},
		{
			name:     "empty level defaults to default",
			level:    "",
			expected: model.ObservationLevelDefault,
		},
		{
			name:     "uppercase level (not handled)",
			level:    "INFO",
			expected: model.ObservationLevelDefault,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapLevelToLangfuseModel(tt.level)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test edge cases and boundary conditions

func TestMappers_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("mapGenerationToLangfuse - nil attributes", func(t *testing.T) {
		obs := observability.GenerationObservation{
			ID:               uuid.New().String(),
			TraceID:          uuid.New().String(),
			TenantID:         uuid.New(),
			SessionID:        uuid.New(),
			Timestamp:        time.Now(),
			Model:            "gpt-4",
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Attributes:       nil, // Nil attributes
		}

		metadata := mapGenerationToLangfuse(obs)

		assert.NotNil(t, metadata)
		assert.Equal(t, "gpt-4", metadata["model"])
	})

	t.Run("mapSpanToLangfuse - nil attributes", func(t *testing.T) {
		obs := observability.SpanObservation{
			ID:         uuid.New().String(),
			TraceID:    uuid.New().String(),
			TenantID:   uuid.New(),
			SessionID:  uuid.New(),
			Timestamp:  time.Now(),
			Name:       "span",
			Attributes: nil,
		}

		metadata := mapSpanToLangfuse(obs)
		assert.NotNil(t, metadata)
	})

	t.Run("mapEventToLangfuse - nil attributes", func(t *testing.T) {
		obs := observability.EventObservation{
			ID:         uuid.New().String(),
			TraceID:    uuid.New().String(),
			TenantID:   uuid.New(),
			SessionID:  uuid.New(),
			Timestamp:  time.Now(),
			Name:       "event",
			Attributes: nil,
		}

		metadata := mapEventToLangfuse(obs)
		assert.NotNil(t, metadata)
	})

	t.Run("mapTraceToLangfuse - nil attributes", func(t *testing.T) {
		obs := observability.TraceObservation{
			ID:         uuid.New().String(),
			TenantID:   uuid.New(),
			SessionID:  uuid.New(),
			Timestamp:  time.Now(),
			Name:       "trace",
			Attributes: nil,
		}

		metadata := mapTraceToLangfuse(obs)
		assert.NotNil(t, metadata)
	})

	t.Run("extractTokenUsage - zero tokens", func(t *testing.T) {
		obs := observability.GenerationObservation{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		}

		usage := extractTokenUsage(obs)

		// Should not include zero values
		assert.NotContains(t, usage, "input")
		assert.NotContains(t, usage, "output")
		assert.NotContains(t, usage, "total")
	})

	t.Run("extractTokenUsage - invalid cache token types", func(t *testing.T) {
		obs := observability.GenerationObservation{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Attributes: map[string]interface{}{
				"cache_read_tokens":  "invalid", // String instead of int
				"cache_write_tokens": 3.14,      // Float instead of int
			},
		}

		usage := extractTokenUsage(obs)

		// Should only include valid token fields
		assert.Equal(t, 100, usage["input"])
		assert.Equal(t, 50, usage["output"])
		assert.Equal(t, 150, usage["total"])
		assert.NotContains(t, usage, "cache_read")
		assert.NotContains(t, usage, "cache_write")
	})

	t.Run("extractTokenUsage - zero cache tokens", func(t *testing.T) {
		obs := observability.GenerationObservation{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
			Attributes: map[string]interface{}{
				"cache_read_tokens":  0,
				"cache_write_tokens": 0,
			},
		}

		usage := extractTokenUsage(obs)

		// Should not include zero cache tokens
		assert.NotContains(t, usage, "cache_read")
		assert.NotContains(t, usage, "cache_write")
	})

	t.Run("mappers preserve attribute types", func(t *testing.T) {
		attrs := map[string]interface{}{
			"string_val": "test",
			"int_val":    42,
			"float_val":  3.14,
			"bool_val":   true,
			"array_val":  []string{"a", "b"},
			"map_val":    map[string]int{"x": 1},
		}

		genObs := observability.GenerationObservation{
			ID:         uuid.New().String(),
			TraceID:    uuid.New().String(),
			TenantID:   uuid.New(),
			SessionID:  uuid.New(),
			Timestamp:  time.Now(),
			Model:      "gpt-4",
			Attributes: attrs,
		}

		metadata := mapGenerationToLangfuse(genObs)

		// All types should be preserved
		assert.Equal(t, "test", metadata["string_val"])
		assert.Equal(t, 42, metadata["int_val"])
		assert.Equal(t, 3.14, metadata["float_val"])
		assert.Equal(t, true, metadata["bool_val"])
		assert.Equal(t, []string{"a", "b"}, metadata["array_val"])
		assert.Equal(t, map[string]int{"x": 1}, metadata["map_val"])
	})
}

// Test attribute merging behavior

func TestMappers_AttributeMerging(t *testing.T) {
	t.Parallel()

	t.Run("custom attributes override built-in fields", func(t *testing.T) {
		obs := observability.GenerationObservation{
			ID:        uuid.New().String(),
			TraceID:   uuid.New().String(),
			TenantID:  uuid.New(),
			SessionID: uuid.New(),
			Timestamp: time.Now(),
			Model:     "gpt-4",
			Provider:  "openai",
			Attributes: map[string]interface{}{
				"model":    "custom-model", // Override model
				"provider": "custom-provider",
			},
		}

		metadata := mapGenerationToLangfuse(obs)

		// Attributes are merged AFTER core fields, so they override
		assert.Equal(t, "custom-model", metadata["model"])
		assert.Equal(t, "custom-provider", metadata["provider"])
	})

	t.Run("empty string fields are excluded", func(t *testing.T) {
		obs := observability.GenerationObservation{
			ID:           uuid.New().String(),
			TraceID:      uuid.New().String(),
			TenantID:     uuid.New(),
			SessionID:    uuid.New(),
			Timestamp:    time.Now(),
			Model:        "", // Empty model
			Provider:     "", // Empty provider
			FinishReason: "", // Empty finish reason
		}

		metadata := mapGenerationToLangfuse(obs)

		assert.NotContains(t, metadata, "model")
		assert.NotContains(t, metadata, "provider")
		assert.NotContains(t, metadata, "finish_reason")
	})

	t.Run("zero integer fields are excluded", func(t *testing.T) {
		obs := observability.GenerationObservation{
			ID:             uuid.New().String(),
			TraceID:        uuid.New().String(),
			TenantID:       uuid.New(),
			SessionID:      uuid.New(),
			Timestamp:      time.Now(),
			Model:          "gpt-4",
			PromptMessages: 0, // Zero count
			Tools:          0,
			ToolCalls:      0,
		}

		metadata := mapGenerationToLangfuse(obs)

		assert.NotContains(t, metadata, "prompt_messages")
		assert.NotContains(t, metadata, "tools_count")
		assert.NotContains(t, metadata, "tool_calls_count")
	})
}
