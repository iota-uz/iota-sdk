package bichat

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composition"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// heldExecutor accepts a job and keeps it until release is closed, ignoring
// cancellation the way the real executor's persistence path does
// (context.WithoutCancel). It then writes its terminal state through the
// shared Redis client.
type heldExecutor struct {
	client    *redis.Client
	key       string
	entered   chan struct{}
	cancelled chan struct{}
	release   chan struct{}
	persisted chan error
}

func (e *heldExecutor) Execute(ctx context.Context, _ services.RunJobPayload) error {
	close(e.entered)
	<-ctx.Done()
	close(e.cancelled)
	<-e.release
	e.persisted <- e.client.Set(context.WithoutCancel(ctx), e.key, "cancelled", time.Minute).Err()
	return nil
}

// Regression for the shutdown gap reported in EAI-UZ/granite#4962: the run
// worker loop was rooted in context.Background() and the shared Redis client
// was closed as soon as the stop deadline expired, while an accepted job was
// still running its terminal bookkeeping (persist + XACK) on that client.
//
// Runs against a real Redis (REDIS_URL) with an executor that holds the job
// past the deadline. Falsely green if the deadline did not actually expire
// before the job finished (asserted via ErrResourceInUse and a live PING
// while the job is held), or if the ack were skipped (asserted via XPENDING
// and XLEN on an independent connection after the job exits).
func TestRunWorkerShutdownDeadlineKeepsSharedRedisForAcceptedJob(t *testing.T) {
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		t.Skip("REDIS_URL is required")
	}
	opts, err := redis.ParseURL(redisURL)
	require.NoError(t, err)

	prefix := "bichat:test:shutdown:" + uuid.NewString()
	stream := prefix + ":jobs"
	group := prefix + ":group"
	stateKey := prefix + ":state"

	observer := redis.NewClient(opts)
	t.Cleanup(func() {
		ctx := context.Background()
		keys, _ := observer.Keys(ctx, prefix+"*").Result()
		if len(keys) > 0 {
			_ = observer.Del(ctx, keys...).Err()
		}
		_ = observer.Close()
	})

	shared := redis.NewClient(opts)
	owned := composition.NewOwnedResource(shared.Close)
	t.Cleanup(func() { _ = shared.Close() })

	queue, err := services.NewRedisRunJobQueue(services.RedisRunJobQueueConfig{Client: shared, Stream: stream, DedupePrefix: prefix + ":dedupe:"})
	require.NoError(t, err)
	executor := &heldExecutor{
		client:    shared,
		key:       stateKey,
		entered:   make(chan struct{}),
		cancelled: make(chan struct{}),
		release:   make(chan struct{}),
		persisted: make(chan error, 1),
	}
	worker, err := services.NewRunJobWorker(services.RunJobWorkerConfig{
		Queue:        queue,
		Executor:     executor,
		Group:        group,
		Consumer:     "test-consumer",
		ReadBlock:    10 * time.Millisecond,
		PollInterval: 10 * time.Millisecond,
		MaxRetries:   2,
	})
	require.NoError(t, err)

	lifecycle, shutdown := context.WithCancel(context.Background())
	defer shutdown()
	startCtx, startCancel := context.WithTimeout(composition.WithLifecycleContext(context.Background(), lifecycle), time.Second)
	stop, err := superviseRunWorker(startCtx, worker, owned.Acquire, nil)
	require.NoError(t, err)
	// The startup timeout ending must not stop the worker loop.
	startCancel()

	job := services.RunJobPayload{TenantID: uuid.New(), UserID: 1, SessionID: uuid.New(), RunID: uuid.New(), RequestID: uuid.New(), UserMessageID: uuid.New()}
	_, _, err = queue.Enqueue(context.Background(), job)
	require.NoError(t, err)
	waitFor(t, executor.entered, "worker never accepted the job")

	// Runtime.Shutdown order: cancel the lifecycle context, run the run
	// worker stop hook, then the runtime hook closes the shared client.
	shutdown()
	waitFor(t, executor.cancelled, "lifecycle cancellation did not reach the job context")

	stopCtx, cancelStop := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelStop()
	require.ErrorIs(t, stop(stopCtx), context.DeadlineExceeded)
	closeErr := owned.Close(stopCtx)
	require.ErrorIs(t, closeErr, composition.ErrResourceInUse)
	require.NoError(t, shared.Ping(context.Background()).Err(), "shared client closed under an accepted job")

	close(executor.release)
	require.NoError(t, <-executor.persisted, "terminal persistence after the deadline failed")
	waitFor(t, owned.Done(), "shared client was not closed after the job released its lease")
	require.ErrorIs(t, shared.Ping(context.Background()).Err(), redis.ErrClosed)

	ctx := context.Background()
	state, err := observer.Get(ctx, stateKey).Result()
	require.NoError(t, err)
	require.Equal(t, "cancelled", state)
	pending, err := observer.XPending(ctx, stream, group).Result()
	require.NoError(t, err)
	require.Zero(t, pending.Count, "accepted job was not acked; it would be re-delivered and executed twice")
	length, err := observer.XLen(ctx, stream).Result()
	require.NoError(t, err)
	require.Zero(t, length)
	retries, err := observer.Exists(ctx, stream+":retry").Result()
	require.NoError(t, err)
	require.Zero(t, retries, "a terminal (cancelled) run must not be scheduled for retry")
}

func waitFor(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		t.Fatal(message)
	}
}
