package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
)

type agentServiceStub struct{}

// NewAgentServiceStub creates a stub implementation of AgentService.
// This is a temporary implementation until the Agent Framework is complete.
func NewAgentServiceStub() services.AgentService {
	return &agentServiceStub{}
}

// ProcessMessage executes the agent for a user message and returns streaming events.
func (s *agentServiceStub) ProcessMessage(ctx context.Context, sessionID uuid.UUID, content string, attachments []domain.Attachment) (services.Generator[services.Event], error) {
	const op = "agentServiceStub.ProcessMessage"
	return nil, errors.New("not implemented - Phase 1 pending")
}

// ResumeWithAnswer resumes agent execution after user answers questions (HITL).
func (s *agentServiceStub) ResumeWithAnswer(ctx context.Context, sessionID uuid.UUID, checkpointID string, answers map[string]string) (services.Generator[services.Event], error) {
	const op = "agentServiceStub.ResumeWithAnswer"
	return nil, errors.New("not implemented - Phase 1 pending")
}
