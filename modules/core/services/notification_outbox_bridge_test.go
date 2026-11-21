package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

// migrationUp is the Up section of the upgrade migration of #1325.
func migrationUp(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("migrations/changes-1794800000.sql")
	require.NoError(t, err)
	up, _, ok := strings.Cut(string(raw), "-- +migrate Down")
	require.True(t, ok)
	return up
}

// An upgrade keeps every user-created notification exactly once: a job the
// removed eventbus handler queued (Data, no payload) still delivers through
// Process; the migration marks its retained outbox row delivered to the
// bridge, so it is not notified again; a retained row the handler never
// enqueued gets no receipt and the bridge notifies it.
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
	notified, err := users.Create(ctx, user.New("Legacy", "Notified", internet.MustParseEmail("legacy-notified@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	missed, err := users.Create(ctx, user.New("Legacy", "Missed", internet.MustParseEmail("legacy-missed@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)

	// Both creations are retained in the outbox; the handler queued only the first.
	for _, u := range []user.User{notified, missed} {
		_, err := f.Pool.Exec(context.Background(), `
			INSERT INTO extruntime.outbox_events (id, tenant_id, event, subject_entity, subject_key, payload, initiator_user_id)
			VALUES (gen_random_uuid(), $1, $2, 'core.user', $3, $4, $5)`,
			f.TenantID(), services.UserCreatedEventID, fmt.Sprint(u.ID()), userCreatedPayload(u.ID(), u.FirstName(), u.LastName()), int64(f.User.ID()))
		require.NoError(t, err)
	}
	legacy := notifications.Event{
		Key: core.UserCreatedNotificationEvent, Version: 1, ID: fmt.Sprintf("%s%d", core.LegacyUserCreatedNotificationPrefix, notified.ID()),
		TenantID: f.TenantID(), ActorUserID: f.User.ID(), OccurredAt: time.Now().UTC(),
		Subject: notifications.SubjectReference{Type: "user", ID: fmt.Sprint(notified.ID())},
		Data:    map[string]string{"user_id": fmt.Sprint(notified.ID()), "name": "Legacy Notified"},
	}
	_, err = jobs.Enqueue(ctx, notifications.DispatchRequest{Event: legacy, Rule: rule, Mode: notifications.NoticeRule, RecipientIDs: []uint{f.User.ID()}})
	require.NoError(t, err)

	_, err = f.Pool.Exec(context.Background(), migrationUp(t))
	require.NoError(t, err)
	receipts := map[string]bool{}
	for _, u := range []user.User{notified, missed} {
		var n int
		require.NoError(t, f.Pool.QueryRow(context.Background(), `
			SELECT count(*) FROM extruntime.subscription_receipts r JOIN extruntime.outbox_events o ON o.id = r.event_id
			WHERE r.subscription = 'notifications' AND o.tenant_id = $1 AND o.subject_key = $2`, f.TenantID(), fmt.Sprint(u.ID())).Scan(&n))
		receipts[u.LastName()] = n == 1
	}
	require.Equal(t, map[string]bool{"Notified": true, "Missed": false}, receipts)

	// The pending legacy job delivers; the bridge delivers only the missed row.
	_, err = outboxDispatcher(t, f).RunOnce(deliveryCtx(f))
	require.NoError(t, err)
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
	require.ElementsMatch(t, []string{"Legacy Notified", "Legacy Missed"}, bodies)
	summaries, err := dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Len(t, summaries, 2)
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
	_, err := composition.Resolve[*events.Dispatcher](scope.Container)
	require.NoError(t, err)
	require.Equal(t, 1, events.Running(), "core starts its own dispatcher")

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
