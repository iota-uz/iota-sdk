package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/iota-uz/iota-sdk/pkg/lens/resource"

	"github.com/stretchr/testify/require"

	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
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
	spec := lensbuild.Dashboard("cached", "Cached", lensbuild.Row(panel.Bar("chart", "Chart", "data").Terminal().Build())).Datasets(lensbuild.QueryDataset("data", "primary", "select 1")).Build()
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

func TestRuntimeShellCacheProvenanceAndRecompute(t *testing.T) {
	t.Parallel()
	runtime := New(Options{})
	spec := lensbuild.Dashboard("progressive", "Progressive", lensbuild.Row(
		panel.Bar("chart", "Chart", "data").Terminal().Build(),
	)).Datasets(lensbuild.QueryDataset("data", "primary", "select 1")).Build()
	ds := &stubDataSource{}
	req := Request{
		DataScope: "tenant:a", Namespace: "test",
		DataSources:          map[string]datasource.DataSource{"primary": ds},
		DataSourceIdentities: map[string]string{"primary": "primary:v1"},
	}
	shell, err := runtime.Execute(context.Background(), spec, req, ShellScope())
	require.NoError(t, err)
	require.Empty(t, shell.Datasets)
	require.Empty(t, shell.Panels)
	require.Equal(t, int32(0), ds.calls.Load())

	first, err := runtime.Execute(context.Background(), spec, req, PanelScope("chart"))
	require.NoError(t, err)
	require.False(t, first.CacheHit)
	second, err := runtime.Execute(context.Background(), spec, req, PanelScope("chart"))
	require.NoError(t, err)
	require.True(t, second.CacheHit)
	require.Equal(t, int32(1), ds.calls.Load())

	req.Recompute = true
	refreshed, err := runtime.Execute(context.Background(), spec, req, PanelScope("chart"))
	require.NoError(t, err)
	require.False(t, refreshed.CacheHit)
	require.Equal(t, int32(2), ds.calls.Load())
}

func TestRuntimeSnapshot_UsesShortestDatasetTTL(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.July, 19, 0, 0, 0, 0, time.UTC)
	store := NewMemorySnapshotStore(MemoryStoreOptions{TTL: time.Hour, Clock: func() time.Time { return now }})
	runtime := New(Options{Store: store, DefaultTTL: time.Hour})
	spec := lensbuild.Dashboard("cached", "Cached", lensbuild.Row(panel.Bar("chart", "Chart", "data").Terminal().Build())).Datasets(lensbuild.QueryDataset("data", "primary", "select 1")).Build()
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

func TestMemorySnapshotBudgetUpdateRetainedAlias(t *testing.T) {
	// Real frame cells and a callback-retained result must not mutate cache state.
	budget := resource.NewBudget(100000)
	store := NewMemorySnapshotStore(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 20000})
	cells := map[string][]string{"x": {"before"}}
	fr, err := frame.New("typed", frame.Field{Name: "x", Type: frame.FieldTypeUnknown, Values: []any{cells}})
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	require.NoError(t, store.SaveChecked(t.Context(), "a", &ExecutionSnapshot{Datasets: map[string]*frame.FrameSet{"x": frames}}, 0))
	cells["x"][0] = "producer"
	var retained *ExecutionSnapshot
	require.NoError(t, store.UpdateChecked(t.Context(), "a", 0, func(s *ExecutionSnapshot) *ExecutionSnapshot { retained = s; return s }))
	retained.Datasets["x"].Primary().Fields[0].Values[0].(map[string][]string)["x"][0] = "callback"
	loaded, ok := store.Load(t.Context(), "a")
	require.True(t, ok)
	require.Equal(t, "before", loaded.Datasets["x"].Primary().Fields[0].Values[0].(map[string][]string)["x"][0])
	used := budget.Used()
	store.Save(t.Context(), "a", &ExecutionSnapshot{Variables: map[string]any{"x": strings.Repeat("x", 20000)}}, 0)
	require.EqualValues(t, 1, store.Stats().Rejections)
	require.Equal(t, used, budget.Used())
	store.Invalidate(t.Context(), "")
	require.Zero(t, budget.Used())
}

func TestMemorySnapshotAdmissionEvictsAndSweepsExpired(t *testing.T) {
	// Fresh Save must recover capacity from cold expired entries without Load.
	budget := resource.NewBudget(2200)
	now := time.Now()
	store := NewMemorySnapshotStore(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 2200, TTL: time.Minute, Clock: func() time.Time { return now }})
	snapshot := func(id string) *ExecutionSnapshot {
		return &ExecutionSnapshot{ID: id, Variables: map[string]any{"x": strings.Repeat("x", 180)}}
	}
	require.NoError(t, store.SaveChecked(t.Context(), "a", snapshot("a"), 0))
	require.NoError(t, store.SaveChecked(t.Context(), "b", snapshot("b"), 0))
	_, ok := store.Load(t.Context(), "a")
	require.False(t, ok)
	now = now.Add(2 * time.Minute)
	require.NoError(t, store.SaveChecked(t.Context(), "c", snapshot("c"), 0))
	_, ok = store.Load(t.Context(), "b")
	require.False(t, ok)
	require.Len(t, store.items, 1)
}

func TestMemorySnapshotSharedPressurePreservesLiveEntries(t *testing.T) {
	// A foreign budget owner must not cause destructive local eviction for an impossible request.
	budget := resource.NewBudget(100000)
	store := NewMemorySnapshotStore(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 100000})
	require.NoError(t, store.SaveChecked(t.Context(), "kept", &ExecutionSnapshot{ID: "kept"}, 0))
	foreign, err := budget.Reserve(budget.Limit() - budget.Used())
	require.NoError(t, err)
	defer foreign.Release()
	used := budget.Used()
	require.ErrorIs(t, store.SaveChecked(t.Context(), "large", &ExecutionSnapshot{Variables: map[string]any{"x": strings.Repeat("x", 10000)}}, 0), resource.ErrLimit)
	got, ok := store.Load(t.Context(), "kept")
	require.True(t, ok)
	require.Equal(t, "kept", got.ID)
	require.Equal(t, used, budget.Used())
	require.Zero(t, store.Stats().Evictions)
	require.ErrorIs(t, store.UpdateChecked(t.Context(), "kept", 0, func(snapshot *ExecutionSnapshot) *ExecutionSnapshot {
		snapshot.Variables = map[string]any{"x": strings.Repeat("x", 10000)}
		return snapshot
	}), resource.ErrLimit)
	got, ok = store.Load(t.Context(), "kept")
	require.True(t, ok)
	require.Empty(t, got.Variables)
	require.Equal(t, used, budget.Used())
}

func TestMemorySnapshotReplacementReclaimsColdEntry(t *testing.T) {
	// Previous single-reservation replacement could not reclaim another local cache entry.
	budget := resource.NewBudget(100000)
	store := NewMemorySnapshotStore(MemoryStoreOptions{Budget: budget, MaxEntryBytes: 100000})
	require.NoError(t, store.SaveChecked(t.Context(), "cold", &ExecutionSnapshot{Variables: map[string]any{"x": strings.Repeat("x", 10000)}}, 0))
	require.NoError(t, store.SaveChecked(t.Context(), "target", &ExecutionSnapshot{ID: "target"}, 0))
	foreign, err := budget.Reserve(budget.Limit() - budget.Used())
	require.NoError(t, err)
	defer foreign.Release()
	require.NoError(t, store.UpdateChecked(t.Context(), "target", 0, func(snapshot *ExecutionSnapshot) *ExecutionSnapshot {
		snapshot.Variables = map[string]any{"x": strings.Repeat("y", 1000)}
		return snapshot
	}))
	got, ok := store.Load(t.Context(), "target")
	require.True(t, ok)
	require.Len(t, got.Variables["x"], 1000)
	_, ok = store.Load(t.Context(), "cold")
	require.False(t, ok)
	require.EqualValues(t, 1, store.Stats().Evictions)
	require.LessOrEqual(t, budget.Used(), budget.Limit())
}

func TestRuntimeSnapshotOwnsDecimalCoefficient(t *testing.T) {
	t.Parallel()
	amount := decimal.RequireFromString("12345678901234567890.123456789")
	original := amount.String()
	store := NewMemorySnapshotStore(MemoryStoreOptions{})
	store.Save(t.Context(), "decimal", &ExecutionSnapshot{Variables: map[string]any{"amount": amount, "pointer": &amount}}, time.Minute)
	replacement, err := decimal.New(42, -1).MarshalBinary()
	require.NoError(t, err)
	require.NoError(t, amount.UnmarshalBinary(replacement))
	snapshot, ok := store.Load(t.Context(), "decimal")
	require.True(t, ok)
	require.Equal(t, original, snapshot.Variables["amount"].(decimal.Decimal).String())
	require.Equal(t, original, snapshot.Variables["pointer"].(*decimal.Decimal).String())
}
