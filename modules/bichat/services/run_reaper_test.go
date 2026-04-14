package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildReaperHarness wires a reaper against a miniredis-backed store +
// event log + index so the sweep exercises the same code path used in
// production without a live Redis or Postgres.
func buildReaperHarness(t *testing.T, staleAfter time.Duration) (*RunReaper, *miniredis.Miniredis, generationRunStore, ActiveRunIndex, *RedisRunEventLog) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	store, err := newRedisGenerationRunStore(redisGenerationRunStoreConfig{Client: client})
	require.NoError(t, err)

	index, err := NewRedisActiveRunIndex(RedisActiveRunIndexConfig{Client: client})
	require.NoError(t, err)

	log, err := NewRedisRunEventLog(RedisRunEventLogConfig{Client: client})
	require.NoError(t, err)

	reaper, err := NewRunReaper(RunReaperConfig{
		Client:         client,
		RunStore:       store,
		EventLog:       log,
		ActiveRunIndex: index,
		PollInterval:   10 * time.Millisecond,
		StaleAfter:     staleAfter,
	})
	require.NoError(t, err)
	return reaper, mr, store, index, log
}

func createStreamingRun(t *testing.T, store generationRunStore, tenantID, sessionID uuid.UUID, heartbeat time.Time) uuid.UUID {
	t.Helper()
	run, err := domain.NewGenerationRun(domain.GenerationRunSpec{
		SessionID:       sessionID,
		TenantID:        tenantID,
		UserID:          1,
		StartedAt:       heartbeat,
		LastHeartbeatAt: heartbeat,
	})
	require.NoError(t, err)
	require.NoError(t, store.CreateRun(context.Background(), run))
	return run.ID()
}

func TestRunReaper_SweepFailsStaleRun(t *testing.T) {
	t.Parallel()

	reaper, _, store, index, log := buildReaperHarness(t, 50*time.Millisecond)
	tenant := uuid.New()
	session := uuid.New()

	// Heartbeat older than the stale threshold.
	runID := createStreamingRun(t, store, tenant, session, time.Now().Add(-time.Second))
	require.NoError(t, index.Upsert(context.Background(), tenant, ActiveRunStatus{
		SessionID: session,
		RunID:     runID,
		Status:    string(domain.GenerationRunStatusStreaming),
		UpdatedAt: time.Now(),
	}))

	// Seed a pending event so we can assert the reaper appends a
	// terminal error after it.
	_, err := log.Append(context.Background(), tenant, runID, RunEvent{
		Type:    "content",
		Payload: json.RawMessage(`{"text":"hi"}`),
	})
	require.NoError(t, err)

	require.NoError(t, reaper.sweep(context.Background()))

	// Run state: transitioned to failed.
	persisted, err := store.GetRunByID(context.Background(), tenant, runID)
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusFailed, persisted.Status(),
		"stale run must be moved to failed status")

	// Index: emptied (PublishAndRemove runs on terminal).
	snap, err := index.Snapshot(context.Background(), tenant)
	require.NoError(t, err)
	assert.Empty(t, snap, "reaped run must be cleared from the sidebar index")

	// Event log: final entry must be the error terminal.
	events, err := log.Replay(context.Background(), tenant, runID, "")
	require.NoError(t, err)
	require.NotEmpty(t, events)
	assert.Equal(t, "error", events[len(events)-1].Type,
		"reaper must append a terminal error event so tailing clients see the transition")
}

func TestRunReaper_SweepLeavesFreshRunAlone(t *testing.T) {
	t.Parallel()

	reaper, _, store, index, _ := buildReaperHarness(t, 10*time.Second)
	tenant := uuid.New()
	session := uuid.New()

	// Heartbeat "just now".
	runID := createStreamingRun(t, store, tenant, session, time.Now())
	require.NoError(t, index.Upsert(context.Background(), tenant, ActiveRunStatus{
		SessionID: session,
		RunID:     runID,
		Status:    string(domain.GenerationRunStatusStreaming),
		UpdatedAt: time.Now(),
	}))

	require.NoError(t, reaper.sweep(context.Background()))

	persisted, err := store.GetRunByID(context.Background(), tenant, runID)
	require.NoError(t, err)
	assert.Equal(t, domain.GenerationRunStatusStreaming, persisted.Status(),
		"fresh run must not be reaped")

	snap, err := index.Snapshot(context.Background(), tenant)
	require.NoError(t, err)
	assert.Len(t, snap, 1, "fresh run must stay in the sidebar index")
}

func TestRunReaper_SweepSkipsNonStreamingEntries(t *testing.T) {
	t.Parallel()

	reaper, _, _, index, _ := buildReaperHarness(t, 10*time.Millisecond)
	tenant := uuid.New()

	// A queued row older than stale — reaper must NOT touch queued
	// entries (they haven't started streaming yet so LastHeartbeatAt
	// semantics don't apply).
	require.NoError(t, index.Upsert(context.Background(), tenant, ActiveRunStatus{
		SessionID: uuid.New(),
		RunID:     uuid.New(),
		Status:    "queued",
		UpdatedAt: time.Now().Add(-time.Minute),
	}))

	require.NoError(t, reaper.sweep(context.Background()))

	snap, err := index.Snapshot(context.Background(), tenant)
	require.NoError(t, err)
	require.Len(t, snap, 1, "queued rows must be left alone by the reaper")
	assert.Equal(t, "queued", snap[0].Status)
}

func TestParseTenantFromActiveRunKey(t *testing.T) {
	t.Parallel()

	id := uuid.New()

	got, ok := parseTenantFromActiveRunKey("bichat:active-runs", "bichat:active-runs:"+id.String())
	require.True(t, ok)
	assert.Equal(t, id, got)

	// Pubsub event channel is not a hash — must be rejected.
	_, ok = parseTenantFromActiveRunKey("bichat:active-runs", "bichat:active-runs:events:"+id.String())
	assert.False(t, ok, "events channel keys must not be treated as tenant hashes")

	// Unrelated key — must be rejected.
	_, ok = parseTenantFromActiveRunKey("bichat:active-runs", "bichat:title:jobs")
	assert.False(t, ok)
}
