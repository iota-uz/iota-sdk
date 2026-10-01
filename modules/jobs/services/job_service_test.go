package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	jobsservices "github.com/iota-uz/iota-sdk/modules/jobs/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/eventbus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testUser(t *testing.T, id uint) user.User {
	t.Helper()
	email, err := internet.NewEmail("user@example.com")
	require.NoError(t, err)
	return user.New(
		"First",
		"Last",
		email,
		"en",
		user.WithID(id),
		user.WithCreatedAt(time.Now()),
		user.WithUpdatedAt(time.Now()),
	)
}

func newJobService(repo *fakeRepo, registry *jobsservices.Registry) *jobsservices.JobService {
	return jobsservices.NewJobService(repo, registry, eventbus.NewEventPublisher(logrus.New()))
}

func userCtx(t *testing.T, userID uint) context.Context {
	t.Helper()
	ctx := composables.WithTenantID(context.Background(), testTenantID)
	return composables.WithUser(ctx, testUser(t, userID))
}

func TestJobService_Enqueue_UnknownKindRejected(t *testing.T) {
	repo := newFakeRepo()
	svc := newJobService(repo, jobsservices.NewRegistry())

	_, err := svc.Enqueue(userCtx(t, 1), "missing.kind", nil)

	require.ErrorIs(t, err, jobsservices.ErrUnknownJobKind)
	assert.Empty(t, repo.rows)
}

func TestJobService_Enqueue_CreatesQueuedJob(t *testing.T) {
	repo := newFakeRepo()
	registry := jobsservices.NewRegistry()
	registry.Register("export", func(ctx context.Context, params map[string]any, progress jobsservices.ProgressReporter) (jobsservices.Result, error) {
		return jobsservices.Result{}, nil
	})
	svc := newJobService(repo, registry)

	created, err := svc.Enqueue(userCtx(t, 7), "export", map[string]any{"format": "xlsx"})

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID())
	assert.Equal(t, job.StatusQueued, created.Status())
	assert.Equal(t, testTenantID, created.TenantID())
	assert.Equal(t, uint(7), created.UserID())
	assert.Equal(t, "xlsx", created.Params()["format"])
}

func TestJobService_Retry_OnlyFailedJobs(t *testing.T) {
	repo := newFakeRepo()
	svc := newJobService(repo, jobsservices.NewRegistry())
	ctx := userCtx(t, 7)

	queued, err := repo.Save(composables.WithTenantID(ctx, testTenantID), job.New(
		"export",
		job.WithTenantID(testTenantID),
		job.WithUserID(7),
	))
	require.NoError(t, err)

	_, err = svc.Retry(ctx, queued.ID())
	require.ErrorIs(t, err, jobsservices.ErrJobNotRetryable)

	started := queued.MarkRunning()
	failed, err := repo.Save(composables.WithTenantID(ctx, testTenantID), started.MarkFailed("boom"))
	require.NoError(t, err)

	requeued, err := svc.Retry(ctx, failed.ID())
	require.NoError(t, err)
	assert.Equal(t, job.StatusQueued, requeued.Status())
	assert.Equal(t, "", requeued.Error())
}

func TestJobService_ScopedToOwner(t *testing.T) {
	repo := newFakeRepo()
	svc := newJobService(repo, jobsservices.NewRegistry())
	ctx := userCtx(t, 7)

	queued, err := repo.Save(composables.WithTenantID(ctx, testTenantID), job.New(
		"export",
		job.WithTenantID(testTenantID),
		job.WithUserID(8),
	))
	require.NoError(t, err)

	_, err = svc.Get(userCtx(t, 7), queued.ID())
	require.ErrorIs(t, err, job.ErrNotFound)

	err = svc.Delete(userCtx(t, 7), queued.ID())
	require.ErrorIs(t, err, job.ErrNotFound)

	err = svc.Delete(userCtx(t, 8), queued.ID())
	require.NoError(t, err)
}

func TestJobService_ListMine_OnlyOwnJobs(t *testing.T) {
	repo := newFakeRepo()
	svc := newJobService(repo, jobsservices.NewRegistry())
	ctx := composables.WithTenantID(context.Background(), testTenantID)
	for _, userID := range []uint{7, 8} {
		_, err := repo.Save(ctx, job.New(
			"export",
			job.WithTenantID(testTenantID),
			job.WithUserID(userID),
		))
		require.NoError(t, err)
	}

	mine, err := svc.ListMine(userCtx(t, 7), 10)

	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, uint(7), mine[0].UserID())
}
