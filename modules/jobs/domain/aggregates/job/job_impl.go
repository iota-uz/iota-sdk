package job

import (
	"time"

	"github.com/google/uuid"
)

type job struct {
	id             uuid.UUID
	tenantID       uuid.UUID
	userID         uint
	kind           string
	params         map[string]any
	status         Status
	progress       int
	phase          string
	attempt        int
	resultUploadID *uint
	errMessage     string
	createdAt      time.Time
	updatedAt      time.Time
	startedAt      *time.Time
	finishedAt     *time.Time
}

type Option func(*job)

func WithID(id uuid.UUID) Option {
	return func(j *job) {
		j.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(j *job) {
		j.tenantID = tenantID
	}
}

func WithUserID(userID uint) Option {
	return func(j *job) {
		j.userID = userID
	}
}

func WithParams(params map[string]any) Option {
	return func(j *job) {
		j.params = params
	}
}

func WithStatus(status Status) Option {
	return func(j *job) {
		j.status = status
	}
}

func WithProgress(progress int) Option {
	return func(j *job) {
		j.progress = clampProgress(progress)
	}
}

func WithPhase(phase string) Option {
	return func(j *job) {
		j.phase = phase
	}
}

func WithAttempt(attempt int) Option {
	return func(j *job) {
		j.attempt = attempt
	}
}

func WithResultUploadID(uploadID *uint) Option {
	return func(j *job) {
		j.resultUploadID = uploadID
	}
}

func WithError(errMessage string) Option {
	return func(j *job) {
		j.errMessage = errMessage
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(j *job) {
		j.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(j *job) {
		j.updatedAt = updatedAt
	}
}

func WithStartedAt(startedAt *time.Time) Option {
	return func(j *job) {
		j.startedAt = startedAt
	}
}

func WithFinishedAt(finishedAt *time.Time) Option {
	return func(j *job) {
		j.finishedAt = finishedAt
	}
}

func New(kind string, opts ...Option) Job {
	j := &job{
		kind:   kind,
		params: map[string]any{},
		status: StatusQueued,
	}
	for _, opt := range opts {
		opt(j)
	}
	return j
}

func (j *job) ID() uuid.UUID          { return j.id }
func (j *job) TenantID() uuid.UUID    { return j.tenantID }
func (j *job) UserID() uint           { return j.userID }
func (j *job) Kind() string           { return j.kind }
func (j *job) Params() map[string]any { return j.params }
func (j *job) Status() Status         { return j.status }
func (j *job) Progress() int          { return j.progress }
func (j *job) Phase() string          { return j.phase }
func (j *job) Attempt() int           { return j.attempt }
func (j *job) ResultUploadID() *uint  { return j.resultUploadID }
func (j *job) Error() string          { return j.errMessage }
func (j *job) CreatedAt() time.Time   { return j.createdAt }
func (j *job) UpdatedAt() time.Time   { return j.updatedAt }
func (j *job) StartedAt() *time.Time  { return j.startedAt }
func (j *job) FinishedAt() *time.Time { return j.finishedAt }

func (j *job) SetID(id uuid.UUID) { j.id = id }

func (j *job) MarkRunning() Job {
	return j.transition(StatusQueued, StatusRunning, func(copy *job) {
		now := time.Now()
		copy.startedAt = &now
	})
}

func (j *job) UpdateProgress(percent int, phase string) Job {
	if j.status != StatusRunning {
		return j
	}
	copy := j.clone()
	copy.progress = clampProgress(percent)
	if phase != "" {
		copy.phase = phase
	}
	return copy
}

func (j *job) MarkDone(uploadID *uint) Job {
	return j.transition(StatusRunning, StatusDone, func(copy *job) {
		now := time.Now()
		copy.finishedAt = &now
		copy.resultUploadID = uploadID
		copy.progress = 100
		copy.errMessage = ""
	})
}

func (j *job) MarkFailed(errMessage string) Job {
	return j.transition(StatusRunning, StatusFailed, func(copy *job) {
		now := time.Now()
		copy.finishedAt = &now
		copy.errMessage = errMessage
	})
}

func (j *job) Requeue() Job {
	return j.transition(StatusFailed, StatusQueued, func(copy *job) {
		copy.progress = 0
		copy.phase = ""
		copy.errMessage = ""
		copy.resultUploadID = nil
		copy.finishedAt = nil
	})
}

// transition returns a copy of the job moved from -> to. When the current
// status does not match from, the job is returned unchanged so illegal
// transitions are inert rather than panicking in worker goroutines.
func (j *job) transition(from, to Status, mutate func(*job)) Job {
	if j.status != from {
		return j
	}
	copy := j.clone()
	copy.status = to
	copy.updatedAt = time.Now()
	if mutate != nil {
		mutate(copy)
	}
	return copy
}

func (j *job) clone() *job {
	copy := *j
	return &copy
}

func clampProgress(percent int) int {
	if percent < 0 {
		return 0
	}
	if percent > 100 {
		return 100
	}
	return percent
}
