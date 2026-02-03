package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
)

// ListOptions provides pagination options for repository queries
type ListOptions struct {
	Limit  int
	Offset int
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
	DeleteSession(ctx context.Context, id uuid.UUID) error

	// Message operations
	SaveMessage(ctx context.Context, msg *types.Message) error
	GetMessage(ctx context.Context, id uuid.UUID) (*types.Message, error)
	GetSessionMessages(ctx context.Context, sessionID uuid.UUID, opts ListOptions) ([]*types.Message, error)
	// TruncateMessagesFrom deletes all messages in a session from a given timestamp forward.
	// Returns the number of messages deleted.
	// Used for regenerate/edit functionality.
	TruncateMessagesFrom(ctx context.Context, sessionID uuid.UUID, from time.Time) (int64, error)

	// Attachment operations
	SaveAttachment(ctx context.Context, attachment Attachment) error
	GetAttachment(ctx context.Context, id uuid.UUID) (Attachment, error)
	GetMessageAttachments(ctx context.Context, messageID uuid.UUID) ([]Attachment, error)
	DeleteAttachment(ctx context.Context, id uuid.UUID) error

	// Artifact operations
	SaveArtifact(ctx context.Context, artifact Artifact) error
	GetArtifact(ctx context.Context, id uuid.UUID) (Artifact, error)
	GetSessionArtifacts(ctx context.Context, sessionID uuid.UUID, opts ListOptions) ([]Artifact, error)
	DeleteArtifact(ctx context.Context, id uuid.UUID) error
	UpdateArtifact(ctx context.Context, id uuid.UUID, name, description string) error
}
