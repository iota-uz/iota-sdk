// Package services provides this package.
package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/redis/go-redis/v9"
)

// Redis key and pubsub topic conventions for the per-tenant active run
// index. The hash holds one entry per streaming (or queued) run keyed by
// session id — sessions are mutually-exclusive with runs via the
// generation_run_store SetNX lock, so this is a 1:1 mapping for active
// work. The pubsub topic carries deltas so subscribers don't have to
// poll HGETALL on a timer.
const (
	defaultActiveRunIndexPrefix    = "bichat:active-runs"
	defaultActiveRunIndexEventsTop = "bichat:active-runs:events"
)

// ActiveRunStatusQueued marks a session whose newest send is parked on the
// per-session FIFO behind an active run. The reaper skips non-streaming
// entries, so queued jobs are never reaped while waiting.
const ActiveRunStatusQueued = "queued"

// ActiveRunStatus is the canonical shape rendered on sidebar dots and
// emitted on the status pubsub topic. Terminal statuses (completed /
// cancelled / failed) are published once and then the hash entry is
// removed so the sidebar badge fades on its own without polling.
type ActiveRunStatus struct {
	SessionID uuid.UUID `json:"session_id"`
	RunID     uuid.UUID `json:"run_id"`
	// Status matches domain.GenerationRunStatus values + "queued" (used
	// by the FIFO queue for messages waiting behind an active run).
	Status string `json:"status"`
	// QueuedRuns is the number of sends parked on the session FIFO behind
	// this run. Maintained additively so it never disturbs Status/RunID of
	// the entry that owns the hash field (the reaper only reaps entries
	// whose Status is streaming).
	QueuedRuns int64     `json:"queued_runs,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ActiveRunIndex is the per-tenant live view of in-flight generations.
//
// Upsert/Remove are called by the chat service on every run state
// transition. Snapshot + Subscribe are used by the sidebar SSE handler
// to bootstrap then live-update a "generating…" indicator next to each
// session card without N round-trips on page load.
type ActiveRunIndex interface {
	// Upsert writes or overwrites the status of a session's active run.
	// Publishes a delta on the events topic so tailing clients see it
	// immediately. Safe to call with terminal statuses — prefer
	// Remove for the GC step so the hash doesn't grow forever.
	Upsert(ctx context.Context, tenantID uuid.UUID, status ActiveRunStatus) error

	// PublishAndRemove publishes the final status delta then removes
	// the hash entry so the sidebar badge can fade. It is the atomic
	// terminal step — using Upsert(terminal) followed by Remove would
	// leak a window where a late subscriber sees a stale streaming
	// entry on HGETALL.
	PublishAndRemove(ctx context.Context, tenantID uuid.UUID, status ActiveRunStatus) error

	// Remove drops a session entry without publishing, used by GC /
	// reaper paths where a publish would double-count.
	Remove(ctx context.Context, tenantID, sessionID uuid.UUID) error

	// Snapshot returns the current live view for the tenant. Ordering
	// is not guaranteed — callers who want a stable sort should order
	// by SessionID or UpdatedAt.
	Snapshot(ctx context.Context, tenantID uuid.UUID) ([]ActiveRunStatus, error)

	// Subscribe returns a channel of delta events. The channel closes
	// when ctx is cancelled or the underlying pubsub connection
	// breaks. Subscribers are tenant-scoped: a listener for tenant A
	// will not receive tenant B events.
	Subscribe(ctx context.Context, tenantID uuid.UUID) (<-chan ActiveRunStatus, error)

	// AddQueuedRuns adjusts the number of sends parked on the session
	// FIFO behind the session's active run. It mutates only the QueuedRuns
	// count of the existing entry — never Status/RunID — so the reaper
	// keeps seeing a streaming run as streaming. When no entry exists and
	// delta > 0, a queued-status entry is created for runID; a queued-only
	// entry whose count reaches zero is removed. The resulting entry is
	// published as a delta.
	AddQueuedRuns(ctx context.Context, tenantID, sessionID, runID uuid.UUID, delta int64) error
}

// RedisActiveRunIndexConfig configures the Redis-backed index.
type RedisActiveRunIndexConfig struct {
	RedisURL    string
	KeyPrefix   string
	EventsTopic string
	Client      *redis.Client
}

// RedisActiveRunIndex is the Redis hash + pubsub implementation.
type RedisActiveRunIndex struct {
	client *redis.Client
	// ownsClient is true only when this instance dialled the connection
	// itself. When false (client supplied externally), Close is a no-op so
	// the shared-client path does not tear down the other components.
	ownsClient  bool
	keyPrefix   string
	eventsTopic string
}

// NewRedisActiveRunIndex constructs an index bound to the supplied Redis
// client, or dials a new connection from RedisURL if Client is nil.
func NewRedisActiveRunIndex(cfg RedisActiveRunIndexConfig) (*RedisActiveRunIndex, error) {
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = defaultActiveRunIndexPrefix
	}
	events := strings.TrimSpace(cfg.EventsTopic)
	if events == "" {
		events = defaultActiveRunIndexEventsTop
	}

	ownsClient := cfg.Client == nil
	client := cfg.Client
	if client == nil {
		c, err := newRedisClient(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		client = c
	}

	return &RedisActiveRunIndex{
		client:      client,
		ownsClient:  ownsClient,
		keyPrefix:   prefix,
		eventsTopic: events,
	}, nil
}

// Upsert implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) Upsert(ctx context.Context, tenantID uuid.UUID, status ActiveRunStatus) error {
	const op serrors.Op = "RedisActiveRunIndex.Upsert"
	if tenantID == uuid.Nil {
		return serrors.New(serrors.Invalid, "tenant id is required").WithOp(op)
	}
	if status.SessionID == uuid.Nil {
		return serrors.New(serrors.Invalid, "session id is required").WithOp(op)
	}
	if status.UpdatedAt.IsZero() {
		status.UpdatedAt = time.Now().UTC()
	}
	body, err := json.Marshal(status)
	if err != nil {
		return serrors.WrapContext(op, err, "marshal status")
	}

	writeCtx := context.WithoutCancel(ctx)
	pipe := idx.client.TxPipeline()
	pipe.HSet(writeCtx, idx.hashKey(tenantID), status.SessionID.String(), body)
	pipe.Publish(writeCtx, idx.eventsChannel(tenantID), body)
	if _, err := pipe.Exec(writeCtx); err != nil {
		return serrors.WrapContext(op, err, "hset+publish")
	}
	return nil
}

// PublishAndRemove implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) PublishAndRemove(ctx context.Context, tenantID uuid.UUID, status ActiveRunStatus) error {
	const op serrors.Op = "RedisActiveRunIndex.PublishAndRemove"
	if tenantID == uuid.Nil {
		return serrors.New(serrors.Invalid, "tenant id is required").WithOp(op)
	}
	if status.SessionID == uuid.Nil {
		return serrors.New(serrors.Invalid, "session id is required").WithOp(op)
	}
	if status.UpdatedAt.IsZero() {
		status.UpdatedAt = time.Now().UTC()
	}
	body, err := json.Marshal(status)
	if err != nil {
		return serrors.WrapContext(op, err, "marshal status")
	}

	writeCtx := context.WithoutCancel(ctx)
	pipe := idx.client.TxPipeline()
	pipe.Publish(writeCtx, idx.eventsChannel(tenantID), body)
	pipe.HDel(writeCtx, idx.hashKey(tenantID), status.SessionID.String())
	if _, err := pipe.Exec(writeCtx); err != nil {
		return serrors.WrapContext(op, err, "publish+hdel")
	}
	return nil
}

// Remove implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) Remove(ctx context.Context, tenantID, sessionID uuid.UUID) error {
	const op serrors.Op = "RedisActiveRunIndex.Remove"
	if tenantID == uuid.Nil || sessionID == uuid.Nil {
		return serrors.New(serrors.Invalid, "tenant id and session id are required").WithOp(op)
	}
	writeCtx := context.WithoutCancel(ctx)
	if err := idx.client.HDel(writeCtx, idx.hashKey(tenantID), sessionID.String()).Err(); err != nil {
		return serrors.WrapContext(op, err, "hdel")
	}
	return nil
}

// Snapshot implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) Snapshot(ctx context.Context, tenantID uuid.UUID) ([]ActiveRunStatus, error) {
	const op serrors.Op = "RedisActiveRunIndex.Snapshot"
	if tenantID == uuid.Nil {
		return nil, serrors.New(serrors.Invalid, "tenant id is required").WithOp(op)
	}
	raw, err := idx.client.HGetAll(ctx, idx.hashKey(tenantID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, serrors.WrapContext(op, err, "hgetall")
	}
	out := make([]ActiveRunStatus, 0, len(raw))
	for _, v := range raw {
		var entry ActiveRunStatus
		if err := json.Unmarshal([]byte(v), &entry); err != nil {
			// Skip malformed entries rather than failing the whole
			// snapshot — a single bad write shouldn't break the sidebar.
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

// Subscribe implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) Subscribe(ctx context.Context, tenantID uuid.UUID) (<-chan ActiveRunStatus, error) {
	const op serrors.Op = "RedisActiveRunIndex.Subscribe"
	if tenantID == uuid.Nil {
		return nil, serrors.New(serrors.Invalid, "tenant id is required").WithOp(op)
	}
	sub := idx.client.Subscribe(ctx, idx.eventsChannel(tenantID))
	// Ensure the subscription is actually established before returning
	// so callers don't race with the first Publish.
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, serrors.WrapContext(op, err, "subscribe")
	}

	out := make(chan ActiveRunStatus)
	go func() {
		defer close(out)
		defer func() { _ = sub.Close() }()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var entry ActiveRunStatus
				if err := json.Unmarshal([]byte(msg.Payload), &entry); err != nil {
					continue
				}
				select {
				case out <- entry:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

// addQueuedRunsScript mutates only the queued_runs count of the session's
// entry. Missing entry + positive delta creates a queued-status entry (the
// reaper skips non-streaming entries); a queued-only entry whose count
// reaches zero is deleted. Returns the encoded entry, or nil when the field
// was removed (or nothing needed creating).
const addQueuedRunsScript = `
local raw = redis.call('HGET', KEYS[1], ARGV[1])
local delta = tonumber(ARGV[2])
local runID = ARGV[3]
local now = ARGV[4]
if raw then
  local entry = cjson.decode(raw)
  local queued = 0
  if entry.queued_runs then queued = entry.queued_runs end
  queued = queued + delta
  if queued < 0 then queued = 0 end
  entry.queued_runs = queued
  if queued == 0 and entry.status == 'queued' then
    redis.call('HDEL', KEYS[1], ARGV[1])
    return nil
  end
  entry.updated_at = now
  local body = cjson.encode(entry)
  redis.call('HSET', KEYS[1], ARGV[1], body)
  return body
end
if delta <= 0 then
  return nil
end
local entry = {session_id=ARGV[1], run_id=runID, status='queued', queued_runs=delta, updated_at=now}
local body = cjson.encode(entry)
redis.call('HSET', KEYS[1], ARGV[1], body)
return body
`

// AddQueuedRuns implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) AddQueuedRuns(ctx context.Context, tenantID, sessionID, runID uuid.UUID, delta int64) error {
	const op serrors.Op = "RedisActiveRunIndex.AddQueuedRuns"
	if tenantID == uuid.Nil || sessionID == uuid.Nil {
		return serrors.New(serrors.Invalid, "tenant id and session id are required").WithOp(op)
	}

	writeCtx := context.WithoutCancel(ctx)
	result, err := idx.client.Eval(writeCtx, addQueuedRunsScript, []string{idx.hashKey(tenantID)},
		sessionID.String(),
		strconv.FormatInt(delta, 10),
		runID.String(),
		time.Now().UTC().Format(time.RFC3339Nano),
	).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return serrors.WrapContext(op, err, "eval queued runs")
	}

	if body, ok := result.(string); ok && body != "" {
		_ = idx.client.Publish(writeCtx, idx.eventsChannel(tenantID), body).Err()
		return nil
	}
	// Field removed (queued-only entry drained to zero): publish a
	// zero-count delta so live subscribers clear the badge.
	removed, marshalErr := json.Marshal(ActiveRunStatus{
		SessionID:  sessionID,
		RunID:      runID,
		Status:     ActiveRunStatusQueued,
		QueuedRuns: 0,
		UpdatedAt:  time.Now().UTC(),
	})
	if marshalErr == nil {
		_ = idx.client.Publish(writeCtx, idx.eventsChannel(tenantID), removed).Err()
	}
	return nil
}

// Close releases the underlying Redis connection. When the client was
// supplied externally (ownsClient == false) this is a no-op; the caller
// that owns the shared *redis.Client is responsible for closing it.
func (idx *RedisActiveRunIndex) Close() error {
	if !idx.ownsClient {
		return nil
	}
	return idx.client.Close()
}

func (idx *RedisActiveRunIndex) hashKey(tenantID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", idx.keyPrefix, tenantID.String())
}

func (idx *RedisActiveRunIndex) eventsChannel(tenantID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", idx.eventsTopic, tenantID.String())
}
