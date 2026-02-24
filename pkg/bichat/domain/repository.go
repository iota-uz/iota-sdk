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

type SessionRepository interface {
	CreateSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, id uuid.UUID) (Session, error)
	UpdateSession(ctx context.Context, session Session) error
	UpdateSessionTitle(ctx context.Context, id uuid.UUID, title string) error
	UpdateSessionTitleIfEmpty(ctx context.Context, id uuid.UUID, title string) (bool, error)
	ListUserSessions(ctx context.Context, userID int64, opts ListOptions) ([]Session, error)
	CountUserSessions(ctx context.Context, userID int64, opts ListOptions) (int, error)
	DeleteSession(ctx context.Context, id uuid.UUID) error
}

type MessageRepository interface {
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
	// When there is no pending question it returns ErrNoPendingQuestion (not nil); callers must check with errors.Is(err, domain.ErrNoPendingQuestion).
	GetPendingQuestionMessage(ctx context.Context, sessionID uuid.UUID) (types.Message, error)
}

type AttachmentRepository interface {
	SaveAttachment(ctx context.Context, attachment Attachment) error
	GetAttachment(ctx context.Context, id uuid.UUID) (Attachment, error)
	GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]Attachment, error)
	DeleteAttachment(ctx context.Context, id uuid.UUID) error
}

type ArtifactRepository interface {
	SaveArtifact(ctx context.Context, artifact Artifact) error
	GetArtifact(ctx context.Context, id uuid.UUID) (Artifact, error)
	GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts ListOptions) ([]Artifact, error)
	DeleteSessionArtifacts(ctx context.Context, sessionID uuid.UUID) (int64, error)
	DeleteArtifact(ctx context.Context, id uuid.UUID) error
	UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error
}

// GenerationRunRepository persists in-progress streaming run state for refresh-safe resume.
type GenerationRunRepository interface {
	// CreateRun inserts a new streaming run; call at stream start. Returns ErrActiveRunExists if session already has an active run.
	CreateRun(ctx context.Context, run GenerationRun) error
	// GetActiveRunBySession returns the active (status=streaming) run for the session, or nil if none.
	GetActiveRunBySession(ctx context.Context, sessionID uuid.UUID) (GenerationRun, error)
	// UpdateRunSnapshot updates partial_content and partial_metadata for the run.
	UpdateRunSnapshot(ctx context.Context, runID uuid.UUID, partialContent string, partialMetadata map[string]any) error
	// CompleteRun marks the run as completed.
	CompleteRun(ctx context.Context, runID uuid.UUID) error
	// CancelRun marks the run as cancelled.
	CancelRun(ctx context.Context, runID uuid.UUID) error
}

// ErrActiveRunExists is returned when creating a run for a session that already has an active run.
var ErrActiveRunExists = errors.New("session already has an active generation run")

// ErrNoActiveRun is returned by GetActiveRunBySession when the session has no active (streaming) run.
var ErrNoActiveRun = errors.New("no active generation run for session")

// ChatRepository defines the persistence interface for chat domain models.
// All operations are tenant-scoped for multi-tenancy isolation.
// Implementations MUST use composables.UseTenantID(ctx) for tenant isolation.
type ChatRepository interface {
	SessionRepository
	MessageRepository
	AttachmentRepository
	ArtifactRepository
	GenerationRunRepository
}
