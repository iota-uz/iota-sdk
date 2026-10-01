package services

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequenceRunExecutor struct {
	mu    sync.Mutex
	errs  []error
	calls chan RunJobPayload
}

func newSequenceRunExecutor(errs ...error) *sequenceRunExecutor {
	return &sequenceRunExecutor{
		errs:  errs,
		calls: make(chan RunJobPayload, 32),
	}
}

func (e *sequenceRunExecutor) Execute(_ context.Context, job RunJobPayload) error {
	e.calls <- job

	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.errs) == 0 {
		return nil
	}
	err := e.errs[0]
	e.errs = e.errs[1:]
	return err
}

func waitForRunJobCount(t *testing.T, exec *sequenceRunExecutor, count int, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)
	received := 0
	for received < count {
		select {
		case <-exec.calls:
			received++
		case <-deadline:
			t.Fatalf("timed out waiting for %d run executions, received %d", count, received)
		}
	}
}

func newTestRunJobWorker(t *testing.T, mr *miniredis.Miniredis, exec *sequenceRunExecutor, extra func(*RunJobWorkerConfig)) (*RunJobWorker, *RedisRunJobQueue, *RedisRunSessionQueue) {
	t.Helper()

	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:run:worker-test",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	sessionQueue, err := NewRedisRunSessionQueue(RedisRunSessionQueueConfig{
		RedisURL:  mr.Addr(),
		KeyPrefix: "bichat:run:worker-test-queue",
	})
	require.NoError(t, err)

	cfg := RunJobWorkerConfig{
		Queue:          queue,
		Executor:       exec,
		SessionQueue:   sessionQueue,
		Group:          "bichat-run-workers",
		Consumer:       "c1",
		PollInterval:   5 * time.Millisecond,
		ReadBlock:      5 * time.Millisecond,
		MaxRetries:     2,
		RetryBaseDelay: 5 * time.Millisecond,
		RetryMaxDelay:  20 * time.Millisecond,
		PendingIdle:    5 * time.Millisecond,
	}
	if extra != nil {
		extra(&cfg)
	}
	worker, err := NewRunJobWorker(cfg)
	require.NoError(t, err)
	return worker, queue, sessionQueue
}

func TestRunJobWorker_ExecutesJobAndAcks(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	exec := newSequenceRunExecutor()
	worker, queue, _ := newTestRunJobWorker(t, mr, exec, nil)

	payload := RunJobPayload{
		TenantID:      uuid.New(),
		SessionID:     uuid.New(),
		RequestID:     uuid.New(),
		RunID:         uuid.New(),
		UserMessageID: uuid.New(),
		Content:       "hello",
	}
	_, _, err := queue.Enqueue(context.Background(), payload)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	var received RunJobPayload
	select {
	case received = <-exec.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for run execution")
	}
	assert.Equal(t, payload.RunID, received.RunID)
	assert.Equal(t, payload.Content, received.Content)

	cancel()
	<-done

	// Ack happens after Execute returns — poll for the stream to drain.
	require.Eventually(t, func() bool {
		length, xlenErr := queue.client.XLen(context.Background(), queue.stream).Result()
		return xlenErr == nil && length == 0
	}, 5*time.Second, 5*time.Millisecond, "processed job must be acked and deleted from the stream")
}

func TestRunJobWorker_RetriesTransientFailureThenSucceeds(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	exec := newSequenceRunExecutor(assert.AnError, nil)
	worker, queue, _ := newTestRunJobWorker(t, mr, exec, nil)

	payload := RunJobPayload{
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		RequestID: uuid.New(),
		RunID:     uuid.New(),
		Content:   "retry me",
	}
	_, _, err := queue.Enqueue(context.Background(), payload)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	var first, retried RunJobPayload
	select {
	case first = <-exec.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first execution")
	}
	select {
	case retried = <-exec.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for retry execution")
	}
	assert.Equal(t, payload.RunID, first.RunID)
	assert.Equal(t, payload.RunID, retried.RunID, "retry must execute the same run")
	assert.Equal(t, 1, retried.Attempt, "retry must carry attempt=1")

	cancel()
	<-done

	require.Eventually(t, func() bool {
		length, xlenErr := queue.client.XLen(context.Background(), queue.stream).Result()
		return xlenErr == nil && length == 0
	}, 5*time.Second, 5*time.Millisecond, "job must be acked after the successful retry")
}

func TestRunJobWorker_TerminalFailureAfterMaxRetries(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	exec := newSequenceRunExecutor(assert.AnError, assert.AnError)

	var (
		failMu        sync.Mutex
		failedPayload RunJobPayload
		failureCalled bool
	)
	worker, queue, sessionQueue := newTestRunJobWorker(t, mr, exec, func(cfg *RunJobWorkerConfig) {
		cfg.OnJobTerminalFailure = func(_ context.Context, job RunJobPayload, cause error) {
			failMu.Lock()
			defer failMu.Unlock()
			failedPayload = job
			failureCalled = true
			require.Error(t, cause)
		}
	})

	tenantID := uuid.New()
	sessionID := uuid.New()

	// A job parked behind the failed one must be promoted after the
	// terminal failure so the FIFO does not deadlock.
	queued := RunJobPayload{
		TenantID:  tenantID,
		SessionID: sessionID,
		RequestID: uuid.New(),
		RunID:     uuid.New(),
		Content:   "queued behind",
	}
	require.NoError(t, sessionQueue.Push(context.Background(), tenantID, sessionID, QueuedRunJob{Payload: queued}))

	payload := RunJobPayload{
		TenantID:  tenantID,
		SessionID: sessionID,
		RequestID: uuid.New(),
		RunID:     uuid.New(),
		Content:   "always fails",
	}
	_, _, err := queue.Enqueue(context.Background(), payload)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	waitForRunJobCount(t, exec, 2, 5*time.Second)

	// The hook fires after the second Execute returns — poll for it.
	require.Eventually(t, func() bool {
		failMu.Lock()
		defer failMu.Unlock()
		return failureCalled
	}, 5*time.Second, 5*time.Millisecond, "terminal failure hook must be invoked")

	failMu.Lock()
	assert.Equal(t, payload.RunID, failedPayload.RunID)
	failMu.Unlock()

	// The queued job was drained from the FIFO and re-enqueued onto the
	// stream (PromoteNextQueuedRun) — it may already have been consumed and
	// executed by the same worker before we cancel, so assert on the queue
	// draining rather than on residual stream state.
	require.Eventually(t, func() bool {
		depth, lenErr := sessionQueue.Len(context.Background(), tenantID, sessionID)
		return lenErr == nil && depth == 0
	}, 5*time.Second, 5*time.Millisecond, "queued job must be promoted (drained) after terminal failure")

	cancel()
	<-done
}

func TestRunJobWorker_ReclaimsStalePendingEntries(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	exec := newSequenceRunExecutor()
	worker, queue, _ := newTestRunJobWorker(t, mr, exec, nil)

	payload := RunJobPayload{
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		RequestID: uuid.New(),
		RunID:     uuid.New(),
		Content:   "crashed worker had this",
	}
	_, _, enqueueErr := queue.Enqueue(context.Background(), payload)
	require.NoError(t, enqueueErr)

	// Simulate a crashed worker: read the job into a dead consumer's PEL.
	group := "bichat-run-workers"
	require.NoError(t, client.XGroupCreateMkStream(context.Background(), queue.stream, group, "0").Err())
	_, readErr := client.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group:    group,
		Consumer: "dead-consumer",
		Streams:  []string{queue.stream, ">"},
	}).Result()
	require.NoError(t, readErr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	var received RunJobPayload
	select {
	case received = <-exec.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for reclaimed execution")
	}
	cancel()
	<-done

	assert.Equal(t, payload.RunID, received.RunID, "crashed worker's job must be reclaimed and executed")
}

func TestPromoteNextQueuedRun_PopsHeadAndReenqueues(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisRunJobQueue(RedisRunJobQueueConfig{RedisURL: mr.Addr(), Stream: "bichat:run:promote-test"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	sessionQueue, err := NewRedisRunSessionQueue(RedisRunSessionQueueConfig{RedisURL: mr.Addr(), KeyPrefix: "bichat:run:promote-queue"})
	require.NoError(t, err)

	tenantID := uuid.New()
	sessionID := uuid.New()
	first := RunJobPayload{TenantID: tenantID, SessionID: sessionID, RequestID: uuid.New(), RunID: uuid.New(), Content: "first", Attempt: 3}
	second := RunJobPayload{TenantID: tenantID, SessionID: sessionID, RequestID: uuid.New(), RunID: uuid.New(), Content: "second"}

	require.NoError(t, sessionQueue.Push(context.Background(), tenantID, sessionID, QueuedRunJob{Payload: first}))
	require.NoError(t, sessionQueue.Push(context.Background(), tenantID, sessionID, QueuedRunJob{Payload: second}))

	promoted, err := PromoteNextQueuedRun(context.Background(), sessionQueue, queue, nil, tenantID, sessionID)
	require.NoError(t, err)
	require.True(t, promoted, "head of the FIFO must be promoted")

	entries, xrangeErr := queue.client.XRange(context.Background(), queue.stream, "-", "+").Result()
	require.NoError(t, xrangeErr)
	require.Len(t, entries, 1)
	got, parseErr := ParseRunJobPayload(entries[0].Values)
	require.NoError(t, parseErr)
	assert.Equal(t, first.RunID, got.RunID, "FIFO head must be promoted first")
	assert.Equal(t, 0, got.Attempt, "promoted job must restart with a clean attempt count")

	depth, lenErr := sessionQueue.Len(context.Background(), tenantID, sessionID)
	require.NoError(t, lenErr)
	assert.Equal(t, int64(1), depth, "second job must remain queued")
}

func TestRunJobWorker_OwnershipRefreshPreventsReclaimWhileExecuting(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	// Block the executor until we assert the PEL state mid-execution.
	started := make(chan struct{})
	release := make(chan struct{})
	exec := &blockingRunExecutor{started: started, release: release}
	worker, queue, _ := newTestRunJobWorker(t, mr, nil, func(cfg *RunJobWorkerConfig) {
		cfg.Executor = exec
		cfg.PendingIdle = 50 * time.Millisecond
	})

	payload := RunJobPayload{
		TenantID:  uuid.New(),
		SessionID: uuid.New(),
		RequestID: uuid.New(),
		RunID:     uuid.New(),
		Content:   "long running",
	}
	_, _, enqueueErr := queue.Enqueue(context.Background(), payload)
	require.NoError(t, enqueueErr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for execution to start")
	}

	// Wait past PendingIdle: without the ownership refresh the entry's idle
	// time would exceed the threshold and reclaimPending would XCLAIM it to
	// a second consumer, double-executing the job.
	time.Sleep(150 * time.Millisecond)

	pending, pendingErr := client.XPendingExt(context.Background(), &redis.XPendingExtArgs{
		Stream: queue.stream,
		Group:  "bichat-run-workers",
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	require.NoError(t, pendingErr)
	require.Len(t, pending, 1)
	assert.Equal(t, "c1", pending[0].Consumer, "entry must stay owned by the executing consumer")

	close(release)
	cancel()
	<-done
}

type blockingRunExecutor struct {
	started chan struct{}
	release chan struct{}
}

func (e *blockingRunExecutor) Execute(_ context.Context, _ RunJobPayload) error {
	e.started <- struct{}{}
	<-e.release
	return nil
}

func TestRunJobWorker_RequeuesBusyJobAtFIFOHead(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	exec := newSequenceRunExecutor(fmt.Errorf("%w: %w", ErrRunBusy, assert.AnError))
	worker, queue, sessionQueue := newTestRunJobWorker(t, mr, exec, nil)

	tenantID := uuid.New()
	sessionID := uuid.New()
	payload := RunJobPayload{
		TenantID:  tenantID,
		SessionID: sessionID,
		RequestID: uuid.New(),
		RunID:     uuid.New(),
		Content:   "lost the run race",
	}
	_, _, enqueueErr := queue.Enqueue(context.Background(), payload)
	require.NoError(t, enqueueErr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	require.Eventually(t, func() bool {
		depth, lenErr := sessionQueue.Len(context.Background(), tenantID, sessionID)
		return lenErr == nil && depth == 1
	}, 5*time.Second, 5*time.Millisecond, "busy job must be parked back on the session FIFO")

	cancel()
	<-done

	// Restored at the head with its payload intact, not failed.
	requeued, ok, popErr := sessionQueue.Pop(context.Background(), tenantID, sessionID)
	require.NoError(t, popErr)
	require.True(t, ok)
	assert.Equal(t, payload.RunID, requeued.Payload.RunID)

	require.Eventually(t, func() bool {
		length, xlenErr := queue.client.XLen(context.Background(), queue.stream).Result()
		return xlenErr == nil && length == 0
	}, 5*time.Second, 5*time.Millisecond, "busy job must be acked, not retried")
}

func TestRedisActiveRunIndex_AddQueuedRuns(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	index, indexErr := NewRedisActiveRunIndex(RedisActiveRunIndexConfig{Client: client})
	require.NoError(t, indexErr)

	tenantID := uuid.New()
	sessionID := uuid.New()
	activeRunID := uuid.New()
	queuedRunID := uuid.New()

	// Seed the streaming entry the active run owns.
	require.NoError(t, index.Upsert(context.Background(), tenantID, ActiveRunStatus{
		SessionID: sessionID,
		RunID:     activeRunID,
		Status:    "streaming",
		UpdatedAt: time.Now().UTC(),
	}))

	require.NoError(t, index.AddQueuedRuns(context.Background(), tenantID, sessionID, queuedRunID, 1))
	require.NoError(t, index.AddQueuedRuns(context.Background(), tenantID, sessionID, queuedRunID, 1))

	snapshot, snapErr := index.Snapshot(context.Background(), tenantID)
	require.NoError(t, snapErr)
	require.Len(t, snapshot, 1, "queued count must not fork a second session entry")
	entry := snapshot[0]
	assert.Equal(t, activeRunID, entry.RunID, "streaming entry must keep its run id")
	assert.Equal(t, "streaming", entry.Status, "streaming entry must keep its status")
	assert.Equal(t, int64(2), entry.QueuedRuns)

	require.NoError(t, index.AddQueuedRuns(context.Background(), tenantID, sessionID, queuedRunID, -2))
	snapshot, snapErr = index.Snapshot(context.Background(), tenantID)
	require.NoError(t, snapErr)
	require.Len(t, snapshot, 1)
	assert.Equal(t, int64(0), snapshot[0].QueuedRuns)

	// A queued-only entry (no active run) is removed when it drains.
	otherSession := uuid.New()
	require.NoError(t, index.AddQueuedRuns(context.Background(), tenantID, otherSession, queuedRunID, 1))
	snapshot, snapErr = index.Snapshot(context.Background(), tenantID)
	require.NoError(t, snapErr)
	require.Len(t, snapshot, 2)
	bySession := make(map[uuid.UUID]ActiveRunStatus, len(snapshot))
	for _, s := range snapshot {
		bySession[s.SessionID] = s
	}
	assert.Equal(t, "queued", bySession[otherSession].Status)
	assert.Equal(t, queuedRunID, bySession[otherSession].RunID)

	require.NoError(t, index.AddQueuedRuns(context.Background(), tenantID, otherSession, queuedRunID, -1))
	snapshot, snapErr = index.Snapshot(context.Background(), tenantID)
	require.NoError(t, snapErr)
	require.Len(t, snapshot, 1, "drained queued-only entry must be removed")
}

// False green: a queue below capacity never exercises trimming during restoration.
func TestRedisRunSessionQueue_PushFrontRestoresAtHead(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	sessionQueue, err := NewRedisRunSessionQueue(RedisRunSessionQueueConfig{RedisURL: mr.Addr(), MaxLen: 1})
	require.NoError(t, err)

	tenantID := uuid.New()
	sessionID := uuid.New()
	first := RunJobPayload{TenantID: tenantID, SessionID: sessionID, RunID: uuid.New()}
	second := RunJobPayload{TenantID: tenantID, SessionID: sessionID, RunID: uuid.New()}

	require.NoError(t, sessionQueue.Push(context.Background(), tenantID, sessionID, QueuedRunJob{Payload: second}))

	// first was popped for promotion; a failed enqueue must restore it
	// ahead of second.
	require.NoError(t, sessionQueue.PushFront(context.Background(), tenantID, sessionID, QueuedRunJob{Payload: first}))

	head, ok, popErr := sessionQueue.Pop(context.Background(), tenantID, sessionID)
	require.NoError(t, popErr)
	require.True(t, ok)
	assert.Equal(t, first.RunID, head.Payload.RunID, "PushFront must restore the job at the head")

	next, ok, popErr := sessionQueue.Pop(context.Background(), tenantID, sessionID)
	require.NoError(t, popErr)
	require.True(t, ok)
	assert.Equal(t, second.RunID, next.Payload.RunID)
}

// False green: a successful FIFO push cannot expose premature acknowledgement;
// fail only the FIFO's Redis connection and inspect the healthy stream's PEL.
func TestRunJobWorker_BusyJobRemainsPendingWhenFIFOStorageFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	mr := miniredis.RunT(t)
	fifo := miniredis.RunT(t)
	busy := fmt.Errorf("%w: %w", ErrRunBusy, assert.AnError)
	worker, queue, sessionQueue := newTestRunJobWorker(t, mr, newSequenceRunExecutor(busy, busy), nil)
	client := redis.NewClient(&redis.Options{Addr: fifo.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	sessionQueue.client = client
	payload := RunJobPayload{TenantID: uuid.New(), SessionID: uuid.New(), RunID: uuid.New(), RequestID: uuid.New()}
	require.NoError(t, worker.ensureConsumerGroup(ctx))
	_, _, err := queue.Enqueue(ctx, payload)
	require.NoError(t, err)
	streams, err := queue.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: worker.group, Consumer: worker.consumer, Streams: []string{queue.stream, ">"}, Count: 1,
	}).Result()
	require.NoError(t, err)
	require.Len(t, streams, 1)
	require.Len(t, streams[0].Messages, 1)
	message := streams[0].Messages[0]
	fifo.SetError("ERR FIFO storage unavailable")
	require.Error(t, worker.processMessage(ctx, message))
	pending, err := queue.client.XPending(ctx, queue.stream, worker.group).Result()
	require.NoError(t, err)
	require.Equal(t, int64(1), pending.Count)
	fifo.SetError("")
	require.NoError(t, worker.processMessage(ctx, message))
	head, ok, err := sessionQueue.Pop(ctx, payload.TenantID, payload.SessionID)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, payload.RunID, head.Payload.RunID)
	pending, err = queue.client.XPending(ctx, queue.stream, worker.group).Result()
	require.NoError(t, err)
	require.Zero(t, pending.Count)
}
