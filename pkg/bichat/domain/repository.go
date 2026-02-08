package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ErrNoPendingQuestion is returned by GetPendingQuestionMessage when no message has a pending question.
var ErrNoPendingQuestion = errors.New("no pending question message")

// ListOptions provides pagination options for repository queries
type ListOptions struct {
	Limit  int
	Offset int
	// IncludeArchived includes archived sessions in list results (default: false = active only)
	IncludeArchived bool
	// Types filters GetSessionArtifacts by artifact type (optional)
	Types []ArtifactType
}

// ChatRepository defines the persistence interface for chat domain models.
// All operations are tenant-scoped for multi-tenancy isolation.
// Implementations MUST use composables.UseTenantID(ctx) for tenant isolation.
type ChatRepository interface {
	// Session operations
	CreateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, id uuid.UUID) (Session, error)
	UpdateSession(ctx context.Context, session Session) error
	ListUserSessions(ctx context.Context, userID int64, opts ListOptions) ([]Session, error)
	CountUserSessions(ctx context.Context, userID int64, opts ListOptions) (int, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error

	// Message operations
	SaveMessage(ctx context.Context, msg types.Message) error
	GetMessage(ctx context.Context, id uuid.UUID) (types.Message, error)
	GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts ListOptions) ([]types.Message, error)
	// TruncateMessagesFrom deletes all messages in a session from a given timestamp forward.
	// Returns the number of messages deleted.
	// Used for regenerate/edit functionality.
	TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error)

	// UpdateMessageQuestionData updates the question_data JSONB on a specific message.
	UpdateMessageQuestionData(ctx context.Context, msgID uuid.UUID, qd *types.QuestionData) error
	// GetPendingQuestionMessage returns the message with a pending question for a session.
	// It returns ErrNoPendingQuestion (not nil) when there is no pending question; callers should check with errors.Is(err, domain.ErrNoPendingQuestion).
	GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error)

	// Attachment operations
	SaveAttachment(ctx context.Context, attachment Attachment) error
	GetAttachment(ctx context.Context, id uuid.UUID) (Attachment, error)
	GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]Attachment, error)
	DeleteAttachment(ctx context.Context, id uuid.UUID) error

	// Artifact operations
	SaveArtifact(ctx context.Context, artifact Artifact) error
	GetArtifact(ctx context.Context, id uuid.UUID) (Artifact, error)
	GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts ListOptions) ([]Artifact, error)
	DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error)
	DeleteArtifact(ctx context.Context, id uuid.UUID) error
	UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error
}
