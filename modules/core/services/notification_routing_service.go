package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"slices"

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
	GetByIDs(context.Context, []uint) ([]user.User, error)
}

// RecipientPage is one page of the recipient picker.
type RecipientPage struct {
	Items   []notifications.Recipient
	HasMore bool
}

const recipientPageSize = 20

type NotificationRoutingService struct {
	catalog   *notifications.Catalog
	rules     notifications.RuleRepository
	users     NotificationRecipients
	delivery  NotificationDelivery
	audience  notifications.AudienceRepository
	directory notifications.RecipientDirectory
	channels  *NotificationChannelService
}

// WithChannels enables external channels; without it every rule is in-app only.
func (s *NotificationRoutingService) WithChannels(channels *NotificationChannelService) *NotificationRoutingService {
	s.channels = channels
	return s
}

// UserChannels returns the channels each user opted out of.
func (s *NotificationRoutingService) UserChannels(ctx context.Context, ids []uint) (map[uint][]notifications.Channel, error) {
	if s.channels == nil {
		return map[uint][]notifications.Channel{}, nil
	}
	return s.channels.Disabled(ctx, ids)
}

// SetUserChannels stores which channels a user wants. The user must be someone
// who can receive notifications in the current tenant.
func (s *NotificationRoutingService) SetUserChannels(ctx context.Context, userID uint, enabled []notifications.Channel) error {
	const op = "NotificationRoutingService.SetUserChannels"
	available, err := s.directory.GetByIDs(ctx, []uint{userID})
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if len(available) != 1 || s.channels == nil {
		return serrors.New(serrors.Invalid, "invalid recipient").WithOp(op)
	}
	return s.channels.SetEnabled(ctx, userID, enabled)
}

// Channels lists the channels a rule can select.
func (s *NotificationRoutingService) Channels() []notifications.Channel {
	if s.channels == nil {
		return []notifications.Channel{notifications.ChannelInApp}
	}
	return s.channels.Available()
}

func NewNotificationRoutingService(catalog *notifications.Catalog, rules notifications.RuleRepository, users user.Repository, delivery NotificationDelivery, audience notifications.AudienceRepository, directory notifications.RecipientDirectory) *NotificationRoutingService {
	return &NotificationRoutingService{catalog: catalog, rules: rules, users: users, delivery: delivery, audience: audience, directory: directory}
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

// SearchRecipients returns one page of the people a rule can name individually.
func (s *NotificationRoutingService) SearchRecipients(ctx context.Context, search string, offset int) (RecipientPage, error) {
	items, err := s.directory.Search(ctx, notifications.RecipientQuery{Search: search, Limit: recipientPageSize + 1, Offset: offset})
	if err != nil {
		return RecipientPage{}, err
	}
	if len(items) > recipientPageSize {
		return RecipientPage{Items: items[:recipientPageSize], HasMore: true}, nil
	}
	return RecipientPage{Items: items}, nil
}

// RecipientsByIDs returns the available people among ids.
func (s *NotificationRoutingService) RecipientsByIDs(ctx context.Context, ids []uint) ([]notifications.Recipient, error) {
	return s.directory.GetByIDs(ctx, ids)
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
	if err := s.validateChannels(rule.Channels); err != nil {
		return err
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
	available, err := s.directory.GetByIDs(ctx, rule.UserIDs)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if len(available) != len(rule.UserIDs) {
		return serrors.New(serrors.Invalid, "invalid recipient").WithOp(op)
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
	if err := s.validateActor(ctx, event); err != nil {
		return rule, nil, err
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
	named, err := s.directory.GetByIDs(ctx, rule.UserIDs)
	if err != nil {
		return rule, nil, serrors.Wrap(op, err)
	}
	ids := make([]uint, 0, len(named))
	for _, recipient := range named {
		ids = append(ids, recipient.ID)
	}
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
	channels := rule.EffectiveChannels()
	var disabled map[uint][]notifications.Channel
	if s.channels != nil {
		disabled, err = s.channels.Disabled(ctx, ids)
		if err != nil {
			return 0, serrors.Wrap(op, err)
		}
	}
	// A missing adapter must not block the other channels: its deliveries are
	// recorded and retried by the channel service.
	for _, c := range channels {
		if !c.Valid() {
			return 0, serrors.New(serrors.Invalid, "invalid notification channel").WithOp(op)
		}
	}
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
		userChannels := channels
		if len(disabled[u.ID()]) > 0 {
			userChannels = slices.DeleteFunc(slices.Clone(channels), func(c notifications.Channel) bool { return slices.Contains(disabled[u.ID()], c) })
			if len(userChannels) == 0 {
				continue
			}
		}
		language := string(u.UILanguage())
		if language == "" {
			language = "en"
		}
		content, err := definition.Render(event, language)
		if err != nil {
			return delivered, serrors.Wrap(op, err)
		}
		if definition.ActionURL != nil {
			content.ActionURL, err = definition.ActionURL(event)
			if err != nil {
				return delivered, serrors.Wrap(op, err)
			}
		}
		// Every channel, not only in-app, must carry a safe internal link.
		if _, err := notification.New(u.ID(), "validation", "", notification.WithActionURL(content.ActionURL)); err != nil {
			return delivered, serrors.Wrap(op, err)
		}
		identity := event.DedupeKey
		if identity == "" {
			identity = event.ID
		}
		var userDeliveries []notifications.Delivery
		for _, channel := range userChannels {
			d := notifications.Delivery{TenantID: tenant, EventKey: event.Key, EventID: identity, ContentVersion: event.Version, UserID: u.ID(), Channel: channel, Language: language, Level: level, Title: content.Title, Body: content.Body, ActionURL: content.ActionURL}
			if channel != notifications.ChannelInApp {
				userDeliveries = append(userDeliveries, d)
				continue
			}
			n, err := notification.New(u.ID(), content.Title, content.Body, notification.WithTenantID(tenant), notification.WithEventKey(event.Key), notification.WithActionURL(content.ActionURL), notification.WithLevel(level), notification.WithDedupeKey(dedupe))
			if err != nil {
				return delivered, serrors.Wrap(op, err)
			}
			if _, err = s.delivery.Deliver(ctx, n); err != nil {
				return delivered, serrors.Wrap(op, err)
			}
			d.Status = notifications.DeliverySent
			userDeliveries = append(userDeliveries, d)
		}
		// Recorded with the recipient's inbox write so a later failure cannot
		// leave a saved message without its delivery rows.
		if len(userDeliveries) > 0 && s.channels != nil {
			if err := s.channels.Record(ctx, userDeliveries); err != nil {
				return delivered, serrors.Wrap(op, err)
			}
		}
		delivered++
	}
	return delivered, nil
}

func (s *NotificationRoutingService) validateActor(ctx context.Context, event notifications.Event) error {
	const op = "NotificationRoutingService.validateActor"
	if event.ActorUserID == 0 {
		return nil
	}
	tenant, err := composables.UseTenantID(ctx)
	if err != nil {
		return serrors.Wrap(op, err)
	}
	actors, err := s.users.GetByIDs(ctx, []uint{event.ActorUserID})
	if err != nil {
		return serrors.Wrap(op, err)
	}
	if len(actors) != 1 || actors[0].TenantID() != tenant {
		return serrors.New(serrors.Invalid, "invalid notification actor").WithOp(op)
	}
	return nil
}

func (s *NotificationRoutingService) validateChannels(channels []notifications.Channel) error {
	if s.channels == nil {
		for _, c := range channels {
			if c != notifications.ChannelInApp {
				return serrors.New(serrors.Invalid, "invalid or unavailable notification channel").WithOp("NotificationRoutingService.validateChannels")
			}
		}
		return nil
	}
	return s.channels.Validate(channels)
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
