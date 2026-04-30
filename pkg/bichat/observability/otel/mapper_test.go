package otel

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func attrMap(kvs []attribute.KeyValue) map[string]attribute.Value {
	out := make(map[string]attribute.Value, len(kvs))
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value
	}
	return out
}

func TestGenerationToAttributes_EmptyObservation(t *testing.T) {
	attrs := generationToAttributes(observability.GenerationObservation{})
	assert.Empty(t, attrs, "zero observation must not panic and should produce no attrs")
}

func TestGenerationToAttributes_FullMapping(t *testing.T) {
	tenantID := uuid.New()
	sessionID := uuid.New()

	obs := observability.GenerationObservation{
		Model:            "gpt-5.4-mini",
		Provider:         "openai",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		FinishReason:     "stop",
		UserID:           "user-123",
		UserEmail:        "u@example.com",
		TenantID:         tenantID,
		SessionID:        sessionID,
		Input:            map[string]any{"prompt": "hi"},
		Output:           "hello",
		PromptMessages:   3,
		Tools:            2,
		ToolCalls:        1,
		LatencyMs:        4321,
		Thinking:         "deliberation",
		ObservationReason: "missing_request_id",
		Level:             "info",
		ModelParameters: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  4096,
			"stream":      true,
			"stop":        "END",
			"unsupported": map[string]string{"nested": "skip"},
		},
		Attributes: map[string]interface{}{
			"cache_read_tokens":  25,
			"cache_write_tokens": 10,
		},
	}

	m := attrMap(generationToAttributes(obs))

	require.Contains(t, m, attrGenAIRequestModel)
	assert.Equal(t, "gpt-5.4-mini", m[attrGenAIRequestModel].AsString())
	assert.Equal(t, "openai", m[attrGenAISystem].AsString())
	assert.Equal(t, int64(100), m[attrGenAIUsageInputTokens].AsInt64())
	assert.Equal(t, int64(50), m[attrGenAIUsageOutputTokens].AsInt64())
	assert.Equal(t, int64(150), m[attrGenAIUsageTotalTokens].AsInt64())
	assert.Equal(t, int64(25), m[attrGenAIUsageCacheReadIn].AsInt64())
	assert.Equal(t, int64(10), m[attrGenAIUsageCacheCreateIn].AsInt64())
	assert.Equal(t, []string{"stop"}, m[attrGenAIResponseFinishReas].AsStringSlice())
	assert.Equal(t, "user-123", m[attrEnduserID].AsString())
	assert.Equal(t, "u@example.com", m[attrEnduserEmail].AsString())
	assert.Equal(t, tenantID.String(), m[attrEAITenantID].AsString())
	assert.Equal(t, sessionID.String(), m[attrLangfuseSessionID].AsString())
	assert.Equal(t, int64(3), m[attrGenAIRequestMessages].AsInt64())
	assert.Equal(t, int64(2), m[attrGenAIRequestToolsCount].AsInt64())
	assert.Equal(t, int64(1), m[attrGenAIResponseToolCalls].AsInt64())
	assert.Equal(t, int64(4321), m[attrGenAILatencyMs].AsInt64())
	assert.Equal(t, "deliberation", m[attrGenAIResponseThinking].AsString())
	assert.Equal(t, "missing_request_id", m[attrEAIObservationReas].AsString())
	assert.Equal(t, "info", m[attrEAIObservationLevel].AsString())

	// Input/output marshalled as JSON-ish strings.
	require.Contains(t, m, attrLangfuseObsInput)
	assert.Contains(t, m[attrLangfuseObsInput].AsString(), "prompt")
	assert.Equal(t, "hello", m[attrLangfuseObsOutput].AsString())

	// Model parameters flattened.
	assert.InDelta(t, 0.7, m["gen_ai.request.temperature"].AsFloat64(), 1e-9)
	assert.Equal(t, int64(4096), m["gen_ai.request.max_tokens"].AsInt64())
	assert.Equal(t, true, m["gen_ai.request.stream"].AsBool())
	assert.Equal(t, "END", m["gen_ai.request.stop"].AsString())
	// Nested map skipped.
	_, hasUnsupported := m["gen_ai.request.unsupported"]
	assert.False(t, hasUnsupported, "nested-map model parameter must be skipped")
}

func TestGenerationToAttributes_CacheTokensInt64(t *testing.T) {
	obs := observability.GenerationObservation{
		Model: "m",
		Attributes: map[string]interface{}{
			"cache_read_tokens":  int64(7),
			"cache_write_tokens": int64(3),
		},
	}
	m := attrMap(generationToAttributes(obs))
	assert.Equal(t, int64(7), m[attrGenAIUsageCacheReadIn].AsInt64())
	assert.Equal(t, int64(3), m[attrGenAIUsageCacheCreateIn].AsInt64())
}

func TestGenerationToAttributes_OmitsCostAttributes(t *testing.T) {
	obs := observability.GenerationObservation{
		Model:            "gpt-5.4-mini",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Attributes: map[string]interface{}{
			"cost":                 1.23, // must NOT be exported as an attribute
			"input_price_per_1m":   2.5,
			"output_price_per_1m":  10.0,
		},
	}
	for _, kv := range generationToAttributes(obs) {
		assert.False(t, strings.Contains(string(kv.Key), "cost"),
			"unexpected cost-related attribute %q emitted", kv.Key)
		assert.False(t, strings.Contains(string(kv.Key), "price"),
			"unexpected price-related attribute %q emitted", kv.Key)
	}
}

func TestSpanToAttributes(t *testing.T) {
	obs := observability.SpanObservation{
		Type:     "tool",
		Status:   "success",
		ToolName: "search_db",
		CallID:   "call-1",
		Input:    `{"q":"test"}`,
		Output:   `{"rows":3}`,
		Level:    "info",
		Attributes: map[string]interface{}{
			"row_count": 3,
		},
	}
	m := attrMap(spanToAttributes(obs))
	assert.Equal(t, "tool", m[attrEAISpanType].AsString())
	assert.Equal(t, "success", m[attrEAISpanStatus].AsString())
	assert.Equal(t, "search_db", m[attrEAIToolName].AsString())
	assert.Equal(t, "call-1", m[attrEAIToolCallID].AsString())
	assert.Equal(t, `{"q":"test"}`, m[attrLangfuseObsInput].AsString())
	assert.Equal(t, `{"rows":3}`, m[attrLangfuseObsOutput].AsString())
	assert.Equal(t, "info", m[attrEAIObservationLevel].AsString())
	assert.Equal(t, int64(3), m[attrEAISpanAttrPrefix+"row_count"].AsInt64())
}

func TestSpanToAttributes_Empty(t *testing.T) {
	attrs := spanToAttributes(observability.SpanObservation{})
	assert.Empty(t, attrs)
}

func TestEventToAttributes(t *testing.T) {
	obs := observability.EventObservation{
		Type:    "interrupt",
		Message: "user input required",
		Level:   "warn",
		Attributes: map[string]interface{}{
			"reason": "hitl",
		},
	}
	m := attrMap(eventToAttributes(obs))
	assert.Equal(t, "interrupt", m[attrEAIEventType].AsString())
	assert.Equal(t, "user input required", m[attrEAIEventMessage].AsString())
	assert.Equal(t, "warn", m[attrEAIEventLevel].AsString())
	assert.Equal(t, "hitl", m[attrEAIEventAttrPrefix+"reason"].AsString())
}

func TestTraceToAttributes(t *testing.T) {
	userID := uuid.New()
	tenantID := uuid.New()
	sessionID := uuid.New()
	obs := observability.TraceObservation{
		Status:      "completed",
		UserID:      userID,
		TenantID:    tenantID,
		SessionID:   sessionID,
		TotalTokens: 999,
		Name:        "BiChat Run",
	}
	m := attrMap(traceToAttributes(obs))
	assert.Equal(t, "completed", m[attrEAITraceStatus].AsString())
	assert.Equal(t, userID.String(), m[attrEnduserID].AsString())
	assert.Equal(t, tenantID.String(), m[attrEAITenantID].AsString())
	assert.Equal(t, sessionID.String(), m[attrLangfuseSessionID].AsString())
	assert.Equal(t, int64(999), m[attrGenAIUsageTotalTokens].AsInt64())
	assert.Equal(t, "BiChat Run", m[attrLangfuseTraceName].AsString())
}

func TestJsonString_PassesThroughStrings(t *testing.T) {
	assert.Equal(t, "hello", jsonString("hello"))
	assert.Equal(t, "", jsonString(""))
	assert.Equal(t, "", jsonString(nil))
	got := jsonString(map[string]string{"k": "v"})
	assert.Contains(t, got, "\"k\"")
	assert.Contains(t, got, "\"v\"")
}

func TestReadIntAttr(t *testing.T) {
	v, ok := readIntAttr(map[string]interface{}{"n": 5}, "n")
	assert.True(t, ok)
	assert.Equal(t, 5, v)

	v, ok = readIntAttr(map[string]interface{}{"n": int64(7)}, "n")
	assert.True(t, ok)
	assert.Equal(t, 7, v)

	_, ok = readIntAttr(map[string]interface{}{"n": "x"}, "n")
	assert.False(t, ok)

	_, ok = readIntAttr(nil, "n")
	assert.False(t, ok)
}
