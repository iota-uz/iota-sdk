package types

import (
	"time"

	"github.com/google/uuid"
)

// Message represents a message in a conversation.
// Interface following the same design as other aggregates (e.g. Session).
type Message interface {
	ID() uuid.UUID
	SessionID() uuid.UUID
	Role() Role
	Content() string
	ToolCalls() []ToolCall
	ToolCallID() *string
	Attachments() []Attachment
	Citations() []Citation
	CodeOutputs() []CodeInterpreterOutput
	CreatedAt() time.Time

	HasToolCalls() bool
	HasAttachments() bool
	HasCitations() bool
	IsToolMessage() bool
	HasCodeOutputs() bool
}

type message struct {
	id           uuid.UUID
	sessionID    uuid.UUID
	role         Role
	content      string
	toolCalls    []ToolCall
	toolCallID   *string
	attachments  []Attachment
	citations    []Citation
	codeOutputs  []CodeInterpreterOutput
	createdAt    time.Time
}

// MessageOption is a functional option for configuring a Message.
type MessageOption func(*message)

// WithMessageID sets the message ID.
func WithMessageID(id uuid.UUID) MessageOption {
	return func(m *message) {
		m.id = id
	}
}

// WithSessionID sets the session ID.
func WithSessionID(id uuid.UUID) MessageOption {
	return func(m *message) {
		m.sessionID = id
	}
}

// WithRole sets the message role.
func WithRole(role Role) MessageOption {
	return func(m *message) {
		m.role = role
	}
}

// WithContent sets the message content.
func WithContent(content string) MessageOption {
	return func(m *message) {
		m.content = content
	}
}

// WithToolCalls sets the tool calls.
func WithToolCalls(calls ...ToolCall) MessageOption {
	return func(m *message) {
		m.toolCalls = calls
	}
}

// WithToolCallID sets the tool call ID.
func WithToolCallID(toolCallID string) MessageOption {
	return func(m *message) {
		m.toolCallID = &toolCallID
	}
}

// WithAttachments sets the attachments.
func WithAttachments(attachments ...Attachment) MessageOption {
	return func(m *message) {
		m.attachments = attachments
	}
}

// WithCitations sets the citations.
func WithCitations(citations ...Citation) MessageOption {
	return func(m *message) {
		m.citations = citations
	}
}

// WithCodeOutputs sets the code interpreter outputs.
func WithCodeOutputs(outputs ...CodeInterpreterOutput) MessageOption {
	return func(m *message) {
		m.codeOutputs = outputs
	}
}

// WithCreatedAt sets the created timestamp.
func WithCreatedAt(t time.Time) MessageOption {
	return func(m *message) {
		m.createdAt = t
	}
}

// NewMessage creates a new message with the given options.
func NewMessage(opts ...MessageOption) Message {
	m := &message{
		id:        uuid.New(),
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// UserMessage creates a new user message with the given content.
func UserMessage(content string, opts ...MessageOption) Message {
	m := &message{
		id:        uuid.New(),
		role:      RoleUser,
		content:   content,
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// AssistantMessage creates a new assistant message with the given content.
func AssistantMessage(content string, opts ...MessageOption) Message {
	m := &message{
		id:        uuid.New(),
		role:      RoleAssistant,
		content:   content,
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// ToolResponse creates a new tool message with the given tool call ID and result.
func ToolResponse(toolCallID, result string, opts ...MessageOption) Message {
	m := &message{
		id:         uuid.New(),
		role:       RoleTool,
		content:    result,
		toolCallID: &toolCallID,
		createdAt:  time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// SystemMessage creates a new system message with the given content.
func SystemMessage(content string, opts ...MessageOption) Message {
	m := &message{
		id:        uuid.New(),
		role:      RoleSystem,
		content:   content,
		createdAt: time.Now(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Getter methods for the message interface

func (m *message) ID() uuid.UUID {
	return m.id
}

func (m *message) SessionID() uuid.UUID {
	return m.sessionID
}

func (m *message) Role() Role {
	return m.role
}

func (m *message) Content() string {
	return m.content
}

func (m *message) ToolCalls() []ToolCall {
	return m.toolCalls
}

func (m *message) ToolCallID() *string {
	return m.toolCallID
}

func (m *message) Attachments() []Attachment {
	return m.attachments
}

func (m *message) Citations() []Citation {
	return m.citations
}

func (m *message) CodeOutputs() []CodeInterpreterOutput {
	return m.codeOutputs
}

func (m *message) CreatedAt() time.Time {
	return m.createdAt
}

// HasToolCalls returns true if the message has tool calls.
func (m *message) HasToolCalls() bool {
	return len(m.toolCalls) > 0
}

// HasAttachments returns true if the message has attachments.
func (m *message) HasAttachments() bool {
	return len(m.attachments) > 0
}

// HasCitations returns true if the message has citations.
func (m *message) HasCitations() bool {
	return len(m.citations) > 0
}

// IsToolMessage returns true if the message is a tool response message.
func (m *message) IsToolMessage() bool {
	return m.toolCallID != nil
}

// HasCodeOutputs returns true if the message has code interpreter outputs.
func (m *message) HasCodeOutputs() bool {
	return len(m.codeOutputs) > 0
}
