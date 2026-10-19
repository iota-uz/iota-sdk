package serve

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func reportFixture(t *testing.T) (ReportKey, ReportDefinition[string]) {
	t.Helper()
	spec, frames := testDashboard(t)
	return ReportKey{Namespace: "reports", DataScope: "tenant:a", Version: "sales-v1", Locale: "en"}, ReportDefinition[string]{Config: Config{Spec: spec, Engine: &fakeExecutor{frames: frames}, Snapshots: document.NewMemoryStore(time.Minute, 128), BasePath: "/dash"}, Request: lensruntime.Request{Namespace: "reports", DataScope: "tenant:a", Locale: "en"}, Options: document.BuildOptions{I18n: map[string]string{"owned": "base"}}, State: "one"}
}
func newReportCache(t *testing.T, opts ReportCacheOptions[string]) *ReportCache[string] {
	t.Helper()
	if opts.Budget == nil {
		opts.Budget = resource.NewBudget(8 << 20)
	}
	if opts.MaxPendingBuilds == 0 {
		opts.MaxPendingBuilds = 8
	}
	cache, err := NewReportCache(opts)
	require.NoError(t, err)
	t.Cleanup(cache.Close)
	return cache
}

func TestReportCache_RepeatedDocumentsOwnNestedState(t *testing.T) {
	t.Parallel()
	// Keep key and snapshot identical; fresh keys would hide cache aliasing.
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	first, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer first.Release()
	originalTitle := first.Document.Meta.Title
	first.Document.I18n["owned"] = "changed"
	first.Document.Layout.Rows[0].Panels[0].Groups = append(first.Document.Layout.Rows[0].Panels[0].Groups, document.LayoutGroup{ID: "injected"})
	definition.Options.I18n["owned"] = "producer-changed"
	definition.Config.Spec.Title = "producer-changed"
	second, ok, err := cache.Lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, ok)
	defer second.Release()
	require.Equal(t, first.Document.SnapshotID, second.Document.SnapshotID)
	require.Equal(t, "base", second.Document.I18n["owned"])
	require.Equal(t, originalTitle, second.Document.Meta.Title)
	require.Empty(t, second.Document.Layout.Rows[0].Panels[0].Groups)
	ownedSpec, err := second.Spec()
	require.NoError(t, err)
	require.NotEqual(t, "producer-changed", ownedSpec.Title)
	ownedSpec.Rows[0].Panels[0].Title = "mutated-owned-spec"
	again, err := second.Spec()
	require.NoError(t, err)
	require.NotEqual(t, "mutated-owned-spec", again.Rows[0].Panels[0].Title)
}

func TestReportCache_ConcurrentColdReadersAndCancellation(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	entered, unblock := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	build := func(context.Context) (ReportDefinition[string], error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-unblock
		return definition, nil
	}
	cancelled, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan error, 1)
	go func() {
		r, err := cache.Materialize(cancelled, key, build)
		if r != nil {
			r.Release()
		}
		firstDone <- err
	}()
	<-entered
	secondDone := make(chan *CachedReport[string], 1)
	go func() { r, _ := cache.Materialize(context.Background(), key, build); secondDone <- r }()
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		for _, f := range cache.flights {
			return f.waiters == 2
		}
		return false
	}, time.Second, time.Millisecond)
	cancel()
	require.ErrorIs(t, <-firstDone, context.Canceled)
	close(unblock)
	second := <-secondDone
	require.NotNil(t, second)
	defer second.Release()
	require.EqualValues(t, 1, calls.Load())
	second.Document.I18n["owned"] = "reader"
	third, ok, err := cache.Lookup(context.Background(), key)
	require.NoError(t, err)
	require.True(t, ok)
	defer third.Release()
	require.Equal(t, "base", third.Document.I18n["owned"])
}

func TestReportCache_EvictionDefersStateCleanupUntilLeaseRelease(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	var evictions atomic.Int32
	cache := newReportCache(t, ReportCacheOptions[string]{MaxEntries: 1, OnEvict: func(string) { evictions.Add(1) }})
	first, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	initial := cache.opts.Budget.Used()
	require.Positive(t, initial)
	key.Params = "different"
	definition.State = "two"
	second, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer second.Release()
	require.Zero(t, evictions.Load())
	require.Greater(t, cache.opts.Budget.Used(), initial)
	_, err = definition.Config.Snapshots.Get(context.Background(), first.Document.SnapshotID)
	require.NoError(t, err)
	beforeRelease := cache.opts.Budget.Used()
	first.Release()
	first.Release()
	require.EqualValues(t, 1, evictions.Load())
	require.Equal(t, beforeRelease-initial, cache.opts.Budget.Used())
	_, err = definition.Config.Snapshots.Get(context.Background(), first.Document.SnapshotID)
	require.ErrorIs(t, err, document.ErrSnapshotGone)
}

func TestReportCache_FailedPrepareRemovesSnapshotAndState(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	var evictions atomic.Int32
	var id string
	definition.Prepare = func(doc *document.DashboardDocument) error {
		id = doc.SnapshotID
		return fmt.Errorf("invalid host decoration")
	}
	cache := newReportCache(t, ReportCacheOptions[string]{OnEvict: func(string) { evictions.Add(1) }})
	_, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.Error(t, err)
	require.Eventually(t, func() bool { return evictions.Load() == 1 }, time.Second, time.Millisecond)
	require.NotEmpty(t, id)
	_, err = definition.Config.Snapshots.Get(context.Background(), id)
	require.ErrorIs(t, err, document.ErrSnapshotGone)
	require.Zero(t, cache.Stats().Entries)
	require.Zero(t, cache.opts.Budget.Used())
	definition.Prepare = nil
	rebuilt, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	rebuilt.Release()
}

func TestReportCache_DerivedDistinctQueueBoundAndOwnedHits(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{MaxConcurrentBuilds: 1, MaxPendingBuilds: 1})
	parent, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer parent.Release()
	entered, unblock := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	build := func(context.Context) (*document.DashboardDocument, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-unblock
		return &document.DashboardDocument{SnapshotID: parent.Document.SnapshotID, I18n: map[string]string{"owned": "base"}}, nil
	}
	done := make(chan error, 3)
	go func() {
		_, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "first", DerivedSurfaceInteractive, build)
		done <- err
	}()
	<-entered
	go func() {
		_, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "second", DerivedSurfaceInteractive, build)
		done <- err
	}()
	require.Eventually(t, func() bool { return cache.Stats().Flights == 2 }, time.Second, time.Millisecond)
	for i := 0; i < 100; i++ {
		_, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, fmt.Sprint("extra", i), DerivedSurfaceInteractive, build)
		require.ErrorIs(t, err, resource.ErrLimit)
	}
	go func() {
		_, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "first", DerivedSurfaceInteractive, build)
		done <- err
	}()
	close(unblock)
	for i := 0; i < 3; i++ {
		require.NoError(t, <-done)
	}
	require.EqualValues(t, 2, calls.Load())
	first, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "first", DerivedSurfaceInteractive, build)
	require.NoError(t, err)
	first.I18n["owned"] = "mutated"
	again, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "first", DerivedSurfaceInteractive, build)
	require.NoError(t, err)
	require.Equal(t, "base", again.I18n["owned"])
}

type refusingReportStore struct{ document.SnapshotStore }

func (s refusingReportStore) Put(context.Context, *document.Snapshot) error {
	return resource.NewLimitError("entry bytes", 1000, 100)
}
func TestHandlers_DocumentResourceRejectionIsActionable(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlersWithStore(t, 0, refusingReportStore{document.NewMemoryStore(time.Minute, 8)}, nil)
	response := httptest.NewRecorder()
	handlers.Document(response, httptest.NewRequest(http.MethodGet, "/dash/document", nil))
	require.Equal(t, http.StatusRequestEntityTooLarge, response.Code, response.Body.String())
	require.Contains(t, response.Body.String(), "resource_exhausted")
}
