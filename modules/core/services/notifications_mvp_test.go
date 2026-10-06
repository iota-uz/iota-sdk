package services_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"

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
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestNotificationsMVP_UserCreationToInbox(t *testing.T) {
	f := setupTestWithPermissions(t, permissions.UserCreate, permissions.UserRead)
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
	users := itf.GetService[services.UserService](f)
	rule := notifications.Rule{EventKey: core.UserCreatedNotificationEvent, Enabled: true, UserIDs: []uint{f.User.ID()}}
	require.NoError(t, router.SaveRule(ctx, rule))
	created, err := users.Create(ctx, user.New("Business", "Trial", internet.MustParseEmail("trial@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
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
	require.Len(t, inside, 2)
	require.NoError(t, tx.Rollback(ctx))
	items, err = inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1, "rolling back user creation must roll back its notification")
	rule.Enabled = false
	require.NoError(t, router.SaveRule(ctx, rule))
	_, err = users.Create(ctx, user.New("Disabled", "Trial", internet.MustParseEmail("disabled@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	items, err = inbox.List(ctx, notification.FindParams{})
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestNotificationsMVP_GroupAndRoleAudiences(t *testing.T) {
	f := setupTest(t)
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
	require.Equal(t, rule, saved)
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
