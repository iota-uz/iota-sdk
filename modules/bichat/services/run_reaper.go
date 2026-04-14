// Package services provides this package.
package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/httpdto"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// Default reaper knobs. Polling every 15s with a 60s staleness threshold
// means a wedged run is surfaced to users within a minute without making
// Redis SCAN + HGETALL + GetRunByID a dominant load source.
const (
	defaultReaperPollInterval  = 15 * time.Second
	defaultReaperStaleAfter    = 60 * time.Second
	defaultReaperLockKey       = "bichat:run:reaper:lock"
	defaultReaperLockTTL       = 30 * time.Second
	defaultReaperScanBatchSize = 100
)

// RunReaperConfig configures the stale-run reaper.
type RunReaperConfig struct {
	Client         *redis.Client
	RedisURL       string
	RunStore       generationRunStore
	EventLog       RunEventLog
	ActiveRunIndex ActiveRunIndex
	Logger         *logrus.Logger
	// PollInterval is the time between sweeps. Defaults to 15s.
	PollInterval time.Duration
	// StaleAfter is the maximum age of LastHeartbeatAt before a run is
	// considered wedged. Defaults to 60s. Heartbeats tick every ~2s
	// from the chat service so 60s leaves ~30 missed ticks of headroom.
	StaleAfter time.Duration
	// KeyPrefix matches the active-run index key prefix. Defaults to
	// the index's default, which is the common case.
	KeyPrefix string
}

// RunReaper periodically marks runs whose worker has gone silent as
// failed. The reaper is the critical piece of the "worker crash
// mid-LLM" recovery path: without it, a killed worker leaves a hash
// entry claiming streaming forever and the sidebar never transitions.
type RunReaper struct {
	client         *redis.Client
	runStore       generationRunStore
	eventLog       RunEventLog
	index          ActiveRunIndex
	logger         *logrus.Logger
	pollInterval   time.Duration
	staleAfter     time.Duration
	keyPrefix      string
	now            func() time.Time
	instanceLockID string
}

// NewConfiguredRunReaperFromEnv builds a reaper from REDIS_URL with
// fresh Redis-backed store / event log / index instances. It returns
// (nil, nil) when REDIS_URL is unset so callers can skip the reaper
// without branching on a sentinel. Errors are returned only for
// genuine startup failures (bad Redis connection, etc).
func NewConfiguredRunReaperFromEnv(logger *logrus.Logger) (*RunReaper, error) {
	redisURL, ok := envLookup("REDIS_URL")
	if !ok || strings.TrimSpace(redisURL) == "" {
		return nil, nil
	}
	client, err := newRedisClient(redisURL)
	if err != nil {
		return nil, err
	}
	store, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{Client: client})
	if err != nil {
		return nil, err
	}
	eventLog, err := NewRedisRunEventLog(RedisRunEventLogConfig{Client: client})
	if err != nil {
		return nil, err
	}
	index, err := NewRedisActiveRunIndex(RedisActiveRunIndexConfig{Client: client})
	if err != nil {
		return nil, err
	}
	return NewRunReaper(RunReaperConfig{
		Client:         client,
		RunStore:       store,
		EventLog:       eventLog,
		ActiveRunIndex: index,
		Logger:         logger,
	})
}

// NewRunReaper constructs a reaper bound to the given dependencies. The
// Redis client is required for SCAN; the store is required for reading
// LastHeartbeatAt + driving Fail; the event log and index are optional
// (when nil we still fail the run in the store, but skip the terminal
// event / sidebar publish).
func NewRunReaper(cfg RunReaperConfig) (*RunReaper, error) {
	const op serrors.Op = "NewRunReaper"
	if cfg.RunStore == nil {
		return nil, serrors.E(op, serrors.KindValidation, "run store is required")
	}
	client := cfg.Client
	if client == nil {
		c, err := newRedisClient(cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		client = c
	}

	logger := cfg.Logger
	if logger == nil {
		logger = logrus.StandardLogger()
	}
	poll := cfg.PollInterval
	if poll <= 0 {
		poll = defaultReaperPollInterval
	}
	stale := cfg.StaleAfter
	if stale <= 0 {
		stale = defaultReaperStaleAfter
	}
	prefix := strings.TrimSpace(cfg.KeyPrefix)
	if prefix == "" {
		prefix = defaultActiveRunIndexPrefix
	}

	return &RunReaper{
		client:         client,
		runStore:       cfg.RunStore,
		eventLog:       cfg.EventLog,
		index:          cfg.ActiveRunIndex,
		logger:         logger,
		pollInterval:   poll,
		staleAfter:     stale,
		keyPrefix:      prefix,
		now:            time.Now,
		instanceLockID: uuid.NewString(),
	}, nil
}

// Start runs the reaper loop until ctx is cancelled. It acquires a
// process-wide lock before each sweep so that multiple replicas can
// run the reaper safely — only one sweeps at a time, but if the
// lock-holder crashes another takes over on the next TTL expiry.
func (r *RunReaper) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	// Kick off an immediate sweep so a freshly-booted process doesn't
	// wait a full PollInterval before noticing stale runs from before
	// the restart.
	r.sweepIfLeader(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			r.sweepIfLeader(ctx)
		}
	}
}

// sweepIfLeader is a single tick: acquire the single-writer lock,
// run one full sweep, release. Silently skips when another replica
// already holds the lock.
func (r *RunReaper) sweepIfLeader(ctx context.Context) {
	acquired, err := r.client.SetNX(ctx, defaultReaperLockKey, r.instanceLockID, defaultReaperLockTTL).Result()
	if err != nil {
		r.logger.WithError(err).Warn("run reaper lock acquire failed")
		return
	}
	if !acquired {
		return
	}
	defer r.releaseLock(ctx)

	if err := r.sweep(ctx); err != nil {
		r.logger.WithError(err).Warn("run reaper sweep failed")
	}
}

// releaseLock drops the leader lock ONLY if this instance still holds
// it. The Lua compare-and-delete pattern prevents a slow instance
// from releasing another instance's reacquired lock.
func (r *RunReaper) releaseLock(ctx context.Context) {
	const script = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
else
  return 0
end
`
	_, err := r.client.Eval(ctx, script, []string{defaultReaperLockKey}, r.instanceLockID).Result()
	if err != nil && err != redis.Nil {
		r.logger.WithError(err).Warn("run reaper lock release failed")
	}
}

// sweep visits every per-tenant active-run hash, loads the runs that
// are streaming, and fails any whose heartbeat is older than
// StaleAfter.
func (r *RunReaper) sweep(ctx context.Context) error {
	const op serrors.Op = "RunReaper.sweep"

	cursor := uint64(0)
	cutoff := r.now().Add(-r.staleAfter)
	pattern := r.keyPrefix + ":*"

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		keys, next, err := r.client.Scan(ctx, cursor, pattern, defaultReaperScanBatchSize).Result()
		if err != nil {
			return serrors.E(op, "scan active-run keys", err)
		}
		for _, key := range keys {
			tenantID, ok := parseTenantFromActiveRunKey(r.keyPrefix, key)
			if !ok {
				continue
			}
			if err := r.sweepTenant(ctx, tenantID, cutoff); err != nil {
				r.logger.WithError(err).WithField("tenant_id", tenantID.String()).Warn("run reaper tenant sweep failed")
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (r *RunReaper) sweepTenant(ctx context.Context, tenantID uuid.UUID, cutoff time.Time) error {
	const op serrors.Op = "RunReaper.sweepTenant"

	hashKey := fmt.Sprintf("%s:%s", r.keyPrefix, tenantID.String())
	raw, err := r.client.HGetAll(ctx, hashKey).Result()
	if err != nil {
		return serrors.E(op, "hgetall tenant", err)
	}
	for sessionIDStr, value := range raw {
		var entry ActiveRunStatus
		if err := json.Unmarshal([]byte(value), &entry); err != nil {
			continue
		}
		// Only streaming runs are candidates for reaping. Queued runs
		// haven't heartbeated yet and completed/cancelled/failed ones
		// should already be purged by PublishAndRemove.
		if entry.Status != string(domain.GenerationRunStatusStreaming) {
			continue
		}
		sessionID, err := uuid.Parse(sessionIDStr)
		if err != nil {
			continue
		}

		persisted, err := r.runStore.GetRunByID(ctx, tenantID, entry.RunID)
		if err != nil {
			continue
		}
		last := persisted.LastHeartbeatAt()
		if last.IsZero() {
			// Never heartbeated — use StartedAt as the liveness proxy
			// so a worker that crashes before its first tick still
			// gets reaped.
			last = persisted.StartedAt()
		}
		if last.After(cutoff) {
			continue
		}
		r.failStaleRun(ctx, tenantID, sessionID, entry.RunID, last)
	}
	return nil
}

// failStaleRun transitions one run into the failed terminal state and
// publishes the matching events. Best-effort: each step swallows
// errors so a failure in one step (e.g. Redis connection blip) does
// not abort the whole sweep.
func (r *RunReaper) failStaleRun(ctx context.Context, tenantID, sessionID, runID uuid.UUID, lastSeen time.Time) {
	r.logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID.String(),
		"session_id": sessionID.String(),
		"run_id":     runID.String(),
		"last_seen":  lastSeen.Format(time.RFC3339Nano),
	}).Warn("reaping stale bichat run")

	_ = r.runStore.FailRun(ctx, tenantID, sessionID, runID)

	if r.eventLog != nil {
		errPayload := httpdto.StreamChunkPayload{
			Type:      "error",
			Error:     "generation failed: worker stopped heartbeating",
			Timestamp: r.now().UnixMilli(),
		}
		body, err := json.Marshal(errPayload)
		if err == nil {
			_, _ = r.eventLog.Append(ctx, tenantID, runID, RunEvent{
				Type:    "error",
				Payload: body,
			})
			// Drop the log soon; the sidebar has already updated and
			// reconnecting clients will get the terminal event during
			// the grace window.
			_ = r.eventLog.DropAfterTerminal(ctx, tenantID, runID, 5*time.Minute)
		}
	}

	if r.index != nil {
		_ = r.index.PublishAndRemove(ctx, tenantID, ActiveRunStatus{
			SessionID: sessionID,
			RunID:     runID,
			Status:    string(domain.GenerationRunStatusFailed),
			UpdatedAt: r.now().UTC(),
		})
	}
}

// parseTenantFromActiveRunKey extracts the tenant uuid from a
// bichat:active-runs:{tenant} key. Returns false for keys that don't
// match the expected shape (events channel, etc).
func parseTenantFromActiveRunKey(prefix, key string) (uuid.UUID, bool) {
	remainder := strings.TrimPrefix(key, prefix+":")
	if remainder == key {
		return uuid.Nil, false
	}
	// Skip pubsub event channel keys: bichat:active-runs:events:<tenant>.
	if strings.HasPrefix(remainder, "events:") {
		return uuid.Nil, false
	}
	tenantID, err := uuid.Parse(remainder)
	if err != nil {
		return uuid.Nil, false
	}
	return tenantID, true
}
