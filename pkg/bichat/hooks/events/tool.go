package events

import (
	"github.com/google/uuid"
)

// ToolStartEvent indicates a tool execution has started.
// Emitted when the agent begins executing a tool call.
type ToolStartEvent struct {
	baseEvent
	ToolName  string // Name of the tool being executed
	Arguments string // JSON-encoded arguments passed to the tool
	CallID    string // Unique identifier for this tool call
}

// NewToolStartEvent creates a new ToolStartEvent.
func NewToolStartEvent(sessionID, tenantID uuid.UUID, toolName, arguments, callID string) *ToolStartEvent {
	return &ToolStartEvent{
		baseEvent: newBaseEvent("tool.start", sessionID, tenantID),
		ToolName:  toolName,
		Arguments: arguments,
		CallID:    callID,
	}
}

// ToolCompleteEvent indicates a tool execution has completed successfully.
// Emitted when a tool finishes execution with a result.
type ToolCompleteEvent struct {
	baseEvent
	ToolName   string // Name of the tool executed
	Arguments  string // JSON-encoded arguments
	CallID     string // Unique identifier for this tool call
	Result     string // Tool execution result (as returned to LLM)
	DurationMs int64  // Execution duration in milliseconds
}

// NewToolCompleteEvent creates a new ToolCompleteEvent.
func NewToolCompleteEvent(sessionID, tenantID uuid.UUID, toolName, arguments, callID, result string, durationMs int64) *ToolCompleteEvent {
	return &ToolCompleteEvent{
		baseEvent:  newBaseEvent("tool.complete", sessionID, tenantID),
		ToolName:   toolName,
		Arguments:  arguments,
		CallID:     callID,
		Result:     result,
		DurationMs: durationMs,
	}
}

// ToolErrorEvent indicates a tool execution failed with an error.
// Emitted when a tool call returns an error.
type ToolErrorEvent struct {
	baseEvent
	ToolName   string // Name of the tool that failed
	Arguments  string // JSON-encoded arguments
	CallID     string // Unique identifier for this tool call
	Error      string // Error message
	DurationMs int64  // Time spent before failure (milliseconds)
}

// NewToolErrorEvent creates a new ToolErrorEvent.
func NewToolErrorEvent(sessionID, tenantID uuid.UUID, toolName, arguments, callID, errMsg string, durationMs int64) *ToolErrorEvent {
	return &ToolErrorEvent{
		baseEvent:  newBaseEvent("tool.error", sessionID, tenantID),
		ToolName:   toolName,
		Arguments:  arguments,
		CallID:     callID,
		Error:      errMsg,
		DurationMs: durationMs,
	}
}
