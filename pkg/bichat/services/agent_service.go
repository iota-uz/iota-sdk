package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// AgentService provides the framework bridge for executing agent interactions.
// It connects the chat domain to the underlying agent framework (pkg/bichat/agents).
type AgentService interface {
	// ProcessMessage executes the agent for a user message and returns streaming events.
	// The Generator pattern allows lazy iteration over events as they occur.
	ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (types.Generator[Event], error)

	// ResumeWithAnswer resumes agent execution after user answers questions (HITL).
	// Returns a Generator for streaming the resumed execution.
	ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]types.Answer) (types.Generator[Event], error)
}

// Event represents an event during agent execution.
// This aligns with the ExecutorEvent from pkg/bichat/agents.
type Event struct {
	Type               EventType
	Content            string            // For content chunks
	Citation           *types.Citation   // For citation events
	Usage              *types.DebugUsage // For usage events
	Tool               *ToolEvent        // For tool execution events
	Interrupt          *InterruptEvent   // For HITL interrupts
	ProviderResponseID string            // Provider continuity token (on done events)
	CodeInterpreter    []types.CodeInterpreterResult
	FileAnnotations    []types.FileAnnotation
	Error              error // For error events
	Done               bool  // True when execution complete
}

// EventType identifies the kind of event
type EventType string

const (
	EventTypeContent   EventType = "content"
	EventTypeCitation  EventType = "citation"
	EventTypeUsage     EventType = "usage"
	EventTypeToolStart EventType = "tool_start"
	EventTypeToolEnd   EventType = "tool_end"
	EventTypeInterrupt EventType = "interrupt"
	EventTypeDone      EventType = "done"
	EventTypeError     EventType = "error"
)

// ToolEvent represents a tool execution event
type ToolEvent struct {
	CallID     string
	Name       string
	Arguments  string
	Result     string
	Error      error
	DurationMs int64
	Artifacts  []types.ToolArtifact
}

// InterruptEvent represents a HITL interrupt
type InterruptEvent struct {
	CheckpointID       string
	AgentName          string // Name of the agent that triggered this interrupt
	ProviderResponseID string
	Questions          []Question
}
