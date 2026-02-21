package services

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sequenceTitleService struct {
	mu    sync.Mutex
	errs  []error
	calls chan uuid.UUID
}

func newSequenceTitleService(errs ...error) *sequenceTitleService {
	return &sequenceTitleService{
		errs:  errs,
		calls: make(chan uuid.UUID, 32),
	}
}

func (s *sequenceTitleService) GenerateSessionTitle(_ context.Context, sessionID uuid.UUID) error {
	s.calls <- sessionID

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.errs) == 0 {
		return nil
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	return err
}

func (s *sequenceTitleService) RegenerateSessionTitle(ctx context.Context, sessionID uuid.UUID) error {
	return s.GenerateSessionTitle(ctx, sessionID)
}

func TestRedisTitleJobQueue_EnqueueIsDeduplicated(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisTitleJobQueue(RedisTitleJobQueueConfig{
		RedisURL:  mr.Addr(),
		Stream:    "bichat:title:test",
		DedupeTTL: 5 * time.Minute,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	tenantID := uuid.New()
	sessionID := uuid.New()

	require.NoError(t, queue.Enqueue(context.Background(), tenantID, sessionID))
	require.NoError(t, queue.Enqueue(context.Background(), tenantID, sessionID))

	length, xlenErr := queue.client.XLen(context.Background(), queue.stream).Result()
	require.NoError(t, xlenErr)
	assert.Equal(t, int64(1), length)
}

func TestTitleJobWorker_RetriesAndEventuallySucceeds(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisTitleJobQueue(RedisTitleJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:title:retry",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	svc := newSequenceTitleService(assert.AnError, nil)
	worker, err := NewTitleJobWorker(TitleJobWorkerConfig{
		Queue:          queue,
		TitleService:   svc,
		Group:          "g1",
		Consumer:       "c1",
		PollInterval:   5 * time.Millisecond,
		ReadBlock:      5 * time.Millisecond,
		MaxRetries:     3,
		RetryBaseDelay: 5 * time.Millisecond,
		RetryMaxDelay:  20 * time.Millisecond,
		PendingIdle:    5 * time.Millisecond,
	})
	require.NoError(t, err)

	tenantID := uuid.New()
	sessionID := uuid.New()
	require.NoError(t, queue.Enqueue(context.Background(), tenantID, sessionID))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	waitForCallCount(t, svc, 2, 2*time.Second)
	cancel()
	<-done
}

func TestTitleJobWorker_ReclaimsStalePendingEntries(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	stream := "bichat:title:reclaim"
	group := "g2"
	require.NoError(t, client.XGroupCreateMkStream(context.Background(), stream, group, "$").Err())

	queue, err := NewRedisTitleJobQueue(RedisTitleJobQueueConfig{
		Client: client,
		Stream: stream,
	})
	require.NoError(t, err)

	tenantID := uuid.New()
	sessionID := uuid.New()
	require.NoError(t, queue.Enqueue(context.Background(), tenantID, sessionID))

	_, readErr := client.XReadGroup(context.Background(), &redis.XReadGroupArgs{
		Group:    group,
		Consumer: "other-consumer",
		Streams:  []string{stream, ">"},
		Count:    1,
		Block:    0,
	}).Result()
	require.NoError(t, readErr)

	svc := newSequenceTitleService(nil)
	worker, err := NewTitleJobWorker(TitleJobWorkerConfig{
		Queue:        queue,
		TitleService: svc,
		Group:        group,
		Consumer:     "worker-consumer",
		PollInterval: 5 * time.Millisecond,
		ReadBlock:    5 * time.Millisecond,
		PendingIdle:  1 * time.Millisecond,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	waitForCallCount(t, svc, 1, 2*time.Second)
	cancel()
	<-done
}

func TestTitleJobWorker_ReconciliationEnqueuesMissingSessions(t *testing.T) {
	t.Parallel()

	mr := miniredis.RunT(t)
	queue, err := NewRedisTitleJobQueue(RedisTitleJobQueueConfig{
		RedisURL: mr.Addr(),
		Stream:   "bichat:title:reconcile",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Close() })

	tenantID := uuid.New()
	sessionID := uuid.New()

	var once sync.Once
	fetcher := func(_ context.Context, _ int) ([]titleJobPayload, error) {
		payloads := []titleJobPayload{}
		once.Do(func() {
			payloads = append(payloads, titleJobPayload{
				TenantID:  tenantID,
				SessionID: sessionID,
				Attempt:   0,
			})
		})
		return payloads, nil
	}

	svc := newSequenceTitleService(nil)
	worker, err := NewTitleJobWorker(TitleJobWorkerConfig{
		Queue:          queue,
		TitleService:   svc,
		Group:          "g3",
		Consumer:       "c3",
		PollInterval:   5 * time.Millisecond,
		ReadBlock:      5 * time.Millisecond,
		ReconcileEvery: 5 * time.Millisecond,
		ReconcileBatch: 10,
		FetchMissingFn: fetcher,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = worker.Start(ctx)
	}()

	waitForCallCount(t, svc, 1, 2*time.Second)
	cancel()
	<-done
}

func waitForCallCount(t *testing.T, svc *sequenceTitleService, count int, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)
	received := 0
	for received < count {
		select {
		case <-svc.calls:
			received++
		case <-deadline:
			t.Fatalf("timed out waiting for %d title generation calls, received %d", count, received)
		}
	}
}

func TestParseRetryPayload_Invalid(t *testing.T) {
	t.Parallel()

	_, err := parseRetryPayload("invalid")
	require.Error(t, err)
}
