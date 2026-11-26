package services_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

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

// lockWait bounds a statement that must not wait on another transaction's row
// lock. A deadlock is detected after deadlock_timeout (1s by default).
const lockWait = 10 * time.Second

func dispatchRecipients(t *testing.T, f *itf.TestEnvironment, prefix string, n int) []uint {
	t.Helper()
	repo := persistence.NewUserRepository(persistence.NewUploadRepository())
	ids := make([]uint, 0, n)
	for i := range n {
		u, err := repo.Create(f.Ctx, user.New("Queue", "Recipient", internet.MustParseEmail(fmt.Sprintf("%s%d@example.com", prefix, i)), user.UILanguageEN, user.WithTenantID(f.TenantID())))
		require.NoError(t, err)
		ids = append(ids, u.ID())
	}
	return ids
}

func within(t *testing.T, what string, fn func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		require.NoError(t, err, what)
	case <-time.After(lockWait):
		t.Fatalf("%s waited on a row lock for %s", what, lockWait)
	}
}

// A redelivered event is enqueued while a worker holds its claimed job. The
// enqueue ensures the notice and then the job; the worker holds the job and
// then ensures the same notice. Neither may lock the existing row the other
// holds, or the two transactions deadlock (40P01, issue #1458).
func TestNotificationDispatch_RedeliveryDuringClaimDoesNotLock(t *testing.T) {
	f := setupNotificationTest(t)
	ids := dispatchRecipients(t, f, "claimlock", 1)
	router := itf.GetService[services.NotificationRoutingService](f)
	require.NoError(t, router.SaveRule(f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: ids}))
	dispatch := itf.GetService[services.NotificationDispatchService](f)
	event := notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: f.TenantID()}
	accepted, err := dispatch.Enqueue(f.Ctx, event)
	require.NoError(t, err)
	require.Equal(t, 1, accepted)
	require.NoError(t, f.Tx.Commit(f.Ctx))
	ctx := userCommittedCtx(f)

	worker, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = worker.Rollback(context.Background()) }()
	workerCtx := composables.WithTx(ctx, worker)
	job, err := persistence.NewNotificationDispatchRepository().Next(workerCtx)
	require.NoError(t, err)
	require.NotNil(t, job)

	redelivery, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = redelivery.Rollback(context.Background()) }()
	within(t, "the redelivered enqueue", func() error {
		total, err := dispatch.Enqueue(composables.WithTx(ctx, redelivery), event)
		if err == nil && total != 1 {
			err = fmt.Errorf("redelivery returned %d recipients, want the stored job's 1", total)
		}
		return err
	})
	// What Process does next with its claim, while the redelivery is open.
	within(t, "the worker's notice", func() error {
		_, err := persistence.NewNotificationNoticeRepository().Ensure(workerCtx, notifications.Notice{
			TenantID: f.TenantID(), EventKey: event.Key, EventID: event.ID, Mode: notifications.NoticeRule, Event: event,
		})
		return err
	})
	require.NoError(t, redelivery.Commit(ctx))
	require.NoError(t, worker.Commit(ctx))

	var jobs, notices int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM core.notification_dispatches WHERE tenant_id=$1`, f.TenantID()).Scan(&jobs))
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1`, f.TenantID()).Scan(&notices))
	require.Equal(t, 1, jobs)
	require.Equal(t, 1, notices)
}

// Workers process jobs while their events are redelivered and new events are
// enqueued concurrently; every transaction must succeed. Run under
// -race -count=20 to exercise the interleavings.
func TestNotificationDispatch_ConcurrentEnqueueAndProcess(t *testing.T) {
	f := setupNotificationTest(t)
	ids := dispatchRecipients(t, f, "concurrent", 3)
	router := itf.GetService[services.NotificationRoutingService](f)
	require.NoError(t, router.SaveRule(f.Ctx, notifications.Rule{EventKey: notifications.TestEventKey, Enabled: true, UserIDs: ids}))
	require.NoError(t, f.Tx.Commit(f.Ctx))
	ctx := userCommittedCtx(f)
	dispatch := itf.GetService[services.NotificationDispatchService](f)

	const eventCount, deliveries, workers = 24, 3, 3
	events := make([]notifications.Event, eventCount)
	for i := range events {
		events[i] = notifications.Event{Key: notifications.TestEventKey, ID: uuid.NewString(), TenantID: f.TenantID()}
	}
	enqueue := func(e notifications.Event) error {
		return composables.InTx(ctx, func(txCtx context.Context) error {
			total, err := dispatch.Enqueue(txCtx, e)
			if err == nil && total != len(ids) {
				err = fmt.Errorf("enqueue returned %d recipients, want %d", total, len(ids))
			}
			return err
		})
	}
	// Half of the events are stored before the workers start, so their
	// redeliveries meet claimed jobs; the rest are first enqueued concurrently.
	for _, e := range events[:eventCount/2] {
		require.NoError(t, enqueue(e))
	}

	var mu sync.Mutex
	var errs []error
	fail := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}
	var producers, consumers sync.WaitGroup
	stop := make(chan struct{})
	for range workers {
		consumers.Add(1)
		go func() {
			defer consumers.Done()
			for {
				more, err := dispatch.Process(ctx)
				if err != nil {
					fail(fmt.Errorf("process: %w", err))
				}
				if !more {
					select {
					case <-stop:
						return
					default:
					}
				}
			}
		}()
	}
	for d := range deliveries {
		producers.Add(1)
		go func() {
			defer producers.Done()
			for i := range events {
				// Each producer walks the events in a different order.
				if err := enqueue(events[(i*(d+1)+d)%eventCount]); err != nil {
					fail(fmt.Errorf("enqueue: %w", err))
				}
			}
		}()
	}
	producers.Wait()
	close(stop)
	consumers.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	stats, err := dispatch.Progress(ctx)
	require.NoError(t, err)
	require.EqualValues(t, eventCount, stats.Completed)
	require.Zero(t, stats.Pending)
	require.Zero(t, stats.Retrying)
	var notices, inbox int
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM core.notification_notices WHERE tenant_id=$1`, f.TenantID()).Scan(&notices))
	require.NoError(t, f.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM core.notifications WHERE tenant_id=$1`, f.TenantID()).Scan(&inbox))
	require.Equal(t, eventCount, notices)
	require.Equal(t, eventCount*len(ids), inbox, "a redelivery never queues a second notice")
}
