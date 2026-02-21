package services

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

const (
	defaultTitleQueueGroup          = "bichat-title-workers"
	defaultTitleQueueBatchSize      = 16
	defaultTitleQueuePollInterval   = 300 * time.Millisecond
	defaultTitleQueueReadBlock      = 2 * time.Second
	defaultTitleQueueMaxRetries     = 3
	defaultTitleQueueRetryBaseDelay = 5 * time.Second
	defaultTitleQueueRetryMaxDelay  = 2 * time.Minute
	defaultTitleQueuePendingIdle    = 30 * time.Second
	defaultTitleQueueReconcileEvery = 1 * time.Minute
	defaultTitleQueueReconcileBatch = 200
	defaultTitleQueueJobTimeout     = 20 * time.Second
	defaultTitleQueueRetryKeySuffix = ":retry"
	defaultTitleQueueConsumerPrefix = "consumer"
)

type titleJobPayload struct {
	TenantID  uuid.UUID
	SessionID uuid.UUID
	Attempt   int
}

type MissingSessionsFetcher func(ctx context.Context, limit int) ([]titleJobPayload, error)

type TitleJobWorkerConfig struct {
	Queue          *RedisTitleJobQueue
	TitleService   TitleService
	Pool           *pgxpool.Pool
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
	ReconcileEvery time.Duration
	ReconcileBatch int
	JobTimeout     time.Duration
	RetrySchedule  string
	FetchMissingFn MissingSessionsFetcher
}

// TitleJobWorker consumes Redis title jobs with retry + reconciliation.
type TitleJobWorker struct {
	queue          *RedisTitleJobQueue
	titleService   TitleService
	pool           *pgxpool.Pool
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
	reconcileEvery time.Duration
	reconcileBatch int
	jobTimeout     time.Duration
	retrySchedule  string
	fetchMissing   MissingSessionsFetcher
	now            func() time.Time
}

func NewTitleJobWorker(cfg TitleJobWorkerConfig) (*TitleJobWorker, error) {
	if cfg.Queue == nil {
		return nil, fmt.Errorf("queue is required")
	}
	if cfg.TitleService == nil {
		return nil, fmt.Errorf("title service is required")
	}

	group := strings.TrimSpace(cfg.Group)
	if group == "" {
		group = defaultTitleQueueGroup
	}

	consumer := strings.TrimSpace(cfg.Consumer)
	if consumer == "" {
		consumer = fmt.Sprintf("%s-%d", defaultTitleQueueConsumerPrefix, time.Now().UnixNano())
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 {
		batchSize = defaultTitleQueueBatchSize
	}

	pollEvery := cfg.PollInterval
	if pollEvery <= 0 {
		pollEvery = defaultTitleQueuePollInterval
	}

	readBlock := cfg.ReadBlock
	if readBlock <= 0 {
		readBlock = defaultTitleQueueReadBlock
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultTitleQueueMaxRetries
	}

	retryBaseDelay := cfg.RetryBaseDelay
	if retryBaseDelay <= 0 {
		retryBaseDelay = defaultTitleQueueRetryBaseDelay
	}

	retryMaxDelay := cfg.RetryMaxDelay
	if retryMaxDelay <= 0 {
		retryMaxDelay = defaultTitleQueueRetryMaxDelay
	}

	pendingIdle := cfg.PendingIdle
	if pendingIdle <= 0 {
		pendingIdle = defaultTitleQueuePendingIdle
	}

	reconcileEvery := cfg.ReconcileEvery
	if reconcileEvery <= 0 {
		reconcileEvery = defaultTitleQueueReconcileEvery
	}

	reconcileBatch := cfg.ReconcileBatch
	if reconcileBatch <= 0 {
		reconcileBatch = defaultTitleQueueReconcileBatch
	}

	jobTimeout := cfg.JobTimeout
	if jobTimeout <= 0 {
		jobTimeout = defaultTitleQueueJobTimeout
	}

	retrySchedule := strings.TrimSpace(cfg.RetrySchedule)
	if retrySchedule == "" {
		retrySchedule = cfg.Queue.stream + defaultTitleQueueRetryKeySuffix
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	fetchMissing := cfg.FetchMissingFn
	if fetchMissing == nil && cfg.Pool != nil {
		fetchMissing = defaultMissingSessionsFetcher(cfg.Pool)
	}

	return &TitleJobWorker{
		queue:          cfg.Queue,
		titleService:   cfg.TitleService,
		pool:           cfg.Pool,
		logger:         logger,
		group:          group,
		consumer:       consumer,
		batchSize:      batchSize,
		pollEvery:      pollEvery,
		readBlock:      readBlock,
		maxRetries:     maxRetries,
		retryBaseDelay: retryBaseDelay,
		retryMaxDelay:  retryMaxDelay,
		pendingIdle:    pendingIdle,
		reconcileEvery: reconcileEvery,
		reconcileBatch: reconcileBatch,
		jobTimeout:     jobTimeout,
		retrySchedule:  retrySchedule,
		fetchMissing:   fetchMissing,
		now:            time.Now,
	}, nil
}

func (w *TitleJobWorker) Start(ctx context.Context) error {
	if err := w.ensureConsumerGroup(ctx); err != nil {
		return err
	}

	nextReconcileAt := time.Time{}
	if w.fetchMissing != nil && w.reconcileEvery > 0 {
		nextReconcileAt = w.now()
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if err := w.promoteRetries(ctx); err != nil {
			w.logger.WithError(err).Warn("title worker failed to promote retries")
		}

		if err := w.reclaimPending(ctx); err != nil {
			w.logger.WithError(err).Warn("title worker failed to reclaim pending entries")
		}

		if !nextReconcileAt.IsZero() && !w.now().Before(nextReconcileAt) {
			if err := w.reconcileMissingTitles(ctx); err != nil {
				w.logger.WithError(err).Warn("title worker reconciliation failed")
			}
			nextReconcileAt = w.now().Add(w.reconcileEvery)
		}

		if err := w.consume(ctx); err != nil {
			w.logger.WithError(err).Warn("title worker consume failed")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.pollEvery):
		}
	}
}

func (w *TitleJobWorker) ensureConsumerGroup(ctx context.Context) error {
	err := w.queue.client.XGroupCreateMkStream(ctx, w.queue.stream, w.group, "0").Err()
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("create consumer group: %w", err)
}

func (w *TitleJobWorker) consume(ctx context.Context) error {
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
					Warn("title worker failed to process message")
			}
		}
	}

	return nil
}

func (w *TitleJobWorker) reclaimPending(ctx context.Context) error {
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
				Warn("title worker failed to claim stale pending message")
			continue
		}
		for _, msg := range claimed {
			if procErr := w.processMessage(ctx, msg); procErr != nil {
				w.logger.WithError(procErr).
					WithField("message_id", msg.ID).
					Warn("title worker failed processing reclaimed message")
			}
		}
	}

	return nil
}

func (w *TitleJobWorker) processMessage(ctx context.Context, msg redis.XMessage) error {
	payload, err := parseTitleJobPayload(msg.Values)
	if err != nil {
		w.ack(ctx, msg.ID)
		return fmt.Errorf("invalid title job payload: %w", err)
	}

	jobCtx := context.Background()
	if w.pool != nil {
		jobCtx = composables.WithPool(jobCtx, w.pool)
	}
	jobCtx = composables.WithTenantID(jobCtx, payload.TenantID)
	jobCtx, cancel := context.WithTimeout(jobCtx, w.jobTimeout)
	defer cancel()

	if runErr := w.titleService.GenerateSessionTitle(jobCtx, payload.SessionID); runErr == nil {
		w.ack(ctx, msg.ID)
		_, _ = w.queue.client.Del(ctx, w.queue.dedupeKey(payload.TenantID, payload.SessionID)).Result()
		return nil
	}

	nextAttempt := payload.Attempt + 1
	if nextAttempt < w.maxRetries {
		if scheduleErr := w.scheduleRetry(ctx, titleJobPayload{
			TenantID:  payload.TenantID,
			SessionID: payload.SessionID,
			Attempt:   nextAttempt,
		}); scheduleErr != nil {
			return scheduleErr
		}
		w.ack(ctx, msg.ID)
		return fmt.Errorf("title generation failed, scheduled retry attempt=%d", nextAttempt)
	}

	w.ack(ctx, msg.ID)
	_, _ = w.queue.client.Del(ctx, w.queue.dedupeKey(payload.TenantID, payload.SessionID)).Result()
	return fmt.Errorf("title generation failed after max retries")
}

func (w *TitleJobWorker) scheduleRetry(ctx context.Context, payload titleJobPayload) error {
	delay := w.retryDelay(payload.Attempt)
	member := serializeRetryPayload(payload)
	score := float64(w.now().Add(delay).UnixNano())

	if err := w.queue.client.ZAdd(ctx, w.retrySchedule, redis.Z{
		Score:  score,
		Member: member,
	}).Err(); err != nil {
		return fmt.Errorf("schedule retry: %w", err)
	}

	return nil
}

func (w *TitleJobWorker) promoteRetries(ctx context.Context) error {
	nowScore := strconv.FormatFloat(float64(w.now().UnixNano()), 'f', -1, 64)
	members, err := w.queue.client.ZRangeByScore(ctx, w.retrySchedule, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   nowScore,
		Count: int64(w.batchSize),
	}).Result()
	if err != nil {
		return fmt.Errorf("read retry schedule: %w", err)
	}

	for _, member := range members {
		payload, parseErr := parseRetryPayload(member)
		if parseErr != nil {
			_, _ = w.queue.client.ZRem(ctx, w.retrySchedule, member).Result()
			continue
		}

		_, addErr := w.queue.client.XAdd(ctx, &redis.XAddArgs{
			Stream: w.queue.stream,
			Values: map[string]any{
				"tenant_id":   payload.TenantID.String(),
				"session_id":  payload.SessionID.String(),
				"attempt":     strconv.Itoa(payload.Attempt),
				"enqueued_at": w.now().UTC().Format(time.RFC3339Nano),
			},
		}).Result()
		if addErr != nil {
			return fmt.Errorf("promote retry to stream: %w", addErr)
		}

		_, _ = w.queue.client.ZRem(ctx, w.retrySchedule, member).Result()
	}

	return nil
}

func (w *TitleJobWorker) reconcileMissingTitles(ctx context.Context) error {
	if w.fetchMissing == nil {
		return nil
	}

	payloads, err := w.fetchMissing(ctx, w.reconcileBatch)
	if err != nil {
		return err
	}

	for _, p := range payloads {
		if enqueueErr := w.queue.Enqueue(ctx, p.TenantID, p.SessionID); enqueueErr != nil {
			w.logger.WithError(enqueueErr).
				WithField("tenant_id", p.TenantID.String()).
				WithField("session_id", p.SessionID.String()).
				Warn("failed to enqueue missing-title session during reconciliation")
		}
	}

	return nil
}

func (w *TitleJobWorker) ack(ctx context.Context, msgID string) {
	if _, err := w.queue.client.XAck(ctx, w.queue.stream, w.group, msgID).Result(); err != nil {
		w.logger.WithError(err).WithField("message_id", msgID).Warn("failed to ack title queue message")
	}
	_, _ = w.queue.client.XDel(ctx, w.queue.stream, msgID).Result()
}

func (w *TitleJobWorker) retryDelay(attempt int) time.Duration {
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

func defaultMissingSessionsFetcher(pool *pgxpool.Pool) MissingSessionsFetcher {
	return func(ctx context.Context, limit int) ([]titleJobPayload, error) {
		// Intentionally scans all tenants: reconciliation is a global background sweep.
		rows, err := pool.Query(ctx, `
			SELECT tenant_id, id
			FROM bichat.sessions
			WHERE btrim(title) = ''
			LIMIT $1
		`, limit)
		if err != nil {
			return nil, fmt.Errorf("query sessions with missing title: %w", err)
		}
		defer rows.Close()

		result := make([]titleJobPayload, 0, limit)
		for rows.Next() {
			var tenantID uuid.UUID
			var sessionID uuid.UUID
			if scanErr := rows.Scan(&tenantID, &sessionID); scanErr != nil {
				return nil, fmt.Errorf("scan missing-title session: %w", scanErr)
			}
			result = append(result, titleJobPayload{
				TenantID:  tenantID,
				SessionID: sessionID,
				Attempt:   0,
			})
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return nil, fmt.Errorf("iterate missing-title sessions: %w", rowsErr)
		}

		return result, nil
	}
}

func parseTitleJobPayload(values map[string]any) (titleJobPayload, error) {
	tenantRaw, ok := values["tenant_id"]
	if !ok {
		return titleJobPayload{}, fmt.Errorf("tenant_id is required")
	}
	sessionRaw, ok := values["session_id"]
	if !ok {
		return titleJobPayload{}, fmt.Errorf("session_id is required")
	}

	tenantID, err := uuid.Parse(fmt.Sprint(tenantRaw))
	if err != nil {
		return titleJobPayload{}, fmt.Errorf("parse tenant_id: %w", err)
	}
	sessionID, err := uuid.Parse(fmt.Sprint(sessionRaw))
	if err != nil {
		return titleJobPayload{}, fmt.Errorf("parse session_id: %w", err)
	}

	attempt := 0
	if attemptRaw, ok := values["attempt"]; ok {
		if parsed, parseErr := strconv.Atoi(strings.TrimSpace(fmt.Sprint(attemptRaw))); parseErr == nil {
			attempt = parsed
		}
	}

	return titleJobPayload{
		TenantID:  tenantID,
		SessionID: sessionID,
		Attempt:   attempt,
	}, nil
}

func serializeRetryPayload(payload titleJobPayload) string {
	return fmt.Sprintf("%s|%s|%d", payload.TenantID.String(), payload.SessionID.String(), payload.Attempt)
}

func parseRetryPayload(value string) (titleJobPayload, error) {
	parts := strings.Split(value, "|")
	if len(parts) != 3 {
		return titleJobPayload{}, fmt.Errorf("invalid retry payload")
	}

	tenantID, err := uuid.Parse(parts[0])
	if err != nil {
		return titleJobPayload{}, fmt.Errorf("parse tenant id: %w", err)
	}
	sessionID, err := uuid.Parse(parts[1])
	if err != nil {
		return titleJobPayload{}, fmt.Errorf("parse session id: %w", err)
	}
	attempt, err := strconv.Atoi(parts[2])
	if err != nil {
		return titleJobPayload{}, fmt.Errorf("parse attempt: %w", err)
	}

	return titleJobPayload{
		TenantID:  tenantID,
		SessionID: sessionID,
		Attempt:   attempt,
	}, nil
}
