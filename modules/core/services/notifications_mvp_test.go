package services_test

import (
	"fmt"
	"testing"

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
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestNotificationsMVP_UserCreationToInbox(t *testing.T) {
	f := setupNotificationTest(t, permissions.UserCreate, permissions.UserRead)
	for _, p := range []permission.Permission{permissions.UserCreate, permissions.UserRead} {
		require.NoError(t, persistence.NewPermissionRepository().Save(f.Ctx, p))
	}
	actor, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(f.Ctx, user.New("Business", "Admin", internet.MustParseEmail("admin@example.com"), user.UILanguageEN,
		user.WithTenantID(f.TenantID()), user.WithPermissions([]permission.Permission{permissions.UserCreate, permissions.UserRead})))
	require.NoError(t, err)
	f.User = actor
	// The production event handler deliberately uses a fresh context after commit.
	require.NoError(t, f.Tx.Commit(f.Ctx))
	ctx := userCommittedCtx(f)
	router := itf.GetService[services.NotificationRoutingService](f)
	inbox := itf.GetService[services.NotificationService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	users := itf.GetService[services.UserService](f)
	rule := notifications.Rule{EventKey: core.UserCreatedNotificationEvent, Enabled: true, UserIDs: []uint{f.User.ID()}}
	require.NoError(t, router.SaveRule(ctx, rule))
	created, err := users.Create(ctx, user.New("Business", "Trial", internet.MustParseEmail("trial@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	_, err = dispatch.Process(ctx)
	require.NoError(t, err)
	items, err := inbox.List(ctx, notification.FindParams{UnreadOnly: true})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, core.UserCreatedNotificationEvent, items[0].EventKey())
	require.Equal(t, "Business Trial", items[0].Body())
	require.Equal(t, fmt.Sprintf("/users/%d", created.ID()), items[0].ActionURL())
	require.NoError(t, inbox.MarkRead(ctx, items[0].ID()))
	count, err := inbox.UnreadCount(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	tx, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	txCtx := composables.WithTx(ctx, tx)
	_, err = users.Create(txCtx, user.New("Rolled", "Back", internet.MustParseEmail("rollback@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	inside, err := inbox.List(txCtx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, inside, 1)
	jobs, err := dispatch.Jobs(txCtx)
	require.NoError(t, err)
	require.Len(t, jobs, 2, "caller transaction contains the notification intent")
	require.NoError(t, tx.Rollback(ctx))
	items, err = inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1, "rolling back user creation must roll back its notification")
	failureTx, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = failureTx.Rollback(ctx) }()
	failureCtx := composables.WithTx(ctx, failureTx)
	_, err = failureTx.Exec(ctx, fmt.Sprintf(`ALTER TABLE core.notification_dispatches ADD CONSTRAINT notification_failure_test CHECK (tenant_id <> '%s'::uuid) NOT VALID`, f.TenantID()))
	require.NoError(t, err)
	createdWithoutNotification, err := users.Create(failureCtx, user.New("Notification", "Failure", internet.MustParseEmail("notification-failure@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	_, err = failureTx.Exec(ctx, `ALTER TABLE core.notification_dispatches DROP CONSTRAINT notification_failure_test`)
	require.NoError(t, err, "notification SQL errors must not abort the caller transaction")
	require.NoError(t, failureTx.Commit(ctx))
	_, err = persistence.NewUserRepository(persistence.NewUploadRepository()).GetByID(ctx, createdWithoutNotification.ID())
	require.NoError(t, err, "user creation must remain committed after a notification failure")
	items, err = inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1, "failed notification writes must roll back to the savepoint")
	rule.Enabled = false
	require.NoError(t, router.SaveRule(ctx, rule))
	_, err = users.Create(ctx, user.New("Disabled", "Trial", internet.MustParseEmail("disabled@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
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
	components := modules.Components()
	for i, component := range components {
		if component.Descriptor().Name == "core" {
			components[i] = core.NewComponent(&core.ModuleOptions{DisableNotificationWorker: true, PermissionSchema: defaults.PermissionSchema()})
		}
	}
	return itf.Setup(t, itf.WithComponents(components...), itf.WithUser(itf.User(permissions...)))
}
