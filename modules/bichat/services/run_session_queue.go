// Package services provides this package.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/redis/go-redis/v9"
)

// Per-session FIFO queue key conventions. Distinct from the global run
// queue (bichat:run:jobs): the session queue holds messages waiting
// behind an already-active run in the same session, preserving send
// order so "send A, send B while A is running" runs A then B.
const (
	defaultRunSessionQueuePrefix = "bichat:run:queue"
	defaultRunSessionQueueMaxLen = 200
	// sessionQueueTTL lets forgotten queues GC eventually; the normal
	// path pops entries as they land. 24h is comfortable headroom for
	// a session that's been abandoned mid-queue.
	defaultRunSessionQueueTTL = 24 * time.Hour
)

// QueuedRunJob is the shape pushed into the per-session FIFO. It is the
// subset of RunJobPayload the FIFO needs to reconstruct a valid enqueue
// when the active run terminates.
type QueuedRunJob struct {
	Payload  RunJobPayload `json:"payload"`
	QueuedAt time.Time     `json:"queued_at"`
}

// RunSessionQueue is the per-session FIFO. Push appends to the back,
// Pop drains from the front, Len reports depth (used by the sidebar to
// show "3 waiting" badges).
type RunSessionQueue interface {
	Push(ctx context.Context, tenantID, sessionID uuid.UUID, job QueuedRunJob) error
	Pop(ctx context.Context, tenantID, sessionID uuid.UUID) (QueuedRunJob, bool, error)
	Len(ctx context.Context, tenantID, sessionID uuid.UUID) (int64, error)
}

// RedisRunSessionQueueConfig configures the Redis-backed FIFO.
type RedisRunSessionQueueConfig struct {
	Client    *redis.Client
	RedisURL  string
	KeyPrefix string
	MaxLen    int64
	TTL       time.Duration
}

// RedisRunSessionQueue is the Redis list implementation.
type RedisRunSessionQueue struct {
	client    *redis.Client
	keyPrefix string
	maxLen    int64
	ttl       time.Duration
}

// NewRedisRunSessionQueue constructs a FIFO bound to the supplied Redis
// client, or dials a new connection from RedisURL if Client is nil.
func NewRedisRunSessionQueue(cfg RedisRunSessionQueueConfig) (*RedisRunSessionQueue, error) {
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = defaultRunSessionQueuePrefix
	}
	maxLen := cfg.MaxLen
	if maxLen <= 0 {
		maxLen = defaultRunSessionQueueMaxLen
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultRunSessionQueueTTL
	}

	client := cfg.Client
	if client == nil {
		c, err := newRedisClient(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		client = c
	}

	return &RedisRunSessionQueue{
		client:    client,
		keyPrefix: prefix,
		maxLen:    maxLen,
		ttl:       ttl,
	}, nil
}

// Push implements RunSessionQueue.
//
// Uses RPUSH so Pop (LPOP) drains in FIFO order. After pushing the new
// entry we trim the list to maxLen with LTRIM and refresh the TTL so
// an active session's queue never GCs from under us. If the list has
// grown past maxLen the OLDEST entries are dropped — losing the
// middle of the queue is preferable to dropping the newest messages
// the user just sent.
func (q *RedisRunSessionQueue) Push(ctx context.Context, tenantID, sessionID uuid.UUID, job QueuedRunJob) error {
	const op serrors.Op = "RedisRunSessionQueue.Push"
	if tenantID == uuid.Nil || sessionID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant id and session id are required")
	}
	if job.QueuedAt.IsZero() {
		job.QueuedAt = time.Now().UTC()
	}
	body, err := json.Marshal(job)
	if err != nil {
		return serrors.E(op, "marshal queued job", err)
	}
	key := q.listKey(tenantID, sessionID)
	writeCtx := context.WithoutCancel(ctx)
	pipe := q.client.TxPipeline()
	pipe.RPush(writeCtx, key, body)
	if q.maxLen > 0 {
		pipe.LTrim(writeCtx, key, -q.maxLen, -1)
	}
	pipe.Expire(writeCtx, key, q.ttl)
	if _, err := pipe.Exec(writeCtx); err != nil {
		return serrors.E(op, "push queued job", err)
	}
	return nil
}

// Pop implements RunSessionQueue. Returns (job, true, nil) when a job
// was drained, (zero, false, nil) on empty, or a wrapped error.
func (q *RedisRunSessionQueue) Pop(ctx context.Context, tenantID, sessionID uuid.UUID) (QueuedRunJob, bool, error) {
	const op serrors.Op = "RedisRunSessionQueue.Pop"
	if tenantID == uuid.Nil || sessionID == uuid.Nil {
		return QueuedRunJob{}, false, serrors.E(op, serrors.KindValidation, "tenant id and session id are required")
	}
	raw, err := q.client.LPop(ctx, q.listKey(tenantID, sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return QueuedRunJob{}, false, nil
		}
		return QueuedRunJob{}, false, serrors.E(op, "lpop", err)
	}
	var job QueuedRunJob
	if err := json.Unmarshal(raw, &job); err != nil {
		return QueuedRunJob{}, false, serrors.E(op, "unmarshal queued job", err)
	}
	return job, true, nil
}

// Len implements RunSessionQueue.
func (q *RedisRunSessionQueue) Len(ctx context.Context, tenantID, sessionID uuid.UUID) (int64, error) {
	const op serrors.Op = "RedisRunSessionQueue.Len"
	if tenantID == uuid.Nil || sessionID == uuid.Nil {
		return 0, serrors.E(op, serrors.KindValidation, "tenant id and session id are required")
	}
	// LLEN returns 0 (not redis.Nil) for a missing key, so no redis.Nil
	// branch is needed here.
	n, err := q.client.LLen(ctx, q.listKey(tenantID, sessionID)).Result()
	if err != nil {
		return 0, serrors.E(op, "llen", err)
	}
	return n, nil
}

func (q *RedisRunSessionQueue) listKey(tenantID, sessionID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", q.keyPrefix, tenantID.String(), sessionID.String())
}
