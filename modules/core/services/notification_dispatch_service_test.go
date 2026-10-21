package services_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/notifications"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

func TestNotificationDispatch_BatchesRetryRestartAndIsolation(t *testing.T) {
	f := setupNotificationTest(t)
	router := itf.GetService[services.NotificationRoutingService](f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	recipients := persistence.NewUserRepository(persistence.NewUploadRepository())
	ids := make([]uint, 0, 205)
	for i := range 205 {
		u, err := recipients.Create(f.Ctx, user.New("Queue", "Recipient", internet.MustParseEmail(fmt.Sprintf("queue%d@example.com", i)), user.UILanguageEN, user.WithTenantID(f.TenantID())))
		require.NoError(t, err)
		ids = append(ids, u.ID())
	}
	rule := notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: ids}
	require.NoError(t, router.SaveRule(f.Ctx, rule))
	event := notifications.Event{Key: rule.EventKey, ID: uuid.NewString(), DedupeKey: uuid.NewString(), TenantID: f.TenantID()}
	accepted, err := dispatch.Enqueue(f.Ctx, event)
	require.NoError(t, err)
	require.Equal(t, 205, accepted)
	require.NoError(t, f.Tx.Commit(f.Ctx))
	ctx := userCommittedCtx(f)
	jobs, err := dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	jobID := jobs[0].ID
	rule.Enabled = false
	require.NoError(t, router.SaveRule(ctx, rule))
	_, err = f.Pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE core.notifications ADD CONSTRAINT dispatch_failure_test CHECK(tenant_id<>'%s'::uuid OR user_id<>%d) NOT VALID`, f.TenantID(), ids[50]))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = f.Pool.Exec(ctx, `ALTER TABLE core.notifications DROP CONSTRAINT IF EXISTS dispatch_failure_test`)
	})
	worked, err := dispatch.Process(ctx)
	require.True(t, worked)
	require.Error(t, err)
	stats, err := dispatch.Progress(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Retrying)
	var stored int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1", f.TenantID()).Scan(&stored))
	require.Zero(t, stored, "failed batches must be atomic even after earlier recipient writes")
	jobs, err = dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Zero(t, jobs[0].Processed)
	require.Equal(t, 1, jobs[0].Attempts)
	_, err = f.Pool.Exec(ctx, `ALTER TABLE core.notifications DROP CONSTRAINT dispatch_failure_test`)
	require.NoError(t, err)
	otherCtx := composables.WithTenantID(ctx, uuid.New())
	require.ErrorIs(t, dispatch.Retry(otherCtx, jobID), notifications.ErrDispatchNotFound)
	otherJobs, err := dispatch.Jobs(otherCtx)
	require.NoError(t, err)
	require.Empty(t, otherJobs)
	require.NoError(t, dispatch.Retry(ctx, jobID))
	worked, err = dispatch.Process(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	jobs, err = dispatch.Jobs(ctx)
	require.NoError(t, err)
	require.Equal(t, 100, jobs[0].Processed)
	require.Equal(t, 100, jobs[0].Delivered)
	restarted := services.NewNotificationDispatchService(router, persistence.NewNotificationDispatchRepository(), itf.GetService[services.NotificationService](f))
	worked, err = restarted.Process(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	jobs, err = restarted.Jobs(ctx)
	require.NoError(t, err)
	require.Equal(t, 200, jobs[0].Processed)
	worked, err = restarted.Process(ctx)
	require.NoError(t, err)
	require.True(t, worked)
	stats, err = restarted.Progress(ctx)
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.Completed)
	require.Zero(t, stats.Pending)
	require.Zero(t, stats.Retrying)
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1", f.TenantID()).Scan(&stored))
	require.Equal(t, 205, stored)
	rule.Enabled = true
	rule.UserIDs = []uint{ids[0]}
	require.NoError(t, router.SaveRule(ctx, rule))
	event.ID = uuid.NewString()
	accepted, err = restarted.Enqueue(ctx, event)
	require.NoError(t, err)
	require.Equal(t, 205, accepted, "replayed events preserve their original audience snapshot")
	worked, err = restarted.Process(ctx)
	require.NoError(t, err)
	require.False(t, worked)
}

func TestNotificationDispatch_ClaimsSkipLockedAndRollback(t *testing.T) {
	f := setupNotificationTest(t)
	recipients := persistence.NewUserRepository(persistence.NewUploadRepository())
	u, err := recipients.Create(f.Ctx, user.New("Queue", "Lock", internet.MustParseEmail("queuelock@example.com"), user.UILanguageEN, user.WithTenantID(f.TenantID())))
	require.NoError(t, err)
	router := itf.GetService[services.NotificationRoutingService](f)
	require.NoError(t, router.SaveRule(f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: []uint{u.ID()}}))
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	accepted, err := dispatch.Enqueue(f.Ctx, notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: f.TenantID()})
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	require.NoError(t, f.Tx.Commit(f.Ctx))
	ctx := userCommittedCtx(f)
	repo := persistence.NewNotificationDispatchRepository()
	first, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = first.Rollback(ctx) }()
	job, err := repo.Next(composables.WithTx(ctx, first))
	require.NoError(t, err)
	require.NotNil(t, job)
	second, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = second.Rollback(ctx) }()
	unavailable, err := repo.Next(composables.WithTx(ctx, second))
	require.NoError(t, err)
	require.Nil(t, unavailable, "another worker must skip the locked batch")
	require.NoError(t, first.Rollback(ctx))
	available, err := repo.Next(composables.WithTx(ctx, second))
	require.NoError(t, err)
	require.Equal(t, job.ID, available.ID)
}
