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
	// GenerationRunStatusFailed marks a run that was terminated by an
	// unrecoverable error (LLM provider, tool crash, stale heartbeat).
	// Semantically distinct from Cancelled: cancelled was user-initiated,
	// failed was system-initiated. Frontend surfaces a "Regenerate" CTA
	// on failed runs and does not persist the partial message.
	GenerationRunStatusFailed GenerationRunStatus = "failed"
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
	// CancelRequested is the out-of-band flag a Stop RPC flips while the
	// worker is streaming. Workers poll this between executor iterations
	// and promote it into a GenerationRunStatusCancelled transition.
	// Rehydrated in-flight runs preserve the request so a reclaiming
	// worker sees the user's intent.
	CancelRequested bool
	// LastHeartbeatAt is refreshed by the worker during execution. The
	// reaper uses it to detect wedged runs (e.g. worker crash mid-LLM
	// call) and transition them to GenerationRunStatusFailed without
	// needing to observe the process itself.
	LastHeartbeatAt time.Time
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
	// CancelRequested reports whether a Stop RPC has flipped the cancel
	// flag on this run. Workers poll this between executor iterations.
	CancelRequested() bool
	// LastHeartbeatAt is the most recent wall-clock time at which the
	// worker proved the run was still making progress. Zero while the
	// run is queued or has never heartbeated.
	LastHeartbeatAt() time.Time

	UpdateSnapshot(partialContent string, partialMetadata map[string]any, now time.Time) (GenerationRun, error)
	Complete(now time.Time) (GenerationRun, error)
	Cancel(now time.Time) (GenerationRun, error)
	// Fail transitions a streaming run to the terminal failed status.
	// It is used by workers on unrecoverable errors and by the reaper
	// when a run's heartbeat goes stale. Callers should also write a
	// corresponding terminal event onto the RunEventLog so tailing
	// clients observe the transition.
	Fail(now time.Time) (GenerationRun, error)
	// RequestCancel sets the out-of-band cancel flag. It does NOT move
	// the run to the Cancelled status; the worker does that after it
	// observes the flag and winds execution down. Returns a new run
	// with CancelRequested==true and a refreshed LastUpdatedAt.
	RequestCancel(now time.Time) (GenerationRun, error)
	// Heartbeat refreshes LastHeartbeatAt + LastUpdatedAt without
	// mutating the snapshot. Only valid while streaming.
	Heartbeat(now time.Time) (GenerationRun, error)
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
	cancelRequested bool
	lastHeartbeatAt time.Time
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
	case GenerationRunStatusStreaming, GenerationRunStatusCompleted, GenerationRunStatusCancelled, GenerationRunStatusFailed:
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
		cancelRequested: spec.CancelRequested,
		lastHeartbeatAt: spec.LastHeartbeatAt,
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
		cancelRequested: spec.CancelRequested,
		lastHeartbeatAt: spec.LastHeartbeatAt,
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
func (r *generationRun) StartedAt() time.Time       { return r.startedAt }
func (r *generationRun) LastUpdatedAt() time.Time   { return r.lastUpdatedAt }
func (r *generationRun) CancelRequested() bool      { return r.cancelRequested }
func (r *generationRun) LastHeartbeatAt() time.Time { return r.lastHeartbeatAt }

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

// Fail transitions a streaming run to the terminal failed status. Mirrors
// Cancel's guard: only a live run can fail; double-failing is a no-op
// transition error so callers don't accidentally clobber a completed run.
func (r *generationRun) Fail(now time.Time) (GenerationRun, error) {
	if r.status != GenerationRunStatusStreaming {
		return nil, ErrInvalidGenerationRunTransition
	}
	c := r.copy()
	c.status = GenerationRunStatusFailed
	c.lastUpdatedAt = normalizeRunNow(now)
	return c, nil
}

// RequestCancel sets the out-of-band cancel flag. The run stays in the
// streaming state until the worker observes the flag and drives a Cancel
// transition. Safe to call repeatedly; already-requested stays requested.
func (r *generationRun) RequestCancel(now time.Time) (GenerationRun, error) {
	if r.status != GenerationRunStatusStreaming {
		return nil, ErrInvalidGenerationRunTransition
	}
	c := r.copy()
	c.cancelRequested = true
	c.lastUpdatedAt = normalizeRunNow(now)
	return c, nil
}

// Heartbeat refreshes the liveness timestamps without touching the snapshot
// or status. Only meaningful while streaming.
func (r *generationRun) Heartbeat(now time.Time) (GenerationRun, error) {
	if r.status != GenerationRunStatusStreaming {
		return nil, ErrInvalidGenerationRunTransition
	}
	ts := normalizeRunNow(now)
	c := r.copy()
	c.lastHeartbeatAt = ts
	c.lastUpdatedAt = ts
	return c, nil
}
