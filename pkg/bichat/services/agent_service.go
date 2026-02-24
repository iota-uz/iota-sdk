package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

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
