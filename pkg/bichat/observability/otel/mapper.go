package otel

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
	"go.opentelemetry.io/otel/attribute"
)

// Attribute keys used by this provider.
//
// GenAI keys follow the OpenTelemetry GenAI semantic conventions
// (https://opentelemetry.io/docs/specs/semconv/gen-ai/). Cost is intentionally
// NOT emitted — it is computed server-side by the trace backend (e.g. Langfuse)
// from gen_ai.request.model + token counts.
const (
	// GenAI semantic-convention keys.
	attrGenAIRequestModel        = "gen_ai.request.model"
	attrGenAISystem              = "gen_ai.system"
	attrGenAIUsageInputTokens    = "gen_ai.usage.input_tokens"
	attrGenAIUsageOutputTokens   = "gen_ai.usage.output_tokens"
	attrGenAIUsageTotalTokens    = "gen_ai.usage.total_tokens"
	attrGenAIUsageCacheReadIn    = "gen_ai.usage.cache_read_input_tokens"
	attrGenAIUsageCacheCreateIn  = "gen_ai.usage.cache_creation_input_tokens"
	attrGenAIResponseFinishReas  = "gen_ai.response.finish_reasons"
	attrGenAIRequestMessages     = "gen_ai.request.messages_count"
	attrGenAIRequestToolsCount   = "gen_ai.request.tools_count"
	attrGenAIResponseToolCalls   = "gen_ai.response.tool_calls_count"
	attrGenAILatencyMs           = "gen_ai.latency.ms"
	attrGenAIResponseThinking    = "gen_ai.response.thinking"
	attrGenAIRequestParamsPrefix = "gen_ai.request."

	// Enduser keys.
	attrEnduserID    = "enduser.id"
	attrEnduserEmail = "enduser.email"

	// EAI / Langfuse-specific keys.
	attrEAITenantID         = "eai.tenant.id"
	attrEAIObservationLevel = "eai.observation.level"
	attrEAIObservationReas  = "eai.observation.reason"
	attrEAISpanType         = "eai.span.type"
	attrEAISpanStatus       = "eai.span.status"
	attrEAIToolName         = "eai.tool.name"
	attrEAIToolCallID       = "eai.tool.call_id"
	attrEAIEventType        = "eai.event.type"
	attrEAIEventLevel       = "eai.event.level"
	attrEAIEventMessage     = "eai.event.message"
	attrEAIAttrPrefix       = "eai.attr."
	attrEAIEventAttrPrefix  = "eai.event.attr."
	attrEAISpanAttrPrefix   = "eai.span.attr."
	attrEAITraceAttrPrefix  = "eai.trace.attr."
	attrEAITraceStatus      = "eai.trace.status"

	attrLangfuseSessionID  = "langfuse.session.id"
	attrLangfuseObsInput   = "langfuse.observation.input"
	attrLangfuseObsOutput  = "langfuse.observation.output"
	attrLangfuseTraceName  = "langfuse.trace.name"
	attrLangfuseTraceTags  = "langfuse.trace.tags"
	attrLangfuseUpdateKind = "langfuse.update.kind"
)

// generationToAttributes maps a GenerationObservation to OTel GenAI semantic
// convention attributes. It does NOT emit any cost-related keys: cost is
// derived server-side from gen_ai.request.model + token counts.
func generationToAttributes(obs observability.GenerationObservation) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 16)

	if obs.Model != "" {
		attrs = append(attrs, attribute.String(attrGenAIRequestModel, obs.Model))
	}
	if obs.Provider != "" {
		attrs = append(attrs, attribute.String(attrGenAISystem, obs.Provider))
	}
	if obs.PromptTokens > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIUsageInputTokens, obs.PromptTokens))
	}
	if obs.CompletionTokens > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIUsageOutputTokens, obs.CompletionTokens))
	}
	if obs.TotalTokens > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIUsageTotalTokens, obs.TotalTokens))
	}
	if v, ok := readIntAttr(obs.Attributes, "cache_read_tokens"); ok && v > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIUsageCacheReadIn, v))
	}
	if v, ok := readIntAttr(obs.Attributes, "cache_write_tokens"); ok && v > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIUsageCacheCreateIn, v))
	}
	if obs.FinishReason != "" {
		attrs = append(attrs, attribute.StringSlice(attrGenAIResponseFinishReas, []string{obs.FinishReason}))
	}
	if obs.UserID != "" {
		attrs = append(attrs, attribute.String(attrEnduserID, obs.UserID))
	}
	if obs.UserEmail != "" {
		attrs = append(attrs, attribute.String(attrEnduserEmail, obs.UserEmail))
	}
	if !isZeroUUID(obs.TenantID) {
		attrs = append(attrs, attribute.String(attrEAITenantID, obs.TenantID.String()))
	}
	if !isZeroUUID(obs.SessionID) {
		attrs = append(attrs, attribute.String(attrLangfuseSessionID, obs.SessionID.String()))
	}
	if s := jsonString(obs.Input); s != "" {
		attrs = append(attrs, attribute.String(attrLangfuseObsInput, s))
	}
	if s := jsonString(obs.Output); s != "" {
		attrs = append(attrs, attribute.String(attrLangfuseObsOutput, s))
	}
	if obs.PromptMessages > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIRequestMessages, obs.PromptMessages))
	}
	if obs.Tools > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIRequestToolsCount, obs.Tools))
	}
	if obs.ToolCalls > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIResponseToolCalls, obs.ToolCalls))
	}
	if obs.LatencyMs > 0 {
		attrs = append(attrs, attribute.Int64(attrGenAILatencyMs, obs.LatencyMs))
	}
	if obs.Thinking != "" {
		attrs = append(attrs, attribute.String(attrGenAIResponseThinking, obs.Thinking))
	}
	if obs.ObservationReason != "" {
		attrs = append(attrs, attribute.String(attrEAIObservationReas, obs.ObservationReason))
	}
	if obs.Level != "" {
		attrs = append(attrs, attribute.String(attrEAIObservationLevel, obs.Level))
	}

	for k, v := range obs.ModelParameters {
		if k == "" {
			continue
		}
		key := attrGenAIRequestParamsPrefix + k
		if kv, ok := scalarToKV(key, v); ok {
			attrs = append(attrs, kv)
		}
	}

	return attrs
}

// spanToAttributes maps a SpanObservation to attributes using the eai.span.*
// namespace plus shared langfuse.observation.input/output keys.
func spanToAttributes(obs observability.SpanObservation) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 8)

	if obs.Type != "" {
		attrs = append(attrs, attribute.String(attrEAISpanType, obs.Type))
	}
	if obs.Status != "" {
		attrs = append(attrs, attribute.String(attrEAISpanStatus, obs.Status))
	}
	if obs.ToolName != "" {
		attrs = append(attrs, attribute.String(attrEAIToolName, obs.ToolName))
	}
	if obs.CallID != "" {
		attrs = append(attrs, attribute.String(attrEAIToolCallID, obs.CallID))
	}
	if obs.Input != "" {
		attrs = append(attrs, attribute.String(attrLangfuseObsInput, obs.Input))
	}
	if obs.Output != "" {
		attrs = append(attrs, attribute.String(attrLangfuseObsOutput, obs.Output))
	}
	if obs.Level != "" {
		attrs = append(attrs, attribute.String(attrEAIObservationLevel, obs.Level))
	}
	if !isZeroUUID(obs.TenantID) {
		attrs = append(attrs, attribute.String(attrEAITenantID, obs.TenantID.String()))
	}
	if !isZeroUUID(obs.SessionID) {
		attrs = append(attrs, attribute.String(attrLangfuseSessionID, obs.SessionID.String()))
	}

	for k, v := range obs.Attributes {
		if k == "" {
			continue
		}
		if kv, ok := scalarToKV(attrEAISpanAttrPrefix+k, v); ok {
			attrs = append(attrs, kv)
		}
	}

	return attrs
}

// eventToAttributes maps an EventObservation to attributes using the
// eai.event.* namespace.
func eventToAttributes(obs observability.EventObservation) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 6)

	if obs.Type != "" {
		attrs = append(attrs, attribute.String(attrEAIEventType, obs.Type))
	}
	if obs.Message != "" {
		attrs = append(attrs, attribute.String(attrEAIEventMessage, obs.Message))
	}
	if obs.Level != "" {
		attrs = append(attrs, attribute.String(attrEAIEventLevel, obs.Level))
	}
	if !isZeroUUID(obs.TenantID) {
		attrs = append(attrs, attribute.String(attrEAITenantID, obs.TenantID.String()))
	}
	if !isZeroUUID(obs.SessionID) {
		attrs = append(attrs, attribute.String(attrLangfuseSessionID, obs.SessionID.String()))
	}

	for k, v := range obs.Attributes {
		if k == "" {
			continue
		}
		if kv, ok := scalarToKV(attrEAIEventAttrPrefix+k, v); ok {
			attrs = append(attrs, kv)
		}
	}

	return attrs
}

// traceToAttributes maps a TraceObservation to attributes. Cost is omitted
// intentionally — backends compute it from per-generation tokens.
func traceToAttributes(obs observability.TraceObservation) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 6)

	if obs.Status != "" {
		attrs = append(attrs, attribute.String(attrEAITraceStatus, obs.Status))
	}
	if !isZeroUUID(obs.UserID) {
		attrs = append(attrs, attribute.String(attrEnduserID, obs.UserID.String()))
	}
	if !isZeroUUID(obs.TenantID) {
		attrs = append(attrs, attribute.String(attrEAITenantID, obs.TenantID.String()))
	}
	if !isZeroUUID(obs.SessionID) {
		attrs = append(attrs, attribute.String(attrLangfuseSessionID, obs.SessionID.String()))
	}
	if obs.TotalTokens > 0 {
		attrs = append(attrs, attribute.Int(attrGenAIUsageTotalTokens, obs.TotalTokens))
	}
	if obs.Name != "" {
		attrs = append(attrs, attribute.String(attrLangfuseTraceName, obs.Name))
	}

	for k, v := range obs.Attributes {
		if k == "" {
			continue
		}
		if kv, ok := scalarToKV(attrEAITraceAttrPrefix+k, v); ok {
			attrs = append(attrs, kv)
		}
	}

	return attrs
}

// readIntAttr extracts an int-like value from a generic attributes map,
// accepting both int and int64 (protective in case bridge changes types).
func readIntAttr(m map[string]interface{}, key string) (int, bool) {
	if m == nil {
		return 0, false
	}
	raw, ok := m[key]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	}
	return 0, false
}

// scalarToKV converts a scalar value to an attribute.KeyValue. It returns
// (_, false) for nested maps, slices, or unsupported types — callers may
// safely skip those.
func scalarToKV(key string, v interface{}) (attribute.KeyValue, bool) {
	switch val := v.(type) {
	case string:
		if val == "" {
			return attribute.KeyValue{}, false
		}
		return attribute.String(key, val), true
	case bool:
		return attribute.Bool(key, val), true
	case int:
		return attribute.Int(key, val), true
	case int32:
		return attribute.Int(key, int(val)), true
	case int64:
		return attribute.Int64(key, val), true
	case float32:
		return attribute.Float64(key, float64(val)), true
	case float64:
		return attribute.Float64(key, val), true
	case fmt.Stringer:
		s := val.String()
		if s == "" {
			return attribute.KeyValue{}, false
		}
		return attribute.String(key, s), true
	}
	return attribute.KeyValue{}, false
}

// jsonString marshals v to a JSON string. Returns "" for nil or marshal error
// (observability must never break the app).
func jsonString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		// Avoid double-encoding: pass-through plain strings.
		s = strings.TrimSpace(s)
		if s == "" {
			return ""
		}
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// isZeroUUID reports whether u is the zero UUID.
func isZeroUUID(u uuid.UUID) bool {
	return u == uuid.Nil
}
