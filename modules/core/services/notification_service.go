package services

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationService struct{ repo notification.Repository }

func NewNotificationService(repo notification.Repository) *NotificationService {
	return &NotificationService{repo: repo}
}

// Deliver is a trusted producer API. Recipient ownership is enforced by the repository.
func (s *NotificationService) Deliver(ctx context.Context, n notification.Notification) (notification.Notification, error) {
	const op serrors.Op = "NotificationService.Deliver"
	if err := notification.Validate(n); err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}
	return s.repo.Create(ctx, n)
}
func (s *NotificationService) List(ctx context.Context, p notification.FindParams) ([]notification.Notification, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E("NotificationService.List", err)
	}
	return s.repo.List(ctx, u.ID(), p.Bounded())
}
func (s *NotificationService) UnreadCount(ctx context.Context) (int64, error) {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return 0, serrors.E("NotificationService.UnreadCount", err)
	}
	return s.repo.UnreadCount(ctx, u.ID())
}
func (s *NotificationService) MarkRead(ctx context.Context, id uuid.UUID) error {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return serrors.E("NotificationService.MarkRead", err)
	}
	return s.repo.MarkRead(ctx, u.ID(), id)
}
func (s *NotificationService) MarkAllRead(ctx context.Context) error {
	u, err := composables.UseUser(ctx)
	if err != nil {
		return serrors.E("NotificationService.MarkAllRead", err)
	}
	return s.repo.MarkAllRead(ctx, u.ID())
}
