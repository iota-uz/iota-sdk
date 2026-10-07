package services

import (
	"context"
	"crypto/sha256"
	"fmt"

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

func NewNotificationRoutingService(catalog *notifications.Catalog, rules notifications.RuleRepository, users user.Repository, delivery NotificationDelivery, audience notifications.AudienceRepository) *NotificationRoutingService {
	return &NotificationRoutingService{catalog: catalog, rules: rules, users: users, delivery: delivery, audience: audience}
}
func (s *NotificationRoutingService) Catalog() *notifications.Catalog { return s.catalog }
func (s *NotificationRoutingService) Rule(ctx context.Context, key string) (notifications.Rule, error) {
	rule, err := s.rules.Get(ctx, key)
	if err != nil {
		return rule, err
	}
	if definition, ok := s.catalog.Get(key); ok && !rule.Configured && !rule.Enabled && len(definition.DefaultRecipientKeys) > 0 {
		rule.Enabled = true
		rule.RecipientKeys = append([]string{}, definition.DefaultRecipientKeys...)
	}
	return rule, nil
}
func (s *NotificationRoutingService) Recipients(ctx context.Context) ([]user.User, error) {
	return s.users.GetAll(ctx)
}
func (s *NotificationRoutingService) SaveRule(ctx context.Context, rule notifications.Rule) error {
	const op = "NotificationRoutingService.SaveRule"
	definition, ok := s.catalog.Get(rule.EventKey)
	if !ok {
		return serrors.New(serrors.Invalid, "unknown notification event").WithOp(op)
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if len(rule.UserIDs)+len(rule.GroupIDs)+len(rule.RoleIDs)+len(rule.RecipientKeys) > 1000 {
		return serrors.New(serrors.Invalid, "too many recipients").WithOp(op)
	}
	seen := map[uint]bool{}
	for _, id := range rule.UserIDs {
		if id == 0 || seen[id] {
			return serrors.New(serrors.Invalid, "invalid or duplicate recipient").WithOp(op)
		}
		seen[id] = true
	}
	if rule.Enabled && len(rule.UserIDs)+len(rule.GroupIDs)+len(rule.RoleIDs)+len(rule.RecipientKeys) == 0 {
		return serrors.New(serrors.Invalid, "enabled event requires recipients").WithOp(op)
	}
	if rule.Level != "" && !rule.Level.Valid() {
		return serrors.New(serrors.Invalid, "invalid notification level").WithOp(op)
	}
	keys := map[string]bool{}
	for _, key := range definition.RecipientKeys {
		keys[key.Key] = true
	}
	for _, key := range rule.RecipientKeys {
		if !keys[key] {
			return serrors.New(serrors.Invalid, "invalid or duplicate event recipient").WithOp(op)
		}
		delete(keys, key)
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
	event, err := s.catalog.Normalize(event)
	if err != nil {
		return 0, serrors.New(serrors.Invalid, err.Error()).WithOp("NotificationRoutingService.Publish")
	}
	rule, ids, err := s.ResolveRecipients(ctx, event)
	if err != nil {
		return 0, err
	}
	return s.DeliverRecipients(ctx, event, rule, ids)
}
func (s *NotificationRoutingService) ResolveRecipients(ctx context.Context, event notifications.Event) (notifications.Rule, []uint, error) {
	const op = "NotificationRoutingService.ResolveRecipients"
	rule := notifications.Rule{}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return rule, nil, serrors.Wrap(op, err)
	}
	event, err = s.catalog.Normalize(event)
	if err != nil || tenant != event.TenantID {
		return rule, nil, serrors.New(serrors.Invalid, "invalid notification event envelope").WithOp(op)
	}
	definition, _ := s.catalog.Get(event.Key)
	if event.ActorUserID != 0 {
		actors, err := s.users.GetByIDs(ctx, []uint{event.ActorUserID})
		if err != nil {
			return rule, nil, serrors.Wrap(op, err)
		}
		if len(actors) != 1 || actors[0].TenantID() != tenant {
			return rule, nil, serrors.New(serrors.Invalid, "invalid notification actor").WithOp(op)
		}
	}
	rule, err = s.rules.Get(ctx, event.Key)
	if err != nil {
		return rule, nil, serrors.Wrap(op, err)
	}
	if !rule.Configured && !rule.Enabled && len(definition.DefaultRecipientKeys) > 0 {
		rule.Enabled = true
		rule.RecipientKeys = append([]string{}, definition.DefaultRecipientKeys...)
	}
	if !rule.Enabled {
		return rule, nil, nil
	}
	ids := append([]uint{}, rule.UserIDs...)
	if len(rule.GroupIDs)+len(rule.RoleIDs) > 0 {
		resolved, err := s.audience.Resolve(ctx, rule.GroupIDs, rule.RoleIDs)
		if err != nil {
			return rule, nil, serrors.Wrap(op, err)
		}
		ids = append(ids, resolved...)
	}
	for _, key := range rule.RecipientKeys {
		found := false
		for _, recipient := range definition.RecipientKeys {
			if recipient.Key == key {
				found = true
				resolved, err := recipient.Resolve(ctx, event)
				if err != nil {
					return rule, nil, serrors.Wrap(op, err)
				}
				ids = append(ids, resolved...)
				break
			}
		}
		if !found {
			return rule, nil, serrors.New(serrors.Invalid, "unknown event recipient").WithOp(op)
		}
	}
	unique := make([]uint, 0, len(ids))
	seen := map[uint]bool{}
	for _, id := range ids {
		if id != 0 && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return rule, unique, nil
}
func (s *NotificationRoutingService) DeliverRecipients(ctx context.Context, event notifications.Event, rule notifications.Rule, ids []uint) (int, error) {
	const op = "NotificationRoutingService.DeliverRecipients"
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	event, err = s.catalog.Normalize(event)
	if err != nil || tenant != event.TenantID {
		return 0, serrors.New(serrors.Invalid, "invalid notification event envelope").WithOp(op)
	}
	if !rule.Enabled {
		return 0, nil
	}
	definition, _ := s.catalog.Get(event.Key)
	users, err := s.users.GetByIDs(ctx, ids)
	if err != nil {
		return 0, serrors.Wrap(op, err)
	}
	level := rule.Level
	if level == "" {
		level = definition.DefaultLevel
	}
	if level == "" {
		level = notification.LevelInfo
	}
	dedupe := event.DedupeKey
	if dedupe == "" {
		dedupe = event.ID
	}
	dedupe = event.Key + ":" + dedupe
	if len(dedupe) > 300 {
		dedupe = fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(dedupe)))
	}
	delivered := 0
	seen := map[uint]bool{}
	for _, u := range users {
		if seen[u.ID()] || u.TenantID() != tenant || u.IsBlocked() || u.Type() != user.TypeUser || u.Status() != user.StatusActive || (definition.RequiredPermission != nil && !u.Can(definition.RequiredPermission)) {
			continue
		}
		seen[u.ID()] = true
		if definition.RecipientGuard != nil {
			allowed, err := definition.RecipientGuard(ctx, event, u)
			if err != nil {
				return delivered, serrors.Wrap(op, err)
			}
			if !allowed {
				continue
			}
		}
		content, err := definition.Render(event, string(u.UILanguage()))
		if err != nil {
			return delivered, serrors.Wrap(op, err)
		}
		if definition.ActionURL != nil {
			content.ActionURL, err = definition.ActionURL(event)
			if err != nil {
				return delivered, serrors.Wrap(op, err)
			}
		}
		n, err := notification.New(u.ID(), content.Title, content.Body, notification.WithTenantID(tenant), notification.WithEventKey(event.Key), notification.WithActionURL(content.ActionURL), notification.WithLevel(level), notification.WithDedupeKey(dedupe))
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
