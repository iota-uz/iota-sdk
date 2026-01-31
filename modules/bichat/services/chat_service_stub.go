package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type chatServiceStub struct{}

// NewChatServiceStub creates a stub implementation of ChatService.
// This is a temporary implementation until the Agent Framework is complete.
func NewChatServiceStub() services.ChatService {
	return &chatServiceStub{}
}

// CreateSession creates a new chat session.
func (s *chatServiceStub) CreateSession(ctx context.Context, tenantID uuid.UUID, userID int64, title string) (*domain.Session, error) {
	const op serrors.Op = "chatServiceStub.CreateSession"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// GetSession retrieves a session by ID.
func (s *chatServiceStub) GetSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceStub.GetSession"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// ListUserSessions lists all sessions for a user.
func (s *chatServiceStub) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]*domain.Session, error) {
	const op serrors.Op = "chatServiceStub.ListUserSessions"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// ArchiveSession archives a session.
func (s *chatServiceStub) ArchiveSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceStub.ArchiveSession"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// PinSession pins a session.
func (s *chatServiceStub) PinSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceStub.PinSession"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// UnpinSession unpins a session.
func (s *chatServiceStub) UnpinSession(ctx context.Context, sessionID uuid.UUID) (*domain.Session, error) {
	const op serrors.Op = "chatServiceStub.UnpinSession"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// SendMessage sends a message to a session.
func (s *chatServiceStub) SendMessage(ctx context.Context, req services.SendMessageRequest) (*services.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceStub.SendMessage"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// SendMessageStream sends a message and streams the response.
func (s *chatServiceStub) SendMessageStream(ctx context.Context, req services.SendMessageRequest, onChunk func(services.StreamChunk)) error {
	const op serrors.Op = "chatServiceStub.SendMessageStream"
	return serrors.E(op, "not implemented - Phase 1 pending")
}

// GetSessionMessages retrieves all messages for a session.
func (s *chatServiceStub) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]*types.Message, error) {
	const op serrors.Op = "chatServiceStub.GetSessionMessages"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// ResumeWithAnswer resumes execution after user answers questions.
func (s *chatServiceStub) ResumeWithAnswer(ctx context.Context, req services.ResumeRequest) (*services.SendMessageResponse, error) {
	const op serrors.Op = "chatServiceStub.ResumeWithAnswer"
	return nil, serrors.E(op, "not implemented - Phase 1 pending")
}

// GenerateSessionTitle generates a title for a session based on first message.
func (s *chatServiceStub) GenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	const op serrors.Op = "chatServiceStub.GenerateSessionTitle"
	return serrors.E(op, "not implemented - Phase 1 pending")
}
