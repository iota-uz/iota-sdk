package services_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/google/uuid"

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
	"github.com/iota-uz/iota-sdk/pkg/defaults"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

// outboxDispatcher is a dispatcher of one process: it delivers the committed
// outbox events to the notification bridge only.
func outboxDispatcher(t *testing.T, f *itf.TestEnvironment) *events.Dispatcher {
	t.Helper()
	router := itf.GetService[services.NotificationRoutingService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	d, err := events.NewDispatcher(events.Config{
		DB:        f.Pool,
		Consumers: events.StaticConsumers(services.NewNotificationOutboxBridge(router.Catalog(), dispatch)),
		Subjects: host.SubjectLoaderFunc(func(context.Context, uuid.UUID, int64, spi.SubjectKind) (spi.Subject, error) {
			return spi.Subject{}, events.ErrNoSubject
		}),
	})
	require.NoError(t, err)
	return d
}

func deliveryCtx(f *itf.TestEnvironment) context.Context {
	return itf.TestExecution(composables.WithPool(context.Background(), f.Pool))
}

// userCreatedRow reads the outbox envelope of the created user's event.
func userCreatedRow(t *testing.T, f *itf.TestEnvironment, userID uint) events.Envelope {
	t.Helper()
	ev := events.Envelope{Event: services.UserCreatedEventID, TenantID: f.TenantID(), Subject: &spi.ObjectRef{}}
	require.NoError(t, f.Pool.QueryRow(context.Background(), `
		SELECT id::text, seq, subject_entity, subject_key, payload, initiator_user_id, occurred_at
		FROM extruntime.outbox_events WHERE tenant_id = $1 AND event = $2 AND subject_key = $3`,
		f.TenantID(), services.UserCreatedEventID, fmt.Sprint(userID)).
		Scan(&ev.ID, &ev.Seq, &ev.Subject.Entity, &ev.Subject.Key, &ev.Payload, &ev.InitiatorUserID, &ev.OccurredAt))
	return ev
}

func setupUserCreatedNotification(t *testing.T) (*itf.TestEnvironment, context.Context) {
	t.Helper()
	f := setupNotificationTest(t, permissions.UserCreate, permissions.UserRead)
	for _, p := range []permission.Permission{permissions.UserCreate, permissions.UserRead} {
		require.NoError(t, persistence.NewPermissionRepository().Save(f.Ctx, p))
	}
	actor, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(f.Ctx, user.New("Business", "Admin", internet.MustParseEmail(uuid.NewString()+"@example.com"), user.UILanguageEN,
		user.WithTenantID(f.TenantID()), user.WithPermissions([]permission.Permission{permissions.UserCreate, permissions.UserRead})))
	require.NoError(t, err)
	f.User = actor
	// Delivery reads committed outbox rows, so the fixture is committed.
	require.NoError(t, f.Tx.Commit(f.Ctx))
	return f, userCommittedCtx(f)
}

// A phone-only account has no name and must still produce its notification.
// Falsely green if a name is supplied or the notification rule has no recipient.
func TestNotificationsMVP_UserCreatedWithoutName(t *testing.T) {
	f, ctx := setupUserCreatedNotification(t)
	router := itf.GetService[services.NotificationRoutingService](f)
	inbox := itf.GetService[services.NotificationService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	require.NoError(t, router.SaveRule(ctx, notifications.Rule{EventKey: core.UserCreatedNotificationEvent, Enabled: true, UserIDs: []uint{f.User.ID()}}))
	created, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(ctx, user.New("", "", internet.MustParseEmail("phone-only@generated.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	actorID := int64(f.User.ID())
	bridge := services.NewNotificationOutboxBridge(router.Catalog(), dispatch)
	require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
		tx, err := composables.UseTx(txCtx)
		require.NoError(t, err)
		return bridge.Deliver(txCtx, tx.(pgx.Tx), events.Envelope{
			ID: uuid.NewString(), TenantID: f.TenantID(), Event: services.UserCreatedEventID, InitiatorUserID: &actorID,
			Subject: &spi.ObjectRef{Entity: "core.user", Key: fmt.Sprint(created.ID())}, OccurredAt: time.Now(),
			Payload: json.RawMessage(fmt.Sprintf(`{"userId":"%d","email":"phone-only@generated.com","firstName":"","lastName":"","userType":"user"}`, created.ID())),
		})
	}))
	_, err = dispatch.Process(ctx)
	require.NoError(t, err)
	items, err := inbox.List(ctx, notification.FindParams{UnreadOnly: true})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, fmt.Sprintf("#%d", created.ID()), items[0].Body())
	require.Equal(t, fmt.Sprintf("/users/%d", created.ID()), items[0].ActionURL())
}

// The user-created notification is delivered from the outbox: it commits with
// the delivery of the committed event, a rolled back creation leaves neither
// a user nor a notification, a process that dies right after the commit loses
// nothing and a redelivery of the same outbox row queues no second notice.
func TestNotificationsMVP_UserCreationToInbox(t *testing.T) {
	f, ctx := setupUserCreatedNotification(t)
	router := itf.GetService[services.NotificationRoutingService](f)
	inbox := itf.GetService[services.NotificationService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	users := itf.GetService[services.UserService](f)
	rule := notifications.Rule{EventKey: core.UserCreatedNotificationEvent, Enabled: true, UserIDs: []uint{f.User.ID()}}
	require.NoError(t, router.SaveRule(ctx, rule))

	// The process dies right after the commit: nothing was delivered yet.
	created, err := users.Create(ctx, user.New("Business", "Trial", internet.MustParseEmail("trial@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	jobs, err := dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Empty(t, jobs, "the notification is not queued by the creating transaction")

	// The next process delivers the committed event.
	_, err = outboxDispatcher(t, f).RunOnce(deliveryCtx(f))
	require.NoError(t, err)
	_, err = dispatch.Process(ctx)
	require.NoError(t, err)
	items, err := inbox.List(ctx, notification.FindParams{UnreadOnly: true})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, core.UserCreatedNotificationEvent, items[0].EventKey())
	require.Equal(t, "Business Trial", items[0].Body())
	require.Equal(t, fmt.Sprintf("/users/%d", created.ID()), items[0].ActionURL())
	jobs, err = dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.NoError(t, inbox.MarkRead(ctx, items[0].ID()))
	count, err := inbox.UnreadCount(ctx)
	require.NoError(t, err)
	require.Zero(t, count)

	// A redelivery of the same outbox row queues no second notice.
	row := userCreatedRow(t, f, created.ID())
	bridge := services.NewNotificationOutboxBridge(router.Catalog(), dispatch)
	for range 2 {
		require.NoError(t, composables.InTx(ctx, func(txCtx context.Context) error {
			tx, err := composables.UseTx(txCtx)
			require.NoError(t, err)
			return bridge.Deliver(txCtx, tx.(pgx.Tx), row)
		}))
	}
	_, err = outboxDispatcher(t, f).RunOnce(deliveryCtx(f))
	require.NoError(t, err)
	_, err = dispatch.Process(ctx)
	require.NoError(t, err)
	jobs, err = dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Len(t, jobs, 1, "a redelivered outbox row is the same notification")
	items, err = inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1)

	// A rolled back creation leaves neither the user nor a notification.
	tx, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	txCtx, _ := composables.OwnTx(ctx, tx)
	_, err = users.Create(txCtx, user.New("Rolled", "Back", internet.MustParseEmail("rollback@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	_, err = outboxDispatcher(t, f).RunOnce(deliveryCtx(f))
	require.NoError(t, err)
	_, err = dispatch.Process(ctx)
	require.NoError(t, err)
	found, err := users.GetByEmail(ctx, "rollback@example.com")
	require.Error(t, err, "the rolled back user does not exist: %v", found)
	items, err = inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1, "rolling back user creation must roll back its notification")

	// A disabled rule delivers nothing.
	rule.Enabled = false
	require.NoError(t, router.SaveRule(ctx, rule))
	_, err = users.Create(ctx, user.New("Disabled", "Trial", internet.MustParseEmail("disabled@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	_, err = outboxDispatcher(t, f).RunOnce(deliveryCtx(f))
	require.NoError(t, err)
	_, err = dispatch.Process(ctx)
	require.NoError(t, err)
	items, err = inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestNotificationsMVP_GroupAndRoleAudiences(t *testing.T) {
	f := setupNotificationTest(t)
	ctx := f.Ctx
	users := persistence.NewUserRepository(persistence.NewUploadRepository())
	recipient, err := users.Create(ctx, user.New("Audience", "Recipient", internet.MustParseEmail("audience@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	groupID := uuid.New()
	var roleID uint
	_, err = f.Tx.Exec(ctx, "INSERT INTO user_groups(id,type,name,tenant_id) VALUES($1,'user',$2,$3)", groupID, groupID.String(), f.TenantID())
	require.NoError(t, err)
	err = f.Tx.QueryRow(ctx, "INSERT INTO roles(type,name,tenant_id) VALUES('user',$1,$2) RETURNING id", uuid.NewString(), f.TenantID()).Scan(&roleID)
	require.NoError(t, err)
	_, err = f.Tx.Exec(ctx, "INSERT INTO group_users(group_id,user_id) VALUES($1,$2)", groupID, recipient.ID())
	require.NoError(t, err)
	_, err = f.Tx.Exec(ctx, "INSERT INTO group_roles(group_id,role_id) VALUES($1,$2)", groupID, roleID)
	require.NoError(t, err)
	router := itf.GetService[services.NotificationRoutingService](f)
	rule := notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{recipient.ID()}, GroupIDs: []uuid.UUID{groupID}, RoleIDs: []uint{roleID}}
	require.NoError(t, router.SaveRule(ctx, rule))
	saved, err := router.Rule(ctx, rule.EventKey)
	require.NoError(t, err)
	require.True(t, saved.Configured)
	require.Equal(t, rule.UserIDs, saved.UserIDs)
	require.Equal(t, rule.GroupIDs, saved.GroupIDs)
	require.Equal(t, rule.RoleIDs, saved.RoleIDs)
	require.Equal(t, rule.Enabled, saved.Enabled)
	event := notifications.Event{Key: rule.EventKey, ID: uuid.NewString(), TenantID: f.TenantID()}
	count, err := router.Publish(ctx, event)
	require.NoError(t, err)
	require.Equal(t, 1, count, "overlapping users, groups and inherited roles deliver once")
	rule.UserIDs = nil
	rule.RoleIDs = nil
	require.NoError(t, router.SaveRule(ctx, rule))
	event.ID = uuid.NewString()
	count, err = router.Publish(ctx, event)
	require.NoError(t, err)
	require.Equal(t, 1, count, "group-only rules deliver")
	rule.RoleIDs = []uint{roleID}
	rule.GroupIDs = nil
	require.NoError(t, router.SaveRule(ctx, rule))
	event.ID = uuid.NewString()
	count, err = router.Publish(ctx, event)
	require.NoError(t, err)
	require.Equal(t, 1, count, "role inherited through group is effective")
	_, err = f.Tx.Exec(ctx, "DELETE FROM group_users WHERE group_id=$1 AND user_id=$2", groupID, recipient.ID())
	require.NoError(t, err)
	event.ID = uuid.NewString()
	count, err = router.Publish(ctx, event)
	require.NoError(t, err)
	require.Zero(t, count, "membership is resolved at delivery")
	_, err = f.Tx.Exec(ctx, "INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)", recipient.ID(), roleID)
	require.NoError(t, err)
	event.ID = uuid.NewString()
	count, err = router.Publish(ctx, event)
	require.NoError(t, err)
	require.Equal(t, 1, count, "direct roles are effective")
	otherCtx := composables.WithTenantID(ctx, uuid.New())
	audience := persistence.NewNotificationAudienceRepository()
	ids, err := audience.Resolve(otherCtx, []uuid.UUID{groupID}, []uint{roleID})
	require.NoError(t, err)
	require.Empty(t, ids)
	require.Error(t, router.SaveRule(otherCtx, rule), "foreign tenant roles cannot be saved")
	rule.RoleIDs = []uint{roleID, roleID}
	require.Error(t, router.SaveRule(ctx, rule))
	rule.RoleIDs = nil
	rule.GroupIDs = []uuid.UUID{uuid.New()}
	require.Error(t, router.SaveRule(ctx, rule))
}

func setupNotificationTest(t *testing.T, permissions ...permission.Permission) *itf.TestEnvironment {
	t.Helper()
	return setupNotificationChannelTest(t, nil, nil, permissions...)
}

func setupNotificationChannelTest(t *testing.T, adapters []notifications.ChannelAdapter, resolver notifications.AddressResolver, permissions ...permission.Permission) *itf.TestEnvironment {
	t.Helper()
	components := modules.Components()
	for i, component := range components {
		if component.Descriptor().Name == "core" {
			components[i] = core.NewComponent(&core.ModuleOptions{DisableNotificationWorker: true, PermissionSchema: defaults.PermissionSchema(), NotificationChannels: adapters, NotificationAddressResolver: resolver})
		}
	}
	return itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User(permissions...)))
}
