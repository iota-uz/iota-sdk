package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

var (
	// ErrUnknownJobKind is returned when enqueueing a kind no component registered.
	ErrUnknownJobKind = errors.New("unknown job kind")
	// ErrJobNotRetryable is returned when retrying a job that has not failed.
	ErrJobNotRetryable = errors.New("job is not retryable")
)

// DefaultMyJobsLimit is how many of the user's recent jobs the operations
// widget asks for by default.
const DefaultMyJobsLimit = 20

type JobService struct {
	repo      job.Repository
	registry  *Registry
	publisher eventbus.EventBus
}

func NewJobService(
	repo job.Repository,
	registry *Registry,
	publisher eventbus.EventBus,
) *JobService {
	return &JobService{
		repo:      repo,
		registry:  registry,
		publisher: publisher,
	}
}

// Enqueue validates the kind, persists a queued job owned by the calling
// user, and publishes a CreatedEvent. The HTTP request returns immediately;
// a background worker claims and executes the job.
func (s *JobService) Enqueue(ctx context.Context, kind string, params map[string]any) (job.Job, error) {
	const op serrors.Op = "JobService.Enqueue"

	if _, ok := s.registry.Get(kind); !ok {
		return nil, serrors.E(op, ErrUnknownJobKind, fmt.Sprintf("kind %q has no registered handler", kind))
	}
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if params == nil {
		params = map[string]any{}
	}

	created, err := s.repo.Save(ctx, job.New(
		kind,
		job.WithTenantID(tenantID),
		job.WithUserID(user.ID()),
		job.WithParams(params),
	))
	if err != nil {
		return nil, serrors.E(op, err)
	}
	createdEvent, err := job.NewCreatedEvent(ctx, created)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	s.publisher.Publish(createdEvent)
	return created, nil
}

// Get returns a job. When the context carries a user, ownership is enforced.
func (s *JobService) Get(ctx context.Context, id uuid.UUID) (job.Job, error) {
	const op serrors.Op = "JobService.Get"
	user, userErr := composables.UseUser(ctx)
	if userErr != nil {
		found, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		return found, nil
	}
	found, err := s.repo.GetByIDForUser(ctx, id, user.ID())
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return found, nil
}

// ListMine returns the calling user's jobs, newest first.
func (s *JobService) ListMine(ctx context.Context, limit int) ([]job.Job, error) {
	const op serrors.Op = "JobService.ListMine"
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if limit <= 0 {
		limit = DefaultMyJobsLimit
	}
	jobs, err := s.repo.ListByUser(ctx, user.ID(), limit)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return jobs, nil
}

// Retry requeues a failed job so the worker executes it again. Only failed
// jobs are retryable; the params and kind are preserved.
func (s *JobService) Retry(ctx context.Context, id uuid.UUID) (job.Job, error) {
	const op serrors.Op = "JobService.Retry"
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	found, err := s.repo.GetByIDForUser(ctx, id, user.ID())
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if found.Status() != job.StatusFailed {
		return nil, serrors.E(op, ErrJobNotRetryable)
	}
	requeued, err := s.repo.Save(ctx, found.Requeue())
	if err != nil {
		return nil, serrors.E(op, err)
	}
	return requeued, nil
}

// Delete dismisses one of the calling user's jobs.
func (s *JobService) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "JobService.Delete"
	user, err := composables.UseUser(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	found, err := s.repo.GetByIDForUser(ctx, id, user.ID())
	if err != nil {
		return serrors.E(op, err)
	}
	if err := s.repo.Delete(ctx, found.ID()); err != nil {
		return serrors.E(op, err)
	}
	return nil
}
