package services

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestActiveRunIndex(t *testing.T) (*RedisActiveRunIndex, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	idx, err := NewRedisActiveRunIndex(RedisActiveRunIndexConfig{Client: client})
	require.NoError(t, err)
	return idx, client
}

func TestActiveRunIndex_UpsertAndSnapshot(t *testing.T) {
	t.Parallel()

	idx, _ := newTestActiveRunIndex(t)
	tenant := uuid.New()

	entry := ActiveRunStatus{
		SessionID: uuid.New(),
		RunID:     uuid.New(),
		Status:    "streaming",
	}
	require.NoError(t, idx.Upsert(context.Background(), tenant, entry))

	snap, err := idx.Snapshot(context.Background(), tenant)
	require.NoError(t, err)
	require.Len(t, snap, 1)
	assert.Equal(t, entry.SessionID, snap[0].SessionID)
	assert.Equal(t, entry.RunID, snap[0].RunID)
	assert.Equal(t, "streaming", snap[0].Status)
	assert.False(t, snap[0].UpdatedAt.IsZero(), "Upsert must stamp UpdatedAt when missing")
}

func TestActiveRunIndex_UpsertOverwritesStatus(t *testing.T) {
	t.Parallel()

	idx, _ := newTestActiveRunIndex(t)
	tenant := uuid.New()
	session := uuid.New()

	require.NoError(t, idx.Upsert(context.Background(), tenant, ActiveRunStatus{
		SessionID: session,
		RunID:     uuid.New(),
		Status:    "queued",
	}))
	require.NoError(t, idx.Upsert(context.Background(), tenant, ActiveRunStatus{
		SessionID: session,
		RunID:     uuid.New(),
		Status:    "streaming",
	}))

	snap, err := idx.Snapshot(context.Background(), tenant)
	require.NoError(t, err)
	require.Len(t, snap, 1, "same session must have exactly one live entry")
	assert.Equal(t, "streaming", snap[0].Status)
}

func TestActiveRunIndex_PublishAndRemoveClearsEntry(t *testing.T) {
	t.Parallel()

	idx, _ := newTestActiveRunIndex(t)
	tenant := uuid.New()
	session := uuid.New()

	require.NoError(t, idx.Upsert(context.Background(), tenant, ActiveRunStatus{
		SessionID: session,
		RunID:     uuid.New(),
		Status:    "streaming",
	}))
	require.NoError(t, idx.PublishAndRemove(context.Background(), tenant, ActiveRunStatus{
		SessionID: session,
		RunID:     uuid.New(),
		Status:    "completed",
	}))

	snap, err := idx.Snapshot(context.Background(), tenant)
	require.NoError(t, err)
	assert.Empty(t, snap, "PublishAndRemove must drop the entry so late snapshots don't see a stale streaming state")
}

func TestActiveRunIndex_SnapshotIsTenantScoped(t *testing.T) {
	t.Parallel()

	idx, _ := newTestActiveRunIndex(t)
	tenantA := uuid.New()
	tenantB := uuid.New()

	require.NoError(t, idx.Upsert(context.Background(), tenantA, ActiveRunStatus{
		SessionID: uuid.New(), RunID: uuid.New(), Status: "streaming",
	}))
	require.NoError(t, idx.Upsert(context.Background(), tenantB, ActiveRunStatus{
		SessionID: uuid.New(), RunID: uuid.New(), Status: "streaming",
	}))

	snapA, err := idx.Snapshot(context.Background(), tenantA)
	require.NoError(t, err)
	require.Len(t, snapA, 1)

	snapB, err := idx.Snapshot(context.Background(), tenantB)
	require.NoError(t, err)
	require.Len(t, snapB, 1)

	assert.NotEqual(t, snapA[0].SessionID, snapB[0].SessionID,
		"tenants must not see each other's active runs")
}

func TestActiveRunIndex_SubscribeReceivesDeltas(t *testing.T) {
	t.Parallel()

	idx, _ := newTestActiveRunIndex(t)
	tenant := uuid.New()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch, err := idx.Subscribe(ctx, tenant)
	require.NoError(t, err)

	// Subscribe returns after the subscription handshake, so Publishes
	// issued after this point are guaranteed to be delivered.
	entry := ActiveRunStatus{
		SessionID: uuid.New(),
		RunID:     uuid.New(),
		Status:    "streaming",
	}
	require.NoError(t, idx.Upsert(context.Background(), tenant, entry))

	select {
	case got, ok := <-ch:
		require.True(t, ok, "subscribe channel closed before event")
		assert.Equal(t, entry.SessionID, got.SessionID)
		assert.Equal(t, "streaming", got.Status)
	case <-ctx.Done():
		t.Fatal("timed out waiting for subscribe delta")
	}
}
