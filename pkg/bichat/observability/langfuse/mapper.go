package langfuse

import (
	"encoding/json"

	"github.com/iota-uz/iota-sdk/pkg/bichat/observability"
)

// mapGenerationToLangfuse converts a BiChat GenerationObservation to Langfuse-compatible metadata.
// Returns a map of attributes for the Langfuse generation object.
func mapGenerationToLangfuse(obs observability.GenerationObservation) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Core generation metadata
	if obs.Model != "" {
		metadata["model"] = obs.Model
	}
	if obs.Provider != "" {
		metadata["provider"] = obs.Provider
	}
	if obs.FinishReason != "" {
		metadata["finish_reason"] = obs.FinishReason
	}

	// Request details
	if obs.PromptMessages > 0 {
		metadata["prompt_messages"] = obs.PromptMessages
	}
	if obs.Tools > 0 {
		metadata["tools_count"] = obs.Tools
	}
	if obs.ToolCalls > 0 {
		metadata["tool_calls_count"] = obs.ToolCalls
	}

	// Optional debug content (may contain PII - only include if present)
	if obs.PromptContent != "" {
		metadata["prompt_content"] = obs.PromptContent
	}
	if obs.CompletionText != "" {
		metadata["completion_text"] = obs.CompletionText
	}

	// Merge custom attributes
	for k, v := range obs.Attributes {
		metadata[k] = v
	}

	return metadata
}

// extractTokenUsage extracts token usage from a GenerationObservation.
// Returns usage map in Langfuse format (input, output, total).
func extractTokenUsage(obs observability.GenerationObservation) map[string]int {
	usage := make(map[string]int)

	if obs.PromptTokens > 0 {
		usage["input"] = obs.PromptTokens
	}
	if obs.CompletionTokens > 0 {
		usage["output"] = obs.CompletionTokens
	}
	if obs.TotalTokens > 0 {
		usage["total"] = obs.TotalTokens
	}

	// Extract cache tokens from metadata if present
	if obs.Attributes != nil {
		if cacheWrite, ok := obs.Attributes["cache_write_tokens"].(int); ok && cacheWrite > 0 {
			usage["cache_write"] = cacheWrite
		}
		if cacheRead, ok := obs.Attributes["cache_read_tokens"].(int); ok && cacheRead > 0 {
			usage["cache_read"] = cacheRead
		}
	}

	return usage
}

// mapSpanToLangfuse converts a BiChat SpanObservation to Langfuse-compatible metadata.
func mapSpanToLangfuse(obs observability.SpanObservation) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Span type and operation details
	if obs.Type != "" {
		metadata["span_type"] = obs.Type
	}
	if obs.Status != "" {
		metadata["status"] = obs.Status
	}

	// Tool-specific fields
	if obs.ToolName != "" {
		metadata["tool_name"] = obs.ToolName
	}
	if obs.CallID != "" {
		metadata["call_id"] = obs.CallID
	}

	// Input/output (may be large - consider truncation in production)
	if obs.Input != "" {
		metadata["input"] = obs.Input
	}
	if obs.Output != "" {
		metadata["output"] = obs.Output
	}

	// Merge custom attributes
	for k, v := range obs.Attributes {
		metadata[k] = v
	}

	return metadata
}

// mapEventToLangfuse converts a BiChat EventObservation to Langfuse-compatible metadata.
func mapEventToLangfuse(obs observability.EventObservation) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Event details
	if obs.Type != "" {
		metadata["event_type"] = obs.Type
	}
	if obs.Message != "" {
		metadata["message"] = obs.Message
	}
	if obs.Level != "" {
		metadata["level"] = obs.Level
	}

	// Merge custom attributes
	for k, v := range obs.Attributes {
		metadata[k] = v
	}

	return metadata
}

// mapTraceToLangfuse converts a BiChat TraceObservation to Langfuse-compatible metadata.
func mapTraceToLangfuse(obs observability.TraceObservation) map[string]interface{} {
	metadata := make(map[string]interface{})

	// Trace-level metadata
	if obs.Status != "" {
		metadata["status"] = obs.Status
	}
	if obs.UserID.String() != "00000000-0000-0000-0000-000000000000" {
		metadata["user_id"] = obs.UserID.String()
	}
	if obs.TotalCost > 0 {
		metadata["total_cost"] = obs.TotalCost
	}
	if obs.TotalTokens > 0 {
		metadata["total_tokens"] = obs.TotalTokens
	}

	// Merge custom attributes
	for k, v := range obs.Attributes {
		metadata[k] = v
	}

	return metadata
}

// toJSONString safely converts an object to JSON string.
// Returns empty string if serialization fails.
func toJSONString(v interface{}) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(data)
}
