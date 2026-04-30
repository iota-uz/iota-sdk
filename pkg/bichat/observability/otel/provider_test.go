package otel

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func newTestProvider(t *testing.T, enabled bool) (*Provider, *tracetest.InMemoryExporter) {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})
	p := NewProvider(Config{Enabled: enabled}, WithTracer(tp.Tracer("test")))
	return p, exporter
}

func findAttr(t *testing.T, attrs []attribute.KeyValue, key string) (attribute.Value, bool) {
	t.Helper()
	for _, kv := range attrs {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

func requireAttrString(t *testing.T, attrs []attribute.KeyValue, key, want string) {
	t.Helper()
	v, ok := findAttr(t, attrs, key)
	require.Truef(t, ok, "missing attribute %q", key)
	require.Equal(t, want, v.AsString())
}

func requireAttrInt(t *testing.T, attrs []attribute.KeyValue, key string, want int64) {
	t.Helper()
	v, ok := findAttr(t, attrs, key)
	require.Truef(t, ok, "missing attribute %q", key)
	require.Equal(t, want, v.AsInt64())
}

func TestProvider_RecordGeneration_EmitsGenAISemconv(t *testing.T) {
	p, exporter := newTestProvider(t, true)

	err := p.RecordGeneration(context.Background(), observability.GenerationObservation{
		ID:               "gen-1",
		TraceID:          "trace-1",
		Model:            "gpt-5.4-mini",
		Provider:         "openai",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Timestamp:        time.Now(),
		Duration:         500 * time.Millisecond,
		Attributes: map[string]interface{}{
			"cache_read_tokens": 25,
		},
	})
	require.NoError(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	s := spans[0]
	assert.Equal(t, "gpt-5.4-mini", s.Name)

	requireAttrString(t, s.Attributes, attrGenAIRequestModel, "gpt-5.4-mini")
	requireAttrString(t, s.Attributes, attrGenAISystem, "openai")
	requireAttrInt(t, s.Attributes, attrGenAIUsageInputTokens, 100)
	requireAttrInt(t, s.Attributes, attrGenAIUsageOutputTokens, 50)
	requireAttrInt(t, s.Attributes, attrGenAIUsageTotalTokens, 150)
	requireAttrInt(t, s.Attributes, attrGenAIUsageCacheReadIn, 25)
}

func TestProvider_RecordGeneration_DisabledIsNoOp(t *testing.T) {
	p, exporter := newTestProvider(t, false)

	err := p.RecordGeneration(context.Background(), observability.GenerationObservation{
		ID: "gen-1", TraceID: "trace-1", Model: "x", Timestamp: time.Now(),
	})
	require.NoError(t, err)
	assert.Empty(t, exporter.GetSpans(), "disabled provider must not emit spans")
}

// allowedAttrPrefixes is the closed set of attribute namespaces the OTel
// provider may emit. Any key outside this list is a regression — either a
// cost/price leak (the original bug class) or a custom attr that bypasses the
// mapper. Backends consume specific namespaces; emitting outside them risks
// silent UI gaps OR re-enables the cost-on-telemetry-path failure mode.
var allowedAttrPrefixes = []string{
	"gen_ai.",
	"eai.",
	"langfuse.",
	"enduser.",
	"service.",
	"deployment.",
}

func TestProvider_NoCostAttributesEmitted(t *testing.T) {
	p, exporter := newTestProvider(t, true)

	err := p.RecordGeneration(context.Background(), observability.GenerationObservation{
		ID:               "gen-2",
		TraceID:          "trace-2",
		Model:            "claude-sonnet-4-5",
		Provider:         "anthropic",
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Timestamp:        time.Now(),
		Attributes: map[string]interface{}{
			"cost":                    0.42,
			"total_cost":              0.42,
			"input_cost":              0.30,
			"output_cost":              0.12,
			"input_price_per_1m":      3.0,
			"output_price_per_1m":     15.0,
			"cache_read_tokens":       10,
			"cache_write_tokens":      5,
			"cache_read_price_per_1m": 0.3,
			"billing.amount":          0.99,
			"gen_ai.usage.expense":    0.55,
		},
	})
	require.NoError(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	for _, kv := range spans[0].Attributes {
		key := string(kv.Key)
		// Substring belt: catches obvious cost/price leaks regardless of namespace.
		lower := strings.ToLower(key)
		assert.False(t, strings.Contains(lower, "cost"),
			"cost attribute leaked: %s (this is the bug class we're closing)", key)
		assert.False(t, strings.Contains(lower, "price"),
			"price attribute leaked: %s", key)
		assert.False(t, strings.Contains(lower, "expense"),
			"expense attribute leaked: %s", key)
		assert.False(t, strings.Contains(lower, "billing"),
			"billing attribute leaked: %s", key)

		// Allowlist suspenders: every emitted key must live in one of the
		// known namespaces. A future refactor that emits e.g. "openai.tokens.in"
		// would slip past the substring check above but trip this assertion.
		matched := false
		for _, prefix := range allowedAttrPrefixes {
			if strings.HasPrefix(key, prefix) {
				matched = true
				break
			}
		}
		assert.True(t, matched,
			"attribute %q has no allowed prefix; add to allowedAttrPrefixes if intentional, "+
				"otherwise the mapper is leaking unmapped data", key)
	}
}

func TestProvider_RecordSpan_SetsEAISpanKeysAndParentLink(t *testing.T) {
	p, exporter := newTestProvider(t, true)

	err := p.RecordSpan(context.Background(), observability.SpanObservation{
		ID:       "span-1",
		TraceID:  "trace-x",
		ParentID: "parent-span",
		Name:     "tool.execute",
		Type:     "tool",
		Status:   "success",
		ToolName: "search_db",
		CallID:   "call-1",
		Timestamp: time.Now(),
		Duration:  10 * time.Millisecond,
	})
	require.NoError(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	s := spans[0]
	assert.Equal(t, "tool.execute", s.Name)
	requireAttrString(t, s.Attributes, attrEAISpanType, "tool")
	requireAttrString(t, s.Attributes, attrEAISpanStatus, "success")
	requireAttrString(t, s.Attributes, attrEAIToolName, "search_db")
	requireAttrString(t, s.Attributes, attrEAIToolCallID, "call-1")

	// Parent SpanContext must reflect the derived parent span ID.
	expectedParent := deriveSpanID("parent-span")
	assert.Equal(t, expectedParent, s.Parent.SpanID(),
		"span must link to the derived parent span ID")
	expectedTrace := deriveTraceID("trace-x")
	assert.Equal(t, expectedTrace, s.SpanContext.TraceID(),
		"span trace ID must be derived from obs.TraceID")
}

func TestProvider_RecordEvent_SetsEAIEventKeys(t *testing.T) {
	p, exporter := newTestProvider(t, true)

	err := p.RecordEvent(context.Background(), observability.EventObservation{
		ID:        "evt-1",
		TraceID:   "trace-y",
		Name:      "context.overflow",
		Type:      "context",
		Message:   "budget exceeded",
		Level:     "warn",
		Timestamp: time.Now(),
	})
	require.NoError(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	s := spans[0]
	assert.Equal(t, "context.overflow", s.Name)
	requireAttrString(t, s.Attributes, attrEAIEventType, "context")
	requireAttrString(t, s.Attributes, attrEAIEventMessage, "budget exceeded")
	requireAttrString(t, s.Attributes, attrEAIEventLevel, "warn")
}

func TestDeriveTraceID_Deterministic(t *testing.T) {
	a := deriveTraceID("trace-1")
	b := deriveTraceID("trace-1")
	assert.Equal(t, a, b, "trace ID derivation must be deterministic")
	assert.True(t, a.IsValid())

	c := deriveTraceID("trace-2")
	assert.NotEqual(t, a, c)

	zero := deriveTraceID("")
	assert.False(t, zero.IsValid(), "empty input yields zero trace ID")
}

func TestDeriveSpanID_Deterministic(t *testing.T) {
	a := deriveSpanID("span-1")
	b := deriveSpanID("span-1")
	assert.Equal(t, a, b)
	assert.True(t, a.IsValid())

	zero := deriveSpanID("")
	assert.False(t, zero.IsValid())
}

func TestProvider_UpdateTraceName_EmitsMarkerSpan(t *testing.T) {
	p, exporter := newTestProvider(t, true)
	require.NoError(t, p.UpdateTraceName(context.Background(), "trace-z", "My Chat"))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	requireAttrString(t, spans[0].Attributes, attrLangfuseTraceName, "My Chat")
	requireAttrString(t, spans[0].Attributes, attrLangfuseUpdateKind, "name")
}

func TestProvider_UpdateTraceTags_EmitsMarkerSpan(t *testing.T) {
	p, exporter := newTestProvider(t, true)
	require.NoError(t, p.UpdateTraceTags(context.Background(), "trace-z", []string{"a", "b"}))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	v, ok := findAttr(t, spans[0].Attributes, attrLangfuseTraceTags)
	require.True(t, ok)
	assert.Equal(t, []string{"a", "b"}, v.AsStringSlice())
	requireAttrString(t, spans[0].Attributes, attrLangfuseUpdateKind, "tags")
}

func TestConfig_Validate(t *testing.T) {
	c := Config{Enabled: true, Endpoint: "https://x/y"}
	require.NoError(t, c.Validate())
	assert.InDelta(t, 1.0, c.SampleRate, 1e-9, "default SampleRate=1.0")

	bad := Config{Enabled: true}
	assert.Error(t, bad.Validate(), "Endpoint required when Enabled")

	rangeBad := Config{Enabled: true, Endpoint: "x", SampleRate: 2.0}
	assert.Error(t, rangeBad.Validate())
}

func TestLangfuseAuthHeaders_NoEnv(t *testing.T) {
	t.Setenv("LANGFUSE_PUBLIC_KEY", "")
	t.Setenv("LANGFUSE_SECRET_KEY", "")
	got := LangfuseAuthHeaders()
	assert.Empty(t, got)
}

func TestLangfuseAuthHeaders_WithEnv(t *testing.T) {
	t.Setenv("LANGFUSE_PUBLIC_KEY", "pk")
	t.Setenv("LANGFUSE_SECRET_KEY", "sk")
	got := LangfuseAuthHeaders()
	require.Contains(t, got, "Authorization")
	assert.True(t, strings.HasPrefix(got["Authorization"], "Basic "))
}

func TestInitTracerProvider_DisabledReturnsNoop(t *testing.T) {
	shutdown, err := InitTracerProvider(context.Background(), Config{Enabled: false})
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}

func TestInitTracerProvider_EmptyEndpointReturnsNoop(t *testing.T) {
	shutdown, err := InitTracerProvider(context.Background(), Config{Enabled: true})
	require.NoError(t, err)
	require.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}
