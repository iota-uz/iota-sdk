package job

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrNotFound is returned when a job does not exist within the caller's scope.
	ErrNotFound = errors.New("job not found")
)

// Repository persists jobs. All methods except ClaimNext and
// ListFinishedBefore are tenant-scoped via the context; the two system-level
// methods are used by the background worker, which sweeps across tenants.
type Repository interface {
	// Save inserts the job when its id is nil, otherwise updates it.
	Save(ctx context.Context, j Job) (Job, error)

	GetByID(ctx context.Context, id uuid.UUID) (Job, error)

	// GetByIDForUser additionally requires the job to belong to userID.
	GetByIDForUser(ctx context.Context, id uuid.UUID, userID uint) (Job, error)

	Delete(ctx context.Context, id uuid.UUID) error

	// ListByUser returns the user's jobs, newest first.
	ListByUser(ctx context.Context, userID uint, limit int) ([]Job, error)

	// ClaimNext atomically transitions the oldest queued job to running and
	// returns it. The second return value is false when no job is queued.
	ClaimNext(ctx context.Context) (Job, bool, error)

	// UpdateProgress persists progress percent and phase label without a
	// full row rewrite; it is cheap enough to call from a throttled reporter.
	UpdateProgress(ctx context.Context, id uuid.UUID, percent int, phase string) error

	// ListFinishedBefore returns terminal jobs finished before the cutoff so
	// the worker's retention sweep can expire them and their result files.
	ListFinishedBefore(ctx context.Context, cutoff time.Time, limit int) ([]Job, error)
}
