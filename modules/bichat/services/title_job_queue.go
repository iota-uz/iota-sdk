package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTitleQueueStream       = "bichat:title:jobs"
	defaultTitleQueueDedupePrefix = "bichat:title:dedupe"
	defaultTitleQueueDedupeTTL    = 30 * time.Minute
)

// TitleJobQueue enqueues async title generation work.
type TitleJobQueue interface {
	Enqueue(ctx context.Context, tenantID uuid.UUID, sessionID uuid.UUID) error
}

// RedisTitleJobQueueConfig configures Redis stream queueing for title generation.
type RedisTitleJobQueueConfig struct {
	RedisURL     string
	Stream       string
	DedupePrefix string
	DedupeTTL    time.Duration
	Client       *redis.Client
}

// RedisTitleJobQueue enqueues title generation jobs into Redis stream.
type RedisTitleJobQueue struct {
	client       *redis.Client
	stream       string
	dedupePrefix string
	dedupeTTL    time.Duration
}

func NewRedisTitleJobQueue(cfg RedisTitleJobQueueConfig) (*RedisTitleJobQueue, error) {
	stream := strings.TrimSpace(cfg.Stream)
	if stream == "" {
		stream = defaultTitleQueueStream
	}

	dedupePrefix := strings.TrimSpace(cfg.DedupePrefix)
	if dedupePrefix == "" {
		dedupePrefix = defaultTitleQueueDedupePrefix
	}

	dedupeTTL := cfg.DedupeTTL
	if dedupeTTL <= 0 {
		dedupeTTL = defaultTitleQueueDedupeTTL
	}

	client := cfg.Client
	if client == nil {
		c, err := newRedisClient(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		client = c
	}

	return &RedisTitleJobQueue{
		client:       client,
		stream:       stream,
		dedupePrefix: dedupePrefix,
		dedupeTTL:    dedupeTTL,
	}, nil
}

func (q *RedisTitleJobQueue) Enqueue(ctx context.Context, tenantID uuid.UUID, sessionID uuid.UUID) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("tenant id is required")
	}
	if sessionID == uuid.Nil {
		return fmt.Errorf("session id is required")
	}

	dedupeKey := q.dedupeKey(tenantID, sessionID)
	queued, err := q.client.SetNX(ctx, dedupeKey, "1", q.dedupeTTL).Result()
	if err != nil {
		return fmt.Errorf("set title queue dedupe key: %w", err)
	}
	if !queued {
		return nil
	}

	_, err = q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{
			"tenant_id":   tenantID.String(),
			"session_id":  sessionID.String(),
			"attempt":     "0",
			"enqueued_at": time.Now().UTC().Format(time.RFC3339Nano),
		},
	}).Result()
	if err != nil {
		_, _ = q.client.Del(ctx, dedupeKey).Result()
		return fmt.Errorf("enqueue title job: %w", err)
	}

	return nil
}

func (q *RedisTitleJobQueue) Close() error {
	return q.client.Close()
}

func (q *RedisTitleJobQueue) dedupeKey(tenantID uuid.UUID, sessionID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", q.dedupePrefix, tenantID.String(), sessionID.String())
}

func newRedisClient(redisURL string) (*redis.Client, error) {
	redisURL = strings.TrimSpace(redisURL)
	if redisURL == "" {
		return nil, fmt.Errorf("redis url is required")
	}

	var opts *redis.Options
	var err error
	if strings.Contains(redisURL, "://") {
		opts, err = redis.ParseURL(redisURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
	} else {
		opts = &redis.Options{Addr: redisURL}
	}

	client := redis.NewClient(opts)
	if pingErr := client.Ping(context.Background()).Err(); pingErr != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", pingErr)
	}

	return client, nil
}
