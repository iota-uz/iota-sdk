package langfuse

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/henomis/langfuse-go/model"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestProvider creates a provider with a mock client for testing.
// This bypasses the normal NewLangfuseProvider constructor which creates a real client.
func newTestProvider(client LangfuseClient, config Config) *LangfuseProvider {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)

	return &LangfuseProvider{
		client: client,
		config: config,
		state:  newState(),
		log:    log,
	}
}

// validConfig returns a minimal valid configuration for testing.
func validConfig() Config {
	return Config{
		Host:          "https://test.langfuse.com",
		PublicKey:     "test-public-key",
		SecretKey:     "test-secret-key",
		FlushInterval: 1 * time.Second,
		SampleRate:    1.0,
		Enabled:       true,
	}
}

// buildCompleteObservation creates a fully populated GenerationObservation for testing.
func buildCompleteObservation() observability.GenerationObservation {
	return observability.GenerationObservation{
		ID:               "gen-" + uuid.New().String(),
		TraceID:          uuid.New().String(),
		TenantID:         uuid.New(),
		SessionID:        uuid.New(),
		Timestamp:        time.Now(),
		Model:            "claude-sonnet-4-6",
		Provider:         "anthropic",
		PromptMessages:   3,
		PromptTokens:     1500,
		Tools:            5,
		PromptContent:    "Test prompt",
		CompletionTokens: 500,
		TotalTokens:      2000,
		LatencyMs:        1200,
		FinishReason:     "stop",
		ToolCalls:        0,
		CompletionText:   "Test completion",
		Duration:         1200 * time.Millisecond,
		Attributes: map[string]interface{}{
			"custom_field": "custom_value",
		},
	}
}

// buildSpanObservation creates a SpanObservation for testing.
func buildSpanObservation() observability.SpanObservation {
	return observability.SpanObservation{
		ID:        "span-" + uuid.New().String(),
		TraceID:   uuid.New().String(),
		ParentID:  "",
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		Timestamp: time.Now(),
		Name:      "tool.execute",
		Type:      "tool",
		Input:     `{"query": "SELECT * FROM users"}`,
		Output:    `{"rows": 10, "success": true}`,
		Duration:  500 * time.Millisecond,
		Status:    "success",
		ToolName:  "sql_query",
		CallID:    "call-123",
		Attributes: map[string]interface{}{
			"database": "postgres",
		},
	}
}

// buildEventObservation creates an EventObservation for testing.
func buildEventObservation() observability.EventObservation {
	return observability.EventObservation{
		ID:        "event-" + uuid.New().String(),
		TraceID:   uuid.New().String(),
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		Timestamp: time.Now(),
		Name:      "context.overflow",
		Type:      "context",
		Message:   "Context window exceeded",
		Level:     "warn",
		Attributes: map[string]interface{}{
			"overflow_tokens": 1000,
		},
	}
}

// buildTraceObservation creates a TraceObservation for testing.
func buildTraceObservation() observability.TraceObservation {
	return observability.TraceObservation{
		ID:          uuid.New().String(),
		TenantID:    uuid.New(),
		SessionID:   uuid.New(),
		Timestamp:   time.Now(),
		Name:        "BI Analysis Session",
		Duration:    5 * time.Minute,
		Status:      "completed",
		UserID:      uuid.New(),
		TotalCost:   0.0125,
		TotalTokens: 15000,
		Attributes: map[string]interface{}{
			"session_type": "analysis",
		},
	}
}

// TestRecordGeneration_ClientError_NonBlocking verifies that Generation errors don't block execution.
func TestRecordGeneration_ClientError_NonBlocking(t *testing.T) {
	t.Parallel()

	mock := NewMockClient().WithGenerationError(errors.New("SDK error"))
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildCompleteObservation()

	// Should not panic
	require.NotPanics(t, func() {
		err := provider.RecordGeneration(ctx, obs)
		assert.NoError(t, err) // Non-blocking: error logged but not returned
	})

	// Should have attempted the call
	assert.Equal(t, 1, mock.GenerationCallCount())
}

// TestRecordGeneration_MalformedData verifies handling of malformed observation data.
func TestRecordGeneration_MalformedData(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := observability.GenerationObservation{
		ID:        "", // Empty ID
		SessionID: uuid.Nil,
		TenantID:  uuid.Nil,
		Model:     "", // Empty model
		// Minimal/invalid data
	}

	// Should not panic even with malformed data
	require.NotPanics(t, func() {
		err := provider.RecordGeneration(ctx, obs)
		assert.NoError(t, err)
	})

	// Trace should still be created (with defaults)
	assert.Positive(t, mock.TraceCallCount())
}

// TestRecordSpan_ClientError_NonBlocking verifies that Span errors don't block execution.
func TestRecordSpan_ClientError_NonBlocking(t *testing.T) {
	t.Parallel()

	mock := NewMockClient().WithSpanError(errors.New("SDK error"))
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildSpanObservation()

	// Should not panic
	require.NotPanics(t, func() {
		err := provider.RecordSpan(ctx, obs)
		assert.NoError(t, err)
	})

	// Should have attempted the call
	assert.Equal(t, 1, mock.SpanCallCount())
}

// TestRecordEvent_ClientError_NonBlocking verifies that Event errors don't block execution.
func TestRecordEvent_ClientError_NonBlocking(t *testing.T) {
	t.Parallel()

	mock := NewMockClient().WithEventError(errors.New("SDK error"))
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildEventObservation()

	// Should not panic
	require.NotPanics(t, func() {
		err := provider.RecordEvent(ctx, obs)
		assert.NoError(t, err)
	})

	// Should have attempted the call
	assert.Equal(t, 1, mock.EventCallCount())
}

// TestRecordTrace_ClientError_NonBlocking verifies that Trace errors don't block execution.
func TestRecordTrace_ClientError_NonBlocking(t *testing.T) {
	t.Parallel()

	mock := NewMockClient().WithTraceError(errors.New("SDK error"))
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildTraceObservation()

	// Should not panic
	require.NotPanics(t, func() {
		err := provider.RecordTrace(ctx, obs)
		assert.NoError(t, err)
	})

	// Should have attempted the call
	assert.Equal(t, 1, mock.TraceCallCount())
}

// TestEnsureTrace_ClientError_Recovers verifies that ensureTrace handles client errors gracefully.
func TestEnsureTrace_ClientError_Recovers(t *testing.T) {
	t.Parallel()

	mock := NewMockClient().WithTraceError(errors.New("trace creation failed"))
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildCompleteObservation()

	// Should not panic even if trace creation fails
	require.NotPanics(t, func() {
		err := provider.RecordGeneration(ctx, obs)
		assert.NoError(t, err) // Non-blocking
	})

	// Trace creation should have been attempted
	assert.Positive(t, mock.TraceCallCount())
}

// TestFlush_DoesNotPanic verifies that Flush never panics.
func TestFlush_DoesNotPanic(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()

	// Should not panic
	require.NotPanics(t, func() {
		err := provider.Flush(ctx)
		assert.NoError(t, err)
	})

	// Should call mock Flush
	assert.Equal(t, 1, mock.FlushCallCount())
}

// TestShutdown_DoesNotPanic verifies that Shutdown never panics.
func TestShutdown_DoesNotPanic(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()

	// Should not panic
	require.NotPanics(t, func() {
		err := provider.Shutdown(ctx)
		assert.NoError(t, err)
	})

	// Should call Flush and clear state
	assert.Equal(t, 1, mock.FlushCallCount())
}

// TestRecordGeneration_CostCalculation verifies token-based cost calculations.
func TestRecordGeneration_CostCalculation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		inputTokens      int
		outputTokens     int
		inputPricePer1M  float64
		outputPricePer1M float64
		expectedCost     float64
	}{
		{
			name:             "standard pricing",
			inputTokens:      1_000_000,
			outputTokens:     1_000_000,
			inputPricePer1M:  3.0,
			outputPricePer1M: 15.0,
			expectedCost:     18.0, // (1M/1M)*3 + (1M/1M)*15
		},
		{
			name:             "small usage",
			inputTokens:      1000,
			outputTokens:     500,
			inputPricePer1M:  3.0,
			outputPricePer1M: 15.0,
			expectedCost:     0.0105, // (1000/1M)*3 + (500/1M)*15
		},
		{
			name:             "no pricing data",
			inputTokens:      1000,
			outputTokens:     500,
			inputPricePer1M:  0,
			outputPricePer1M: 0,
			expectedCost:     0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := NewMockClient()
			provider := newTestProvider(mock, validConfig())

			ctx := context.Background()
			obs := buildCompleteObservation()
			obs.PromptTokens = tt.inputTokens
			obs.CompletionTokens = tt.outputTokens
			obs.TotalTokens = tt.inputTokens + tt.outputTokens

			if tt.inputPricePer1M > 0 || tt.outputPricePer1M > 0 {
				obs.Attributes = map[string]interface{}{
					"input_price_per_1m":  tt.inputPricePer1M,
					"output_price_per_1m": tt.outputPricePer1M,
				}
			}

			err := provider.RecordGeneration(ctx, obs)
			require.NoError(t, err)

			calls := mock.GetGenerationCalls()
			require.Len(t, calls, 1)

			// Verify usage calculation
			gen := calls[0].Generation
			assert.Equal(t, tt.inputTokens, gen.Usage.Input)
			assert.Equal(t, tt.outputTokens, gen.Usage.Output)

			if tt.expectedCost > 0 {
				assert.InDelta(t, tt.expectedCost, gen.Usage.TotalCost, 0.0001)
			}
		})
	}
}

// TestRecordGeneration_CacheTokens verifies cache token handling.
func TestRecordGeneration_CacheTokens(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildCompleteObservation()
	obs.Attributes = map[string]interface{}{
		"cache_write_tokens": 100,
		"cache_read_tokens":  50,
	}

	err := provider.RecordGeneration(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetGenerationCalls()
	require.Len(t, calls, 1)

	// Cache tokens should be in metadata
	metadata, ok := calls[0].Generation.Metadata.(map[string]interface{})
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, 100, metadata["cache_write_tokens"])
	assert.Equal(t, 50, metadata["cache_read_tokens"])
}

// TestRecordGeneration_ZeroTokens verifies handling of zero token counts.
func TestRecordGeneration_ZeroTokens(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildCompleteObservation()
	obs.PromptTokens = 0
	obs.CompletionTokens = 0
	obs.TotalTokens = 0

	err := provider.RecordGeneration(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetGenerationCalls()
	require.Len(t, calls, 1)

	// Should handle zero tokens gracefully
	gen := calls[0].Generation
	assert.NotNil(t, gen.Usage)
	assert.Equal(t, 0, gen.Usage.Input)
	assert.Equal(t, 0, gen.Usage.Output)
}

// TestRecordGeneration_NegativeTokens verifies handling of invalid negative token counts.
func TestRecordGeneration_NegativeTokens(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildCompleteObservation()
	obs.PromptTokens = -100 // Invalid
	obs.CompletionTokens = -50
	obs.TotalTokens = -150

	// Should not panic with negative values
	require.NotPanics(t, func() {
		err := provider.RecordGeneration(ctx, obs)
		require.NoError(t, err)
	})

	calls := mock.GetGenerationCalls()
	require.Len(t, calls, 1)
}

// TestRecordGeneration_VeryLargeTokens verifies handling of very large token counts.
func TestRecordGeneration_VeryLargeTokens(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildCompleteObservation()
	obs.PromptTokens = 1_000_000
	obs.CompletionTokens = 500_000
	obs.TotalTokens = 1_500_000

	// Should not panic with large numbers
	require.NotPanics(t, func() {
		err := provider.RecordGeneration(ctx, obs)
		require.NoError(t, err)
	})

	calls := mock.GetGenerationCalls()
	require.Len(t, calls, 1)

	gen := calls[0].Generation
	assert.Equal(t, 1_000_000, gen.Usage.Input)
	assert.Equal(t, 500_000, gen.Usage.Output)
}

// TestRecordGeneration_EmptyMetadata verifies handling of nil/empty attributes.
func TestRecordGeneration_EmptyMetadata(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildCompleteObservation()
	obs.Attributes = nil

	err := provider.RecordGeneration(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetGenerationCalls()
	require.Len(t, calls, 1)

	// Should handle nil attributes gracefully
	metadata, ok := calls[0].Generation.Metadata.(map[string]interface{})
	assert.True(t, ok, "metadata should be a map")
	assert.NotNil(t, metadata)
}

// TestRecordSpan_ToolMetadata verifies that tool-specific metadata is recorded.
func TestRecordSpan_ToolMetadata(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildSpanObservation()
	obs.ToolName = "sql_execute"
	obs.CallID = "call-456"
	obs.Attributes = map[string]interface{}{
		"database": "postgres",
		"query":    "SELECT * FROM users",
	}

	err := provider.RecordSpan(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetSpanCalls()
	require.Len(t, calls, 1)

	// Verify tool metadata
	metadata, ok := calls[0].Span.Metadata.(map[string]interface{})
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "sql_execute", metadata["tool_name"])
	assert.Equal(t, "call-456", metadata["call_id"])
	assert.Equal(t, "postgres", metadata["database"])
	assert.Equal(t, "SELECT * FROM users", metadata["query"])
}

// TestRecordSpan_NestedSpans verifies hierarchical span tracking.
func TestRecordSpan_NestedSpans(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	sessionID := uuid.New()

	// Parent span
	parentObs := buildSpanObservation()
	parentObs.ID = "parent-span"
	parentObs.SessionID = sessionID
	parentObs.Name = "parent.operation"

	err := provider.RecordSpan(ctx, parentObs)
	require.NoError(t, err)

	// Child span
	childObs := buildSpanObservation()
	childObs.ID = "child-span"
	childObs.SessionID = sessionID
	childObs.ParentID = "parent-span"
	childObs.Name = "child.operation"

	err = provider.RecordSpan(ctx, childObs)
	require.NoError(t, err)

	// Should have recorded both spans
	calls := mock.GetSpanCalls()
	require.Len(t, calls, 2)

	// Verify hierarchy
	assert.Equal(t, "parent-span", calls[0].Span.ID)
	assert.Nil(t, calls[0].ParentID)

	assert.Equal(t, "child-span", calls[1].Span.ID)
	// Parent ID should be set if state mapping exists
}

// TestRecordSpan_SpanWithError verifies error status recording.
func TestRecordSpan_SpanWithError(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildSpanObservation()
	obs.Status = "error"
	obs.Output = `{"error": "database connection failed"}`

	err := provider.RecordSpan(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetSpanCalls()
	require.Len(t, calls, 1)

	// Verify error status
	metadata, ok := calls[0].Span.Metadata.(map[string]interface{})
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "error", metadata["status"])
}

// TestRecordSpan_LongRunningSpan verifies handling of spans with long durations.
func TestRecordSpan_LongRunningSpan(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildSpanObservation()
	obs.Duration = 10 * time.Minute

	err := provider.RecordSpan(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetSpanCalls()
	require.Len(t, calls, 1)

	// Should handle long durations
	span := calls[0].Span
	assert.NotNil(t, span.EndTime)
	// EndTime should be StartTime + Duration
	expectedEnd := span.StartTime.Add(obs.Duration)
	assert.Equal(t, expectedEnd, *span.EndTime)
}

// TestRecordSpan_EmptyName verifies handling of spans with empty names.
func TestRecordSpan_EmptyName(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildSpanObservation()
	obs.Name = ""

	// Should not panic with empty name
	require.NotPanics(t, func() {
		err := provider.RecordSpan(ctx, obs)
		require.NoError(t, err)
	})

	calls := mock.GetSpanCalls()
	require.Len(t, calls, 1)
}

// TestRecordEvent_CustomMetadata verifies custom event attributes are preserved.
func TestRecordEvent_CustomMetadata(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildEventObservation()
	obs.Attributes = map[string]interface{}{
		"custom_field":  "custom_value",
		"numeric_field": 42,
		"nested_object": map[string]interface{}{
			"key": "value",
		},
	}

	err := provider.RecordEvent(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetEventCalls()
	require.Len(t, calls, 1)

	// Verify custom metadata
	metadata, ok := calls[0].Event.Metadata.(map[string]interface{})
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "custom_value", metadata["custom_field"])
	assert.Equal(t, 42, metadata["numeric_field"])
}

// TestRecordEvent_WithSampling_Probability verifies event sampling behavior.
func TestRecordEvent_WithSampling_Probability(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	config := validConfig()
	config.SampleRate = 0.0 // Disable sampling

	provider := newTestProvider(mock, config)

	ctx := context.Background()
	obs := buildEventObservation()

	err := provider.RecordEvent(ctx, obs)
	require.NoError(t, err)

	// With 0% sample rate, no events should be recorded
	calls := mock.GetEventCalls()
	assert.Empty(t, calls)
}

// TestRecordEvent_EventError verifies event-level error handling.
func TestRecordEvent_EventError(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildEventObservation()
	obs.Level = "error"
	obs.Message = "Critical system failure"
	obs.Attributes = map[string]interface{}{
		"error_code": "E500",
		"stack":      "...",
	}

	err := provider.RecordEvent(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetEventCalls()
	require.Len(t, calls, 1)

	// Verify error level mapping
	event := calls[0].Event
	assert.Equal(t, model.ObservationLevelError, event.Level)
}

// TestRecordTrace_WithSampling_Probability verifies trace sampling behavior.
func TestRecordTrace_WithSampling_Probability(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	config := validConfig()
	config.SampleRate = 0.0 // Disable sampling

	provider := newTestProvider(mock, config)

	ctx := context.Background()
	obs := buildTraceObservation()

	err := provider.RecordTrace(ctx, obs)
	require.NoError(t, err)

	// With 0% sample rate, no traces should be recorded
	calls := mock.GetTraceCalls()
	assert.Empty(t, calls)
}

// TestRecordTrace_TraceError verifies trace-level error and cost tracking.
func TestRecordTrace_TraceError(t *testing.T) {
	t.Parallel()

	mock := NewMockClient()
	provider := newTestProvider(mock, validConfig())

	ctx := context.Background()
	obs := buildTraceObservation()
	obs.Status = "error"
	obs.TotalCost = 0.0256
	obs.TotalTokens = 25000

	err := provider.RecordTrace(ctx, obs)
	require.NoError(t, err)

	calls := mock.GetTraceCalls()
	require.Len(t, calls, 1)

	// Verify cost and status in metadata
	metadata, ok := calls[0].Trace.Metadata.(map[string]interface{})
	require.True(t, ok, "metadata should be a map")
	assert.Equal(t, "error", metadata["status"])
	assert.InEpsilon(t, 0.0256, metadata["total_cost"], 1e-9)
	assert.Equal(t, 25000, metadata["total_tokens"])
}
