package services_test

import (
	"fmt"
	"testing"

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
