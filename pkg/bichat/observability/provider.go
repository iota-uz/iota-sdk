package observability

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Provider is the main observability interface for BiChat.
// It enables recording generations, spans, events, and traces without coupling to specific backends.
//
// Implementations can send data to OpenTelemetry, Langfuse, custom databases, or logging systems.
// The NoOpProvider provides a zero-overhead default when observability is disabled.
type Provider interface {
	// RecordGeneration records a completed LLM generation (request + response pair).
	// This captures token usage, latency, model metadata, and finish reason.
	RecordGeneration(ctx context.Context, obs GenerationObservation) error

	// RecordSpan records a completed operation span (tool execution, context compilation, etc.).
	// This captures operation timing, inputs, outputs, and metadata.
	RecordSpan(ctx context.Context, obs SpanObservation) error

	// RecordEvent records a point-in-time event (state changes, interrupts, etc.).
	// This captures discrete moments without duration tracking.
	RecordEvent(ctx context.Context, obs EventObservation) error

	// RecordTrace records a complete trace (session-level hierarchy of generations, spans, events).
	// This enables session replay, cost analysis, and performance profiling.
	RecordTrace(ctx context.Context, obs TraceObservation) error
}

// GenerationObservation represents a completed LLM generation (request + response pair).
type GenerationObservation struct {
	// Identity
	ID        string    // Unique generation ID
	TraceID   string    // Parent trace ID (session ID)
	ParentID  string    // Parent span ID for nesting (e.g., agent span)
	TenantID  uuid.UUID // Multi-tenant isolation
	SessionID uuid.UUID // Chat session
	UserID    string    // User who initiated the session (for trace enrichment)
	Timestamp time.Time // When generation started

	// Model metadata
	Model    string // Model identifier (e.g., "claude-3-5-sonnet-20241022")
	Provider string // Provider name (e.g., "anthropic", "openai")

	// Request details
	PromptMessages int    // Number of messages sent
	PromptTokens   int    // Estimated tokens in prompt
	Tools          int    // Number of tools provided
	PromptContent  string // Optional: prompt text for debugging (may contain PII)

	// Response details
	CompletionTokens int           // Tokens in completion
	TotalTokens      int           // Total tokens (prompt + completion)
	LatencyMs        int64         // Generation latency in milliseconds
	FinishReason     string        // "stop", "tool_calls", "length", "error", etc.
	ToolCalls        int           // Number of tool calls in response
	CompletionText   string        // Optional: completion text for debugging (may contain PII)
	Duration         time.Duration // Total duration

	// Input/Output for trace visualization
	Input  interface{} // Prompt messages or user input
	Output interface{} // LLM response text

	// Metadata
	Attributes map[string]interface{} // Extensible metadata (tags, custom fields)
}

// SpanObservation represents a completed operation span.
type SpanObservation struct {
	// Identity
	ID        string    // Unique span ID
	TraceID   string    // Parent trace ID (session ID)
	ParentID  string    // Parent span ID (for nested operations)
	TenantID  uuid.UUID // Multi-tenant isolation
	SessionID uuid.UUID // Chat session
	Timestamp time.Time // When span started

	// Operation details
	Name     string        // Operation name (e.g., "tool.execute", "context.compile")
	Type     string        // Span type ("tool", "context", "custom")
	Input    string        // Operation input (JSON, text)
	Output   string        // Operation output (JSON, text, error)
	Duration time.Duration // Operation duration
	Status   string        // "success", "error", "cancelled"

	// Tool-specific fields (when Type == "tool")
	ToolName string // Tool name (e.g., "search_database")
	CallID   string // Tool call correlation ID

	// Metadata
	Attributes map[string]interface{} // Extensible metadata
}

// EventObservation represents a point-in-time event.
type EventObservation struct {
	// Identity
	ID        string    // Unique event ID
	TraceID   string    // Parent trace ID (session ID)
	TenantID  uuid.UUID // Multi-tenant isolation
	SessionID uuid.UUID // Chat session
	Timestamp time.Time // When event occurred

	// Event details
	Name    string // Event name (e.g., "interrupt", "context.overflow")
	Type    string // Event type ("session", "context", "custom")
	Message string // Human-readable description
	Level   string // "info", "warn", "error"

	// Metadata
	Attributes map[string]interface{} // Extensible metadata
}

// TraceObservation represents a complete trace (session-level hierarchy).
type TraceObservation struct {
	// Identity
	ID        string    // Trace ID (typically session ID)
	TenantID  uuid.UUID // Multi-tenant isolation
	SessionID uuid.UUID // Chat session
	Timestamp time.Time // Trace start time

	// Trace metadata
	Name        string        // Trace name (e.g., "BI Analysis Session")
	Duration    time.Duration // Total trace duration
	Status      string        // "active", "completed", "error"
	UserID      uuid.UUID     // User who initiated the session
	TotalCost   float64       // Estimated total cost (if available)
	TotalTokens int           // Total tokens across all generations

	// Metadata
	Attributes map[string]interface{} // Extensible metadata
}
