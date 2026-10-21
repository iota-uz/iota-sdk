package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationDispatchWorker_SurvivesStartupCancellationAndStops(t *testing.T) {
	f := setupNotificationTest(t)
	recipient, err := persistence.NewUserRepository(persistence.NewUploadRepository()).Create(f.Ctx, user.New("Worker", "Recipient", internet.MustParseEmail("notification-worker@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	f.User = recipient
	router := itf.GetService[services.NotificationRoutingService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	worker := itf.GetService[services.NotificationDispatchWorker](f)
	require.NoError(t, router.SaveRule(f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{f.User.ID()}}))
	accepted, err := dispatch.Enqueue(f.Ctx, notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: f.TenantID()})
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	ctx := userCommittedCtx(f)
	startupCtx, cancelStartup := context.WithCancel(ctx)
	stop, err := worker.Start(startupCtx)
	require.NoError(t, err)
	cancelStartup()
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, stop(stopCtx))
	})
	// Publishing after startup cancellation proves the first tick cannot mask a stopped worker.
	require.NoError(t, f.Tx.Commit(f.Ctx))
	require.EventuallyWithT(t, func(check *assert.CollectT) {
		var stored int
		err := f.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1 AND user_id=$2", f.TenantID(), f.User.ID()).Scan(&stored)
		require.NoError(check, err)
		require.Equal(check, 1, stored)
		stats, err := dispatch.Progress(ctx)
		require.NoError(check, err)
		require.EqualValues(check, 1, stats.Completed)
	}, 10*time.Second, 50*time.Millisecond)
	stopCtx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelStop()
	require.NoError(t, stop(stopCtx), "Stop must wait for the worker goroutine to terminate")
}
