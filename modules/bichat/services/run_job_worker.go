// Package services provides this package.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	defaultRunQueueGroup        = "bichat-run-workers"
	defaultRunQueueBatchSize    = 8
	defaultRunQueuePollInterval = 300 * time.Millisecond
	defaultRunQueueReadBlock    = 2 * time.Second
	// MaxRetries counts total delivery attempts per job. The default of 2
	// means exactly one retry: generation and tool failures never reach the
	// worker as errors (Execute drives them to a terminal run itself), so
	// what comes back as an error is infrastructure — worth a single retry,
	// then terminal failure.
	defaultRunQueueMaxRetries     = 2
	defaultRunQueueRetryBaseDelay = 5 * time.Second
	defaultRunQueueRetryMaxDelay  = 2 * time.Minute
	defaultRunQueuePendingIdle    = 30 * time.Second
	// Generation runs can last many minutes (deep mode, big tool loops).
	// The job timeout only guards against a wedged executor; run liveness
	// against the reaper is maintained by the executor's own heartbeats.
	defaultRunQueueJobTimeout     = 30 * time.Minute
	defaultRunQueueRetryKeySuffix = ":retry"
	defaultRunQueueConsumerPrefix = "run-consumer"
)

// RunJobWorkerConfig configures the Redis run worker.
type RunJobWorkerConfig struct {
	Queue    *RedisRunJobQueue
	Executor RunExecutor
	// SessionQueue enables FIFO promotion when a job is lost to terminal
	// infrastructure failure (the executor's own terminal path promotes on
	// every normal completion) and re-queues jobs that lost a run conflict.
	// Optional.
	SessionQueue *RedisRunSessionQueue
	// ActiveRunIndex keeps the sidebar's queued-run count in sync with
	// promotions driven by this worker. Optional.
	ActiveRunIndex ActiveRunIndex
	// OnJobTerminalFailure is invoked when a job exhausts its retries on
	// infrastructure errors. Wired to chatServiceImpl.FailStalledRun so the
	// run reaches a terminal state and the client's cursor reader sees an
	// error event instead of hanging forever. Optional.
	OnJobTerminalFailure func(ctx context.Context, job RunJobPayload, cause error)
	Pool                 *pgxpool.Pool
	// Users hydrates the authenticated actor for tools and checkpoint persistence.
	// Optional for custom executors that do not use authenticated context;
	// ServiceContainer.NewRunJobWorker always supplies the tenant-scoped repository.
	Users interface {
		GetByID(context.Context, uint) (user.User, error)
	}
	Logger         *logrus.Logger
	Group          string
	Consumer       string
	BatchSize      int
	PollInterval   time.Duration
	ReadBlock      time.Duration
	MaxRetries     int
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	PendingIdle    time.Duration
	JobTimeout     time.Duration
}

// RunJobWorker consumes generation jobs off the bichat:run:jobs stream with
// the same delivery guarantees as TitleJobWorker: consumer-group fan-out,
// XPENDING/XCLAIM crash handoff, and bounded retries for transient
// infrastructure faults. Generation and tool failures are terminal by
// contract — RunExecutor.Execute drives those runs to a terminal state
// itself and returns nil.
type RunJobWorker struct {
	queue                *RedisRunJobQueue
	executor             RunExecutor
	sessionQueue         *RedisRunSessionQueue
	activeRunIndex       ActiveRunIndex
	onJobTerminalFailure func(ctx context.Context, job RunJobPayload, cause error)
	pool                 *pgxpool.Pool
	users                interface {
		GetByID(context.Context, uint) (user.User, error)
	}
	logger         *logrus.Logger
	group          string
	consumer       string
	batchSize      int
	pollEvery      time.Duration
	readBlock      time.Duration
	maxRetries     int
	retryBaseDelay time.Duration
	retryMaxDelay  time.Duration
	pendingIdle    time.Duration
	jobTimeout     time.Duration
	retrySchedule  string
	now            func() time.Time
}

func NewRunJobWorker(cfg RunJobWorkerConfig) (*RunJobWorker, error) {
	if cfg.Queue == nil {
		return nil, fmt.Errorf("queue is required")
	}
	if cfg.Executor == nil {
		return nil, fmt.Errorf("executor is required")
	}

	group := strings.TrimSpace(cfg.Group)
	if group == "" {
		group = defaultRunQueueGroup
	}

	consumer := strings.TrimSpace(cfg.Consumer)
	if consumer == "" {
		consumer = fmt.Sprintf("%s-%d", defaultRunQueueConsumerPrefix, time.Now().UnixNano())
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultRunQueueBatchSize
	}

	pollEvery := cfg.PollInterval
	if pollEvery <= 0 {
		pollEvery = defaultRunQueuePollInterval
	}

	readBlock := cfg.ReadBlock
	if readBlock <= 0 {
		readBlock = defaultRunQueueReadBlock
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultRunQueueMaxRetries
	}

	retryBaseDelay := cfg.RetryBaseDelay
	if retryBaseDelay <= 0 {
		retryBaseDelay = defaultRunQueueRetryBaseDelay
	}

	retryMaxDelay := cfg.RetryMaxDelay
	if retryMaxDelay <= 0 {
		retryMaxDelay = defaultRunQueueRetryMaxDelay
	}

	pendingIdle := cfg.PendingIdle
	if pendingIdle <= 0 {
		pendingIdle = defaultRunQueuePendingIdle
	}

	jobTimeout := cfg.JobTimeout
	if jobTimeout <= 0 {
		jobTimeout = defaultRunQueueJobTimeout
	}

	retrySchedule := cfg.Queue.stream + defaultRunQueueRetryKeySuffix

	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	return &RunJobWorker{
		queue:                cfg.Queue,
		executor:             cfg.Executor,
		sessionQueue:         cfg.SessionQueue,
		activeRunIndex:       cfg.ActiveRunIndex,
		onJobTerminalFailure: cfg.OnJobTerminalFailure,
		pool:                 cfg.Pool,
		users:                cfg.Users,
		logger:               logger,
		group:                group,
		consumer:             consumer,
		batchSize:            batchSize,
		pollEvery:            pollEvery,
		readBlock:            readBlock,
		maxRetries:           maxRetries,
		retryBaseDelay:       retryBaseDelay,
		retryMaxDelay:        retryMaxDelay,
		pendingIdle:          pendingIdle,
		jobTimeout:           jobTimeout,
		retrySchedule:        retrySchedule,
		now:                  time.Now,
	}, nil
}

// Start runs the consume loop until ctx is cancelled.
func (w *RunJobWorker) Start(ctx context.Context) error {
	if err := w.ensureConsumerGroup(ctx); err != nil {
		return err
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := w.promoteRetries(ctx); err != nil {
			w.logger.WithError(err).Warn("run worker failed to promote retries")
		}

		if err := w.reclaimPending(ctx); err != nil {
			w.logger.WithError(err).Warn("run worker failed to reclaim pending entries")
		}

		if err := w.consume(ctx); err != nil {
			w.logger.WithError(err).Warn("run worker consume failed")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.pollEvery):
		}
	}
}

func (w *RunJobWorker) ensureConsumerGroup(ctx context.Context) error {
	err := w.queue.client.XGroupCreateMkStream(ctx, w.queue.stream, w.group, "0").Err()
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("create consumer group: %w", err)
}

func (w *RunJobWorker) consume(ctx context.Context) error {
	streams, err := w.queue.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    w.group,
		Consumer: w.consumer,
		Streams:  []string{w.queue.stream, ">"},
		Count:    int64(w.batchSize),
		Block:    w.readBlock,
	}).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("xreadgroup: %w", err)
	}

	for _, stream := range streams {
		for _, msg := range stream.Messages {
			if procErr := w.processMessage(ctx, msg); procErr != nil {
				w.logger.WithError(procErr).
					WithField("message_id", msg.ID).
					Warn("run worker failed to process message")
			}
		}
	}

	return nil
}

// reclaimPending hands off jobs whose worker crashed mid-run: entries idle
// in the consumer group's PEL longer than PendingIdle are claimed and
// re-executed here.
func (w *RunJobWorker) reclaimPending(ctx context.Context) error {
	pending, err := w.queue.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: w.queue.stream,
		Group:  w.group,
		Idle:   w.pendingIdle,
		Start:  "-",
		End:    "+",
		Count:  int64(w.batchSize),
	}).Result()
	if err != nil {
		return fmt.Errorf("xpendingext: %w", err)
	}

	for _, p := range pending {
		claimed, claimErr := w.queue.client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   w.queue.stream,
			Group:    w.group,
			Consumer: w.consumer,
			MinIdle:  w.pendingIdle,
			Messages: []string{p.ID},
		}).Result()
		if claimErr != nil {
			w.logger.WithError(claimErr).
				WithField("message_id", p.ID).
				Warn("run worker failed to claim stale pending message")
			continue
		}
		for _, msg := range claimed {
			if procErr := w.processMessage(ctx, msg); procErr != nil {
				w.logger.WithError(procErr).
					WithField("message_id", msg.ID).
					Warn("run worker failed processing reclaimed message")
			}
		}
	}

	return nil
}

func (w *RunJobWorker) processMessage(ctx context.Context, msg redis.XMessage) error {
	payload, err := ParseRunJobPayload(msg.Values)
	if err != nil {
		// A payload that cannot be parsed will never succeed — ack it so
		// the PEL does not grow forever.
		w.ack(ctx, msg.ID)
		return fmt.Errorf("invalid run job payload: %w", err)
	}

	jobCtx := context.Background()
	if w.pool != nil {
		jobCtx = composables.WithPool(jobCtx, w.pool)
	}
	jobCtx = composables.WithTenantID(jobCtx, payload.TenantID)
	jobCtx, cancel := context.WithTimeout(jobCtx, w.jobTimeout)
	defer cancel()

	// Terminal bookkeeping (ack, request release, FIFO promotion) must
	// survive loop shutdown: a cancelled ack re-delivers an already-
	// executed job, so these run on a non-cancelable context.
	bookkeepingCtx := context.WithoutCancel(ctx)

	// A generation can run for many minutes — far past PendingIdle. Keep
	// refreshing the entry's ownership with a JUSTID claim so reclaimPending
	// on another replica cannot XCLAIM and double-execute a live job. The
	// refresh dies with this call; if the worker crashes, idle time grows
	// again and the job is legitimately reclaimed.
	refreshStop := make(chan struct{})
	defer close(refreshStop)
	go w.refreshOwnership(ctx, msg.ID, refreshStop)

	var execErr error
	if w.users != nil {
		if payload.UserID <= 0 {
			execErr = fmt.Errorf("run job actor is required")
		} else {
			actor, err := w.users.GetByID(jobCtx, uint(payload.UserID))
			if err != nil {
				execErr = fmt.Errorf("load run job actor: %w", err)
			} else if actor == nil || int64(actor.ID()) != payload.UserID || actor.TenantID() != payload.TenantID {
				execErr = fmt.Errorf("run job actor does not match tenant and user")
			} else {
				jobCtx = composables.WithUser(jobCtx, actor)
			}
		}
	}
	if execErr == nil {
		execErr = w.executor.Execute(jobCtx, payload)
	}
	if execErr == nil {
		// Execute returning nil means the run reached a terminal state
		// (completed, cancelled, or failed-with-broadcast — all owned by
		// the executor). Ack + release the request claim so a retry with
		// the same request id starts fresh.
		w.ack(bookkeepingCtx, msg.ID)
		w.releaseRequest(bookkeepingCtx, payload)
		return nil
	}

	if errors.Is(execErr, ErrRunBusy) {
		// A newer send won the session between this job's promotion and its
		// execution. Park the job back at the head of the FIFO; the new
		// run's terminal transition re-promotes it. Not a failure: no
		// retry counter, no terminal event.
		if w.sessionQueue == nil {
			w.ack(bookkeepingCtx, msg.ID)
			return fmt.Errorf("run job busy but session queue is unconfigured run_id=%s: %w", payload.RunID, execErr)
		}
		if pushErr := w.sessionQueue.PushFront(bookkeepingCtx, payload.TenantID, payload.SessionID, QueuedRunJob{Payload: payload}); pushErr != nil {
			// No ack: the entry stays in the PEL and is reclaimed for a
			// later attempt instead of being lost.
			return fmt.Errorf("requeue busy run job run_id=%s: %w", payload.RunID, pushErr)
		}
		w.ack(bookkeepingCtx, msg.ID)
		if w.activeRunIndex != nil {
			_ = w.activeRunIndex.AddQueuedRuns(bookkeepingCtx, payload.TenantID, payload.SessionID, payload.RunID, 1)
		}
		return nil
	}

	// Infrastructure error (executor assembly failed: DB down, Redis blip,
	// session gone). Retry once, then fail the run terminally.
	nextAttempt := payload.Attempt + 1
	if nextAttempt < w.maxRetries {
		if scheduleErr := w.scheduleRetry(bookkeepingCtx, payload, nextAttempt); scheduleErr != nil {
			return scheduleErr
		}
		w.ack(bookkeepingCtx, msg.ID)
		return fmt.Errorf("run job failed, scheduled retry attempt=%d run_id=%s", nextAttempt, payload.RunID)
	}

	w.ack(bookkeepingCtx, msg.ID)
	w.releaseRequest(bookkeepingCtx, payload)
	w.failTerminally(bookkeepingCtx, payload)
	return fmt.Errorf("run job failed after max retries run_id=%s", payload.RunID)
}

// refreshOwnership keeps the stream entry in this consumer's name while the
// generation runs, so its PEL idle time never crosses PendingIdle.
func (w *RunJobWorker) refreshOwnership(ctx context.Context, msgID string, stop <-chan struct{}) {
	interval := w.pendingIdle / 2
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// XCLAIM JUSTID resets the entry's idle time without delivering
			// or incrementing its delivery counter.
			_, err := w.queue.client.XClaimJustID(ctx, &redis.XClaimArgs{
				Stream:   w.queue.stream,
				Group:    w.group,
				Consumer: w.consumer,
				MinIdle:  0,
				Messages: []string{msgID},
			}).Result()
			if err != nil && ctx.Err() != nil {
				return
			}
		}
	}
}

// failTerminally surfaces an unrecoverable infrastructure failure: the run
// transitions to failed (hook), and any job parked behind it on the session
// FIFO is promoted so the queue does not deadlock behind a lost run.
func (w *RunJobWorker) failTerminally(ctx context.Context, payload RunJobPayload) {
	if w.onJobTerminalFailure != nil {
		w.onJobTerminalFailure(ctx, payload, errors.New("run job failed after max retries"))
	}
	if w.sessionQueue != nil {
		if _, err := PromoteNextQueuedRun(ctx, w.sessionQueue, w.queue, w.activeRunIndex, payload.TenantID, payload.SessionID); err != nil {
			w.logger.WithError(err).
				WithField("session_id", payload.SessionID.String()).
				Warn("run worker failed to promote queued run after terminal failure")
		}
	}
}

func (w *RunJobWorker) releaseRequest(ctx context.Context, payload RunJobPayload) {
	if payload.RequestID == uuid.Nil {
		return
	}
	_ = w.queue.ReleaseRequest(ctx, payload.TenantID, payload.RequestID)
}

func (w *RunJobWorker) scheduleRetry(ctx context.Context, payload RunJobPayload, attempt int) error {
	delay := w.retryDelay(attempt)
	payload.Attempt = attempt
	payload.EnqueuedAt = w.now().UTC()

	body, err := marshalRunRetryPayload(payload)
	if err != nil {
		return fmt.Errorf("marshal run retry payload: %w", err)
	}

	if err := w.queue.client.ZAdd(ctx, w.retrySchedule, redis.Z{
		Score:  float64(w.now().Add(delay).UnixNano()),
		Member: body,
	}).Err(); err != nil {
		return fmt.Errorf("schedule run retry: %w", err)
	}

	return nil
}

func (w *RunJobWorker) promoteRetries(ctx context.Context) error {
	nowScore := strconv.FormatFloat(float64(w.now().UnixNano()), 'f', -1, 64)
	members, err := w.queue.client.ZRangeByScore(ctx, w.retrySchedule, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   nowScore,
		Count: int64(w.batchSize),
	}).Result()
	if err != nil {
		return fmt.Errorf("read run retry schedule: %w", err)
	}

	for _, member := range members {
		payload, parseErr := unmarshalRunRetryPayload(member)
		if parseErr != nil {
			_, _ = w.queue.client.ZRem(ctx, w.retrySchedule, member).Result()
			continue
		}

		// ZRem is the claim: exactly one replica can remove the member, so
		// only one enqueues it. On enqueue failure the member goes back on
		// the schedule (due now) so the retry is not lost.
		removed, remErr := w.queue.client.ZRem(ctx, w.retrySchedule, member).Result()
		if remErr != nil || removed == 0 {
			continue
		}
		if _, addErr := w.queue.EnqueueClaimed(ctx, payload); addErr != nil {
			_ = w.queue.client.ZAdd(ctx, w.retrySchedule, redis.Z{
				Score:  float64(w.now().UnixNano()),
				Member: member,
			}).Err()
			return fmt.Errorf("promote run retry to stream: %w", addErr)
		}
	}

	return nil
}

func (w *RunJobWorker) ack(ctx context.Context, msgID string) {
	if _, err := w.queue.client.XAck(ctx, w.queue.stream, w.group, msgID).Result(); err != nil {
		w.logger.WithError(err).WithField("message_id", msgID).Warn("failed to ack run queue message")
	}
	_, _ = w.queue.client.XDel(ctx, w.queue.stream, msgID).Result()
}

func (w *RunJobWorker) retryDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return w.retryBaseDelay
	}

	multiplier := math.Pow(2, float64(attempt-1))
	delay := time.Duration(float64(w.retryBaseDelay) * multiplier)
	if delay > w.retryMaxDelay {
		return w.retryMaxDelay
	}
	return delay
}

func marshalRunRetryPayload(payload RunJobPayload) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func unmarshalRunRetryPayload(member string) (RunJobPayload, error) {
	var payload RunJobPayload
	if err := json.Unmarshal([]byte(member), &payload); err != nil {
		return RunJobPayload{}, err
	}
	if payload.TenantID == uuid.Nil || payload.SessionID == uuid.Nil || payload.RunID == uuid.Nil {
		return RunJobPayload{}, fmt.Errorf("retry payload missing identity fields")
	}
	return payload, nil
}
