package services

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

type ruleStub struct {
	rule  notifications.Rule
	saved bool
}

func (r *ruleStub) Get(context.Context, string) (notifications.Rule, error) { return r.rule, nil }
func (r *ruleStub) Save(_ context.Context, rule notifications.Rule) error {
	r.rule = rule
	r.saved = true
	return nil
}

type recipientsStub struct{ users []user.User }

func (r recipientsStub) GetAll(context.Context) ([]user.User, error) { return r.users, nil }
func (r recipientsStub) GetByIDs(_ context.Context, ids []uint) ([]user.User, error) {
	out := []user.User{}
	for _, u := range r.users {
		for _, id := range ids {
			if id == u.ID() {
				out = append(out, u)
			}
		}
	}
	return out, nil
}

type deliveryStub struct {
	last  notification.Notification
	count int
	err   error
}

func (d *deliveryStub) Deliver(_ context.Context, n notification.Notification) (notification.Notification, error) {
	if d.err != nil {
		return nil, d.err
	}
	d.last = n
	d.count++
	return n, nil
}
func routingFixture(t *testing.T) (context.Context, *NotificationRoutingService, *ruleStub, *deliveryStub) {
	t.Helper()
	tenant := uuid.New()
	ctx := composables.WithTenantID(context.Background(), tenant)
	catalog := notifications.NewCatalog()
	require.NoError(t, catalog.Register(notifications.TestDefinition()))
	rules := &ruleStub{rule: notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{1}}}
	delivery := &deliveryStub{}
	email, err := internet.NewEmail("recipient@example.com")
	require.NoError(t, err)
	users := recipientsStub{users: []user.User{user.New("Recipient", "User", email, user.UILanguage("ru"), user.WithID(1), user.WithTenantID(tenant))}}
	return ctx, &NotificationRoutingService{catalog: catalog, rules: rules, users: users, delivery: delivery}, rules, delivery
}
func TestNotificationRoutingPublish(t *testing.T) {
	for _, scenario := range []string{"enabled", "disabled", "cross tenant", "empty identity", "blocked", "delivery failure"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, s, rules, delivery := routingFixture(t)
			tenant, err := composables.UseTenantID(ctx)
			require.NoError(t, err)
			event := notifications.Event{Key: notifications.TestEventKey, ID: "event-1", TenantID: tenant}
			switch scenario {
			case "disabled":
				rules.rule.Enabled = false
			case "cross tenant":
				event.TenantID = uuid.New()
			case "empty identity":
				event.ID = ""
			case "blocked":
				s.users = recipientsStub{users: []user.User{user.New("", "", nil, user.UILanguage("en"), user.WithID(1), user.WithTenantID(tenant), user.WithIsBlocked(true))}}
			case "delivery failure":
				delivery.err = errors.New("database unavailable")
			}
			count, err := s.Publish(ctx, event)
			if scenario == "cross tenant" || scenario == "empty identity" || scenario == "delivery failure" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			if scenario == "enabled" {
				require.Equal(t, 1, count)
				require.Equal(t, 1, delivery.count)
			} else {
				require.Zero(t, count)
				require.Zero(t, delivery.count)
			}
		})
	}
}
func TestNotificationRuleRejectsInvalidRecipients(t *testing.T) {
	for _, ids := range [][]uint{{}, {1, 1}, {2}, {0}} {
		t.Run("invalid", func(t *testing.T) {
			ctx, s, rules, _ := routingFixture(t)
			require.Error(t, s.SaveRule(ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: ids}))
			require.False(t, rules.saved)
		})
	}
	ctx, s, rules, _ := routingFixture(t)
	require.NoError(t, s.SaveRule(ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{1}}))
	require.True(t, rules.saved)
}

func TestNotificationRoutingSemanticRecipientsAndGuard(t *testing.T) {
	ctx, s, rules, delivery := routingFixture(t)
	tenant, err := composables.UseTenantID(ctx)
	require.NoError(t, err)
	d := notifications.TestDefinition()
	d.Key = "core.document.created.v1"
	d.DefaultLevel = notification.LevelSuccess
	calls := 0
	d.RecipientKeys = []notifications.RecipientDefinition{{Key: "creator", Name: map[string]string{"en": "Creator"}, Resolve: func(context.Context, notifications.Event) ([]uint, error) { calls++; return []uint{1, 1, 999}, nil }}}
	allowed := true
	d.RecipientGuard = func(context.Context, notifications.Event, user.User) (bool, error) { return allowed, nil }
	require.NoError(t, s.catalog.Register(d))
	rules.rule = notifications.Rule{EventKey: d.Key, Configured: true, Enabled: true, UserIDs: []uint{1}, RecipientKeys: []string{"creator"}, Level: notification.LevelWarning}
	require.NoError(t, s.SaveRule(ctx, rules.rule))
	event := notifications.Event{Key: d.Key, ID: "document-event", TenantID: tenant}
	rule, ids, err := s.ResolveRecipients(ctx, event)
	require.NoError(t, err)
	require.Equal(t, []uint{1, 999}, ids)
	require.Equal(t, 1, calls)
	count, err := s.DeliverRecipients(ctx, event, rule, ids)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, 1, delivery.count)
	allowed = false
	count, err = s.DeliverRecipients(ctx, event, rule, ids)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Equal(t, 1, calls)
	rule.RecipientKeys = []string{"unregistered"}
	require.Error(t, s.SaveRule(ctx, rule))
	rule.RecipientKeys = []string{"creator", "creator"}
	require.Error(t, s.SaveRule(ctx, rule))
	rule.RecipientKeys = []string{"creator"}
	rule.Level = "critical"
	require.Error(t, s.SaveRule(ctx, rule))
}
