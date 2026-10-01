// Package job provides this package.
package job

import (
	"time"

	"github.com/google/uuid"
)

// Status describes the lifecycle state of an async job.
type Status string

const (
	// StatusQueued means the job is waiting for a worker to claim it.
	StatusQueued Status = "queued"
	// StatusRunning means a worker is actively executing the job.
	StatusRunning Status = "running"
	// StatusDone means the job finished successfully.
	StatusDone Status = "done"
	// StatusFailed means the job terminated with an error.
	StatusFailed Status = "failed"
)

// IsTerminal reports whether the status is terminal (no further transitions).
func (s Status) IsTerminal() bool {
	return s == StatusDone || s == StatusFailed
}

// String returns the raw status string.
func (s Status) String() string {
	return string(s)
}

// Job is a user-triggered long-running operation (export, heavy report, bulk
// mutation) tracked with progress so the UI can poll it instead of holding a
// single long-lived HTTP request open.
type Job interface {
	ID() uuid.UUID
	SetID(uuid.UUID)

	TenantID() uuid.UUID
	UserID() uint

	Kind() string
	Params() map[string]any

	Status() Status
	Progress() int
	Phase() string
	Attempt() int

	ResultUploadID() *uint
	Error() string

	CreatedAt() time.Time
	UpdatedAt() time.Time
	StartedAt() *time.Time
	FinishedAt() *time.Time

	// MarkRunning transitions queued -> running.
	MarkRunning() Job
	// UpdateProgress records progress percent (0-100) and an optional phase label.
	UpdateProgress(percent int, phase string) Job
	// MarkDone transitions running -> done, optionally referencing a stored result file.
	MarkDone(uploadID *uint) Job
	// MarkFailed transitions running -> failed with a user-visible error message.
	MarkFailed(errMessage string) Job
	// Requeue transitions failed -> queued so the user can retry the operation.
	Requeue() Job
}
