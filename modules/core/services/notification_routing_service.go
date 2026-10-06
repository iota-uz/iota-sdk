package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type NotificationDelivery interface {
	Deliver(context.Context, notification.Notification) (notification.Notification, error)
}
type NotificationRecipients interface {
	GetAll(context.Context) ([]user.User, error)
	GetByIDs(context.Context, []uint) ([]user.User, error)
}
type NotificationRoutingService struct {
	catalog  *notifications.Catalog
	rules    notifications.RuleRepository
	users    NotificationRecipients
	delivery NotificationDelivery
}

func NewNotificationRoutingService(catalog *notifications.Catalog, rules notifications.RuleRepository, users user.Repository, delivery *NotificationService) *NotificationRoutingService {
	return &NotificationRoutingService{catalog: catalog, rules: rules, users: users, delivery: delivery}
}
func (s *NotificationRoutingService) Catalog() *notifications.Catalog { return s.catalog }
func (s *NotificationRoutingService) Rule(ctx context.Context, key string) (notifications.Rule, error) {
	return s.rules.Get(ctx, key)
}
func (s *NotificationRoutingService) Recipients(ctx context.Context) ([]user.User, error) {
	return s.users.GetAll(ctx)
}
func (s *NotificationRoutingService) SaveRule(ctx context.Context, rule notifications.Rule) error {
	const op = "NotificationRoutingService.SaveRule"
	if _, ok := s.catalog.Get(rule.EventKey); !ok {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("unknown notification event"))
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.E(op, err)
	}
	if len(rule.UserIDs) > 1000 {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("too many recipients"))
	}
	seen := map[uint]bool{}
	for _, id := range rule.UserIDs {
		if id == 0 || seen[id] {
			return serrors.E(op, serrors.KindValidation, fmt.Errorf("invalid or duplicate recipient"))
		}
		seen[id] = true
	}
	if rule.Enabled && len(rule.UserIDs) == 0 {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("enabled event requires recipients"))
	}
	users, err := s.users.GetByIDs(ctx, rule.UserIDs)
	if err != nil {
		return serrors.E(op, err)
	}
	if len(users) != len(rule.UserIDs) {
		return serrors.E(op, serrors.KindValidation, fmt.Errorf("invalid recipient"))
	}
	for _, u := range users {
		if u.TenantID() != tenant || u.IsBlocked() || u.Type() != user.TypeUser || u.Status() != user.StatusActive {
			return serrors.E(op, serrors.KindValidation, fmt.Errorf("invalid recipient"))
		}
	}
	return s.rules.Save(ctx, rule)
}
func (s *NotificationRoutingService) Publish(ctx context.Context, event notifications.Event) (int, error) {
	const op = "NotificationRoutingService.Publish"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	if event.TenantID == uuid.Nil || event.TenantID != tenant || strings.TrimSpace(event.ID) == "" {
		return 0, serrors.E(op, serrors.KindValidation, fmt.Errorf("invalid notification event envelope"))
	}
	definition, ok := s.catalog.Get(event.Key)
	if !ok {
		return 0, serrors.E(op, serrors.KindValidation, fmt.Errorf("unknown notification event"))
	}
	rule, err := s.rules.Get(ctx, event.Key)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	if !rule.Enabled || len(rule.UserIDs) == 0 {
		return 0, nil
	}
	users, err := s.users.GetByIDs(ctx, rule.UserIDs)
	if err != nil {
		return 0, serrors.E(op, err)
	}
	delivered := 0
	seen := map[uint]bool{}
	for _, u := range users {
		if seen[u.ID()] || u.TenantID() != tenant || u.IsBlocked() || u.Type() != user.TypeUser || u.Status() != user.StatusActive || (definition.RequiredPermission != nil && !u.Can(definition.RequiredPermission)) {
			continue
		}
		seen[u.ID()] = true
		content, err := definition.Render(event, string(u.UILanguage()))
		if err != nil {
			return delivered, serrors.E(op, err)
		}
		n, err := notification.New(u.ID(), content.Title, content.Body, notification.WithTenantID(tenant), notification.WithEventKey(event.Key), notification.WithActionURL(content.ActionURL), notification.WithDedupeKey(event.Key+":"+event.ID))
		if err != nil {
			return delivered, serrors.E(op, err)
		}
		if _, err = s.delivery.Deliver(ctx, n); err != nil {
			return delivered, serrors.E(op, err)
		}
		delivered++
	}
	return delivered, nil
}
