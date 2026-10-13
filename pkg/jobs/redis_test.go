package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alicebob/miniredis/v2"
)

func newRedisStore(t *testing.T, retention time.Duration) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store, err := NewRedisStore(client, retention, WithStaleAfter(time.Hour))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return store, mr
}

func TestRedisStore_JobRoundTrip(t *testing.T) {
	store, _ := newRedisStore(t, time.Hour)
	ctx := context.Background()
	j := Job{
		ID:        uuid.New(),
		TenantID:  uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UserID:    7,
		Kind:      "report.export",
		Params:    map[string]any{"format": "xlsx"},
		Status:    StatusRunning,
		Progress:  42,
		Phase:     "writing rows",
		CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		UpdatedAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	require.NoError(t, store.Create(ctx, j))

	got, ok, err := store.Get(ctx, j.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, j.Kind, got.Kind)
	assert.Equal(t, StatusRunning, got.Status)
	assert.Equal(t, 42, got.Progress)
	assert.Equal(t, "writing rows", got.Phase)
	assert.Equal(t, "xlsx", got.Params["format"])

	j.Status = StatusDone
	j.Progress = 100
	require.NoError(t, store.Save(ctx, j))
	got, ok, err = store.Get(ctx, j.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, StatusDone, got.Status)

	require.NoError(t, store.Delete(ctx, j.ID))
	_, ok, err = store.Get(ctx, j.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestRedisStore_ActiveJobKeyHasTTL(t *testing.T) {
	store, mr := newRedisStore(t, time.Hour)
	ctx := context.Background()
	j := Job{ID: uuid.New(), TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), UserID: 7, Kind: "export", Status: StatusRunning, CreatedAt: time.Now()}
	require.NoError(t, store.Create(ctx, j))

	ttl := mr.TTL(jobKey(j.ID))
	assert.True(t, ttl > 0, "active job key must expire")

	j.Status = StatusDone
	j.FinishedAt = time.Now()
	require.NoError(t, store.Save(ctx, j))
	ttl = mr.TTL(jobKey(j.ID))
	assert.True(t, ttl > 0 && ttl <= time.Hour, "terminal job key must carry the retention TTL")
}

func TestRedisStore_ListByUserSkipsExpired(t *testing.T) {
	store, mr := newRedisStore(t, time.Hour)
	ctx := context.Background()
	tenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	first := Job{ID: uuid.New(), TenantID: tenant, UserID: 7, Kind: "a", Status: StatusDone, CreatedAt: time.Now().Add(-time.Minute)}
	second := Job{ID: uuid.New(), TenantID: tenant, UserID: 7, Kind: "b", Status: StatusDone, CreatedAt: time.Now()}
	require.NoError(t, store.Create(ctx, first))
	require.NoError(t, store.Create(ctx, second))

	list, err := store.ListByUser(ctx, tenant, 7, 10)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "b", list[0].Kind)

	// Expired job hash but stale index entry: listing skips it.
	mr.Del(jobKey(first.ID))
	list, err = store.ListByUser(ctx, tenant, 7, 10)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Other users see nothing.
	list, err = store.ListByUser(ctx, tenant, 8, 10)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestRedisStore_ResultRoundTrip(t *testing.T) {
	store, _ := newRedisStore(t, time.Hour)
	ctx := context.Background()
	j := Job{ID: uuid.New(), TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), UserID: 7, Kind: "export", Status: StatusDone, ResultName: "report.xlsx", CreatedAt: time.Now()}
	require.NoError(t, store.Create(ctx, j))

	_, _, ok, err := store.GetResult(ctx, j.ID)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, store.SaveResult(ctx, j.ID, "report.xlsx", []byte("xlsx-bytes")))

	name, data, ok, err := store.GetResult(ctx, j.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "report.xlsx", name)
	assert.Equal(t, "xlsx-bytes", string(data))
}

func TestRedisStore_TouchRefreshesUpdatedAt(t *testing.T) {
	store, _ := newRedisStore(t, time.Hour)
	ctx := context.Background()
	old := time.Now().Add(-time.Hour).UTC().Truncate(time.Millisecond)
	j := Job{ID: uuid.New(), TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), UserID: 7, Kind: "export", Status: StatusRunning, CreatedAt: old, UpdatedAt: old}
	require.NoError(t, store.Create(ctx, j))

	at := time.Now().UTC().Truncate(time.Millisecond)
	require.NoError(t, store.Touch(ctx, j.ID, at))

	got, ok, err := store.Get(ctx, j.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.WithinDuration(t, at, got.UpdatedAt, time.Second)
}
