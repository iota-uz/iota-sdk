package document

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMemoryStore_PutGetAppendAreCloneSafe(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore(time.Hour, 2)
	snapshot := &Snapshot{
		ID:     "snapshot-1",
		Params: map[string]any{"filters": map[string]any{"tenant": "one"}},
		Frames: map[FrameRef]Frame{"root": testFrame(1)},
	}
	require.NoError(t, store.Put(context.Background(), snapshot))
	snapshot.Params["filters"].(map[string]any)["tenant"] = "mutated"
	snapshot.Frames["root"].Rows[0][0] = 99

	loaded, err := store.Get(context.Background(), "snapshot-1")
	require.NoError(t, err)
	require.Equal(t, "one", loaded.Params["filters"].(map[string]any)["tenant"])
	require.Equal(t, 1, loaded.Frames["root"].Rows[0][0])
	loaded.Params["filters"].(map[string]any)["tenant"] = "changed again"

	deeper := map[FrameRef]Frame{"detail": testFrame(2), "root": testFrame(3)}
	require.NoError(t, store.Append(context.Background(), "snapshot-1", deeper))
	deeper["detail"].Rows[0][0] = 100
	loaded, err = store.Get(context.Background(), "snapshot-1")
	require.NoError(t, err)
	require.Equal(t, "one", loaded.Params["filters"].(map[string]any)["tenant"])
	require.Equal(t, 1, loaded.Frames["root"].Rows[0][0], "append must not replace an already materialized frame")
	require.Equal(t, 2, loaded.Frames["detail"].Rows[0][0])
}

func TestMemoryStore_ExpiryAndUnknownSnapshots(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 19, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(time.Minute, 2).(*memoryStore)
	store.clock = func() time.Time { return now }
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "expiring"}))
	now = now.Add(time.Minute + time.Nanosecond)

	_, err := store.Get(context.Background(), "expiring")
	require.ErrorIs(t, err, ErrSnapshotGone)
	require.ErrorIs(t, store.Append(context.Background(), "expiring", map[FrameRef]Frame{"late": testFrame(1)}), ErrSnapshotGone)
	_, err = store.Get(context.Background(), "unknown")
	require.ErrorIs(t, err, ErrSnapshotGone)
}

func TestMemoryStore_DefaultsAndBound(t *testing.T) {
	t.Parallel()
	defaults := NewMemoryStore(0, 0).(*memoryStore)
	require.Equal(t, 30*time.Minute, defaults.ttl)
	require.Equal(t, defaultMaxEntries, defaults.maxEntries)

	store := NewMemoryStore(time.Hour, 1)
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "first"}))
	require.NoError(t, store.Put(context.Background(), &Snapshot{ID: "second"}))
	_, err := store.Get(context.Background(), "first")
	require.True(t, errors.Is(err, ErrSnapshotGone))
	_, err = store.Get(context.Background(), "second")
	require.NoError(t, err)
}

func TestMemoryStore_ContextAndValidation(t *testing.T) {
	t.Parallel()
	store := NewMemoryStore(time.Hour, 1)
	require.Error(t, store.Put(context.Background(), nil))
	require.Error(t, store.Put(context.Background(), &Snapshot{}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, store.Put(ctx, &Snapshot{ID: "canceled"}), context.Canceled)
	_, err := store.Get(ctx, "canceled")
	require.ErrorIs(t, err, context.Canceled)
}

func testFrame(value int) Frame {
	return Frame{Columns: []Column{{Name: "value", Type: ColumnNumber}}, Rows: [][]any{{value}}}
}
