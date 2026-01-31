package domain

import (
	"time"

	"github.com/google/uuid"
)

// MessageRole represents the role of a message sender
type MessageRole string

const (
	// RoleUser indicates a message from the user
	RoleUser MessageRole = "user"
	// RoleAssistant indicates a message from the AI assistant
	RoleAssistant MessageRole = "assistant"
	// RoleTool indicates a message from a tool execution
	RoleTool MessageRole = "tool"
	// RoleSystem indicates a system message
	RoleSystem MessageRole = "system"
)

// String returns the string representation of MessageRole
func (r MessageRole) String() string {
	return string(r)
}

// IsUser returns true if the role is user
func (r MessageRole) IsUser() bool {
	return r == RoleUser
}

// IsAssistant returns true if the role is assistant
func (r MessageRole) IsAssistant() bool {
	return r == RoleAssistant
}

// IsTool returns true if the role is tool
func (r MessageRole) IsTool() bool {
	return r == RoleTool
}

// IsSystem returns true if the role is system
func (r MessageRole) IsSystem() bool {
	return r == RoleSystem
}

// Valid returns true if the role is a valid value
func (r MessageRole) Valid() bool {
	switch r {
	case RoleUser, RoleAssistant, RoleTool, RoleSystem:
		return true
	default:
		return false
	}
}

// ToolCall represents a single tool invocation by the assistant
type ToolCall struct {
	ID        string
	Name      string
	Arguments string // JSON string
}

// Message represents a single message in a chat session.
// This is a struct (not interface) following idiomatic Go patterns.
type Message struct {
	ID          uuid.UUID
	SessionID   uuid.UUID
	Role        MessageRole
	Content     string
	ToolCalls   []ToolCall
	ToolCallID  *string
	Attachments []Attachment
	Citations   []Citation
	CreatedAt   time.Time
}

// NewMessage creates a new message with the given parameters.
// Use functional options for optional fields.
func NewMessage(opts ...MessageOption) *Message {
	m := &Message{
		ID:          uuid.New(),
		ToolCalls:   []ToolCall{},
		Attachments: []Attachment{},
		Citations:   []Citation{},
		CreatedAt:   time.Now(),
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

// MessageOption is a functional option for creating messages
type MessageOption func(*Message)

// WithMessageID sets the message ID
func WithMessageID(id uuid.UUID) MessageOption {
	return func(m *Message) {
		m.ID = id
	}
}

// WithSessionID sets the session ID
func WithSessionID(sessionID uuid.UUID) MessageOption {
	return func(m *Message) {
		m.SessionID = sessionID
	}
}

// WithRole sets the message role
func WithRole(role MessageRole) MessageOption {
	return func(m *Message) {
		m.Role = role
	}
}

// WithContent sets the message content
func WithContent(content string) MessageOption {
	return func(m *Message) {
		m.Content = content
	}
}

// WithToolCalls sets the tool calls
func WithToolCalls(toolCalls []ToolCall) MessageOption {
	return func(m *Message) {
		m.ToolCalls = toolCalls
	}
}

// WithToolCallID sets the tool call ID
func WithToolCallID(toolCallID string) MessageOption {
	return func(m *Message) {
		m.ToolCallID = &toolCallID
	}
}

// WithAttachments sets the attachments
func WithAttachments(attachments []Attachment) MessageOption {
	return func(m *Message) {
		m.Attachments = attachments
	}
}

// WithCitations sets the citations
func WithCitations(citations []Citation) MessageOption {
	return func(m *Message) {
		m.Citations = citations
	}
}

// WithCreatedAt sets the created at timestamp
func WithCreatedAt(createdAt time.Time) MessageOption {
	return func(m *Message) {
		m.CreatedAt = createdAt
	}
}

// HasToolCalls returns true if the message has tool calls
func (m *Message) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

// HasAttachments returns true if the message has attachments
func (m *Message) HasAttachments() bool {
	return len(m.Attachments) > 0
}

// HasCitations returns true if the message has citations
func (m *Message) HasCitations() bool {
	return len(m.Citations) > 0
}

// IsToolMessage returns true if this is a tool response message
func (m *Message) IsToolMessage() bool {
	return m.ToolCallID != nil
}
