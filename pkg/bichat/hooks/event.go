// Package hooks provides this package.
package hooks

import (
	"time"

	"github.com/google/uuid"
)

// Event represents something that happened in the BI-chat system.
// Events are published to the EventBus and processed by EventHandlers
// for observability, cost tracking, and custom integrations.
type Event interface {
	// Type returns the event type identifier
	Type() string
	// Timestamp returns when the event occurred
	Timestamp() time.Time
	// SessionID returns the chat session this event belongs to
	SessionID() uuid.UUID
	// TenantID returns the tenant this event belongs to
	TenantID() uuid.UUID
}

// EventType identifies the kind of event.
type EventType string

const (
	// Agent lifecycle events
	EventAgentStart    EventType = "agent.start"
	EventAgentComplete EventType = "agent.complete"
	EventAgentError    EventType = "agent.error"

	// LLM API interaction events
	EventLLMRequest  EventType = "llm.request"
	EventLLMResponse EventType = "llm.response"
	EventLLMStream   EventType = "llm.stream"

	// Tool execution events
	EventToolStart    EventType = "tool.start"
	EventToolComplete EventType = "tool.complete"
	EventToolError    EventType = "tool.error"

	// Context management events
	EventContextCompile  EventType = "context.compile"
	EventContextCompact  EventType = "context.compact"
	EventContextOverflow EventType = "context.overflow"

	// Session and message events
	EventSessionCreate       EventType = "session.create"
	EventSessionTitleUpdated EventType = "session.title_updated"
	EventMessageSave         EventType = "message.save"
	EventInterrupt           EventType = "interrupt"
)
