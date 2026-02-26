package testutil

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// NoOpChatRepository is a shared default test double for domain.ChatRepository.
type NoOpChatRepository struct{}

var errNotFound = errors.New("not found")

var _ domain.ChatRepository = (*NoOpChatRepository)(nil)

func (m *NoOpChatRepository) CreateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *NoOpChatRepository) GetSession(ctx context.Context, id uuid.UUID) (domain.Session, error) {
	return nil, errNotFound
}

func (m *NoOpChatRepository) UpdateSession(ctx context.Context, session domain.Session) error {
	return nil
}

func (m *NoOpChatRepository) UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) error {
	return nil
}

func (m *NoOpChatRepository) UpdateSessionTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) (bool, error) {
	return false, nil
}

func (m *NoOpChatRepository) ListUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.Session, error) {
	return []domain.Session{}, nil
}

func (m *NoOpChatRepository) CountUserSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	return 0, nil
}

func (m *NoOpChatRepository) DeleteSession(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *NoOpChatRepository) SaveMessage(ctx context.Context, msg types.Message) error {
	return nil
}

func (m *NoOpChatRepository) GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error) {
	return nil, errNotFound
}

func (m *NoOpChatRepository) GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]types.Message, error) {
	return []types.Message{}, nil
}

func (m *NoOpChatRepository) TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error) {
	return 0, nil
}

func (m *NoOpChatRepository) UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error {
	return nil
}

func (m *NoOpChatRepository) GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error) {
	return nil, domain.ErrNoPendingQuestion
}

func (m *NoOpChatRepository) SaveAttachment(ctx context.Context, attachment domain.Attachment) error {
	return nil
}

func (m *NoOpChatRepository) GetAttachment(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	return nil, errNotFound
}

func (m *NoOpChatRepository) GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]domain.Attachment, error) {
	return []domain.Attachment{}, nil
}

func (m *NoOpChatRepository) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *NoOpChatRepository) SaveArtifact(ctx context.Context, artifact domain.Artifact) error {
	return nil
}

func (m *NoOpChatRepository) GetArtifact(ctx context.Context, id uuid.UUID) (domain.Artifact, error) {
	return nil, errNotFound
}

func (m *NoOpChatRepository) GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts domain.ListOptions) ([]domain.Artifact, error) {
	return []domain.Artifact{}, nil
}

func (m *NoOpChatRepository) DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	return 0, nil
}

func (m *NoOpChatRepository) DeleteArtifact(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *NoOpChatRepository) UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error {
	return nil
}

func (m *NoOpChatRepository) CreateRun(ctx context.Context, run domain.GenerationRun) error {
	return nil
}

func (m *NoOpChatRepository) GetActiveRunBySession(ctx context.Context, sessionID uuid.UUID) (domain.GenerationRun, error) {
	return nil, domain.ErrNoActiveRun
}

func (m *NoOpChatRepository) GetRunByID(ctx context.Context, runID uuid.UUID) (domain.GenerationRun, error) {
	return nil, domain.ErrRunNotFound
}

func (m *NoOpChatRepository) UpdateRunSnapshot(ctx context.Context, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error {
	return nil
}

func (m *NoOpChatRepository) CompleteRun(ctx context.Context, runID uuid.UUID) error {
	return nil
}

func (m *NoOpChatRepository) CancelRun(ctx context.Context, runID uuid.UUID) error {
	return nil
}

func (m *NoOpChatRepository) ListAccessibleSessionSummaries(ctx context.Context, userID int64, opts domain.ListOptions) ([]domain.SessionSummary, error) {
	return []domain.SessionSummary{}, nil
}

func (m *NoOpChatRepository) CountAccessibleSessions(ctx context.Context, userID int64, opts domain.ListOptions) (int, error) {
	return 0, nil
}

func (m *NoOpChatRepository) ListAllSessionSummaries(ctx context.Context, requestingUserID int64, opts domain.ListOptions, ownerUserID *int64) ([]domain.SessionSummary, error) {
	return []domain.SessionSummary{}, nil
}

func (m *NoOpChatRepository) CountAllSessions(ctx context.Context, opts domain.ListOptions, ownerUserID *int64) (int, error) {
	return 0, nil
}

func (m *NoOpChatRepository) ResolveSessionAccess(ctx context.Context, sessionID uuid.UUID, userID int64) (domain.SessionAccess, error) {
	return domain.NewSessionAccess(domain.SessionMemberRoleNone, domain.SessionAccessSourceNone)
}

func (m *NoOpChatRepository) ListSessionMembers(ctx context.Context, sessionID uuid.UUID) ([]domain.SessionMember, error) {
	return []domain.SessionMember{}, nil
}

func (m *NoOpChatRepository) GetTenantUser(ctx context.Context, userID int64) (domain.SessionUser, error) {
	return domain.NewSessionUser(userID, "Test", "User")
}

func (m *NoOpChatRepository) UpsertSessionMember(ctx context.Context, command domain.SessionMemberUpsert) error {
	return nil
}

func (m *NoOpChatRepository) RemoveSessionMember(ctx context.Context, command domain.SessionMemberRemoval) error {
	return nil
}

func (m *NoOpChatRepository) CountSessionParticipants(ctx context.Context, sessionID uuid.UUID) (int, error) {
	return 0, nil
}

func (m *NoOpChatRepository) ListTenantUsers(ctx context.Context) ([]domain.SessionUser, error) {
	return []domain.SessionUser{}, nil
}
