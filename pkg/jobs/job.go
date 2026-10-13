// Package jobs provides a lightweight primitive for user-triggered
// long-running operations (exports, heavy reports, bulk actions): register a
// handler, enqueue it from an HTTP request, and let the standard components
// poll progress and offer the result for download.
//
// State lives in a Store — Redis when configured (shared across replicas,
// TTL-evicted), in-process memory otherwise. Execution runs in the enqueueing
// process, so no database tables, queues, or workers are required.
package jobs

import (
	"time"

	"github.com/google/uuid"
)

// Status describes the lifecycle state of a job.
type Status string

const (
	// StatusQueued means the job is created but execution has not started.
	StatusQueued Status = "queued"
	// StatusRunning means the handler is executing.
	StatusRunning Status = "running"
	// StatusDone means the job finished successfully.
	StatusDone Status = "done"
	// StatusFailed means the job terminated with an error.
	StatusFailed Status = "failed"
)

// IsTerminal reports whether the status is terminal.
func (s Status) IsTerminal() bool {
	return s == StatusDone || s == StatusFailed
}

// String returns the raw status string.
func (s Status) String() string {
	return string(s)
}

// Job is a plain, serialisable snapshot of one operation. It is copied in and
// out of the Store, so it carries no synchronisation primitives.
type Job struct {
	ID        uuid.UUID
	TenantID  uuid.UUID
	UserID    uint
	Kind      string
	Params    map[string]any
	Status    Status
	Progress  int
	Phase     string
	Error     string
	ResultURL string
	// ResultName is set when the handler returned result bytes that the
	// runner stored in the Store; the download endpoint streams them.
	ResultName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
}

// HasStoredResult reports whether downloadable bytes exist in the Store.
func (j Job) HasStoredResult() bool {
	return j.Status == StatusDone && j.ResultName != "" && j.ResultURL == ""
}

// OwnedBy reports whether the job belongs to the given tenant/user pair.
func (j Job) OwnedBy(tenantID uuid.UUID, userID uint) bool {
	return j.TenantID == tenantID && j.UserID == userID
}
