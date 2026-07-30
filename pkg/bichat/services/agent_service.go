// Package services provides this package.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

var ErrInvalidContinuation = errors.New("invalid internal continuation")

// ContinuationEvent is a trusted application event that resumes an existing
// chat session. It is never attributed to a user.
type ContinuationEvent struct {
	Trigger       string          `json:"trigger"`
	CorrelationID string          `json:"correlation_id"`
	Payload       json.RawMessage `json:"payload,omitempty"`
}

type continuationEventContextKey struct{}

// WithContinuationEvent attaches a trusted continuation to the execution
// context. Application tools can correlate durable workflow state without
// relying on the model to copy opaque identifiers from the rendered prompt.
func WithContinuationEvent(ctx context.Context, event ContinuationEvent) context.Context {
	return context.WithValue(ctx, continuationEventContextKey{}, event)
}

// UseContinuationEvent returns the trusted continuation that started the
// current turn. Ordinary user turns do not carry one.
func UseContinuationEvent(ctx context.Context) (ContinuationEvent, bool) {
	event, ok := ctx.Value(continuationEventContextKey{}).(ContinuationEvent)
	if !ok || event.Validate() != nil {
		return ContinuationEvent{}, false
	}
	return event, true
}

func (e ContinuationEvent) Validate() error {
	if strings.TrimSpace(e.Trigger) == "" || strings.TrimSpace(e.CorrelationID) == "" {
		return ErrInvalidContinuation
	}
	if len(e.Payload) > 0 && !json.Valid(e.Payload) {
		return ErrInvalidContinuation
	}
	return nil
}

func (e ContinuationEvent) Prompt() (string, error) {
	if err := e.Validate(); err != nil {
		return "", err
	}
	if len(e.Payload) == 0 {
		e.Payload = json.RawMessage(`{}`)
	}
	data, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidContinuation, err)
	}
	return "INTERNAL CONTINUATION EVENT\n" +
		"This event was emitted by a trusted application workflow, not by the user. " +
		"Resume the existing goal using the conversation history and event payload. " +
		"Do not ask the user to repeat identifiers already present here.\n" +
		string(data), nil
}

// AgentService provides the framework bridge for executing agent interactions.
// It connects the chat domain to the underlying agent framework (pkg/bichat/agents).
type AgentService interface {
	// ProcessMessage executes the agent for a user message and returns streaming events.
	// The Generator pattern allows lazy iteration over events as they occur.
	// Events are agents.ExecutorEvent; use agents.EventType* constants to switch on event type.
	ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (types.Generator[agents.ExecutorEvent], error)

	// ResumeWithAnswer resumes agent execution after user answers questions (HITL).
	// Returns a Generator for streaming the resumed execution.
	ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]types.Answer) (types.Generator[agents.ExecutorEvent], error)
}

// ContinuationAgentService is implemented by agents that support trusted
// internal continuation turns.
type ContinuationAgentService interface {
	ProcessContinuation(ctx context.Context, sessionID uuid.UUID, event ContinuationEvent) (types.Generator[agents.ExecutorEvent], error)
}
