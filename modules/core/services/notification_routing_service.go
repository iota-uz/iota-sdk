package services

import (
	"context"
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
	audience notifications.AudienceRepository
}

func NewNotificationRoutingService(catalog *notifications.Catalog, rules notifications.RuleRepository, users user.Repository, delivery *NotificationService, audience notifications.AudienceRepository) *NotificationRoutingService {
	return &NotificationRoutingService{catalog: catalog, rules: rules, users: users, delivery: delivery, audience: audience}
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
		return serrors.New(serrors.Invalid, "unknown notification event").WithOp(op)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if len(rule.UserIDs)+len(rule.GroupIDs)+len(rule.RoleIDs) > 1000 {
		return serrors.New(serrors.Invalid, "too many recipients").WithOp(op)
	}
	seen := map[uint]bool{}
	for _, id := range rule.UserIDs {
		if id == 0 || seen[id] {
			return serrors.New(serrors.Invalid, "invalid or duplicate recipient").WithOp(op)
		}
		seen[id] = true
	}
	if rule.Enabled && len(rule.UserIDs)+len(rule.GroupIDs)+len(rule.RoleIDs) == 0 {
		return serrors.New(serrors.Invalid, "enabled event requires recipients").WithOp(op)
	}
	users, err := s.users.GetByIDs(ctx, rule.UserIDs)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if len(users) != len(rule.UserIDs) {
		return serrors.New(serrors.Invalid, "invalid recipient").WithOp(op)
	}
	for _, u := range users {
		if u.TenantID() != tenant || u.IsBlocked() || u.Type() != user.TypeUser || u.Status() != user.StatusActive {
			return serrors.New(serrors.Invalid, "invalid recipient").WithOp(op)
		}
	}
	if err := s.validateAudience(ctx, rule); err != nil {
		return serrors.Wrap(op, err)
	}
	return s.rules.Save(ctx, rule)
}
func (s *NotificationRoutingService) Publish(ctx context.Context, event notifications.Event) (int, error) {
	const op = "NotificationRoutingService.Publish"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	if event.TenantID == uuid.Nil || event.TenantID != tenant || strings.TrimSpace(event.ID) == "" {
		return 0, serrors.New(serrors.Invalid, "invalid notification event envelope").WithOp(op)
	}
	definition, ok := s.catalog.Get(event.Key)
	if !ok {
		return 0, serrors.New(serrors.Invalid, "unknown notification event").WithOp(op)
	}
	rule, err := s.rules.Get(ctx, event.Key)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	if !rule.Enabled {
		return 0, nil
	}
	ids := append([]uint{}, rule.UserIDs...)
	if len(rule.GroupIDs)+len(rule.RoleIDs) > 0 {
		resolved, err := s.audience.Resolve(ctx, rule.GroupIDs, rule.RoleIDs)
		if err != nil {
			return 0, serrors.Wrap(op, err)
		}
		ids = append(ids, resolved...)
	}
	unique := make([]uint, 0, len(ids))
	selected := map[uint]bool{}
	for _, id := range ids {
		if !selected[id] {
			selected[id] = true
			unique = append(unique, id)
		}
	}
	users, err := s.users.GetByIDs(ctx, unique)
	if err != nil {
		return 0, serrors.Wrap(op, err)
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
			return delivered, serrors.Wrap(op, err)
		}
		n, err := notification.New(u.ID(), content.Title, content.Body, notification.WithTenantID(tenant), notification.WithEventKey(event.Key), notification.WithActionURL(content.ActionURL), notification.WithDedupeKey(event.Key+":"+event.ID))
		if err != nil {
			return delivered, serrors.Wrap(op, err)
		}
		if _, err = s.delivery.Deliver(ctx, n); err != nil {
			return delivered, serrors.Wrap(op, err)
		}
		delivered++
	}
	return delivered, nil
}

func (s *NotificationRoutingService) Groups(ctx context.Context) ([]notifications.GroupOption, error) {
	return s.audience.Groups(ctx)
}
func (s *NotificationRoutingService) Roles(ctx context.Context) ([]notifications.RoleOption, error) {
	return s.audience.Roles(ctx)
}
func (s *NotificationRoutingService) validateAudience(ctx context.Context, rule notifications.Rule) error {
	invalid := func() error {
		return serrors.New(serrors.Invalid, "invalid or duplicate audience").WithOp("NotificationRoutingService.validateAudience")
	}
	if len(rule.GroupIDs) > 0 {
		groups, err := s.Groups(ctx)
		if err != nil {
			return err
		}
		available := map[uuid.UUID]bool{}
		for _, g := range groups {
			available[g.ID] = true
		}
		for _, id := range rule.GroupIDs {
			if id == uuid.Nil || !available[id] {
				return invalid()
			}
			delete(available, id)
		}
	}
	if len(rule.RoleIDs) > 0 {
		roles, err := s.Roles(ctx)
		if err != nil {
			return err
		}
		available := map[uint]bool{}
		for _, r := range roles {
			available[r.ID] = true
		}
		for _, id := range rule.RoleIDs {
			if id == 0 || !available[id] {
				return invalid()
			}
			delete(available, id)
		}
	}
	return nil
}
