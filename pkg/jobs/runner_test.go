package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTenantID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

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

func userCtx(t *testing.T, userID uint) context.Context {
	t.Helper()
	ctx := composables.WithTenantID(context.Background(), testTenantID)
	return composables.WithUser(ctx, testUser(t, userID))
}

func newTestRunner(store Store, registry *Registry) *Runner {
	return NewRunner(store, registry, RunnerOptions{})
}

func waitForTerminal(t *testing.T, store Store, id uuid.UUID) Job {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		j, ok, err := store.Get(context.Background(), id)
		require.NoError(t, err)
		if ok && j.Status.IsTerminal() {
			return j
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("job did not reach a terminal status in time")
	return Job{}
}

func TestRunner_EnqueueExecutesAndStoresResult(t *testing.T) {
	store := newTestStore()
	registry := NewRegistry()
	registry.Register("report.export", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		require.Equal(t, "all", params["scope"])
		progress.SetTotal(2)
		progress.Add(1)
		progress.SetPhase("halfway")
		progress.Add(1)
		return Result{FileName: "report.xlsx", Data: []byte("xlsx-bytes")}, nil
	})
	runner := newTestRunner(store, registry)

	created, err := runner.Enqueue(userCtx(t, 7), "report.export", map[string]any{"scope": "all"})
	require.NoError(t, err)
	assert.Equal(t, StatusQueued, created.Status)

	finished := waitForTerminal(t, store, created.ID)
	assert.Equal(t, StatusDone, finished.Status)
	assert.Equal(t, 100, finished.Progress)

	name, data, ok, err := store.GetResult(context.Background(), created.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "report.xlsx", name)
	assert.Equal(t, "xlsx-bytes", string(data))
}

func TestRunner_EnqueueUnknownKindRejected(t *testing.T) {
	store := newTestStore()
	runner := newTestRunner(store, NewRegistry())

	_, err := runner.Enqueue(userCtx(t, 7), "missing.kind", nil)

	require.ErrorIs(t, err, ErrUnknownJobKind)
}

func TestRunner_HandlerErrorFailsJob(t *testing.T) {
	store := newTestStore()
	registry := NewRegistry()
	registry.Register("explode", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		return Result{}, assert.AnError
	})
	runner := newTestRunner(store, registry)

	created, err := runner.Enqueue(userCtx(t, 7), "explode", nil)
	require.NoError(t, err)

	finished := waitForTerminal(t, store, created.ID)
	assert.Equal(t, StatusFailed, finished.Status)
	assert.Contains(t, finished.Error, assert.AnError.Error())
}

func TestRunner_HandlerPanicBecomesFailure(t *testing.T) {
	store := newTestStore()
	registry := NewRegistry()
	registry.Register("panic", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		panic("handler exploded")
	})
	runner := newTestRunner(store, registry)

	created, err := runner.Enqueue(userCtx(t, 7), "panic", nil)
	require.NoError(t, err)

	finished := waitForTerminal(t, store, created.ID)
	assert.Equal(t, StatusFailed, finished.Status)
	assert.Contains(t, finished.Error, "panicked")
}

func TestRunner_ResultURLPassthrough(t *testing.T) {
	store := newTestStore()
	registry := NewRegistry()
	registry.Register("external", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		return Result{URL: "/files/existing.xlsx"}, nil
	})
	runner := newTestRunner(store, registry)

	created, err := runner.Enqueue(userCtx(t, 7), "external", nil)
	require.NoError(t, err)

	finished := waitForTerminal(t, store, created.ID)
	assert.Equal(t, StatusDone, finished.Status)
	assert.Equal(t, "/files/existing.xlsx", finished.ResultURL)
	assert.False(t, finished.HasStoredResult())
}

func TestRunner_RetryOnlyFailedJobs(t *testing.T) {
	store := newTestStore()
	registry := NewRegistry()
	calls := 0
	registry.Register("flaky", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		calls++
		if calls == 1 {
			return Result{}, assert.AnError
		}
		return Result{}, nil
	})
	runner := newTestRunner(store, registry)
	ctx := userCtx(t, 7)

	created, err := runner.Enqueue(ctx, "flaky", nil)
	require.NoError(t, err)
	failed := waitForTerminal(t, store, created.ID)
	require.Equal(t, StatusFailed, failed.Status)

	requeued, err := runner.Retry(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusQueued, requeued.Status)

	done := waitForTerminal(t, store, created.ID)
	assert.Equal(t, StatusDone, done.Status)

	_, err = runner.Retry(ctx, created.ID)
	require.ErrorIs(t, err, ErrNotRetryable)
}

func TestRunner_OwnerScopedAccess(t *testing.T) {
	store := newTestStore()
	registry := NewRegistry()
	registry.Register("noop", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		return Result{}, nil
	})
	runner := newTestRunner(store, registry)

	created, err := runner.Enqueue(userCtx(t, 7), "noop", nil)
	require.NoError(t, err)
	runner.Wait()

	_, err = runner.Get(userCtx(t, 8), created.ID)
	require.ErrorIs(t, err, ErrNotFound)

	err = runner.Delete(userCtx(t, 8), created.ID)
	require.ErrorIs(t, err, ErrNotFound)

	_, _, err = runner.GetResult(userCtx(t, 8), created.ID)
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, runner.Delete(userCtx(t, 7), created.ID))
}

func TestRunner_ListMineOnlyOwnJobs(t *testing.T) {
	store := newTestStore()
	registry := NewRegistry()
	registry.Register("noop", func(ctx context.Context, params map[string]any, progress ProgressReporter) (Result, error) {
		return Result{}, nil
	})
	runner := newTestRunner(store, registry)
	ctx := composables.WithTenantID(context.Background(), testTenantID)
	// Seed directly: same tenant, different users.
	for _, userID := range []uint{7, 8} {
		require.NoError(t, store.Create(ctx, Job{
			ID:        uuid.New(),
			TenantID:  testTenantID,
			UserID:    userID,
			Kind:      "noop",
			Status:    StatusDone,
			CreatedAt: time.Now(),
		}))
	}

	mine, err := runner.ListMine(userCtx(t, 7), 10)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, uint(7), mine[0].UserID)
}

func TestRunner_ReapsStaleRunningJob(t *testing.T) {
	store := newTestStore()
	runner := newTestRunner(store, NewRegistry())
	ctx := composables.WithTenantID(context.Background(), testTenantID)
	id := uuid.New()
	stale := time.Now().Add(-2 * DefaultStaleAfter)
	require.NoError(t, store.Create(ctx, Job{
		ID:        id,
		TenantID:  testTenantID,
		UserID:    7,
		Kind:      "export",
		Status:    StatusRunning,
		CreatedAt: stale,
		UpdatedAt: stale,
	}))

	got, err := runner.Get(userCtx(t, 7), id)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, got.Status)
	assert.Contains(t, got.Error, "interrupted")

	// The reaped state is persisted.
	stored, ok, err := store.Get(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, StatusFailed, stored.Status)
}
