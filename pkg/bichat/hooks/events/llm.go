package events

import (
	"github.com/google/uuid"
)

// LLMRequestEvent captures details of an LLM API request.
// Emitted before sending a request to the LLM provider.
type LLMRequestEvent struct {
	baseEvent
	Model           string // Model identifier (e.g., "claude-3-5-sonnet-20241022")
	Provider        string // Provider name (e.g., "anthropic", "openai")
	Messages        int    // Number of messages in the request
	Tools           int    // Number of tools provided
	EstimatedTokens int    // Estimated token count (if available)
}

// NewLLMRequestEvent creates a new LLMRequestEvent.
func NewLLMRequestEvent(sessionID, tenantID uuid.UUID, model, provider string, messages, tools, estimatedTokens int) *LLMRequestEvent {
	return &LLMRequestEvent{
		baseEvent:       newBaseEvent("llm.request", sessionID, tenantID),
		Model:           model,
		Provider:        provider,
		Messages:        messages,
		Tools:           tools,
		EstimatedTokens: estimatedTokens,
	}
}

// LLMResponseEvent captures details of an LLM API response.
// Emitted after receiving a complete (non-streaming) response from the LLM.
type LLMResponseEvent struct {
	baseEvent
	Model            string // Model identifier
	Provider         string // Provider name
	PromptTokens     int    // Input tokens consumed
	CompletionTokens int    // Output tokens generated
	TotalTokens      int    // Total tokens (prompt + completion + cache)
	CacheWriteTokens int    // Cache write tokens (prompt caching)
	CacheReadTokens  int    // Cache read tokens (prompt caching)
	LatencyMs        int64  // Response latency in milliseconds
	FinishReason     string // "stop", "tool_calls", "length", etc.
	ToolCalls        int    // Number of tool calls in the response
}

// NewLLMResponseEvent creates a new LLMResponseEvent.
func NewLLMResponseEvent(
	sessionID, tenantID uuid.UUID,
	model, provider string,
	promptTokens, completionTokens, totalTokens int,
	latencyMs int64,
	finishReason string,
	toolCalls int,
) *LLMResponseEvent {
	return &LLMResponseEvent{
		baseEvent:        newBaseEvent("llm.response", sessionID, tenantID),
		Model:            model,
		Provider:         provider,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		CacheWriteTokens: 0, // Set separately if needed
		CacheReadTokens:  0, // Set separately if needed
		LatencyMs:        latencyMs,
		FinishReason:     finishReason,
		ToolCalls:        toolCalls,
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
) *LLMResponseEvent {
	return &LLMResponseEvent{
		baseEvent:        newBaseEvent("llm.response", sessionID, tenantID),
		Model:            model,
		Provider:         provider,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		CacheWriteTokens: cacheWriteTokens,
		CacheReadTokens:  cacheReadTokens,
		LatencyMs:        latencyMs,
		FinishReason:     finishReason,
		ToolCalls:        toolCalls,
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
