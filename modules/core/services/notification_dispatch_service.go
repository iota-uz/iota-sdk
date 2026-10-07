package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const NotificationDispatchBatchSize = 100

type NotificationDispatchService struct {
	router *NotificationRoutingService
	repo   notifications.DispatchRepository
	inbox  *NotificationService
}

func NewNotificationDispatchService(router *NotificationRoutingService, repo notifications.DispatchRepository, inbox *NotificationService) *NotificationDispatchService {
	return &NotificationDispatchService{router: router, repo: repo, inbox: inbox}
}
func (s *NotificationDispatchService) Enqueue(ctx context.Context, event notifications.Event) (int, error) {
	const op serrors.Op = "NotificationDispatchService.Enqueue"
	normalized, err := s.router.Catalog().Normalize(event)
	if err != nil {
		return 0, serrors.New(serrors.Invalid, err.Error()).WithOp(op)
	}
	rule, ids, err := s.router.ResolveRecipients(ctx, normalized)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	if !rule.Enabled {
		return 0, nil
	}
	job, err := s.repo.Enqueue(ctx, normalized, rule, ids)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	return len(job.RecipientIDs), nil
}
func (s *NotificationDispatchService) Progress(ctx context.Context) (notifications.DispatchStats, error) {
	return s.repo.Stats(ctx)
}
func (s *NotificationDispatchService) Jobs(ctx context.Context) ([]notifications.DispatchSummary, error) {
	return s.repo.List(ctx)
}
func (s *NotificationDispatchService) Retry(ctx context.Context, id uuid.UUID) error {
	return s.repo.Retry(ctx, id)
}

// Process commits notification writes and their checkpoint together. A stopped
// process leaves the last committed checkpoint available to any replica.
func (s *NotificationDispatchService) Process(ctx context.Context) (bool, error) {
	const op serrors.Op = "NotificationDispatchService.Process"
	if ctx.Value(constants.TxKey) != nil {
		return false, serrors.New(serrors.FailedPrecondition, "Dispatch processing requires its own transaction").WithOp(op)
	}
	var jobID uuid.UUID
	committed := []notification.Notification{}
	err := composables.InTx(ctx, func(txCtx context.Context) error {
		job, err := s.repo.Next(txCtx)
		if err != nil {
			return err
		}
		if job == nil {
			return nil
		}
		jobID = job.ID
		end := min(job.Cursor+NotificationDispatchBatchSize, len(job.RecipientIDs))
		txCtx = context.WithValue(txCtx, notificationDeliveryBufferKey{}, &committed)
		delivered, err := s.router.DeliverRecipients(txCtx, job.Event, job.Rule, job.RecipientIDs[job.Cursor:end])
		if err != nil {
			return err
		}
		return s.repo.Advance(txCtx, job.ID, end-job.Cursor, delivered)
	})
	if err != nil {
		if jobID != uuid.Nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
			defer cancel()
			if retryErr := s.repo.Fail(cleanup, jobID); retryErr != nil {
				return true, serrors.WrapContext(op, retryErr, "failed to record dispatch retry after delivery failure")
			}
		}
		return jobID != uuid.Nil, serrors.Wrap(op, err)
	}
	for _, n := range committed {
		if ctx.Err() != nil {
			break
		}
		s.inbox.NotifyCommitted(ctx, n)
	}
	return jobID != uuid.Nil, nil
}
