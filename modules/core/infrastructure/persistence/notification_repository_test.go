package persistence_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/notification"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/require"
)

func TestNotificationRepository_OwnershipDedupeAndRead(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	r := persistence.NewNotificationRepository()
	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	email, err := internet.NewEmail("notifications@example.com")
	require.NoError(t, err)
	recipient, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(f.Ctx, user.New("Notification", "Recipient", email, user.UILanguageEN, user.WithTenantID(tenant)))
	require.NoError(t, err)
	n, err := notification.New(recipient.ID(), "Created", "Body", notification.WithDedupeKey("event-1"), notification.WithActionURL("/users"))
	require.NoError(t, err)
	mismatch, err := notification.New(recipient.ID(), "Wrong tenant", "", notification.WithTenantID(uuid.New()))
	require.NoError(t, err)
	_, err = r.Create(f.Ctx, mismatch)
	require.Error(t, err)
	first, err := r.Create(f.Ctx, n)
	require.NoError(t, err)
	second, err := r.Create(f.Ctx, n)
	require.NoError(t, err)
	require.Equal(t, first.ID(), second.ID())
	count, err := r.UnreadCount(f.Ctx, recipient.ID())
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	list, err := r.List(f.Ctx, recipient.ID(), notification.FindParams{UnreadOnly: true})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.ErrorIs(t, r.MarkRead(f.Ctx, recipient.ID()+10000, first.ID()), notification.ErrNotFound)
	otherCtx := composables.WithTenantID(f.Ctx, uuid.New())
	list, err = r.List(otherCtx, recipient.ID(), notification.FindParams{})
	require.NoError(t, err)
	require.Empty(t, list)
	require.ErrorIs(t, r.MarkRead(otherCtx, recipient.ID(), first.ID()), notification.ErrNotFound)
	require.NoError(t, r.MarkRead(f.Ctx, recipient.ID(), first.ID()))
	require.NoError(t, r.MarkRead(f.Ctx, recipient.ID(), first.ID()))
	list, err = r.List(f.Ctx, recipient.ID(), notification.FindParams{})
	require.NoError(t, err)
	require.NotNil(t, list[0].ReadAt())
	count, err = r.UnreadCount(f.Ctx, recipient.ID())
	require.NoError(t, err)
	require.Zero(t, count)
	plain, err := notification.New(recipient.ID(), "Another", "")
	require.NoError(t, err)
	_, err = r.Create(f.Ctx, plain)
	require.NoError(t, err)
	require.NoError(t, r.MarkAllRead(f.Ctx, recipient.ID()))
	count, err = r.UnreadCount(f.Ctx, recipient.ID())
	require.NoError(t, err)
	require.Zero(t, count)
	_, err = r.Create(otherCtx, plain)
	require.Error(t, err)
}

func TestNotificationRepository_CursorAndLevel(t *testing.T) {
	t.Parallel()
	f := setupTest(t)
	r := persistence.NewNotificationRepository()
	tenant, err := composables.UseTenantID(f.Ctx)
	require.NoError(t, err)
	email, err := internet.NewEmail("cursor-notifications@example.com")
	require.NoError(t, err)
	recipient, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(f.Ctx, user.New("Cursor", "Recipient", email, user.UILanguageEN, user.WithTenantID(tenant)))
	require.NoError(t, err)
	at := time.Now().UTC().Truncate(time.Microsecond)
	for i := 0; i < 3; i++ {
		n, err := notification.New(recipient.ID(), "Cursor", "", notification.WithCreatedAt(at), notification.WithLevel(notification.LevelWarning))
		require.NoError(t, err)
		_, err = r.Create(f.Ctx, n)
		require.NoError(t, err)
	}
	first, err := r.List(f.Ctx, recipient.ID(), notification.FindParams{Limit: 1})
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, notification.LevelWarning, first[0].Level())
	second, err := r.List(f.Ctx, recipient.ID(), notification.FindParams{Limit: 1, Cursor: notification.CursorFor(first[0])})
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.NotEqual(t, first[0].ID(), second[0].ID())
	third, err := r.List(f.Ctx, recipient.ID(), notification.FindParams{Limit: 1, Cursor: notification.CursorFor(second[0])})
	require.NoError(t, err)
	require.Len(t, third, 1)
	require.NotEqual(t, second[0].ID(), third[0].ID())
	last, err := r.List(f.Ctx, recipient.ID(), notification.FindParams{Cursor: notification.CursorFor(third[0])})
	require.NoError(t, err)
	require.Empty(t, last)
	_, err = r.List(f.Ctx, recipient.ID(), notification.FindParams{Cursor: "invalid"})
	require.Error(t, err)
}
