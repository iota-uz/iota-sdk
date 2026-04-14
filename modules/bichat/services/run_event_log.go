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

// Redis conventions for the per-run event log.
const (
	defaultRunEventLogPrefix = "bichat:run-events"
	defaultRunEventLogMaxLen = 5_000
	defaultRunEventLogTTL    = 2 * time.Hour
	defaultRunEventTailBlock = 5 * time.Second
	// RunEventStreamStart is the sentinel cursor for "replay from the
	// beginning of the stream". Use it when a client has no Last-Event-ID.
	RunEventStreamStart = "0"
)

// Terminal event types: once one of these lands in a stream the log is
// effectively closed and Tail consumers should exit. Keep this list in sync
// with bichatservices.ChunkType* and with run_executor.go emit paths.
var runEventTerminalTypes = map[string]struct{}{
	"done":      {},
	"cancelled": {},
	"error":     {},
	"failed":    {},
}

// IsRunEventTerminal reports whether the event type ends the stream.
func IsRunEventTerminal(eventType string) bool {
	_, ok := runEventTerminalTypes[eventType]
	return ok
}

// RunEvent is a single entry in a run's event log. Payload is an opaque
// JSON blob — usually a marshalled httpdto.StreamChunkPayload — so the
// transport layer does not need to know about internal service types.
type RunEvent struct {
	// StreamID is the Redis stream id assigned at append time, e.g.
	// "1712345678000-0". Clients use this verbatim for Last-Event-ID.
	StreamID string
	// Type is the event kind and powers terminal detection.
	Type string
	// Payload is the JSON-encoded data line sent to the SSE client.
	Payload json.RawMessage
}

// RunEventLog is the durable replay+tail surface for a single run.
type RunEventLog interface {
	// Append writes a new event and returns the assigned stream id. The
	// key TTL is refreshed on every append so an active run keeps its
	// event window open even if the TTL constant is much smaller than the
	// run's worst-case runtime.
	Append(ctx context.Context, tenantID, runID uuid.UUID, event RunEvent) (string, error)

	// Replay returns all events with stream id strictly greater than `from`.
	// Use RunEventStreamStart ("0") to start at the very beginning.
	Replay(ctx context.Context, tenantID, runID uuid.UUID, from string) ([]RunEvent, error)

	// Tail returns a channel that yields each event appended after `from`
	// and is closed when a terminal event is observed, the stream is
	// deleted, or ctx is cancelled. The channel is unbuffered: callers
	// MUST consume without long blocking or the XREAD goroutine will
	// back-pressure itself.
	Tail(ctx context.Context, tenantID, runID uuid.UUID, from string) (<-chan RunEvent, error)

	// DropAfterTerminal sets a short TTL on the stream key so terminated
	// runs get garbage-collected without bloating Redis. Safe to call on
	// an already-expired key.
	DropAfterTerminal(ctx context.Context, tenantID, runID uuid.UUID, ttl time.Duration) error
}

// RedisRunEventLogConfig configures the Redis-backed log. Zero values fall
// back to the defaults above.
type RedisRunEventLogConfig struct {
	RedisURL  string
	KeyPrefix string
	MaxLen    int64
	TTL       time.Duration
	BlockTime time.Duration
	Client    *redis.Client
}

// RedisRunEventLog is the Redis Streams implementation.
type RedisRunEventLog struct {
	client    *redis.Client
	keyPrefix string
	maxLen    int64
	ttl       time.Duration
	blockTime time.Duration
}

// NewRedisRunEventLog constructs a log bound to the supplied client, or
// dials a new connection from RedisURL if Client is nil.
func NewRedisRunEventLog(cfg RedisRunEventLogConfig) (*RedisRunEventLog, error) {
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = defaultRunEventLogPrefix
	}
	maxLen := cfg.MaxLen
	if maxLen <= 0 {
		maxLen = defaultRunEventLogMaxLen
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultRunEventLogTTL
	}
	blockTime := cfg.BlockTime
	if blockTime <= 0 {
		blockTime = defaultRunEventTailBlock
	}

	client := cfg.Client
	if client == nil {
		c, err := newRedisClient(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		client = c
	}

	return &RedisRunEventLog{
		client:    client,
		keyPrefix: prefix,
		maxLen:    maxLen,
		ttl:       ttl,
		blockTime: blockTime,
	}, nil
}

// Append implements RunEventLog.
func (l *RedisRunEventLog) Append(ctx context.Context, tenantID, runID uuid.UUID, event RunEvent) (string, error) {
	const op serrors.Op = "RedisRunEventLog.Append"

	if tenantID == uuid.Nil {
		return "", serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	if runID == uuid.Nil {
		return "", serrors.E(op, serrors.KindValidation, "run id is required")
	}
	if strings.TrimSpace(event.Type) == "" {
		return "", serrors.E(op, serrors.KindValidation, "event type is required")
	}

	// context.WithoutCancel: appending a terminal event while the caller's
	// request context is already cancelled (e.g. client disconnect) still
	// needs to land — other tailing clients depend on seeing the terminal
	// marker. This mirrors the policy in RunJobQueue.Enqueue.
	writeCtx := context.WithoutCancel(ctx)

	key := l.streamKey(tenantID, runID)
	id, err := l.client.XAdd(writeCtx, &redis.XAddArgs{
		Stream: key,
		MaxLen: l.maxLen,
		Approx: true,
		Values: map[string]any{
			"type":    event.Type,
			"payload": []byte(event.Payload),
		},
	}).Result()
	if err != nil {
		return "", serrors.E(op, "xadd run event", err)
	}

	// Refresh the TTL on every write so a long-running execution doesn't
	// have its replay window expire mid-run.
	if err := l.client.Expire(writeCtx, key, l.ttl).Err(); err != nil {
		return "", serrors.E(op, "refresh run event ttl", err)
	}
	return id, nil
}

// Replay implements RunEventLog.
func (l *RedisRunEventLog) Replay(ctx context.Context, tenantID, runID uuid.UUID, from string) ([]RunEvent, error) {
	const op serrors.Op = "RedisRunEventLog.Replay"
	key := l.streamKey(tenantID, runID)

	start := l.replayStart(from)
	entries, err := l.client.XRange(ctx, key, start, "+").Result()
	if err != nil {
		return nil, serrors.E(op, "xrange run events", err)
	}

	out := make([]RunEvent, 0, len(entries))
	for _, entry := range entries {
		evt, perr := decodeRunEvent(entry)
		if perr != nil {
			return nil, serrors.E(op, perr)
		}
		out = append(out, evt)
	}
	return out, nil
}

// Tail implements RunEventLog.
//
// Behaviour: a background goroutine loops over XREAD BLOCK, forwarding each
// new entry to the returned channel. The goroutine (and the channel) exits
// when (a) context is cancelled, (b) a terminal event is observed, or (c)
// Redis returns a non-recoverable error. Redis `nil` on the BLOCK timeout
// is treated as a normal idle and triggers another read.
func (l *RedisRunEventLog) Tail(ctx context.Context, tenantID, runID uuid.UUID, from string) (<-chan RunEvent, error) {
	const op serrors.Op = "RedisRunEventLog.Tail"
	if tenantID == uuid.Nil {
		return nil, serrors.E(op, serrors.KindValidation, "tenant id is required")
	}
	if runID == uuid.Nil {
		return nil, serrors.E(op, serrors.KindValidation, "run id is required")
	}
	key := l.streamKey(tenantID, runID)
	cursor := l.tailStart(from)

	out := make(chan RunEvent)
	go func() {
		defer close(out)
		for {
			if ctx.Err() != nil {
				return
			}
			res, err := l.client.XRead(ctx, &redis.XReadArgs{
				Streams: []string{key, cursor},
				Count:   32,
				Block:   l.blockTime,
			}).Result()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					continue
				}
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return
				}
				return
			}
			for _, stream := range res {
				for _, msg := range stream.Messages {
					evt, perr := decodeRunEvent(msg)
					if perr != nil {
						continue
					}
					cursor = evt.StreamID
					select {
					case out <- evt:
					case <-ctx.Done():
						return
					}
					if IsRunEventTerminal(evt.Type) {
						return
					}
				}
			}
		}
	}()
	return out, nil
}

// DropAfterTerminal implements RunEventLog.
func (l *RedisRunEventLog) DropAfterTerminal(ctx context.Context, tenantID, runID uuid.UUID, ttl time.Duration) error {
	const op serrors.Op = "RedisRunEventLog.DropAfterTerminal"
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	key := l.streamKey(tenantID, runID)
	if err := l.client.Expire(ctx, key, ttl).Err(); err != nil {
		return serrors.E(op, "set terminal ttl", err)
	}
	return nil
}

// Close releases the underlying Redis connection.
func (l *RedisRunEventLog) Close() error {
	return l.client.Close()
}

func (l *RedisRunEventLog) streamKey(tenantID, runID uuid.UUID) string {
	return fmt.Sprintf("%s:%s:%s", l.keyPrefix, tenantID.String(), runID.String())
}

// replayStart normalises a cursor for XRANGE. XRANGE is inclusive on both
// ends so callers who want "events AFTER id X" need the exclusive "(X"
// form. An empty cursor or the sentinel "0" means "from the beginning"
// and maps to "-".
func (l *RedisRunEventLog) replayStart(from string) string {
	from = strings.TrimSpace(from)
	if from == "" || from == RunEventStreamStart || from == "0-0" {
		return "-"
	}
	if strings.HasPrefix(from, "(") {
		return from
	}
	return "(" + from
}

// tailStart normalises a cursor for XREAD. XREAD is exclusive on the
// `from` side so we pass the raw id — the sentinel "0" means "from the
// beginning" which is what a brand-new SSE consumer gets.
func (l *RedisRunEventLog) tailStart(from string) string {
	from = strings.TrimSpace(from)
	if from == "" {
		return RunEventStreamStart
	}
	if strings.HasPrefix(from, "(") {
		return strings.TrimPrefix(from, "(")
	}
	return from
}

func decodeRunEvent(msg redis.XMessage) (RunEvent, error) {
	typVal, ok := msg.Values["type"]
	if !ok {
		return RunEvent{}, fmt.Errorf("missing event type field (id=%s)", msg.ID)
	}
	typ, ok := typVal.(string)
	if !ok {
		return RunEvent{}, fmt.Errorf("event type is not a string (id=%s)", msg.ID)
	}

	var payload json.RawMessage
	if raw, ok := msg.Values["payload"]; ok {
		bytes, err := coerceBytes(raw)
		if err != nil {
			return RunEvent{}, fmt.Errorf("decode payload (id=%s): %w", msg.ID, err)
		}
		payload = append(payload, bytes...)
	}
	return RunEvent{
		StreamID: msg.ID,
		Type:     typ,
		Payload:  payload,
	}, nil
}
