package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jobs/domain/aggregates/job"
	jobsservices "github.com/iota-uz/iota-sdk/modules/jobs/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

func newTestWorker(t *testing.T, repo *fakeRepo, store *fakeUploadStore, registry *jobsservices.Registry) *jobsservices.Worker {
	t.Helper()
	return jobsservices.NewWorker(
		nil,
		repo,
		registry,
		store,
		nil,
		nil,
		nil,
		logrus.New(),
		jobsservices.WorkerOptions{
			PollInterval:    5 * time.Millisecond,
			CleanupInterval: time.Hour,
		},
	)
}

func enqueueTestJob(t *testing.T, repo *fakeRepo, kind string) job.Job {
	t.Helper()
	ctx := composables.WithTenantID(context.Background(), testTenantID)
	created, err := repo.Save(ctx, job.New(
		kind,
		job.WithTenantID(testTenantID),
		job.WithUserID(1),
		job.WithParams(map[string]any{"scope": "all"}),
	))
	require.NoError(t, err)
	return created
}

func waitForTerminal(t *testing.T, repo *fakeRepo, id uuid.UUID) job.Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if j := repo.byID(id); j != nil && j.Status().IsTerminal() {
			return j
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("job did not reach a terminal status in time")
	return nil
}

func TestWorker_ExecutesJobAndStoresResult(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeUploadStore()
	registry := jobsservices.NewRegistry()
	registry.Register("report.export", func(ctx context.Context, params map[string]any, progress jobsservices.ProgressReporter) (jobsservices.Result, error) {
		require.Equal(t, "all", params["scope"])
		progress.SetTotal(2)
		progress.Add(1)
		progress.SetPhase("halfway")
		progress.Add(1)
		return jobsservices.Result{FileName: "report.xlsx", Data: []byte("xlsx-bytes")}, nil
	})
	created := enqueueTestJob(t, repo, "report.export")

	ctx, cancel := context.WithCancel(context.Background())
	worker := newTestWorker(t, repo, store, registry)
	go func() { _ = worker.Start(ctx) }()
	defer cancel()

	finished := waitForTerminal(t, repo, created.ID())

	require.Equal(t, job.StatusDone, finished.Status())
	require.NotNil(t, finished.ResultUploadID())
	assert.Equal(t, 100, finished.Progress())

	require.Len(t, store.created, 1)
	assert.Equal(t, "report.xlsx", store.created[0].Name)
	assert.Equal(t, "xlsx-bytes", store.created[0].Data)

	require.NotEmpty(t, repo.progress)
	last := repo.progress[len(repo.progress)-1]
	assert.Equal(t, 100, last.Percent)
	assert.Equal(t, "halfway", last.Phase)
}

func TestWorker_FailedHandlerMarksFailed(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeUploadStore()
	registry := jobsservices.NewRegistry()
	registry.Register("explode", func(ctx context.Context, params map[string]any, progress jobsservices.ProgressReporter) (jobsservices.Result, error) {
		return jobsservices.Result{}, fakeError("kaboom")
	})
	created := enqueueTestJob(t, repo, "explode")

	ctx, cancel := context.WithCancel(context.Background())
	worker := newTestWorker(t, repo, store, registry)
	go func() { _ = worker.Start(ctx) }()
	defer cancel()

	finished := waitForTerminal(t, repo, created.ID())

	assert.Equal(t, job.StatusFailed, finished.Status())
	assert.Contains(t, finished.Error(), "kaboom")
	assert.Nil(t, finished.ResultUploadID())
	assert.Empty(t, store.created)
}

func TestWorker_UnknownKindFailsVisibly(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeUploadStore()
	created := enqueueTestJob(t, repo, "never.registered")

	ctx, cancel := context.WithCancel(context.Background())
	worker := newTestWorker(t, repo, store, jobsservices.NewRegistry())
	go func() { _ = worker.Start(ctx) }()
	defer cancel()

	finished := waitForTerminal(t, repo, created.ID())

	assert.Equal(t, job.StatusFailed, finished.Status())
	assert.Contains(t, finished.Error(), "unknown job kind")
}

func TestWorker_HandlerPanicBecomesFailure(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeUploadStore()
	registry := jobsservices.NewRegistry()
	registry.Register("panic", func(ctx context.Context, params map[string]any, progress jobsservices.ProgressReporter) (jobsservices.Result, error) {
		panic("handler exploded")
	})
	created := enqueueTestJob(t, repo, "panic")

	ctx, cancel := context.WithCancel(context.Background())
	worker := newTestWorker(t, repo, store, registry)
	go func() { _ = worker.Start(ctx) }()
	defer cancel()

	finished := waitForTerminal(t, repo, created.ID())

	assert.Equal(t, job.StatusFailed, finished.Status())
	assert.Contains(t, finished.Error(), "panicked")
}

func TestWorker_ResultWithoutDataNeedsNoUpload(t *testing.T) {
	repo := newFakeRepo()
	store := newFakeUploadStore()
	registry := jobsservices.NewRegistry()
	registry.Register("bulk", func(ctx context.Context, params map[string]any, progress jobsservices.ProgressReporter) (jobsservices.Result, error) {
		return jobsservices.Result{}, nil
	})
	created := enqueueTestJob(t, repo, "bulk")

	ctx, cancel := context.WithCancel(context.Background())
	worker := newTestWorker(t, repo, store, registry)
	go func() { _ = worker.Start(ctx) }()
	defer cancel()

	finished := waitForTerminal(t, repo, created.ID())

	assert.Equal(t, job.StatusDone, finished.Status())
	assert.Nil(t, finished.ResultUploadID())
	assert.Empty(t, store.created)
}
