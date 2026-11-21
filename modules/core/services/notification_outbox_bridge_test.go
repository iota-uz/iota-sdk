package services_test

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
)

func userCreatedPayload(id uint, first, last string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"userId":"%d","email":"u%d@example.com","firstName":%q,"lastName":%q,"userType":"user"}`, id, id, first, last))
}

func deliverEnvelope(t *testing.T, ctx context.Context, bridge events.Consumer, ev events.Envelope) error {
	t.Helper()
	return composables.InTx(ctx, func(txCtx context.Context) error {
		tx, err := composables.UseTx(txCtx)
		require.NoError(t, err)
		return bridge.Deliver(txCtx, tx.(pgx.Tx), ev)
	})
}

// An upgrade keeps every user-created notification exactly once. A job the
// removed eventbus handler queued (Data, no payload) still delivers through
// Process. The bridge enqueues under the handler's identity
// ("user-created:<user ID>"), so a retained outbox row the handler already
// notified is a no-op, a row it missed is notified, and a replica still
// running the handler during a rolling upgrade, finishing after the bridge,
// queues nothing new. A new Data-only submission is still refused.
func TestNotificationOutboxBridge_UpgradeFromTheEventbusHandler(t *testing.T) {
	f, ctx := setupUserCreatedNotification(t)
	router := itf.GetService[services.NotificationRoutingService](f)
	inbox := itf.GetService[services.NotificationService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	jobs, err := composition.Resolve[notifications.DispatchRepository](f.Container)
	require.NoError(t, err)
	rule := notifications.Rule{EventKey: core.UserCreatedNotificationEvent, Enabled: true, UserIDs: []uint{f.User.ID()}}
	require.NoError(t, router.SaveRule(ctx, rule))
	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	created := map[string]user.User{}
	for _, name := range []string{"Notified", "Missed", "Racing"} {
		u, err := users.Create(ctx, user.New("Legacy", name, internet.MustParseEmail("legacy-"+strings.ToLower(name)+"@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
		require.NoError(t, err)
		created[name] = u
		_, err = f.Pool.Exec(context.Background(), `
			INSERT INTO extruntime.outbox_events (id, tenant_id, event, subject_entity, subject_key, payload, initiator_user_id)
			VALUES (gen_random_uuid(), $1, $2, 'core.user', $3, $4, $5)`,
			f.TenantID(), services.UserCreatedEventID, fmt.Sprint(u.ID()), userCreatedPayload(u.ID(), u.FirstName(), u.LastName()), int64(f.User.ID()))
		require.NoError(t, err)
	}
	legacyEvent := func(u user.User) notifications.Event {
		return notifications.Event{
			Key: core.UserCreatedNotificationEvent, Version: 1, ID: fmt.Sprintf("%s%d", core.LegacyUserCreatedNotificationPrefix, u.ID()),
			TenantID: f.TenantID(), ActorUserID: f.User.ID(), OccurredAt: time.Now().UTC(),
			Subject: notifications.SubjectReference{Type: "user", ID: fmt.Sprint(u.ID())},
			Data:    map[string]string{"user_id": fmt.Sprint(u.ID()), "name": "Legacy " + u.LastName()},
		}
	}
	legacyEnqueue := func(u user.User) {
		t.Helper()
		_, err := jobs.Enqueue(ctx, notifications.DispatchRequest{Event: legacyEvent(u), Rule: rule, Mode: notifications.NoticeRule, RecipientIDs: []uint{f.User.ID()}})
		require.NoError(t, err)
	}

	// The handler queued the first creation before the upgrade.
	legacyEnqueue(created["Notified"])
	// A new Data-only submission is refused even under the legacy identity.
	_, err = dispatch.Enqueue(ctx, legacyEvent(created["Missed"]))
	require.ErrorContains(t, err, "needs the payload of its event")

	// The bridge delivers the retained rows; an old replica then finishes
	// its enqueue of the third creation.
	_, err = outboxDispatcher(t, f).RunOnce(deliveryCtx(f))
	require.NoError(t, err)
	legacyEnqueue(created["Racing"])
	for {
		more, err := dispatch.Process(ctx)
		require.NoError(t, err)
		if !more {
			break
		}
	}
	items, err := inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	bodies := []string{}
	for _, item := range items {
		bodies = append(bodies, item.Body())
	}
	require.ElementsMatch(t, []string{"Legacy Notified", "Legacy Missed", "Legacy Racing"}, bodies, "one notification per creation")
	summaries, err := dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 3)
	for _, s := range summaries {
		require.Empty(t, s.LastError)
		require.NotNil(t, s.CompletedAt)
	}
}

// The initiator of a committed event is historical attribution: an initiator
// deleted before the delivery is left out of the semantic recipients and the
// rule's other recipients are still notified.
func TestNotificationOutboxBridge_DeletedInitiatorDoesNotBlockOtherRecipients(t *testing.T) {
	f, ctx := setupUserCreatedNotification(t)
	router := itf.GetService[services.NotificationRoutingService](f)
	inbox := itf.GetService[services.NotificationService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	require.NoError(t, router.SaveRule(ctx, notifications.Rule{EventKey: core.UserCreatedNotificationEvent, Enabled: true,
		UserIDs: []uint{f.User.ID()}, RecipientKeys: []string{"actor"}}))
	created, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(ctx, user.New("After", "Actor", internet.MustParseEmail("after-actor@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	gone := int64(987654321) // no such user: the initiator was deleted
	bridge := services.NewNotificationOutboxBridge(router.Catalog(), dispatch)
	require.NoError(t, deliverEnvelope(t, ctx, bridge, events.Envelope{
		ID: uuid.NewString(), TenantID: f.TenantID(), Event: services.UserCreatedEventID, InitiatorUserID: &gone,
		Subject: &spi.ObjectRef{Entity: "core.user", Key: fmt.Sprint(created.ID())}, OccurredAt: time.Now(),
		Payload: userCreatedPayload(created.ID(), "After", "Actor"),
	}))
	_, err = dispatch.Process(ctx)
	require.NoError(t, err)
	items, err := inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "After Actor", items[0].Body())
}

// A composition of the SDK modules without pkg/extruntime/wiring (no
// extension runtime) delivers user-created notifications: the core module
// runs its own Go-only outbox dispatcher.
func TestNotificationOutboxBridge_CoreCompositionWithoutWiringDelivers(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:       "core_outbox_without_wiring",
		Components: modules.Components(),
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation:  itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	scope := h.Scope(t)
	ctx := itf.TestExecution(composables.WithSession(composables.WithParams(scope.Ctx, itf.DefaultParams()), itf.MockSession()))
	require.Equal(t, 1, events.Running(), "core starts its own dispatcher")
	_, err := composition.Resolve[*events.Dispatcher](scope.Container)
	require.True(t, composition.IsNotProvided(err), "the fallback dispatcher is private to core")

	perms := []permission.Permission{permissions.UserCreate, permissions.UserRead}
	var admin user.User
	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		for _, p := range perms {
			if err := persistence.NewPermissionRepository().Save(txCtx, p); err != nil {
				return err
			}
		}
		var err error
		admin, err = persistence.NewUserRepository(persistence.NewUploadRepository()).Create(txCtx, user.New("Core", "Admin",
			internet.MustParseEmail("core-admin@example.com"), user.UILanguageEN, user.WithTenantID(scope.Tenant.ID), user.WithPermissions(perms)))
		return err
	}))
	ctx = composables.WithUser(ctx, admin)
	router, err := composition.Resolve[*services.NotificationRoutingService](scope.Container)
	require.NoError(t, err)
	require.NoError(t, router.SaveRule(ctx, notifications.Rule{EventKey: core.UserCreatedNotificationEvent, Enabled: true, UserIDs: []uint{admin.ID()}}))
	users, err := composition.Resolve[*services.UserService](scope.Container)
	require.NoError(t, err)
	_, err = users.Create(ctx, user.New("Core", "Only", internet.MustParseEmail("core-only@example.com"), user.UILanguageEN, user.WithTenantID(scope.Tenant.ID)))
	require.NoError(t, err)

	inbox, err := composition.Resolve[*services.NotificationService](scope.Container)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		items, err := inbox.List(ctx, notification.FindParams{})
		return err == nil && len(items) == 1 && items[0].Body() == "Core Only"
	}, 30*time.Second, 100*time.Millisecond, "the core dispatcher and notification worker deliver the notification")
}

// outboxOwner is a component that serves the outbox itself, as the extension
// runtime composed directly does.
type outboxOwner struct{}

func (outboxOwner) Descriptor() composition.Descriptor {
	return composition.Descriptor{Name: "test.outbox-owner"}
}
func (outboxOwner) LocaleFS() []*embed.FS { return nil }
func (outboxOwner) Build(b *composition.Builder) error {
	composition.Contribute(b, func(*composition.Container) ([]events.DispatcherOwner, error) {
		return []events.DispatcherOwner{{Name: "test"}}, nil
	})
	return nil
}

// Core leaves the outbox to a component that declares it serves it (the
// extension runtime composed without wiring keeps its own dispatcher and
// settings, DisableBackground included).
func TestNotificationOutboxBridge_CoreDefersToADeclaredDispatcherOwner(t *testing.T) {
	h := itf.NewHarness(t, itf.HarnessConfig{
		Name:       "core_outbox_deferred",
		Components: append(modules.Components(), outboxOwner{}),
		Database:   itf.DatabaseConfig{Provisioning: itf.ProvisioningPerTestDatabase},
		Isolation:  itf.IsolationConfig{Mode: itf.IsolationCommitted},
	})
	h.Scope(t)
	require.Zero(t, events.Running(), "core starts no dispatcher next to the owner's")
}
