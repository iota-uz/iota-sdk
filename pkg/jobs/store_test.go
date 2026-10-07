package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore() *memoryStore {
	return NewMemoryStore(time.Hour).(*memoryStore)
}

func TestMemoryStore_RoundTrip(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	j := Job{ID: uuid.New(), TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), UserID: 7, Kind: "export", Status: StatusQueued, CreatedAt: time.Now()}

	require.NoError(t, s.Create(ctx, j))

	got, ok, err := s.Get(ctx, j.ID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "export", got.Kind)

	require.NoError(t, s.Delete(ctx, j.ID))
	_, ok, err = s.Get(ctx, j.ID)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMemoryStore_ResultRoundTrip(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	id := uuid.New()

	_, _, ok, err := s.GetResult(ctx, id)
	require.NoError(t, err)
	assert.False(t, ok)

	require.NoError(t, s.SaveResult(ctx, id, "report.xlsx", []byte("bytes")))

	name, data, ok, err := s.GetResult(ctx, id)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "report.xlsx", name)
	assert.Equal(t, "bytes", string(data))
}

func TestMemoryStore_ListByUserNewestFirstAndScoped(t *testing.T) {
	s := newTestStore()
	ctx := context.Background()
	tenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	for i := 0; i < 3; i++ {
		require.NoError(t, s.Create(ctx, Job{
			ID:        uuid.New(),
			TenantID:  tenant,
			UserID:    7,
			Kind:      "export",
			Status:    StatusDone,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}))
	}
	require.NoError(t, s.Create(ctx, Job{ID: uuid.New(), TenantID: tenant, UserID: 8, Kind: "export", Status: StatusDone, CreatedAt: time.Now()}))

	list, err := s.ListByUser(ctx, tenant, 7, 10)
	require.NoError(t, err)
	require.Len(t, list, 3)
	for i := 1; i < len(list); i++ {
		assert.True(t, list[i-1].CreatedAt.After(list[i].CreatedAt))
	}

	limited, err := s.ListByUser(ctx, tenant, 7, 2)
	require.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestMemoryStore_EvictsTerminalAfterRetention(t *testing.T) {
	s := newTestStore()
	s.retention = time.Hour
	s.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	ctx := context.Background()
	id := uuid.New()
	require.NoError(t, s.Create(ctx, Job{
		ID:         id,
		TenantID:   uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UserID:     7,
		Kind:       "export",
		Status:     StatusDone,
		CreatedAt:  time.Now(),
		FinishedAt: time.Now(),
	}))
	require.NoError(t, s.SaveResult(ctx, id, "f.xlsx", []byte("x")))

	// Any write triggers pruning.
	require.NoError(t, s.Create(ctx, Job{ID: uuid.New(), TenantID: uuid.MustParse("11111111-1111-1111-1111-111111111111"), UserID: 7, Kind: "export", Status: StatusQueued, CreatedAt: time.Now()}))

	_, ok, err := s.Get(ctx, id)
	require.NoError(t, err)
	assert.False(t, ok)
	_, _, ok, err = s.GetResult(ctx, id)
	require.NoError(t, err)
	assert.False(t, ok)
}
