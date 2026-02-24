package events

import (
	"github.com/google/uuid"
)

// AgentStartEvent indicates an agent execution has started.
// Emitted at the beginning of the ReAct loop in the executor.
type AgentStartEvent struct {
	baseEvent
	AgentName string // Name of the agent being executed
	IsResume  bool   // True if this is a resumed execution
}

// NewAgentStartEvent creates a new AgentStartEvent.
func NewAgentStartEvent(sessionID, tenantID uuid.UUID, agentName string, isResume bool) *AgentStartEvent {
	return NewAgentStartEventWithTrace(sessionID, tenantID, sessionID.String(), agentName, isResume)
}

// NewAgentStartEventWithTrace creates a new AgentStartEvent with an explicit trace ID.
func NewAgentStartEventWithTrace(sessionID, tenantID uuid.UUID, traceID, agentName string, isResume bool) *AgentStartEvent {
	return &AgentStartEvent{
		baseEvent: newBaseEventWithTrace("agent.start", sessionID, tenantID, traceID),
		AgentName: agentName,
		IsResume:  isResume,
	}
}

// AgentCompleteEvent indicates an agent execution completed successfully.
// Emitted when the ReAct loop finishes normally.
type AgentCompleteEvent struct {
	baseEvent
	AgentName   string // Name of the agent
	Iterations  int    // Number of ReAct loop iterations
	TotalTokens int    // Accumulated tokens across all LLM calls
	DurationMs  int64  // Total execution duration in milliseconds
}

// NewAgentCompleteEvent creates a new AgentCompleteEvent.
func NewAgentCompleteEvent(sessionID, tenantID uuid.UUID, agentName string, iterations, totalTokens int, durationMs int64) *AgentCompleteEvent {
	return NewAgentCompleteEventWithTrace(sessionID, tenantID, sessionID.String(), agentName, iterations, totalTokens, durationMs)
}

// NewAgentCompleteEventWithTrace creates a new AgentCompleteEvent with an explicit trace ID.
func NewAgentCompleteEventWithTrace(sessionID, tenantID uuid.UUID, traceID, agentName string, iterations, totalTokens int, durationMs int64) *AgentCompleteEvent {
	return &AgentCompleteEvent{
		baseEvent:   newBaseEventWithTrace("agent.complete", sessionID, tenantID, traceID),
		AgentName:   agentName,
		Iterations:  iterations,
		TotalTokens: totalTokens,
		DurationMs:  durationMs,
	}
}

// AgentErrorEvent indicates an agent execution failed with an error.
// Emitted when the ReAct loop encounters a fatal error.
type AgentErrorEvent struct {
	baseEvent
	AgentName  string // Name of the agent
	Iterations int    // Number of iterations completed before error
	Error      string // Error message
	DurationMs int64  // Duration before failure in milliseconds
}

// NewAgentErrorEvent creates a new AgentErrorEvent.
func NewAgentErrorEvent(sessionID, tenantID uuid.UUID, agentName string, iterations int, errMsg string, durationMs int64) *AgentErrorEvent {
	return NewAgentErrorEventWithTrace(sessionID, tenantID, sessionID.String(), agentName, iterations, errMsg, durationMs)
}

// NewAgentErrorEventWithTrace creates a new AgentErrorEvent with an explicit trace ID.
func NewAgentErrorEventWithTrace(sessionID, tenantID uuid.UUID, traceID, agentName string, iterations int, errMsg string, durationMs int64) *AgentErrorEvent {
	return &AgentErrorEvent{
		baseEvent:  newBaseEventWithTrace("agent.error", sessionID, tenantID, traceID),
		AgentName:  agentName,
		Iterations: iterations,
		Error:      errMsg,
		DurationMs: durationMs,
	}
}
