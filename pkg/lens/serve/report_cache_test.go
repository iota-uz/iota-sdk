package serve

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
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

func TestReportCache_RepeatedSameSnapshotRetainsConstantPayload(t *testing.T) {
	t.Parallel()
	// Reusing one exact identity is essential: unique keys never exercise mutation reuse.
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	var calls atomic.Int32
	build := func(context.Context) (ReportDefinition[string], error) { calls.Add(1); return definition, nil }
	var snapshot string
	var retained int64
	for i := 0; i < 64; i++ {
		view, err := cache.Materialize(context.Background(), key, build)
		require.NoError(t, err)
		if i == 0 {
			snapshot = view.Document.SnapshotID
			retained = cache.Stats().RetainedBytes
		}
		require.Equal(t, snapshot, view.Document.SnapshotID)
		require.Equal(t, retained, cache.Stats().RetainedBytes)
		require.Equal(t, "base", view.Document.I18n["owned"])
		view.Document.I18n["owned"] = "mutated"
		view.Document.Layout.Rows[0].Panels[0].Groups = append(view.Document.Layout.Rows[0].Panels[0].Groups, document.LayoutGroup{ID: "growth"})
		view.Release()
	}
	require.EqualValues(t, 1, calls.Load())
	cache.Close()
	require.Zero(t, cache.opts.Budget.Used())
}

func TestReportCache_DerivedWireOnlyOwnsSourceAndBackedChildExpires(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	parent, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer parent.Release()
	source := &document.DashboardDocument{SnapshotID: "wire-only", I18n: map[string]string{"owned": "base"}}
	var wireCalls atomic.Int32
	buildWire := func(context.Context) (*document.DashboardDocument, error) { wireCalls.Add(1); return source, nil }
	first, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "wire", DerivedSurfaceInteractive, buildWire)
	require.NoError(t, err)
	source.I18n["owned"] = "producer"
	first.I18n["owned"] = "reader"
	again, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "wire", DerivedSurfaceInteractive, buildWire)
	require.NoError(t, err)
	require.Equal(t, "base", again.I18n["owned"])
	require.EqualValues(t, 1, wireCalls.Load())
	var backedCalls atomic.Int32
	buildBacked := func(ctx context.Context) (*document.DashboardDocument, error) {
		id := fmt.Sprint("backed-", backedCalls.Add(1))
		err := definition.Config.Snapshots.Put(ctx, &document.Snapshot{ID: id})
		return &document.DashboardDocument{SnapshotID: id}, err
	}
	backed, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "backed", DerivedSurfaceInteractive, buildBacked)
	require.NoError(t, err)
	require.NoError(t, definition.Config.Snapshots.(document.SnapshotDeleter).Delete(context.Background(), backed.SnapshotID))
	rebuilt, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "backed", DerivedSurfaceInteractive, buildBacked)
	require.NoError(t, err)
	require.NotEqual(t, backed.SnapshotID, rebuilt.SnapshotID)
	require.EqualValues(t, 2, backedCalls.Load())
}

func TestReportCache_ParentScopedChildrenSurviveChildLRUEvictionAndLeasedClose(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	build := func(context.Context) (ReportDefinition[string], error) { return definition, nil }
	parentOne, err := cache.Materialize(context.Background(), key, build)
	require.NoError(t, err)
	key.Params = "parent-two"
	parentTwo, err := cache.Materialize(context.Background(), key, build)
	require.NoError(t, err)
	childKey := key
	childKey.Params = "same-child"
	childBuilder := func(ctx context.Context) (*document.DashboardDocument, error) {
		child, err := cache.Materialize(ctx, childKey, build)
		if err != nil {
			return nil, err
		}
		// Force the exact return/adoption gap: the owner must already pin the child.
		cache.mu.Lock()
		entry := cache.bySnapshot[child.Document.SnapshotID]
		cache.mu.Unlock()
		cache.evict(entry)
		child.Release()
		return child.Document, nil
	}
	first, err := cache.MaterializeDerived(context.Background(), parentOne.Document.SnapshotID, "child", DerivedSurfaceInteractive, childBuilder)
	require.NoError(t, err)
	second, err := cache.MaterializeDerived(context.Background(), parentTwo.Document.SnapshotID, "child", DerivedSurfaceInteractive, childBuilder)
	require.NoError(t, err)
	require.NotEqual(t, first.SnapshotID, second.SnapshotID)
	firstLease, err := cache.LookupSnapshotRegistration(context.Background(), first.SnapshotID)
	require.NoError(t, err)
	firstLease.Release()
	cache.mu.Lock()
	one := cache.bySnapshot[parentOne.Document.SnapshotID]
	cache.mu.Unlock()
	cache.evict(one)
	parentOne.Release()
	_, err = definition.Config.Snapshots.Get(context.Background(), first.SnapshotID)
	require.ErrorIs(t, err, document.ErrSnapshotGone)
	secondLease, err := cache.LookupSnapshotRegistration(context.Background(), second.SnapshotID)
	require.NoError(t, err)
	parentTwo.Release()
	cache.Close()
	_, err = definition.Config.Snapshots.Get(context.Background(), second.SnapshotID)
	require.NoError(t, err)
	_, err = definition.Config.Snapshots.Get(context.Background(), parentTwo.Document.SnapshotID)
	require.NoError(t, err)
	secondLease.Release()
	require.Zero(t, cache.opts.Budget.Used())
	_, err = definition.Config.Snapshots.Get(context.Background(), second.SnapshotID)
	require.ErrorIs(t, err, document.ErrSnapshotGone)
}

func TestReportCache_CancelledDerivedRetainsAdmissionUntilActualCallbackExit(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	var cleaned atomic.Int32
	limiter := NewReportBuildLimiter(1, 0)
	cache := newReportCache(t, ReportCacheOptions[string]{BuildLimiter: limiter, OnEvict: func(string) { cleaned.Add(1) }})
	parent, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	entered, exit, callbackExited := make(chan struct{}), make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := cache.MaterializeDerived(ctx, parent.Document.SnapshotID, "slow", DerivedSurfaceInteractive, func(ctx context.Context) (*document.DashboardDocument, error) {
			close(entered)
			<-exit
			defer close(callbackExited)
			return nil, ctx.Err()
		})
		done <- err
	}()
	<-entered
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	parent.Release()
	cache.Close()
	limiter.mu.Lock()
	admitted := limiter.admitted
	limiter.mu.Unlock()
	require.Equal(t, 1, admitted)
	require.Zero(t, cleaned.Load())
	require.Positive(t, cache.opts.Budget.Used())
	close(exit)
	<-callbackExited
	require.Eventually(t, func() bool { return cleaned.Load() == 1 && cache.opts.Budget.Used() == 0 }, time.Second, time.Millisecond)
	limiter.mu.Lock()
	admitted = limiter.admitted
	limiter.mu.Unlock()
	require.Zero(t, admitted)
}

func TestReportCache_SharedLimiterSupportsNestedCrossCacheTypes(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	limiter := NewReportBuildLimiter(1, 0)
	budget := resource.NewBudget(8 << 20)
	parentCache := newReportCache(t, ReportCacheOptions[string]{BuildLimiter: limiter, Budget: budget})
	childCache, err := NewReportCache(ReportCacheOptions[int]{BuildLimiter: limiter, Budget: budget})
	require.NoError(t, err)
	defer childCache.Close()
	parent, err := parentCache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	childKey := key
	childKey.Namespace = "different-state-type"
	child, err := parentCache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "cross-cache", DerivedSurfaceInteractive, func(ctx context.Context) (*document.DashboardDocument, error) {
		childDef := ReportDefinition[int]{Config: definition.Config, Request: definition.Request, State: 42}
		childDef.Request.Namespace = childKey.Namespace
		view, err := childCache.Materialize(ctx, childKey, func(context.Context) (ReportDefinition[int], error) { return childDef, nil })
		if err != nil {
			return nil, err
		}
		defer view.Release()
		return view.Document, nil
	})
	require.NoError(t, err)
	lease, err := childCache.LookupSnapshotRegistration(context.Background(), child.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, 42, lease.State)
	require.Same(t, parent.Handlers.sessions, lease.Handlers.sessions)
	require.Same(t, parent.Handlers.session(parent.Document.SnapshotID), lease.Handlers.session(child.SnapshotID))
	lease.Release()
	parent.Release()
	parentCache.Close()
	childCache.Close()
	require.Zero(t, budget.Used())
}

func TestReportCache_ProducerIdentityAndPrepareIdentityAreProtected(t *testing.T) {
	t.Parallel()
	for _, replace := range []bool{false, true} {
		t.Run(fmt.Sprint(replace), func(t *testing.T) {
			t.Parallel()
			key, definition := reportFixture(t)
			cache := newReportCache(t, ReportCacheOptions[string]{})
			var original string
			if replace {
				definition.Prepare = func(doc *document.DashboardDocument) error {
					original = doc.SnapshotID
					doc.SnapshotID = "replaced"
					return nil
				}
			} else {
				definition.Options.SnapshotID = "caller-selected"
			}
			_, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
			require.Error(t, err)
			if original != "" {
				_, err = definition.Config.Snapshots.Get(context.Background(), original)
				require.ErrorIs(t, err, document.ErrSnapshotGone)
			}
			require.Eventually(t, func() bool { return cache.opts.Budget.Used() == 0 }, time.Second, time.Millisecond)
		})
	}
}

func TestReportCache_ColdCoalescedOwnersAdoptBeforeFirstOwnerRejects(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	parent, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer parent.Release()
	childKey := key
	childKey.Params = "shared-cold-child"
	entered, unblock, firstFailed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	childBuild := func(context.Context) (ReportDefinition[string], error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-unblock
		return definition, nil
	}
	firstDone := make(chan error, 1)
	go func() {
		_, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "rejecting-owner", DerivedSurfaceInteractive, func(ctx context.Context) (*document.DashboardDocument, error) {
			view, err := cache.Materialize(ctx, childKey, childBuild)
			if err != nil {
				return nil, err
			}
			view.Release()
			return nil, fmt.Errorf("host rejected child")
		})
		firstDone <- err
		close(firstFailed)
	}()
	<-entered
	secondDone := make(chan *document.DashboardDocument, 1)
	secondError := make(chan error, 1)
	go func() {
		doc, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "accepting-owner", DerivedSurfaceInteractive, func(ctx context.Context) (*document.DashboardDocument, error) {
			view, err := cache.Materialize(ctx, childKey, childBuild)
			if err != nil {
				return nil, err
			}
			defer view.Release()
			<-firstFailed
			return view.Document, nil
		})
		secondDone <- doc
		secondError <- err
	}()
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		for _, flight := range cache.flights {
			if flight.waiters == 2 {
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond)
	close(unblock)
	require.Error(t, <-firstDone)
	child := <-secondDone
	require.NoError(t, <-secondError)
	require.NotNil(t, child)
	require.EqualValues(t, 1, calls.Load())
	lease, err := cache.LookupSnapshotRegistration(context.Background(), child.SnapshotID)
	require.NoError(t, err)
	lease.Release()
}

func TestReportCache_BoundsSuccessfulCoalescedWaiters(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{MaxWaitersPerBuild: 2})
	entered, unblock := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	build := func(context.Context) (ReportDefinition[string], error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		<-unblock
		return definition, nil
	}
	views := make(chan *CachedReport[string], 2)
	for i := 0; i < 2; i++ {
		go func() { view, _ := cache.Materialize(context.Background(), key, build); views <- view }()
	}
	<-entered
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		for _, flight := range cache.flights {
			return flight.waiters == 2
		}
		return false
	}, time.Second, time.Millisecond)
	_, err := cache.Materialize(context.Background(), key, build)
	require.ErrorIs(t, err, resource.ErrLimit)
	status, ok := resource.StatusCode(err)
	require.True(t, ok)
	require.Equal(t, 503, status)
	close(unblock)
	first, second := <-views, <-views
	require.NotNil(t, first)
	require.NotNil(t, second)
	defer first.Release()
	defer second.Release()
	first.Document.I18n["owned"] = "first"
	require.Equal(t, "base", second.Document.I18n["owned"])
	require.EqualValues(t, 1, calls.Load())
}

func TestReportCache_WireOnlyCustomExportAndMissingProgressiveBacking(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	parent, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer parent.Release()
	exported, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "external-export", DerivedSurfaceInteractive, func(context.Context) (*document.DashboardDocument, error) {
		return &document.DashboardDocument{SnapshotID: "inline-export", Endpoints: document.Endpoints{Export: "/business/export"}}, nil
	})
	require.NoError(t, err)
	require.Equal(t, "/business/export", exported.Endpoints.Export)
	_, err = cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "missing-panel", DerivedSurfaceInteractive, func(context.Context) (*document.DashboardDocument, error) {
		return &document.DashboardDocument{SnapshotID: "missing", Endpoints: document.Endpoints{Panel: "/panel"}}, nil
	})
	require.ErrorIs(t, err, document.ErrSnapshotGone)
}

func TestReportCache_FailedDerivedBuilderReleasesUnusedOwnedChildren(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	var childrenCleaned atomic.Int32
	cache := newReportCache(t, ReportCacheOptions[string]{OnEvict: func(state string) {
		if state == "child" {
			childrenCleaned.Add(1)
		}
	}})
	parent, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer parent.Release()
	baseline := cache.opts.Budget.Used()
	childID := ""
	childKey := key
	childKey.Params = "unused"
	_, err = cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "failure", DerivedSurfaceInteractive, func(ctx context.Context) (*document.DashboardDocument, error) {
		childDef := definition
		childDef.State = "child"
		child, err := cache.Materialize(ctx, childKey, func(context.Context) (ReportDefinition[string], error) { return childDef, nil })
		if err != nil {
			return nil, err
		}
		childID = child.Document.SnapshotID
		child.Release()
		return nil, fmt.Errorf("unused child")
	})
	require.Error(t, err)
	require.EqualValues(t, 1, childrenCleaned.Load())
	require.Equal(t, baseline, cache.opts.Budget.Used())
	_, err = definition.Config.Snapshots.Get(context.Background(), childID)
	require.ErrorIs(t, err, document.ErrSnapshotGone)
}

func TestReportCache_StaleLookupMissRechecksBeforeProducer(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	var calls atomic.Int32
	build := func(context.Context) (ReportDefinition[string], error) { calls.Add(1); return definition, nil }
	first, err := cache.Materialize(context.Background(), key, build)
	require.NoError(t, err)
	defer first.Release()
	canonical, err := key.canonical()
	require.NoError(t, err)
	// Model a request that observed a miss before the first flight published,
	// but entered its own coordinator after that flight had already completed.
	flight := &reportFlight[string]{done: make(chan struct{}), cancel: func() {}}
	cache.run(context.Background(), canonical, key, flight, build)
	<-flight.done
	require.NoError(t, flight.err)
	require.EqualValues(t, 1, calls.Load())
	require.Equal(t, first.Document.SnapshotID, flight.entry.snapshotID)
	cache.mu.Lock()
	entries := cache.lru.Len()
	cache.mu.Unlock()
	require.Equal(t, 1, entries)
}

func TestReportCache_ChildCannotOutliveExpiredParentSnapshot(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	build := func(context.Context) (ReportDefinition[string], error) { return definition, nil }
	parent, err := cache.Materialize(context.Background(), key, build)
	require.NoError(t, err)
	childKey := key
	childKey.Params = "child"
	child, err := cache.MaterializeDerived(context.Background(), parent.Document.SnapshotID, "parent-expiry", DerivedSurfaceInteractive, func(ctx context.Context) (*document.DashboardDocument, error) {
		view, err := cache.Materialize(ctx, childKey, build)
		if err != nil {
			return nil, err
		}
		defer view.Release()
		return view.Document, nil
	})
	require.NoError(t, err)
	parentID := parent.Document.SnapshotID
	parent.Release()
	require.NoError(t, definition.Config.Snapshots.(document.SnapshotDeleter).Delete(context.Background(), parentID))
	_, err = cache.LookupSnapshotRegistration(context.Background(), child.SnapshotID)
	require.ErrorIs(t, err, document.ErrSnapshotGone)
	require.Zero(t, cache.opts.Budget.Used())
}

func TestReportCache_ProjectsRendererIconCallbacksOutOfOwnedSpec(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	var renders atomic.Int32
	definition.Config.Spec.Rows[0].Panels[0].Chrome.Icon = chrome.Component(templ.ComponentFunc(func(context.Context, io.Writer) error { renders.Add(1); return nil }))
	definition.Config.Spec.Rows[0].Panels[0].Chrome.AccentColor = "blue"
	view, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer view.Release()
	owned, err := view.Spec()
	require.NoError(t, err)
	require.Zero(t, renders.Load())
	require.True(t, owned.Rows[0].Panels[0].Chrome.Icon.Empty())
	require.Equal(t, "blue", owned.Rows[0].Panels[0].Chrome.AccentColor)
	require.False(t, definition.Config.Spec.Rows[0].Panels[0].Chrome.Icon.Empty())
}

func TestReportCache_PreparationOwnsProducerControlsBeforeMutation(t *testing.T) {
	t.Parallel()
	// Mutating the callback input must not feed reusable producer options into
	// another cache miss; changing only egress copies would miss this boundary.
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	definition.Options.Header = &document.DocumentHeader{Title: "producer"}
	definition.Options.Filters = []document.Filter{{ID: "product", Kind: document.FilterKindFacet, Facet: &document.FacetFilter{Dimension: "product", OptionsEndpoint: "/options", Selections: []document.FacetSelection{{Label: "producer", RemoveURL: "/remove"}}}}}
	definition.Options.Theme = document.Theme{Palette: map[string]string{"brand": "producer"}}
	definition.Prepare = func(doc *document.DashboardDocument) error {
		if doc.Header.Title != "producer" || doc.Filters[0].Facet.Selections[0].Label != "producer" || doc.I18n["owned"] != "base" || doc.Theme.Palette["brand"] != "producer" {
			return fmt.Errorf("producer controls were changed by an earlier preparation")
		}
		doc.Header.Title = "prepared"
		doc.Filters[0].Facet.Selections[0].Label = "prepared"
		doc.I18n["owned"] = "prepared"
		doc.Theme.Palette["brand"] = "prepared"
		return nil
	}
	for i := 0; i < 2; i++ {
		key.Params = fmt.Sprint("independent-miss-", i)
		view, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
		require.NoError(t, err)
		require.Equal(t, "prepared", view.Document.Header.Title)
		require.Equal(t, "prepared", view.Document.Filters[0].Facet.Selections[0].Label)
		view.Release()
	}
	require.Equal(t, "producer", definition.Options.Header.Title)
	require.Equal(t, "producer", definition.Options.Filters[0].Facet.Selections[0].Label)
	require.Equal(t, "base", definition.Options.I18n["owned"])
	require.Equal(t, "producer", definition.Options.Theme.Palette["brand"])
}

func TestReportCache_PreservesNativeUUIDInputsAndOwnsPointers(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	nullable := uuid.NullUUID{UUID: id, Valid: true}
	key, definition := reportFixture(t)
	cache := newReportCache(t, ReportCacheOptions[string]{})
	definition.Config.Spec.Datasets = append(definition.Config.Spec.Datasets, lens.DatasetSpec{Name: "native-identity", Kind: lens.DatasetKindQuery, Source: "primary", Query: &lens.QuerySpec{Text: "SELECT 1", Params: map[string]lens.ParamValue{"id": {Literal: id}, "pointer": {Literal: &id}, "nullable": {Literal: nullable}}}})
	view, err := cache.Materialize(context.Background(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	defer view.Release()
	spec, err := view.Spec()
	require.NoError(t, err)
	params := spec.Datasets[len(spec.Datasets)-1].Query.Params
	require.Equal(t, id, params["id"].Literal)
	require.Equal(t, id, *params["pointer"].Literal.(*uuid.UUID))
	require.Equal(t, nullable, params["nullable"].Literal)
	original := id
	id = uuid.New()
	require.Equal(t, original, *params["pointer"].Literal.(*uuid.UUID))
	wireID := uuid.New()
	source := &document.DashboardDocument{Frames: map[document.FrameRef]document.Frame{"native": {Rows: [][]any{{wireID, &wireID, nullable}}}}}
	owned, err := ownPreparedDocument(source, 1<<20)
	require.NoError(t, err)
	wireID = uuid.New()
	require.NotEqual(t, wireID, *owned.Frames["native"].Rows[0][1].(*uuid.UUID))
}

func TestOwnedReportDecimalCopiesCoefficientBeforeProducerBinaryMutation(t *testing.T) {
	t.Parallel()
	source := decimal.RequireFromString("12345678901234567890.1234567890123456789")
	nullable := decimal.NewNullDecimal(source.Copy())
	original := source.String()
	spec := lens.DashboardSpec{Datasets: []lens.DatasetSpec{{Query: &lens.QuerySpec{Params: map[string]lens.ParamValue{"amount": {Literal: source}, "pointer": {Literal: &source}, "optional": {Literal: nullable}}}}}}
	owned, _, err := ownReportSpec(spec, 1<<20)
	require.NoError(t, err)
	doc, err := ownPreparedDocument(&document.DashboardDocument{Frames: map[document.FrameRef]document.Frame{"money": {Rows: [][]any{{source, &source, nullable}}}}}, 1<<20)
	require.NoError(t, err)
	replacement, err := decimal.RequireFromString("987.65").MarshalBinary()
	require.NoError(t, err)
	require.NoError(t, source.UnmarshalBinary(replacement))
	require.NoError(t, nullable.Decimal.UnmarshalBinary(replacement))
	params := owned.Datasets[0].Query.Params
	require.Equal(t, original, params["amount"].Literal.(decimal.Decimal).String())
	require.Equal(t, original, params["pointer"].Literal.(*decimal.Decimal).String())
	require.Equal(t, original, params["optional"].Literal.(decimal.NullDecimal).Decimal.String())
	require.Equal(t, original, doc.Frames["money"].Rows[0][0].(decimal.Decimal).String())
	require.Equal(t, original, doc.Frames["money"].Rows[0][1].(*decimal.Decimal).String())
}

func TestReportCacheRejectedPressurePreservesLeasedAndColdIndexes(t *testing.T) {
	t.Parallel()
	for _, leased := range []bool{false, true} {
		t.Run(fmt.Sprint(leased), func(t *testing.T) {
			t.Parallel()
			key, definition := reportFixture(t)
			budget := resource.NewBudget(1 << 20)
			var cleaned atomic.Int32
			cache := newReportCache(t, ReportCacheOptions[string]{Budget: budget, OnEvict: func(string) { cleaned.Add(1) }})
			view, err := cache.Materialize(t.Context(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
			require.NoError(t, err)
			if !leased {
				view.Release()
			}
			held := budget.Used()
			external, err := budget.Reserve(budget.Limit() - held - 10)
			require.NoError(t, err)
			defer external.Release()
			// Even reclaiming every cold report cannot satisfy this shared-owner request.
			requested := held + 20
			if leased {
				requested = 20
			}
			_, err = cache.reserve(requested)
			require.ErrorIs(t, err, resource.ErrLimit)
			require.EqualValues(t, 0, cleaned.Load())
			require.Equal(t, 1, cache.Stats().Entries)
			require.Equal(t, budget.Limit()-10, budget.Used())
			found, ok, err := cache.Lookup(t.Context(), key)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, view.Document.SnapshotID, found.Document.SnapshotID)
			found.Release()
			view.Release()
		})
	}
}

func TestReportCacheColdReclaimIsAtomicAndCleansExactlyOnce(t *testing.T) {
	t.Parallel()
	key, definition := reportFixture(t)
	budget := resource.NewBudget(1 << 20)
	var cleaned atomic.Int32
	cache := newReportCache(t, ReportCacheOptions[string]{Budget: budget, OnEvict: func(string) { cleaned.Add(1) }})
	parent, err := cache.Materialize(t.Context(), key, func(context.Context) (ReportDefinition[string], error) { return definition, nil })
	require.NoError(t, err)
	_, err = cache.MaterializeDerived(t.Context(), parent.Document.SnapshotID, "wire", DerivedSurfaceInteractive, func(context.Context) (*document.DashboardDocument, error) {
		return &document.DashboardDocument{SnapshotID: "inline-owned"}, nil
	})
	require.NoError(t, err)
	parent.Release()
	held := budget.Used()
	external, err := budget.Reserve(budget.Limit() - held - 10)
	require.NoError(t, err)
	replacement, err := cache.reserve(held)
	require.NoError(t, err)
	require.EqualValues(t, 1, cleaned.Load())
	require.Zero(t, cache.Stats().Entries)
	require.Equal(t, budget.Limit()-10, budget.Used())
	cache.Close()
	require.EqualValues(t, 1, cleaned.Load())
	replacement.Release()
	external.Release()
	require.Zero(t, budget.Used())
}
