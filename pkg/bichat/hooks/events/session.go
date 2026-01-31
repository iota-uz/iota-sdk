package events

import (
	"github.com/google/uuid"
)

// SessionCreateEvent indicates a new chat session was created.
// Emitted when a user starts a new conversation.
type SessionCreateEvent struct {
	baseEvent
	UserID int64  // User who created the session
	Title  string // Initial session title (may be empty)
}

// NewSessionCreateEvent creates a new SessionCreateEvent.
func NewSessionCreateEvent(sessionID, tenantID uuid.UUID, userID int64, title string) *SessionCreateEvent {
	return &SessionCreateEvent{
		baseEvent: newBaseEvent("session.create", sessionID, tenantID),
		UserID:    userID,
		Title:     title,
	}
}

// MessageSaveEvent indicates a message was persisted to storage.
// Emitted after a user or assistant message is saved to the database.
type MessageSaveEvent struct {
	baseEvent
	MessageID  uuid.UUID // Unique message identifier
	Role       string    // "user", "assistant", "tool", "system"
	ContentLen int       // Length of message content in characters
	ToolCalls  int       // Number of tool calls (for assistant messages)
}

// NewMessageSaveEvent creates a new MessageSaveEvent.
func NewMessageSaveEvent(sessionID, tenantID, messageID uuid.UUID, role string, contentLen, toolCalls int) *MessageSaveEvent {
	return &MessageSaveEvent{
		baseEvent:  newBaseEvent("message.save", sessionID, tenantID),
		MessageID:  messageID,
		Role:       role,
		ContentLen: contentLen,
		ToolCalls:  toolCalls,
	}
}

// InterruptEvent indicates agent execution was interrupted for human input.
// Emitted when HITL (human-in-the-loop) interrupts execution.
type InterruptEvent struct {
	baseEvent
	InterruptType string // Type of interrupt ("ask_user_question", custom types)
	AgentName     string // Name of agent that was interrupted
	Question      string // Question or prompt for the user (if applicable)
	CheckpointID  string // Checkpoint identifier for resumption
}

// NewInterruptEvent creates a new InterruptEvent.
func NewInterruptEvent(sessionID, tenantID uuid.UUID, interruptType, agentName, question, checkpointID string) *InterruptEvent {
	return &InterruptEvent{
		baseEvent:     newBaseEvent("interrupt", sessionID, tenantID),
		InterruptType: interruptType,
		AgentName:     agentName,
		Question:      question,
		CheckpointID:  checkpointID,
	}
}
