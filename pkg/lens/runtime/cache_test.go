package runtime

import (
	"context"
	"testing"
	"time"

	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestMemorySnapshotStoreTTLBoundedAndCloneSafe(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewMemorySnapshotStore(MemoryStoreOptions{TTL: time.Minute, MaxEntries: 1, Clock: func() time.Time { return now }})
	frames := mustFrameSet(t, "cached")
	store.Save(context.Background(), "a", &ExecutionSnapshot{Datasets: map[string]*frame.FrameSet{"data": frames}}, 0)
	loaded, ok := store.Load(context.Background(), "a")
	require.True(t, ok)
	loaded.Datasets["data"].Primary().Fields[0].Values[0] = "mutated"
	again, ok := store.Load(context.Background(), "a")
	require.True(t, ok)
	require.NotEqual(t, "mutated", again.Datasets["data"].Primary().Fields[0].Values[0])
	store.Save(context.Background(), "b", &ExecutionSnapshot{Datasets: map[string]*frame.FrameSet{}}, 0)
	_, ok = store.Load(context.Background(), "a")
	require.False(t, ok)
	now = now.Add(2 * time.Minute)
	_, ok = store.Load(context.Background(), "b")
	require.False(t, ok)
	stats := store.Stats()
	require.Equal(t, uint64(1), stats.Evictions)
	require.Equal(t, uint64(1), stats.Expirations)
}

func TestRuntimeSnapshotIdentityIncludesDataScope(t *testing.T) {
	t.Parallel()
	runtime := New(Options{})
	spec := lensbuild.Dashboard("cached", "Cached", lensbuild.Row(panel.Bar("chart", "Chart", "data").Build())).Datasets(lensbuild.QueryDataset("data", "primary", "select 1")).Build()
	ds := &stubDataSource{}
	request := func(scope string) Request {
		return Request{DataScope: scope, Namespace: "test", DataSources: map[string]datasource.DataSource{"primary": ds}, DataSourceIdentities: map[string]string{"primary": "primary:v1"}}
	}
	_, err := runtime.Execute(context.Background(), spec, request("tenant:a"), DashboardScope())
	require.NoError(t, err)
	_, err = runtime.Execute(context.Background(), spec, request("tenant:a"), DashboardScope())
	require.NoError(t, err)
	_, err = runtime.Execute(context.Background(), spec, request("tenant:b"), DashboardScope())
	require.NoError(t, err)
	require.Equal(t, int32(2), ds.calls.Load())
}

func TestRuntimeSnapshot_UsesShortestDatasetTTL(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC)
	store := NewMemorySnapshotStore(MemoryStoreOptions{TTL: time.Hour, Clock: func() time.Time { return now }})
	runtime := New(Options{Store: store, DefaultTTL: time.Hour})
	spec := lensbuild.Dashboard("cached", "Cached", lensbuild.Row(panel.Bar("chart", "Chart", "data").Build())).Datasets(lensbuild.QueryDataset("data", "primary", "select 1")).Build()
	spec.Datasets[0].Cache.TTL = time.Minute
	ds := &stubDataSource{}
	req := Request{DataScope: "tenant:a", Namespace: "test", DataSources: map[string]datasource.DataSource{"primary": ds}, DataSourceIdentities: map[string]string{"primary": "primary:v1"}}
	_, err := runtime.Execute(context.Background(), spec, req, DashboardScope())
	require.NoError(t, err)
	now = now.Add(2 * time.Minute)
	_, err = runtime.Execute(context.Background(), spec, req, DashboardScope())
	require.NoError(t, err)
	require.Equal(t, int32(2), ds.calls.Load())
}

func TestMemorySnapshotStore_DeepClonesTypedNestedVariables(t *testing.T) {
	t.Parallel()
	store := NewMemorySnapshotStore(MemoryStoreOptions{})
	type nested struct{ Values []int }
	variables := map[string]any{"rows": []map[string][]int{{"values": {1, 2}}}, "struct": nested{Values: []int{3, 4}}}
	store.Save(context.Background(), "core:key", &ExecutionSnapshot{Variables: variables}, time.Minute)
	loaded, ok := store.Load(context.Background(), "core:key")
	require.True(t, ok)
	loaded.Variables["rows"].([]map[string][]int)[0]["values"][0] = 99
	structValue := loaded.Variables["struct"].(nested)
	structValue.Values[0] = 88
	again, ok := store.Load(context.Background(), "core:key")
	require.True(t, ok)
	require.Equal(t, 1, again.Variables["rows"].([]map[string][]int)[0]["values"][0])
	require.Equal(t, 3, again.Variables["struct"].(nested).Values[0])
}

func TestMemorySnapshotStore_InvalidateMatchesNamespaceBoundary(t *testing.T) {
	t.Parallel()
	store := NewMemorySnapshotStore(MemoryStoreOptions{})
	store.Save(context.Background(), "core:key", &ExecutionSnapshot{}, time.Minute)
	store.Save(context.Background(), "corebank:key", &ExecutionSnapshot{}, time.Minute)
	store.Invalidate(context.Background(), "core")
	_, core := store.Load(context.Background(), "core:key")
	_, corebank := store.Load(context.Background(), "corebank:key")
	require.False(t, core)
	require.True(t, corebank)
}
