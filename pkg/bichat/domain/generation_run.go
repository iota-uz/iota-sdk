package domain

import (
	"time"

	"github.com/google/uuid"
)

// GenerationRunStatus is the status of a streaming generation run.
type GenerationRunStatus string

const (
	GenerationRunStatusStreaming GenerationRunStatus = "streaming"
	GenerationRunStatusCompleted GenerationRunStatus = "completed"
	GenerationRunStatusCancelled GenerationRunStatus = "cancelled"
)

// GenerationRun represents an in-progress or recently finished streaming run.
// Used for refresh-safe resume: partial state is persisted so a reconnecting
// client can resume from the last snapshot.
type GenerationRun interface {
	ID() uuid.UUID
	SessionID() uuid.UUID
	TenantID() uuid.UUID
	UserID() int64
	Status() GenerationRunStatus
	PartialContent() string
	PartialMetadata() map[string]any
	StartedAt() time.Time
	LastUpdatedAt() time.Time
}

type generationRun struct {
	id              uuid.UUID
	sessionID       uuid.UUID
	tenantID        uuid.UUID
	userID          int64
	status          GenerationRunStatus
	partialContent  string
	partialMetadata map[string]any
	startedAt       time.Time
	lastUpdatedAt   time.Time
}

func (r *generationRun) ID() uuid.UUID               { return r.id }
func (r *generationRun) SessionID() uuid.UUID        { return r.sessionID }
func (r *generationRun) TenantID() uuid.UUID         { return r.tenantID }
func (r *generationRun) UserID() int64               { return r.userID }
func (r *generationRun) Status() GenerationRunStatus { return r.status }
func (r *generationRun) PartialContent() string      { return r.partialContent }
func (r *generationRun) PartialMetadata() map[string]any {
	if r.partialMetadata == nil {
		return nil
	}
	return r.partialMetadata
}
func (r *generationRun) StartedAt() time.Time     { return r.startedAt }
func (r *generationRun) LastUpdatedAt() time.Time { return r.lastUpdatedAt }

// GenerationRunOption configures a generation run.
type GenerationRunOption func(*generationRun)

// NewGenerationRun creates a new run with the given options.
func NewGenerationRun(opts ...GenerationRunOption) GenerationRun {
	r := &generationRun{
		id:              uuid.New(),
		status:          GenerationRunStatusStreaming,
		partialMetadata: make(map[string]any),
		startedAt:       time.Now(),
		lastUpdatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// WithGenerationRunID sets the run ID (e.g. when loading from DB).
func WithGenerationRunID(id uuid.UUID) GenerationRunOption {
	return func(r *generationRun) { r.id = id }
}

// WithGenerationRunSessionID sets the session ID.
func WithGenerationRunSessionID(sessionID uuid.UUID) GenerationRunOption {
	return func(r *generationRun) { r.sessionID = sessionID }
}

// WithGenerationRunTenantID sets the tenant ID.
func WithGenerationRunTenantID(tenantID uuid.UUID) GenerationRunOption {
	return func(r *generationRun) { r.tenantID = tenantID }
}

// WithGenerationRunUserID sets the user ID.
func WithGenerationRunUserID(userID int64) GenerationRunOption {
	return func(r *generationRun) { r.userID = userID }
}

// WithGenerationRunStatus sets the status.
func WithGenerationRunStatus(status GenerationRunStatus) GenerationRunOption {
	return func(r *generationRun) { r.status = status }
}

// WithGenerationRunPartialContent sets the partial assistant content.
func WithGenerationRunPartialContent(content string) GenerationRunOption {
	return func(r *generationRun) { r.partialContent = content }
}

// WithGenerationRunPartialMetadata sets the partial metadata (tool_calls, etc.).
func WithGenerationRunPartialMetadata(m map[string]any) GenerationRunOption {
	return func(r *generationRun) {
		if m != nil {
			r.partialMetadata = m
		}
	}
}

// WithGenerationRunStartedAt sets the started-at timestamp.
func WithGenerationRunStartedAt(t time.Time) GenerationRunOption {
	return func(r *generationRun) { r.startedAt = t }
}

// WithGenerationRunLastUpdatedAt sets the last-updated timestamp.
func WithGenerationRunLastUpdatedAt(t time.Time) GenerationRunOption {
	return func(r *generationRun) { r.lastUpdatedAt = t }
}
