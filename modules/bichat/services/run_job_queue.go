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

// Redis object key shapes for the bichat run queue. These are kept in sync
// with the title-queue naming so the two systems coexist without overlap.
const (
	defaultRunQueueStream       = "bichat:run:jobs"
	defaultRunQueueDedupePrefix = "bichat:run:request"
	defaultRunQueueDedupeTTL    = 30 * time.Minute
	defaultRunQueueMaxLen       = 10_000
)

// RunJobPayload describes a generation job handed off from the HTTP handler
// to the background worker. The payload is self-contained so the worker does
// not need the original request context: all deps (tenant, session, model
// overrides, upload references) come in through the JSON blob.
//
// Attachments are referenced by upload ID rather than serialised as full
// domain.Attachment objects. The worker re-resolves uploads via the
// attachment service so binary references stay in PostgreSQL, not Redis.
type RunJobPayload struct {
	// Identity / routing.
	TenantID  uuid.UUID `json:"tenant_id"`
	SessionID uuid.UUID `json:"session_id"`
	UserID    int64     `json:"user_id"`

	// RequestID is the client-supplied idempotency key. Two enqueues with the
	// same RequestID resolve to the same RunID (see Enqueue).
	RequestID uuid.UUID `json:"request_id"`

	// RunID is assigned by the queue on first enqueue and returned to the
	// caller so the controller can point SSE readers at the right event log.
	RunID uuid.UUID `json:"run_id"`

	// Message body.
	Content   string  `json:"content"`
	UploadIDs []int64 `json:"upload_ids,omitempty"`

	// Edit/regenerate support.
	ReplaceFromMessageID *uuid.UUID `json:"replace_from_message_id,omitempty"`

	// Per-request overrides.
	ReasoningEffort *string `json:"reasoning_effort,omitempty"`
	Model           *string `json:"model,omitempty"`
	DebugMode       bool    `json:"debug_mode,omitempty"`

	// Delivery metadata: incremented on retry.
	Attempt    int       `json:"attempt"`
	EnqueuedAt time.Time `json:"enqueued_at"`
}

// RunJobQueue enqueues generation jobs and enforces request-level idempotency.
type RunJobQueue interface {
	// Enqueue posts a job to the stream. When a job with the same RequestID
	// has already been queued within the dedupe TTL window, the existing
	// RunID is returned and deduped is true; no new stream entry is written.
	// The RunID on the returned payload is authoritative.
	Enqueue(ctx context.Context, payload RunJobPayload) (runID uuid.UUID, deduped bool, err error)
}

// RedisRunJobQueueConfig configures the Redis-backed queue. Stream, prefix
// and TTL default to the constants above when left zero.
type RedisRunJobQueueConfig struct {
	RedisURL     string
	Stream       string
	DedupePrefix string
	DedupeTTL    time.Duration
	MaxLen       int64
	Client       *redis.Client
}

// RedisRunJobQueue is the Redis Streams implementation of RunJobQueue.
type RedisRunJobQueue struct {
	client       *redis.Client
	stream       string
	dedupePrefix string
	dedupeTTL    time.Duration
	maxLen       int64
}

// NewRedisRunJobQueue constructs a queue bound to the supplied Redis client,
// or dials a new connection from RedisURL if Client is nil.
func NewRedisRunJobQueue(cfg RedisRunJobQueueConfig) (*RedisRunJobQueue, error) {
	stream := strings.TrimSpace(cfg.Stream)
	if stream == "" {
		stream = defaultRunQueueStream
	}

	dedupePrefix := strings.TrimSpace(cfg.DedupePrefix)
	if dedupePrefix == "" {
		dedupePrefix = defaultRunQueueDedupePrefix
	}

	dedupeTTL := cfg.DedupeTTL
	if dedupeTTL <= 0 {
		dedupeTTL = defaultRunQueueDedupeTTL
	}

	maxLen := cfg.MaxLen
	if maxLen <= 0 {
		maxLen = defaultRunQueueMaxLen
	}

	client := cfg.Client
	if client == nil {
		c, err := newRedisClient(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		client = c
	}

	return &RedisRunJobQueue{
		client:       client,
		stream:       stream,
		dedupePrefix: dedupePrefix,
		dedupeTTL:    dedupeTTL,
		maxLen:       maxLen,
	}, nil
}

// Enqueue implements RunJobQueue.
//
// The idempotency contract is: the first caller with a given RequestID wins
// the SetNX on bichat:run:request:{request_id} and assigns a fresh RunID
// (either the one on the payload or a newly generated UUID). Subsequent
// callers with the same RequestID receive that same RunID and a deduped=true
// flag; no additional stream entry is produced. This lets clients retry the
// same message send across network blips without double-running the agent.
func (q *RedisRunJobQueue) Enqueue(ctx context.Context, payload RunJobPayload) (uuid.UUID, bool, error) {
	const op serrors.Op = "RedisRunJobQueue.Enqueue"

	if payload.TenantID == uuid.Nil {
		return uuid.Nil, false, serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	if payload.SessionID == uuid.Nil {
		return uuid.Nil, false, serrors.E(op, serrors.KindValidation, "session id is required")
	}
	if payload.RequestID == uuid.Nil {
		return uuid.Nil, false, serrors.E(op, serrors.KindValidation, "request id is required")
	}

	// The caller may supply a RunID (e.g. retries promoted off the retry
	// schedule). When empty, mint a fresh one — but do not commit to it
	// until SetNX wins, otherwise a losing caller would return a RunID
	// that isn't actually the authoritative one.
	assignedRunID := payload.RunID
	if assignedRunID == uuid.Nil {
		assignedRunID = uuid.New()
	}

	dedupeKey := q.dedupeKey(payload.RequestID)
	enqueueCtx := context.WithoutCancel(ctx)

	acquired, err := q.client.SetNX(enqueueCtx, dedupeKey, assignedRunID.String(), q.dedupeTTL).Result()
	if err != nil {
		return uuid.Nil, false, serrors.E(op, "set run dedupe key", err)
	}
	if !acquired {
		// Duplicate send: look up the run id assigned by the first writer.
		existing, getErr := q.client.Get(enqueueCtx, dedupeKey).Result()
		if getErr != nil {
			if errors.Is(getErr, redis.Nil) {
				// The dedupe key expired between the SetNX race and our Get.
				// Treat as a stale dedupe record and retry enqueue from scratch.
				return q.Enqueue(ctx, payload)
			}
			return uuid.Nil, false, serrors.E(op, "read run dedupe key", getErr)
		}
		existingID, parseErr := uuid.Parse(existing)
		if parseErr != nil {
			return uuid.Nil, false, serrors.E(op, "parse existing run id", parseErr)
		}
		return existingID, true, nil
	}

	payload.RunID = assignedRunID
	if payload.EnqueuedAt.IsZero() {
		payload.EnqueuedAt = time.Now().UTC()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		_, _ = q.client.Del(enqueueCtx, dedupeKey).Result()
		return uuid.Nil, false, serrors.E(op, "marshal run payload", err)
	}

	_, err = q.client.XAdd(enqueueCtx, &redis.XAddArgs{
		Stream: q.stream,
		MaxLen: q.maxLen,
		Approx: true,
		Values: map[string]any{
			"tenant_id":   payload.TenantID.String(),
			"session_id":  payload.SessionID.String(),
			"request_id":  payload.RequestID.String(),
			"run_id":      payload.RunID.String(),
			"attempt":     fmt.Sprintf("%d", payload.Attempt),
			"enqueued_at": payload.EnqueuedAt.Format(time.RFC3339Nano),
			"payload":     body,
		},
	}).Result()
	if err != nil {
		cleanupCtx := context.WithoutCancel(ctx)
		_, _ = q.client.Del(cleanupCtx, dedupeKey).Result()
		return uuid.Nil, false, serrors.E(op, "xadd run job", err)
	}
	_, _ = q.client.XTrimMaxLenApprox(enqueueCtx, q.stream, q.maxLen, 1).Result()

	return payload.RunID, false, nil
}

// Close releases the underlying Redis connection.
func (q *RedisRunJobQueue) Close() error {
	return q.client.Close()
}

func (q *RedisRunJobQueue) dedupeKey(requestID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", q.dedupePrefix, requestID.String())
}

// ParseRunJobPayload reads a payload back out of a Redis stream entry.
// It tolerates older producers that only wrote scalar indexed fields by
// falling back when the "payload" field is missing.
func ParseRunJobPayload(values map[string]any) (RunJobPayload, error) {
	const op serrors.Op = "ParseRunJobPayload"

	raw, ok := values["payload"]
	if !ok {
		return RunJobPayload{}, serrors.E(op, serrors.KindValidation, "missing payload field")
	}
	body, err := coerceBytes(raw)
	if err != nil {
		return RunJobPayload{}, serrors.E(op, "coerce payload bytes", err)
	}

	var payload RunJobPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return RunJobPayload{}, serrors.E(op, "unmarshal run job payload", err)
	}
	return payload, nil
}

func coerceBytes(v any) ([]byte, error) {
	switch typed := v.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return typed, nil
	default:
		return nil, fmt.Errorf("unsupported payload encoding %T", v)
	}
}
