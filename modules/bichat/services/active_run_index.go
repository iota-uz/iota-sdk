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

// ActiveRunStatus is the canonical shape rendered on sidebar dots and
// emitted on the status pubsub topic. Terminal statuses (completed /
// cancelled / failed) are published once and then the hash entry is
// removed so the sidebar badge fades on its own without polling.
type ActiveRunStatus struct {
	SessionID uuid.UUID `json:"session_id"`
	RunID     uuid.UUID `json:"run_id"`
	// Status matches domain.GenerationRunStatus values + "queued" (used
	// by the FIFO queue for messages waiting behind an active run).
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
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
	client      *redis.Client
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
		keyPrefix:   prefix,
		eventsTopic: events,
	}, nil
}

// newConfiguredActiveRunIndex mirrors the other newConfigured* helpers:
// reads REDIS_URL and returns nil on disable so callers degrade to a
// no-op integration path.
func newConfiguredActiveRunIndex() ActiveRunIndex {
	redisURL, ok := envLookup("REDIS_URL")
	if !ok || strings.TrimSpace(redisURL) == "" {
		return nil
	}
	idx, err := NewRedisActiveRunIndex(RedisActiveRunIndexConfig{RedisURL: redisURL})
	if err != nil {
		return nil
	}
	return idx
}

// Upsert implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) Upsert(ctx context.Context, tenantID uuid.UUID, status ActiveRunStatus) error {
	const op serrors.Op = "RedisActiveRunIndex.Upsert"
	if tenantID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	if status.SessionID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "session id is required")
	}
	if status.UpdatedAt.IsZero() {
		status.UpdatedAt = time.Now().UTC()
	}
	body, err := json.Marshal(status)
	if err != nil {
		return serrors.E(op, "marshal status", err)
	}

	writeCtx := context.WithoutCancel(ctx)
	pipe := idx.client.TxPipeline()
	pipe.HSet(writeCtx, idx.hashKey(tenantID), status.SessionID.String(), body)
	pipe.Publish(writeCtx, idx.eventsChannel(tenantID), body)
	if _, err := pipe.Exec(writeCtx); err != nil {
		return serrors.E(op, "hset+publish", err)
	}
	return nil
}

// PublishAndRemove implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) PublishAndRemove(ctx context.Context, tenantID uuid.UUID, status ActiveRunStatus) error {
	const op serrors.Op = "RedisActiveRunIndex.PublishAndRemove"
	if tenantID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	if status.SessionID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "session id is required")
	}
	if status.UpdatedAt.IsZero() {
		status.UpdatedAt = time.Now().UTC()
	}
	body, err := json.Marshal(status)
	if err != nil {
		return serrors.E(op, "marshal status", err)
	}

	writeCtx := context.WithoutCancel(ctx)
	pipe := idx.client.TxPipeline()
	pipe.Publish(writeCtx, idx.eventsChannel(tenantID), body)
	pipe.HDel(writeCtx, idx.hashKey(tenantID), status.SessionID.String())
	if _, err := pipe.Exec(writeCtx); err != nil {
		return serrors.E(op, "publish+hdel", err)
	}
	return nil
}

// Remove implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) Remove(ctx context.Context, tenantID, sessionID uuid.UUID) error {
	const op serrors.Op = "RedisActiveRunIndex.Remove"
	if tenantID == uuid.Nil || sessionID == uuid.Nil {
		return serrors.E(op, serrors.KindValidation, "tenant id and session id are required")
	}
	writeCtx := context.WithoutCancel(ctx)
	if err := idx.client.HDel(writeCtx, idx.hashKey(tenantID), sessionID.String()).Err(); err != nil {
		return serrors.E(op, "hdel", err)
	}
	return nil
}

// Snapshot implements ActiveRunIndex.
func (idx *RedisActiveRunIndex) Snapshot(ctx context.Context, tenantID uuid.UUID) ([]ActiveRunStatus, error) {
	const op serrors.Op = "RedisActiveRunIndex.Snapshot"
	if tenantID == uuid.Nil {
		return nil, serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	raw, err := idx.client.HGetAll(ctx, idx.hashKey(tenantID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, serrors.E(op, "hgetall", err)
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
		return nil, serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	sub := idx.client.Subscribe(ctx, idx.eventsChannel(tenantID))
	// Ensure the subscription is actually established before returning
	// so callers don't race with the first Publish.
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, serrors.E(op, "subscribe", err)
	}

	out := make(chan ActiveRunStatus)
	go func() {
		defer close(out)
		defer sub.Close()
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

// Close releases the underlying Redis connection.
func (idx *RedisActiveRunIndex) Close() error {
	return idx.client.Close()
}

func (idx *RedisActiveRunIndex) hashKey(tenantID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", idx.keyPrefix, tenantID.String())
}

func (idx *RedisActiveRunIndex) eventsChannel(tenantID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", idx.eventsTopic, tenantID.String())
}
