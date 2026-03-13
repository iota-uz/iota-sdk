// Package domain provides this package.
package domain

import (
	"errors"
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

var (
	ErrInvalidGenerationRun           = errors.New("invalid generation run")
	ErrInvalidGenerationRunTransition = errors.New("invalid generation run transition")
)

type GenerationRunSpec struct {
	ID              uuid.UUID
	SessionID       uuid.UUID
	TenantID        uuid.UUID
	UserID          int64
	Status          GenerationRunStatus
	PartialContent  string
	PartialMetadata map[string]any
	StartedAt       time.Time
	LastUpdatedAt   time.Time
}

// GenerationRun represents an in-progress or recently finished streaming run.
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

	UpdateSnapshot(partialContent string, partialMetadata map[string]any, now time.Time) (GenerationRun, error)
	Complete(now time.Time) (GenerationRun, error)
	Cancel(now time.Time) (GenerationRun, error)
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

func cloneRunMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		out[k] = v
	}
	return out
}

func validateRunStatus(status GenerationRunStatus) bool {
	switch status {
	case GenerationRunStatusStreaming, GenerationRunStatusCompleted, GenerationRunStatusCancelled:
		return true
	default:
		return false
	}
}

func normalizeRunNow(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now()
	}
	return now
}

// NewGenerationRun creates a validated streaming run.
func NewGenerationRun(spec GenerationRunSpec) (GenerationRun, error) {
	if spec.SessionID == uuid.Nil || spec.TenantID == uuid.Nil {
		return nil, ErrInvalidGenerationRun
	}
	if spec.UserID <= 0 {
		return nil, ErrInvalidGenerationRun
	}
	status := spec.Status
	if status == "" {
		status = GenerationRunStatusStreaming
	}
	if status != GenerationRunStatusStreaming {
		return nil, ErrInvalidGenerationRun
	}

	id := spec.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	startedAt := spec.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	lastUpdatedAt := spec.LastUpdatedAt
	if lastUpdatedAt.IsZero() {
		lastUpdatedAt = startedAt
	}

	return &generationRun{
		id:              id,
		sessionID:       spec.SessionID,
		tenantID:        spec.TenantID,
		userID:          spec.UserID,
		status:          GenerationRunStatusStreaming,
		partialContent:  spec.PartialContent,
		partialMetadata: cloneRunMetadata(spec.PartialMetadata),
		startedAt:       startedAt,
		lastUpdatedAt:   lastUpdatedAt,
	}, nil
}

// RehydrateGenerationRun loads a run from persistence.
func RehydrateGenerationRun(spec GenerationRunSpec) (GenerationRun, error) {
	if spec.ID == uuid.Nil || spec.SessionID == uuid.Nil || spec.TenantID == uuid.Nil || spec.UserID <= 0 {
		return nil, ErrInvalidGenerationRun
	}
	if !validateRunStatus(spec.Status) {
		return nil, ErrInvalidGenerationRun
	}
	startedAt := spec.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	lastUpdatedAt := spec.LastUpdatedAt
	if lastUpdatedAt.IsZero() {
		lastUpdatedAt = startedAt
	}

	return &generationRun{
		id:              spec.ID,
		sessionID:       spec.SessionID,
		tenantID:        spec.TenantID,
		userID:          spec.UserID,
		status:          spec.Status,
		partialContent:  spec.PartialContent,
		partialMetadata: cloneRunMetadata(spec.PartialMetadata),
		startedAt:       startedAt,
		lastUpdatedAt:   lastUpdatedAt,
	}, nil
}

func (r *generationRun) copy() *generationRun {
	c := *r
	c.partialMetadata = cloneRunMetadata(r.partialMetadata)
	return &c
}

func (r *generationRun) ID() uuid.UUID               { return r.id }
func (r *generationRun) SessionID() uuid.UUID        { return r.sessionID }
func (r *generationRun) TenantID() uuid.UUID         { return r.tenantID }
func (r *generationRun) UserID() int64               { return r.userID }
func (r *generationRun) Status() GenerationRunStatus { return r.status }
func (r *generationRun) PartialContent() string      { return r.partialContent }
func (r *generationRun) PartialMetadata() map[string]any {
	return cloneRunMetadata(r.partialMetadata)
}
func (r *generationRun) StartedAt() time.Time     { return r.startedAt }
func (r *generationRun) LastUpdatedAt() time.Time { return r.lastUpdatedAt }

func (r *generationRun) UpdateSnapshot(partialContent string, partialMetadata map[string]any, now time.Time) (GenerationRun, error) {
	if r.status != GenerationRunStatusStreaming {
		return nil, ErrInvalidGenerationRunTransition
	}
	c := r.copy()
	c.partialContent = partialContent
	c.partialMetadata = cloneRunMetadata(partialMetadata)
	c.lastUpdatedAt = normalizeRunNow(now)
	return c, nil
}

func (r *generationRun) Complete(now time.Time) (GenerationRun, error) {
	if r.status != GenerationRunStatusStreaming {
		return nil, ErrInvalidGenerationRunTransition
	}
	c := r.copy()
	c.status = GenerationRunStatusCompleted
	c.lastUpdatedAt = normalizeRunNow(now)
	return c, nil
}

func (r *generationRun) Cancel(now time.Time) (GenerationRun, error) {
	if r.status != GenerationRunStatusStreaming {
		return nil, ErrInvalidGenerationRunTransition
	}
	c := r.copy()
	c.status = GenerationRunStatusCancelled
	c.lastUpdatedAt = normalizeRunNow(now)
	return c, nil
}
