package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type notificationDeliveryBufferKey struct{}

type NotificationService struct {
	repo        notification.Repository
	invalidator NotificationInvalidator
}

func NewNotificationService(repo notification.Repository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) WithRealtime(invalidator NotificationInvalidator) *NotificationService {
	s.invalidator = invalidator
	return s
}

func (s *NotificationService) NotifyCommitted(ctx context.Context, n notification.Notification) {
	if s.invalidator != nil {
		s.invalidator.Notify(ctx, n)
	}
}

func (s *NotificationService) invalidate(ctx context.Context) {
	if s.invalidator == nil || ctx.Value(constants.TxKey) != nil {
		return
	}
	u, err := composables.UseUser(ctx)
	if err == nil {
		s.invalidator.Invalidate(ctx, u.TenantID(), u.ID())
	}
}

// Deliver is a trusted producer API. Recipient ownership is enforced by the repository.
func (s *NotificationService) Deliver(ctx context.Context, n notification.Notification) (notification.Notification, error) {
	const op serrors.Op = "NotificationService.Deliver"
	if err := notification.Validate(n); err != nil {
		return nil, serrors.New(serrors.Invalid, "").WithOp(op).WithCause(err)
	}
	saved, err := s.repo.Create(ctx, n)
	if err != nil {
		return nil, err
	}
	if buffer, ok := ctx.Value(notificationDeliveryBufferKey{}).(*[]notification.Notification); ok {
		*buffer = append(*buffer, saved)
	} else if ctx.Value(constants.TxKey) == nil {
		s.NotifyCommitted(ctx, saved)
	}
	return saved, nil
}
func (s *NotificationService) List(ctx context.Context, p notification.FindParams) ([]notification.Notification, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.Wrap("NotificationService.List", err)
	}
	return s.repo.List(ctx, u.ID(), p.Bounded())
}
func (s *NotificationService) UnreadCount(ctx context.Context) (int64, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return 0, serrors.Wrap("NotificationService.UnreadCount", err)
	}
	return s.repo.UnreadCount(ctx, u.ID())
}
func (s *NotificationService) MarkRead(ctx context.Context, id uuid.UUID) error {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return serrors.Wrap("NotificationService.MarkRead", err)
	}
	if err := s.repo.MarkRead(ctx, u.ID(), id); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
func (s *NotificationService) MarkAllRead(ctx context.Context) error {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return serrors.Wrap("NotificationService.MarkAllRead", err)
	}
	if err := s.repo.MarkAllRead(ctx, u.ID()); err != nil {
		return err
	}
	s.invalidate(ctx)
	return nil
}
