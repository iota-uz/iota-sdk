package events

import (
	"github.com/google/uuid"
)

// LLMRequestEvent captures details of an LLM API request.
// Emitted before sending a request to the LLM provider.
type LLMRequestEvent struct {
	baseEvent
	RequestID       string // Deterministic request correlation ID for request/response pairing
	Model           string // Model identifier (e.g., "claude-sonnet-4-6", "gpt-5.2")
	Provider        string // Provider name (e.g., "anthropic", "openai")
	Messages        int    // Number of messages in the request
	Tools           int    // Number of tools provided
	EstimatedTokens int    // Estimated token count (if available)
	UserInput       string // Last user message content (for trace Input)
}

// NewLLMRequestEvent creates a new LLMRequestEvent.
func NewLLMRequestEvent(sessionID, tenantID uuid.UUID, model, provider string, messages, tools, estimatedTokens int, userInput string) *LLMRequestEvent {
	return NewLLMRequestEventWithTrace(sessionID, tenantID, sessionID.String(), uuid.New().String(), model, provider, messages, tools, estimatedTokens, userInput)
}

// NewLLMRequestEventWithTrace creates a new LLMRequestEvent with explicit trace/request IDs.
func NewLLMRequestEventWithTrace(sessionID, tenantID uuid.UUID, traceID, requestID, model, provider string, messages, tools, estimatedTokens int, userInput string) *LLMRequestEvent {
	return &LLMRequestEvent{
		baseEvent:       newBaseEventWithTrace("llm.request", sessionID, tenantID, traceID),
		RequestID:       requestID,
		Model:           model,
		Provider:        provider,
		Messages:        messages,
		Tools:           tools,
		EstimatedTokens: estimatedTokens,
		UserInput:       userInput,
	}
}

// LLMResponseEvent captures details of an LLM API response.
// Emitted after receiving a complete (non-streaming) response from the LLM.
type LLMResponseEvent struct {
	baseEvent
	RequestID         string                 // Correlation ID copied from the matching request event
	Model             string                 // Model identifier
	Provider          string                 // Provider name
	PromptTokens      int                    // Input tokens consumed
	CompletionTokens  int                    // Output tokens generated
	TotalTokens       int                    // Total tokens (prompt + completion + cache)
	CacheWriteTokens  int                    // Cache write tokens (prompt caching)
	CacheReadTokens   int                    // Cache read tokens (prompt caching)
	LatencyMs         int64                  // Response latency in milliseconds
	FinishReason      string                 // "stop", "tool_calls", "length", etc.
	ToolCalls         int                    // Number of tool calls in the response
	ResponseText      string                 // Accumulated response text (for trace Output)
	Thinking          string                 // Final reasoning text emitted for this LLM response
	ObservationReason string                 // Annotation for degraded observability paths
	ToolCallSummary   []LLMToolCallSummary   // Optional tool-call summary for debug/metadata
	ModelParameters   map[string]interface{} // Model parameters (temperature, max_tokens, store, etc.)
}

// LLMToolCallSummary is a lightweight projection of tool calls returned by an LLM response.
type LLMToolCallSummary struct {
	CallID string `json:"callId,omitempty"`
	Name   string `json:"name,omitempty"`
}

// NewLLMResponseEvent creates a new LLMResponseEvent.
func NewLLMResponseEvent(
	sessionID, tenantID uuid.UUID,
	model, provider string,
	promptTokens, completionTokens, totalTokens int,
	latencyMs int64,
	finishReason string,
	toolCalls int,
	responseText string,
) *LLMResponseEvent {
	return NewLLMResponseEventWithTrace(
		sessionID,
		tenantID,
		sessionID.String(),
		"",
		model,
		provider,
		promptTokens,
		completionTokens,
		totalTokens,
		latencyMs,
		finishReason,
		toolCalls,
		responseText,
		"",
		"",
		nil,
	)
}

// NewLLMResponseEventWithTrace creates a new LLMResponseEvent with explicit trace/request IDs.
func NewLLMResponseEventWithTrace(
	sessionID, tenantID uuid.UUID,
	traceID, requestID, model, provider string,
	promptTokens, completionTokens, totalTokens int,
	latencyMs int64,
	finishReason string,
	toolCalls int,
	responseText string,
	thinking string,
	observationReason string,
	toolCallSummary []LLMToolCallSummary,
) *LLMResponseEvent {
	return &LLMResponseEvent{
		baseEvent:         newBaseEventWithTrace("llm.response", sessionID, tenantID, traceID),
		RequestID:         requestID,
		Model:             model,
		Provider:          provider,
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		TotalTokens:       totalTokens,
		CacheWriteTokens:  0, // Set separately if needed
		CacheReadTokens:   0, // Set separately if needed
		LatencyMs:         latencyMs,
		FinishReason:      finishReason,
		ToolCalls:         toolCalls,
		ResponseText:      responseText,
		Thinking:          thinking,
		ObservationReason: observationReason,
		ToolCallSummary:   toolCallSummary,
	}
}

// NewLLMResponseEventWithCache creates a new LLMResponseEvent with cache token tracking.
func NewLLMResponseEventWithCache(
	sessionID, tenantID uuid.UUID,
	model, provider string,
	promptTokens, completionTokens, totalTokens int,
	cacheWriteTokens, cacheReadTokens int,
	latencyMs int64,
	finishReason string,
	toolCalls int,
	responseText string,
) *LLMResponseEvent {
	return NewLLMResponseEventWithCacheAndTrace(
		sessionID,
		tenantID,
		sessionID.String(),
		"",
		model,
		provider,
		promptTokens,
		completionTokens,
		totalTokens,
		cacheWriteTokens,
		cacheReadTokens,
		latencyMs,
		finishReason,
		toolCalls,
		responseText,
		"",
		"",
		nil,
	)
}

// NewLLMResponseEventWithCacheAndTrace creates a cache-aware response event with explicit trace/request IDs.
func NewLLMResponseEventWithCacheAndTrace(
	sessionID, tenantID uuid.UUID,
	traceID, requestID, model, provider string,
	promptTokens, completionTokens, totalTokens int,
	cacheWriteTokens, cacheReadTokens int,
	latencyMs int64,
	finishReason string,
	toolCalls int,
	responseText string,
	thinking string,
	observationReason string,
	toolCallSummary []LLMToolCallSummary,
) *LLMResponseEvent {
	return &LLMResponseEvent{
		baseEvent:         newBaseEventWithTrace("llm.response", sessionID, tenantID, traceID),
		RequestID:         requestID,
		Model:             model,
		Provider:          provider,
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		TotalTokens:       totalTokens,
		CacheWriteTokens:  cacheWriteTokens,
		CacheReadTokens:   cacheReadTokens,
		LatencyMs:         latencyMs,
		FinishReason:      finishReason,
		ToolCalls:         toolCalls,
		ResponseText:      responseText,
		Thinking:          thinking,
		ObservationReason: observationReason,
		ToolCallSummary:   toolCallSummary,
	}
}

// LLMStreamEvent captures a chunk from a streaming LLM response.
// Emitted for each chunk received during streaming.
type LLMStreamEvent struct {
	baseEvent
	Model     string // Model identifier
	Provider  string // Provider name
	ChunkText string // Text content of this chunk
	Index     int    // Chunk sequence number (0-indexed)
	IsFinal   bool   // True if this is the last chunk
}

// NewLLMStreamEvent creates a new LLMStreamEvent.
func NewLLMStreamEvent(sessionID, tenantID uuid.UUID, model, provider, chunkText string, index int, isFinal bool) *LLMStreamEvent {
	return &LLMStreamEvent{
		baseEvent: newBaseEvent("llm.stream", sessionID, tenantID),
		Model:     model,
		Provider:  provider,
		ChunkText: chunkText,
		Index:     index,
		IsFinal:   isFinal,
	}
}
