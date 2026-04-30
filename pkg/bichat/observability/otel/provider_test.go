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
	"go.opentelemetry.io/otel/codes"
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
	p := newProvider(Config{Enabled: enabled}, WithTracer(tp.Tracer("test")))
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
			"output_cost":             0.12,
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

func TestProvider_RecordSpan_SetsEAISpanKeys(t *testing.T) {
	p, exporter := newTestProvider(t, true)

	err := p.RecordSpan(context.Background(), observability.SpanObservation{
		ID:        "span-1",
		Name:      "tool.execute",
		Type:      "tool",
		Status:    "success",
		ToolName:  "search_db",
		CallID:    "call-1",
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
}

// TestProvider_ParentLinkageViaStateMap verifies that when a child observation
// references an earlier observation via ParentID, the child span correctly
// inherits the parent's TraceID and parent SpanID via the in-process state
// map. This is the only way bichat string-IDs can thread into OTel's binary
// SpanContext model without a custom IDGenerator.
func TestProvider_ParentLinkageViaStateMap(t *testing.T) {
	p, exporter := newTestProvider(t, true)

	require.NoError(t, p.RecordSpan(context.Background(), observability.SpanObservation{
		ID:        "agent-1",
		Name:      "agent.execute",
		Type:      "agent",
		Status:    "success",
		Timestamp: time.Now(),
		Duration:  100 * time.Millisecond,
	}))

	require.NoError(t, p.RecordSpan(context.Background(), observability.SpanObservation{
		ID:        "tool-1",
		ParentID:  "agent-1",
		Name:      "tool.execute",
		Type:      "tool",
		Status:    "success",
		Timestamp: time.Now(),
		Duration:  10 * time.Millisecond,
	}))

	spans := exporter.GetSpans()
	require.Len(t, spans, 2)
	parent, child := spans[0], spans[1]
	assert.Equal(t, parent.SpanContext.TraceID(), child.SpanContext.TraceID(),
		"child span must inherit the parent's TraceID")
	assert.Equal(t, parent.SpanContext.SpanID(), child.Parent.SpanID(),
		"child span's parent SpanID must equal the parent's real SpanID")
}

// TestProvider_RecordSpan_ErrorStatus_SetsOTelStatus verifies that a bichat
// observation with Status="error" produces an OTel span with the canonical
// error status, so backends like Jaeger / Datadog flag it as failed in their
// error-rate metrics. A custom eai.span.status alone is invisible there.
func TestProvider_RecordSpan_ErrorStatus_SetsOTelStatus(t *testing.T) {
	p, exporter := newTestProvider(t, true)

	require.NoError(t, p.RecordSpan(context.Background(), observability.SpanObservation{
		ID:        "span-err",
		Name:      "tool.execute",
		Type:      "tool",
		Status:    "error",
		Output:    "boom",
		Timestamp: time.Now(),
		Duration:  5 * time.Millisecond,
	}))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, codes.Error, spans[0].Status.Code,
		"error spans must call SetStatus(codes.Error, …) so backends flag failures")
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
	// Negative SampleRate is the sentinel for "unset" — defaults to 1.0.
	defaulted := Config{Enabled: true, Endpoint: "https://x/y", SampleRate: -1}
	require.NoError(t, defaulted.Validate())
	assert.InDelta(t, 1.0, defaulted.SampleRate, 1e-9, "negative SampleRate defaults to 1.0")

	// SampleRate=0 is preserved verbatim (callers can deliberately drop everything).
	zeroed := Config{Enabled: true, Endpoint: "https://x/y", SampleRate: 0}
	require.NoError(t, zeroed.Validate())
	assert.InDelta(t, 0.0, zeroed.SampleRate, 1e-9, "SampleRate=0 must survive Validate so callers can disable sampling")

	// SampleRate=0.5 round-trips unchanged.
	mid := Config{Enabled: true, Endpoint: "https://x/y", SampleRate: 0.5}
	require.NoError(t, mid.Validate())
	assert.InDelta(t, 0.5, mid.SampleRate, 1e-9)

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
	tp, shutdown, err := InitTracerProvider(context.Background(), Config{Enabled: false})
	require.NoError(t, err)
	assert.Nil(t, tp, "disabled config must not allocate a TracerProvider")
	require.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}

func TestInitTracerProvider_EmptyEndpointReturnsNoop(t *testing.T) {
	tp, shutdown, err := InitTracerProvider(context.Background(), Config{Enabled: true})
	require.NoError(t, err)
	assert.Nil(t, tp, "empty endpoint must not allocate a TracerProvider")
	require.NotNil(t, shutdown)
	assert.NoError(t, shutdown(context.Background()))
}
