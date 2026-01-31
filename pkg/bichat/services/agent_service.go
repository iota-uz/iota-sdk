package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
)

// AgentService provides the framework bridge for executing agent interactions.
// It connects the chat domain to the underlying agent framework (pkg/bichat/agents).
type AgentService interface {
	// ProcessMessage executes the agent for a user message and returns streaming events.
	// The Generator pattern allows lazy iteration over events as they occur.
	ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (Generator[Event], error)

	// ResumeWithAnswer resumes agent execution after user answers questions (HITL).
	// Returns a Generator for streaming the resumed execution.
	ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]string) (Generator[Event], error)
}

// Event represents an event during agent execution.
// This aligns with the ExecutorEvent from pkg/bichat/agents.
type Event struct {
	Type      EventType
	Content   string           // For content chunks
	Citation  *domain.Citation // For citation events
	Usage     *TokenUsage      // For usage events
	Tool      *ToolEvent       // For tool execution events
	Interrupt *InterruptEvent  // For HITL interrupts
	Error     error            // For error events
	Done      bool             // True when execution complete
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
	Name      string
	Arguments string
	Result    string
	Error     error
}

// InterruptEvent represents a HITL interrupt
type InterruptEvent struct {
	CheckpointID string
	Questions    []Question
}

// Generator is a lazy iterator pattern for streaming results.
// Inspired by Python generators, it allows processing items as they arrive
// without buffering the entire result set.
//
// Usage:
//
//	gen, err := agentService.ProcessMessage(ctx, sessionID, content, attachments)
//	if err != nil { return err }
//	defer gen.Close()
//
//	for {
//	    event, err, hasMore := gen.Next()
//	    if err != nil { return err }
//	    if !hasMore { break }
//	    handleEvent(event)
//	}
type Generator[T any] interface {
	// Next returns the next value, error (if any), and whether more values exist.
	// When hasMore is false, the generator is exhausted and should not be called again.
	Next() (value T, err error, hasMore bool)

	// Close releases resources. Should be deferred after creation.
	Close()
}
