package serve

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type executorCall struct {
	panelID string
	request lensruntime.Request
}

type explorationLoaderFunc func(context.Context, lensruntime.ExplorationLoadRequest) (lensruntime.ExplorationDefinition, error)

func (f explorationLoaderFunc) LoadExploration(ctx context.Context, req lensruntime.ExplorationLoadRequest) (lensruntime.ExplorationDefinition, error) {
	return f(ctx, req)
}

type fakeExecutor struct {
	mu          sync.Mutex
	calls       []executorCall
	started     chan struct{}
	blockPanel  string
	blockStart  chan struct{}
	blockDone   chan struct{}
	cancelPanel string
	delay       time.Duration
	frames      map[string]*frame.FrameSet
	pageFrames  map[string]map[int]*frame.FrameSet
	pathFrames  map[string]*frame.FrameSet
	executeErrs map[string]error
	panelErrs   map[string]error
	startOnce   sync.Once
	blockOnce   sync.Once
	cacheHit    bool
	pageHasMore map[string]map[int]bool
	panelSpecs  map[string]panel.Spec
}

func (f *fakeExecutor) Execute(ctx context.Context, spec lens.DashboardSpec, req lensruntime.Request, scope lensruntime.Scope) (*lensruntime.DashboardResult, error) {
	panelID := ""
	if len(scope.PanelIDs) > 0 {
		panelID = scope.PanelIDs[0]
	}
	f.mu.Lock()
	f.calls = append(f.calls, executorCall{panelID: panelID, request: cloneRuntimeRequest(req)})
	executeErr := f.executeErrs[panelID]
	panelErr := f.panelErrs[panelID]
	f.mu.Unlock()
	if f.started != nil && panelID != "" {
		f.startOnce.Do(func() { close(f.started) })
	}
	if executeErr != nil {
		return nil, executeErr
	}
	if f.cancelPanel != "" && panelID == f.cancelPanel {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.blockPanel != "" && panelID == f.blockPanel {
		if f.blockStart != nil {
			f.blockOnce.Do(func() { close(f.blockStart) })
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.blockDone:
		}
	}
	if f.delay > 0 && panelID != "" {
		timer := time.NewTimer(f.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	result := &lensruntime.DashboardResult{
		Spec: spec, Variables: map[string]any{"region": requestValue(req.Request, "region", "all")},
		Panels: make(map[string]*lensruntime.PanelResult), Datasets: make(map[string]*lensruntime.DatasetResult),
		Locale: req.Locale, Timezone: req.Timezone, RequestPath: req.Path, Request: req.Request, StartedAt: time.Unix(100, 0).UTC(),
		Duration: 25 * time.Millisecond, CacheHit: f.cacheHit,
	}
	if scope.MetadataOnly {
		return result, nil
	}
	if panelID == "" {
		host, ok := lens.FindPanel(spec, "host")
		if !ok {
			return nil, errors.New("host panel is missing")
		}
		result.Panels[host.ID] = panelResult(host, f.frames[host.ID], req)
		return result, nil
	}
	target, ok := lens.FindPanel(spec, panelID)
	if !ok {
		return nil, errors.New("scoped panel is missing")
	}
	if actual, exists := f.panelSpecs[panelID]; exists {
		target = actual
	}
	frames := f.frames[panelID]
	if selected := f.pathFrames[panelID+":"+strings.Join(req.Request["lens_explore_path"], "/")]; selected != nil {
		frames = selected
	}
	if pages := f.pageFrames[panelID]; pages != nil {
		page, _ := strconv.Atoi(req.Request.Get(lensruntime.TablePaginationPageQuery))
		frames = pages[page]
	}
	result.Panels[panelID] = panelResult(target, frames, req)
	if pages := f.pageHasMore[panelID]; pages != nil {
		page, _ := strconv.Atoi(req.Request.Get(lensruntime.TablePaginationPageQuery))
		result.Panels[panelID].TablePagination = &lensruntime.TablePagination{
			Page: page, PerPage: len(frames.Primary().Rows()), HasMore: pages[page],
		}
	}
	result.Panels[panelID].Error = panelErr
	return result, nil
}

func TestHandlers_ProgressivePanelsAreIndependentCachedAndScopeIsolated(t *testing.T) {
	t.Parallel()
	spec, frames := testDashboard(t)
	executor := &fakeExecutor{frames: frames, panelErrs: map[string]error{"host": errors.New("host failed")}}
	store := document.NewMemoryStore(time.Minute, 8)
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: store, BasePath: "/dash", Progressive: true,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{
				Locale: "en", DataScope: requestValue(r.URL.Query(), "tenant", "tenant:one"), Request: r.URL.Query(),
			}
		},
	})
	require.NoError(t, err)

	doc := requestDocument(t, handlers, "/dash/document?tenant=tenant:one&region=west")
	require.Equal(t, "/dash/lens/panel", doc.Endpoints.Panel)
	require.True(t, doc.Panels[0].Deferred)
	require.Equal(t, 0, executor.callCount("host"), "shell must not execute a panel")

	failed := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "host"}, "tenant:one")
	require.Equal(t, http.StatusInternalServerError, failed.Code)
	require.NoError(t, doc.Validate(), "a panel failure must not invalidate the shell")

	executor.mu.Lock()
	delete(executor.panelErrs, "host")
	executor.mu.Unlock()
	loaded := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "host"}, "tenant:one")
	require.Equal(t, http.StatusOK, loaded.Code)
	var response PanelResponse
	require.NoError(t, json.Unmarshal(loaded.Body.Bytes(), &response))
	require.Equal(t, int64(25), response.Calculation.DurationMS)
	require.False(t, response.Calculation.CacheHit)
	require.Contains(t, response.Frames, document.FrameRef("panel:host"))
	require.Equal(t, "west", executor.lastCall("host").request.Request.Get("region"))

	cached := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "host"}, "tenant:one")
	require.Equal(t, http.StatusOK, cached.Code)
	require.Equal(t, 2, executor.callCount("host"), "failure plus one successful execution; cache must avoid a third")
	require.NoError(t, json.Unmarshal(cached.Body.Bytes(), &response))
	require.True(t, response.Calculation.CacheHit)

	rateLimited := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "host", Recompute: true}, "tenant:one")
	require.Equal(t, http.StatusTooManyRequests, rateLimited.Code)
	snapshot, err := store.Get(t.Context(), doc.SnapshotID)
	require.NoError(t, err)
	calculation := snapshot.Panels["host"]
	calculation.CalculatedAt = time.Now().Add(-recomputeCooldown)
	snapshot.Panels["host"] = calculation
	require.NoError(t, store.Put(t.Context(), snapshot))

	recomputed := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "host", Recompute: true}, "tenant:one")
	require.Equal(t, http.StatusOK, recomputed.Code)
	require.True(t, executor.lastCall("host").request.Recompute)
	require.Equal(t, "west", executor.lastCall("host").request.Request.Get("region"), "recompute must retain frozen filters")

	foreign := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "host"}, "tenant:two")
	require.Equal(t, http.StatusGone, foreign.Code)
}

func TestExplorationPathStepsPreservePointSelections(t *testing.T) {
	t.Parallel()
	spec, _ := testDashboard(t)
	target, err := resolveTarget(spec, document.NodePath{
		"metric", "metric/focus", "metric/focus/composition", "metric/focus/composition/root",
		"metric/focus/composition/root/a", "metric/focus/composition/detail",
		"metric/focus/composition/detail/b", "metric/focus/composition/end",
	}, "composition")
	require.NoError(t, err)
	require.Equal(t, []explore.PathStep{
		{NodeKey: "root"},
		{NodeKey: "detail", PointKey: "a"},
		{NodeKey: "end", PointKey: "b"},
	}, explorationPathSteps(target))
}

func TestExecuteLevelUsesGenericExplorationRuntime(t *testing.T) {
	t.Parallel()
	spec, frames := testDashboard(t)
	runtime := lensruntime.New(lensruntime.Options{})
	var loaded lensruntime.ExplorationLoadRequest
	computed := panel.Pie("computed-end", "Computed", "computed-data").IDField("id").Terminal().Build()
	handlers, err := New(Config{
		Spec: spec, Engine: runtime, Snapshots: document.NewMemoryStore(time.Minute, 8),
		Exploration: runtime,
		ResolveLoader: func(_ context.Context, request lensruntime.ExplorationLoadRequest, _ lensruntime.Request) (lensruntime.ExplorationLoader, error) {
			return explorationLoaderFunc(func(_ context.Context, request lensruntime.ExplorationLoadRequest) (lensruntime.ExplorationDefinition, error) {
				loaded = request
				return lensruntime.ExplorationDefinition{
					Dashboard: lens.DashboardSpec{
						ID: "computed", Title: "Computed", Rows: []lens.RowSpec{{Panels: []panel.Spec{computed}}},
						Datasets: []lens.DatasetSpec{staticDataset("computed-data", frames["end-panel"])},
					},
					PanelID: computed.ID,
				}, nil
			}), nil
		},
	})
	require.NoError(t, err)
	target, err := resolveTarget(spec, document.NodePath{"root", "a", "detail", "b", "end"}, "composition")
	require.NoError(t, err)
	result, err := handlers.executeLevel(context.Background(), lensruntime.Request{DataScope: "tenant:one"}, nil, target, 0)
	require.NoError(t, err)
	require.Equal(t, "end-panel", result.Panel.ID)
	require.Equal(t, []explore.PathStep{
		{NodeKey: "root"},
		{NodeKey: "detail", PointKey: "a"},
		{NodeKey: "end", PointKey: "b"},
	}, loaded.Steps)
}

func TestHandlers_PanelBatchPreservesPerPanelResultsAndRejectsForeignScope(t *testing.T) {
	t.Parallel()
	spec, frames := testDashboard(t)
	handlers, err := New(Config{
		Spec: spec, Engine: &fakeExecutor{frames: frames}, Snapshots: document.NewMemoryStore(time.Minute, 8),
		BasePath: "/dash", Progressive: true,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: r.URL.Query().Get("tenant"), Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)
	doc := requestDocument(t, handlers, "/dash/document?tenant=tenant:one")
	request := PanelBatchRequest{SnapshotID: doc.SnapshotID, Panels: []PanelRequest{{PanelID: "host"}, {PanelID: "missing"}}}
	recorder := httptest.NewRecorder()
	handlers.Panel(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/panel?tenant=tenant:one", marshal(t, request)))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "application/x-ndjson", recorder.Header().Get("Content-Type"))
	response := decodePanelBatchStream(t, recorder.Body.Bytes())
	require.NotNil(t, response.Panels["host"].Calculation)
	require.Contains(t, response.Panels["host"].Frames, document.FrameRef("panel:host"))
	require.Equal(t, document.QueryErrorBadRequest, response.Panels["missing"].Error.Error)

	foreign := httptest.NewRecorder()
	handlers.Panel(foreign, httptest.NewRequest(http.MethodPost, "/dash/lens/panel?tenant=tenant:two", marshal(t, request)))
	require.Equal(t, http.StatusGone, foreign.Code)
}

func TestHandlers_ProgressivePanelUsesExecutedTotalInsteadOfShellPlaceholder(t *testing.T) {
	t.Parallel()

	const total = 45_561_778_243.57
	primary, err := frame.New("expenses",
		frame.Field{Name: "ring_id", Type: frame.FieldTypeString, Values: []any{"components", "components", "components"}},
		frame.Field{Name: "category_id", Type: frame.FieldTypeString, Values: []any{"acquisition_cost", "operating_expenses", "reinsurance_cost"}},
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"Acquisition", "Operating", "Reinsurance"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{1_068_717_254.18, 29_812_997_444.63, 14_680_063_544.76}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	shellPanel := panel.Donut("expenses", "Expenses", "expenses").
		IDField("category_id").LabelField("category").ValueField("amount").
		TotalBadgeValue(3).Terminal().Build()
	actualPanel := shellPanel
	actualTotal := float64(total)
	actualPanel.TotalBadgeValue = &actualTotal
	spec := lens.DashboardSpec{
		ID: "expenses", Title: "Expenses", Rows: []lens.RowSpec{{Panels: []panel.Spec{shellPanel}}},
		Datasets: []lens.DatasetSpec{staticDataset("expenses", frames)},
	}
	executor := &fakeExecutor{
		frames:     map[string]*frame.FrameSet{"expenses": frames},
		panelSpecs: map[string]panel.Spec{"expenses": actualPanel},
	}
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: document.NewMemoryStore(time.Minute, 8), BasePath: "/dash", Progressive: true,
		Request: func(*http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: "tenant:one"}
		},
	})
	require.NoError(t, err)
	doc := requestDocument(t, handlers, "/dash/document")
	recorder := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "expenses"}, "")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response PanelResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	wire := response.Frames[document.FrameRef("panel:expenses")]
	require.NotNil(t, wire.Total)
	require.InDelta(t, total, *wire.Total, 0.01)
}

func TestHandlers_PanelBatchFlushesFastResultBeforeSlowPanelCompletes(t *testing.T) {
	t.Parallel()
	spec, frames := testDashboard(t)
	slow := panel.Pie("slow", "Slow", "slow-data").IDField("id").Terminal().Build()
	spec.Rows = append(spec.Rows, lens.RowSpec{Panels: []panel.Spec{slow}})
	frames["slow"] = testFrames(t, "slow", 50)
	spec.Datasets = append(spec.Datasets, staticDataset("slow-data", frames["slow"]))
	releaseSlow := make(chan struct{})
	slowStarted := make(chan struct{})
	executor := &fakeExecutor{frames: frames, blockPanel: "slow", blockStart: slowStarted, blockDone: releaseSlow}
	observer := &recordingObserver{}
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: document.NewMemoryStore(time.Minute, 8), BasePath: "/dash", Progressive: true,
		Observer: observer,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: r.URL.Query().Get("tenant"), Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)
	documentResponse := httptest.NewRecorder()
	handlers.Document(documentResponse, httptest.NewRequest(http.MethodGet, "/dash/document?tenant=tenant:one", nil))
	if documentResponse.Code != http.StatusOK {
		for _, observed := range observer.recorded() {
			t.Logf("%s: %v", observed.op, observed.err)
		}
	}
	require.Equal(t, http.StatusOK, documentResponse.Code, documentResponse.Body.String())
	var doc document.DashboardDocument
	require.NoError(t, json.Unmarshal(documentResponse.Body.Bytes(), &doc))
	server := httptest.NewServer(http.HandlerFunc(handlers.Panel))
	defer server.Close()
	request := PanelBatchRequest{SnapshotID: doc.SnapshotID, Panels: []PanelRequest{{PanelID: "slow"}, {PanelID: "host"}}}
	httpRequest, err := http.NewRequest(http.MethodPost, server.URL+"?tenant=tenant:one", marshal(t, request))
	require.NoError(t, err)
	httpRequest.Header.Set("Content-Type", "application/json")
	type firstLine struct {
		response *http.Response
		line     []byte
		err      error
	}
	observed := make(chan firstLine, 1)
	go func() {
		response, requestErr := http.DefaultClient.Do(httpRequest)
		if requestErr != nil {
			observed <- firstLine{err: requestErr}
			return
		}
		scanner := bufio.NewScanner(response.Body)
		if !scanner.Scan() {
			observed <- firstLine{response: response, err: scanner.Err()}
			return
		}
		observed <- firstLine{response: response, line: append([]byte(nil), scanner.Bytes()...)}
	}()
	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		close(releaseSlow)
		t.Fatal("slow panel did not start")
	}
	select {
	case result := <-observed:
		defer func() {
			close(releaseSlow)
			if result.response != nil {
				_ = result.response.Body.Close()
			}
		}()
		require.NoError(t, result.err)
		require.Equal(t, http.StatusOK, result.response.StatusCode)
		var event PanelBatchStreamEvent
		require.NoError(t, json.Unmarshal(result.line, &event))
		require.Equal(t, "host", event.PanelID)
		require.NotNil(t, event.Result)
		require.False(t, event.Complete)
	case <-time.After(time.Second):
		close(releaseSlow)
		t.Fatal("fast panel result was not observable while slow panel remained blocked")
	}
}

func TestHandlers_PrefetchesFirstDrillStatesWhileLowerRootRowsLoad(t *testing.T) {
	spec, frames := testDashboard(t)
	slow := panel.Pie("slow", "Slow", "slow-data").IDField("id").Terminal().Build()
	spec.Rows = append(spec.Rows, lens.RowSpec{Panels: []panel.Spec{slow}})
	frames["slow"] = testFrames(t, "slow", 50)
	spec.Datasets = append(spec.Datasets, staticDataset("slow-data", frames["slow"]))
	releaseSlow := make(chan struct{})
	slowStarted := make(chan struct{})
	executor := &fakeExecutor{frames: frames, blockPanel: "slow", blockStart: slowStarted, blockDone: releaseSlow}
	store := document.NewMemoryStore(time.Minute, 8)
	observer := &recordingObserver{}
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: store, BasePath: "/dash", Progressive: true,
		Observer: observer,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: r.URL.Query().Get("tenant"), Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)
	doc := requestDocument(t, handlers, "/dash/document?tenant=tenant:one")
	server := httptest.NewServer(http.HandlerFunc(handlers.Panel))
	defer server.Close()
	request := PanelBatchRequest{SnapshotID: doc.SnapshotID, Panels: []PanelRequest{{PanelID: "slow"}, {PanelID: "host"}}}
	httpRequest, err := http.NewRequest(http.MethodPost, server.URL+"?tenant=tenant:one", marshal(t, request))
	require.NoError(t, err)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(httpRequest)
	require.NoError(t, err)
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	require.True(t, scanner.Scan(), scanner.Err())
	var first PanelBatchStreamEvent
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &first))
	require.Equal(t, "host", first.PanelID, "the first visual row must be revealed before lower rows")
	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		close(releaseSlow)
		t.Fatal("lower root row did not continue loading")
	}

	rootRef := document.FrameRef("explore:metric/focus/composition:root")
	require.Eventually(t, func() bool {
		snapshot, getErr := store.Get(t.Context(), doc.SnapshotID)
		if getErr != nil {
			return false
		}
		_, materialized := snapshot.Frames[rootRef]
		return materialized
	}, time.Second, 10*time.Millisecond, "the default drill root must materialize in the background")
	require.Equal(t, 1, executor.callCount("root-panel"))
	queryRequest := QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"root"}, Perspective: "composition"}
	queryRecorder := httptest.NewRecorder()
	handlers.Query(queryRecorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query?tenant=tenant:one", marshal(t, queryRequest)))
	require.Equal(t, http.StatusOK, queryRecorder.Code, queryRecorder.Body.String())
	require.Equal(t, 1, executor.callCount("root-panel"), "opening a prefetched drill state must reuse its snapshot frame")
	require.Eventually(t, func() bool {
		names := make(map[MetricName]bool)
		for _, metric := range observer.recordedMetrics() {
			names[metric.Name] = true
		}
		return names[MetricTimeToFirstUsefulKPI] && names[MetricTimeToFirstChild] && names[MetricPrefetchHit] && names[MetricSchedulerSaturation]
	}, time.Second, 10*time.Millisecond)

	close(releaseSlow)
	for scanner.Scan() {
	}
	require.NoError(t, scanner.Err())
}

func TestHandlers_DoesNotPrefetchUntilTheFirstUsefulRowIsComplete(t *testing.T) {
	spec, frames := testDashboard(t)
	slow := panel.Pie("top-slow", "Top slow", "top-slow-data").IDField("id").Terminal().Build()
	spec.Rows[0].Panels = append(spec.Rows[0].Panels, slow)
	frames["top-slow"] = testFrames(t, "top-slow", 50)
	spec.Datasets = append(spec.Datasets, staticDataset("top-slow-data", frames["top-slow"]))
	releaseSlow := make(chan struct{})
	slowStarted := make(chan struct{})
	executor := &fakeExecutor{frames: frames, blockPanel: "top-slow", blockStart: slowStarted, blockDone: releaseSlow}
	store := document.NewMemoryStore(time.Minute, 8)
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: store, BasePath: "/dash", Progressive: true,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: "tenant:one", Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)
	doc := requestDocument(t, handlers, "/dash/document")
	server := httptest.NewServer(http.HandlerFunc(handlers.Panel))
	defer server.Close()
	request := PanelBatchRequest{SnapshotID: doc.SnapshotID, Panels: []PanelRequest{{PanelID: "host"}, {PanelID: "top-slow"}}}
	response, err := http.Post(server.URL, "application/json", marshal(t, request)) //nolint:noctx // Test server lifecycle bounds the request.
	require.NoError(t, err)
	defer response.Body.Close()
	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		close(releaseSlow)
		t.Fatal("second panel in the useful row did not start")
	}
	scanner := bufio.NewScanner(response.Body)
	require.True(t, scanner.Scan(), scanner.Err())
	var first PanelBatchStreamEvent
	require.NoError(t, json.Unmarshal(scanner.Bytes(), &first))
	require.Equal(t, "host", first.PanelID)
	rootRef := document.FrameRef("explore:metric/focus/composition:root")
	require.Never(t, func() bool {
		snapshot, getErr := store.Get(t.Context(), doc.SnapshotID)
		if getErr != nil {
			return false
		}
		_, materialized := snapshot.Frames[rootRef]
		return materialized
	}, 100*time.Millisecond, 10*time.Millisecond, "prefetch must not compete with an unfinished KPI in the first row")

	close(releaseSlow)
	for scanner.Scan() {
	}
	require.NoError(t, scanner.Err())
	require.Eventually(t, func() bool {
		snapshot, getErr := store.Get(t.Context(), doc.SnapshotID)
		if getErr != nil {
			return false
		}
		_, materialized := snapshot.Frames[rootRef]
		return materialized
	}, time.Second, 10*time.Millisecond)
}

func TestBackgroundPrefetchTargetsBoundsDynamicCardinality(t *testing.T) {
	spec, _ := testDashboard(t)
	targets := backgroundPrefetchTargets(spec)
	require.Len(t, targets, 2, "the default root and its concrete first child are safe to prefetch")
	require.Empty(t, targets[0].points)
	require.Equal(t, []string{"a"}, targets[1].points)

	root := &spec.Explorers[0].Branches[0].Perspectives[0].Nodes[0]
	root.Edges = nil
	root.DynamicEdges = true
	root.DynamicTargets = []string{"detail"}
	targets = backgroundPrefetchTargets(spec)
	require.Len(t, targets, 1, "dynamic selections must not fan out before user intent identifies a concrete path")
	require.Equal(t, "root", targets[0].nodeKey)
}

func TestHandlers_DrawerMintsRelativeURLFromScopedSnapshotOnOpen(t *testing.T) {
	t.Parallel()
	spec, frames := testDashboard(t)
	var resolved document.DrawerResolveRequest
	target := "/analytics/drill/approval-rate/lens/document?token=minted-now"
	handlers, err := New(Config{
		Spec: spec, Engine: &fakeExecutor{frames: frames}, Snapshots: document.NewMemoryStore(time.Minute, 8), BasePath: "/dash",
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: r.URL.Query().Get("tenant"), Request: r.URL.Query()}
		},
		DrawerResolver: func(_ *http.Request, input document.DrawerResolveRequest, frozen lensruntime.Request) (string, error) {
			resolved = input
			require.Equal(t, "west", frozen.Request.Get("region"))
			return target, nil
		},
	})
	require.NoError(t, err)
	doc := requestDocument(t, handlers, "/dash/document?tenant=tenant:one&region=west")
	require.Equal(t, "/dash/lens/drawer", doc.Endpoints.Drawer)
	request := document.DrawerResolveRequest{
		SnapshotID: doc.SnapshotID, MetricKey: "approval-rate", Params: map[string]any{"product": []any{"osago"}},
	}
	recorder := httptest.NewRecorder()
	handlers.Drawer(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/drawer?tenant=tenant:one", marshal(t, request)))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response document.DrawerResolveResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "/analytics/drill/approval-rate/lens/document?_lens_snapshot="+url.QueryEscape(doc.SnapshotID)+"&token=minted-now", response.URL)
	require.Equal(t, "approval-rate", resolved.MetricKey)

	foreign := httptest.NewRecorder()
	handlers.Drawer(foreign, httptest.NewRequest(http.MethodPost, "/dash/lens/drawer?tenant=tenant:two", marshal(t, request)))
	require.Equal(t, http.StatusGone, foreign.Code)

	target = "https://evil.example/drill"
	unsafe := httptest.NewRecorder()
	handlers.Drawer(unsafe, httptest.NewRequest(http.MethodPost, "/dash/lens/drawer?tenant=tenant:one", marshal(t, request)))
	require.Equal(t, http.StatusInternalServerError, unsafe.Code)
}

func TestHandlers_ReleaseCancelsScopedSnapshotPrefetch(t *testing.T) {
	spec, frames := testDashboard(t)
	handlers, err := New(Config{
		Spec: spec, Engine: &fakeExecutor{frames: frames}, Snapshots: document.NewMemoryStore(time.Minute, 8), BasePath: "/dash",
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: r.URL.Query().Get("tenant"), Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)
	doc := requestDocument(t, handlers, "/dash/document?tenant=tenant:one")
	require.Equal(t, "/dash/lens/release", doc.Endpoints.Release)

	session := handlers.session(doc.SnapshotID)
	started := make(chan struct{})
	cancelled := make(chan struct{})
	prefetch := session.submit(t.Context(), "child", priorityIdlePrefetch, 0, func(ctx context.Context) (any, error) {
		close(started)
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	})
	session.enableBackground()
	<-started
	releaseRequest := document.ReleaseRequest{SnapshotID: doc.SnapshotID}

	foreign := httptest.NewRecorder()
	handlers.Release(foreign, httptest.NewRequest(http.MethodPost, "/dash/lens/release?tenant=tenant:two", marshal(t, releaseRequest)))
	require.Equal(t, http.StatusGone, foreign.Code)
	select {
	case <-cancelled:
		t.Fatal("cross-scope release cancelled another tenant's work")
	default:
	}

	recorder := httptest.NewRecorder()
	handlers.Release(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/release?tenant=tenant:one", marshal(t, releaseRequest)))
	require.Equal(t, http.StatusNoContent, recorder.Code, recorder.Body.String())
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("release endpoint did not cancel speculative work")
	}
	require.ErrorIs(t, (<-prefetch.result).err, context.Canceled)
}

func TestHandlers_QueryAndExportRejectCrossScopeSnapshotsInEveryMode(t *testing.T) {
	t.Parallel()

	for _, progressive := range []bool{false, true} {
		progressive := progressive
		t.Run(fmt.Sprintf("progressive=%t", progressive), func(t *testing.T) {
			t.Parallel()
			spec, frames := testDashboard(t)
			handlers, err := New(Config{
				Spec: spec, Engine: &fakeExecutor{frames: frames}, Snapshots: document.NewMemoryStore(time.Minute, 8),
				BasePath: "/dash", Progressive: progressive,
				Request: func(r *http.Request) lensruntime.Request {
					return lensruntime.Request{
						Locale: "en", DataScope: requestValue(r.URL.Query(), "tenant", "tenant:one"), Request: r.URL.Query(),
					}
				},
			})
			require.NoError(t, err)
			doc := requestDocument(t, handlers, "/dash/document?tenant=tenant:one")

			queryBody := marshal(t, QueryRequest{
				SnapshotID: doc.SnapshotID, Path: document.NodePath{"detail"}, Perspective: "composition",
			})
			query := httptest.NewRecorder()
			handlers.Query(query, httptest.NewRequest(http.MethodPost, "/dash/lens/query?tenant=tenant:two", queryBody))
			require.Equal(t, http.StatusGone, query.Code, query.Body.String())

			exported := httptest.NewRecorder()
			handlers.Export(exported, httptest.NewRequest(http.MethodGet, "/dash/export?snapshot="+url.QueryEscape(doc.SnapshotID)+"&tenant=tenant:two", nil))
			require.Equal(t, http.StatusGone, exported.Code, exported.Body.String())
		})
	}
}

func TestHandlers_EvidenceQueryCarriesAllowlistedSortAcrossPages(t *testing.T) {
	t.Parallel()
	handlers, executor, _ := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document")
	response := queryLevel(t, handlers, QueryRequest{
		SnapshotID: doc.SnapshotID, Path: document.NodePath{"evidence"}, Perspective: "evidence", Page: 2,
		Sort: &document.TableSort{Field: "amount", Direction: document.SortDescending},
	})
	require.NotNil(t, response.Page)
	call := executor.lastCall("evidence-panel")
	require.Equal(t, "amount", call.request.Request.Get(lensruntime.TableSortFieldQuery))
	require.Equal(t, "desc", call.request.Request.Get(lensruntime.TableSortDirectionQuery))
}

func TestHandlers_ProgressiveTableSearchUsesFullScopedFrameAndDoesNotReplaceBaseCache(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("claims",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"1", "2", "3"}},
		frame.Field{Name: "product", Type: frame.FieldTypeString, Values: []any{"Motor", "Travel", "Motor Plus"}},
		frame.Field{Name: "claims", Type: frame.FieldTypeNumber, Values: []any{2.0, 3.0, 4.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	table := panel.Table("claims", "Claims", "claims-data").Searchable().Columns(
		panel.TableColumn{Field: "product", Label: "Product"},
		panel.TableColumn{Field: "claims", Label: "Claims", Total: true},
	).Terminal().Build()
	spec := lens.DashboardSpec{
		ID: "searchable-table", Title: "Searchable table",
		Rows:     []lens.RowSpec{{Panels: []panel.Spec{table}}},
		Datasets: []lens.DatasetSpec{staticDataset("claims-data", frames)},
	}
	executor := &fakeExecutor{frames: map[string]*frame.FrameSet{"claims": frames}}
	observer := &recordingObserver{}
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: document.NewMemoryStore(time.Minute, 8), Progressive: true,
		Observer: observer,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: "tenant:test", Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)

	doc := requestDocument(t, handlers, "/document")
	searched := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "claims", Search: "motor"}, "tenant:test")
	observed := observer.recorded()
	observedMessage := ""
	if len(observed) > 0 {
		observedMessage = observed[0].err.Error()
	}
	require.Equal(t, http.StatusOK, searched.Code, "%s: %s", searched.Body.String(), observedMessage)
	var response PanelResponse
	require.NoError(t, json.Unmarshal(searched.Body.Bytes(), &response))
	require.Equal(t, 2, response.Summary.FilteredRows)
	require.InDelta(t, 6.0, response.Summary.Values["claims"], 1e-9)

	base := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "claims"}, "tenant:test")
	require.Equal(t, http.StatusOK, base.Code, base.Body.String())
	require.NoError(t, json.Unmarshal(base.Body.Bytes(), &response))
	require.Equal(t, 3, response.Summary.FilteredRows)
	require.InDelta(t, 9.0, response.Summary.Values["claims"], 1e-9)
	require.Equal(t, 1, executor.callCount("claims"), "search must reuse and preserve the full unfiltered panel cache")

	cached := requestPanel(t, handlers, doc.SnapshotID, PanelRequest{PanelID: "claims"}, "tenant:test")
	require.Equal(t, http.StatusOK, cached.Code, cached.Body.String())
	require.Equal(t, 1, executor.callCount("claims"))
}

type failPanelExecutor struct {
	base *lensruntime.Runtime
}

func (f failPanelExecutor) Execute(ctx context.Context, spec lens.DashboardSpec, req lensruntime.Request, scope lensruntime.Scope) (*lensruntime.DashboardResult, error) {
	result, err := f.base.Execute(ctx, spec, req, scope)
	if err != nil || scope.MetadataOnly || len(scope.PanelIDs) != 1 || scope.PanelIDs[0] != "slow" || scope.IncludeExportEvidence {
		return result, err
	}
	result.Panels["slow"].Error = errors.New("slow panel failed")
	result.Panels["slow"].Frames = nil
	return result, nil
}

func TestHandlers_ProgressiveExportMaterializesMissingPanels(t *testing.T) {
	t.Parallel()
	fastFrames := testFrames(t, "fast", 10)
	slowFrames := testFrames(t, "slow", 20)
	fast := panel.Stat("fast", "Fast", "fast-data").Terminal().Build()
	slow := panel.Stat("slow", "Slow", "slow-data").Terminal().Build()
	spec := lens.DashboardSpec{
		ID: "progressive-export", Title: "Progressive export",
		Rows:     []lens.RowSpec{{Panels: []panel.Spec{fast, slow}}},
		Datasets: []lens.DatasetSpec{staticDataset("fast-data", fastFrames), staticDataset("slow-data", slowFrames)},
	}
	runtime := lensruntime.New(lensruntime.Options{})
	handlers, err := New(Config{
		Spec: spec, Engine: failPanelExecutor{base: runtime}, Snapshots: document.NewMemoryStore(time.Minute, 8),
		BasePath: "/dash", Progressive: true,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: "tenant:test", Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)

	before := requestDocument(t, handlers, "/dash/document")
	exported := httptest.NewRecorder()
	handlers.Export(exported, httptest.NewRequest(http.MethodGet, "/dash/export?snapshot="+before.SnapshotID, nil))
	require.Equal(t, http.StatusOK, exported.Code, exported.Body.String())
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", exported.Header().Get("Content-Type"))

	afterFailure := requestDocument(t, handlers, "/dash/document")
	failed := requestPanel(t, handlers, afterFailure.SnapshotID, PanelRequest{PanelID: "slow"}, "tenant:test")
	require.Equal(t, http.StatusInternalServerError, failed.Code)
	exported = httptest.NewRecorder()
	handlers.Export(exported, httptest.NewRequest(http.MethodGet, "/dash/export?snapshot="+afterFailure.SnapshotID, nil))
	require.Equal(t, http.StatusOK, exported.Code, exported.Body.String())
}

type observedError struct {
	op  string
	err error
}

type recordingObserver struct {
	mu      sync.Mutex
	errors  []observedError
	metrics []Metric
}

func (o *recordingObserver) OnError(_ context.Context, op string, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.errors = append(o.errors, observedError{op: op, err: err})
}

func (o *recordingObserver) OnMetric(_ context.Context, metric Metric) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.metrics = append(o.metrics, metric)
}

func (o *recordingObserver) recorded() []observedError {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]observedError(nil), o.errors...)
}

func (o *recordingObserver) recordedMetrics() []Metric {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]Metric(nil), o.metrics...)
}

func (f *fakeExecutor) callCount(panelID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, call := range f.calls {
		if call.panelID == panelID {
			count++
		}
	}
	return count
}

func (f *fakeExecutor) lastCall(panelID string) executorCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := len(f.calls) - 1; index >= 0; index-- {
		if f.calls[index].panelID == panelID {
			return f.calls[index]
		}
	}
	return executorCall{}
}

func TestHandlers_DocumentQueryCacheAndAppend(t *testing.T) {
	t.Parallel()
	handlers, executor, store := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document?region=west&locale=ru")

	require.Equal(t, "/dash/lens/query", doc.Endpoints.Query)
	require.Equal(t, "/dash/export", doc.Endpoints.Export)
	require.Contains(t, doc.Frames, document.FrameRef("explore:metric/focus/composition:root"))
	require.NotContains(t, doc.Frames, document.FrameRef("explore:metric/focus/composition:detail"))
	require.Equal(t, 1, executor.callCount("root-panel"))

	root := queryLevel(t, handlers, QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"root"}, Perspective: "composition"})
	require.Contains(t, root.Frames, document.FrameRef("explore:metric/focus/composition:root"))
	require.Equal(t, 1, executor.callCount("root-panel"))

	detail := queryLevel(t, handlers, QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "detail"}, Perspective: "composition"})
	require.Contains(t, detail.Frames, document.FrameRef("explore:metric/focus/composition:detail"))
	require.Equal(t, 1, executor.callCount("detail-panel"))
	require.Equal(t, "west", executor.lastCall("detail-panel").request.Overrides["region"])
	require.Equal(t, "west", executor.lastCall("detail-panel").request.Request.Get("region"))
	require.Equal(t, "ru", executor.lastCall("detail-panel").request.Locale)

	queryLevel(t, handlers, QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"detail"}, Perspective: "composition"})
	require.Equal(t, 1, executor.callCount("detail-panel"))
	snapshot, err := store.Get(t.Context(), doc.SnapshotID)
	require.NoError(t, err)
	require.Contains(t, snapshot.Frames, document.FrameRef("explore:metric/focus/composition:detail"))
}

// TestHandlers_DocumentExecutesSourceDataPanels pins the audit-table document
// contract: a node's declared source table executes at document time through a
// plain panel-scoped run (no lens_explore_* level coordinates), its frame
// ships as source:<perspective>:<node>, and the level carries the disclosure.
func TestHandlers_DocumentExecutesSourceDataPanels(t *testing.T) {
	t.Parallel()
	spec, frames := testDashboard(t)
	sourceTable := panel.Table("source-panel", "Source rows", "root-data").IDField("id").Columns(
		panel.TableColumn{Field: "label", Label: "Label"},
	).Terminal().Build()
	root := spec.Explorers[0].Branches[0].Perspectives[0].Nodes[0]
	root.SourceData = &explore.SourceData{Label: "Исходные данные", Panel: sourceTable}
	spec.Explorers[0].Branches[0].Perspectives[0].Nodes[0] = root
	frames["source-panel"] = testFrames(t, "source", 42)
	executor := &fakeExecutor{frames: frames}
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: document.NewMemoryStore(time.Minute, 32), BasePath: "/dash", InlineDepth: 0,
		Request: func(r *http.Request) lensruntime.Request {
			return lensruntime.Request{Locale: "en", DataScope: "tenant:test", Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)

	doc := requestDocument(t, handlers, "/dash/document")
	sourceRef := document.FrameRef("source:metric/focus/composition:root")
	require.Contains(t, doc.Frames, sourceRef)
	level := doc.Drill.Edges["metric/focus/composition/root"]
	require.NotNil(t, level.Source)
	require.Equal(t, "Исходные данные", level.Source.Label)
	require.Equal(t, sourceRef, level.Source.Frame)
	require.Equal(t, 1, executor.callCount("source-panel"))
	call := executor.lastCall("source-panel")
	require.Empty(t, call.request.Request.Get("lens_explorer"))
	require.Empty(t, call.request.Request.Get("lens_explore_node"))
}

func TestHandlers_PointDrillsCacheDistinctFrames(t *testing.T) {
	t.Parallel()
	handlers, executor, store := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document")

	// Same node, two sibling point selections: each must execute and cache its
	// own frame instead of replaying the sibling's.
	first := queryLevel(t, handlers, QueryRequest{
		SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "2025", "detail"}, Perspective: "composition",
	})
	require.Contains(t, first.Frames, document.FrameRef("explore:metric/focus/composition:detail"))
	require.Equal(t, 1, executor.callCount("detail-panel"))
	require.Equal(t, []string{"root", "2025", "detail"},
		executor.lastCall("detail-panel").request.Request["lens_explore_path"])

	queryLevel(t, handlers, QueryRequest{
		SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "2024", "detail"}, Perspective: "composition",
	})
	require.Equal(t, 2, executor.callCount("detail-panel"))
	require.Equal(t, []string{"root", "2024", "detail"},
		executor.lastCall("detail-panel").request.Request["lens_explore_path"])

	// A repeated selection is a cache hit.
	queryLevel(t, handlers, QueryRequest{
		SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "2025", "detail"}, Perspective: "composition",
	})
	require.Equal(t, 2, executor.callCount("detail-panel"))

	// Point-free requests keep the plain node reference and stay shared, so
	// they can still be served from the frames the document inlined.
	queryLevel(t, handlers, QueryRequest{
		SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "detail"}, Perspective: "composition",
	})
	queryLevel(t, handlers, QueryRequest{
		SnapshotID: doc.SnapshotID, Path: document.NodePath{"detail"}, Perspective: "composition",
	})
	require.Equal(t, 3, executor.callCount("detail-panel"))

	snapshot, err := store.Get(t.Context(), doc.SnapshotID)
	require.NoError(t, err)
	require.Contains(t, snapshot.Frames, document.FrameRef("explore:metric/focus/composition:detail@2025"))
	require.Contains(t, snapshot.Frames, document.FrameRef("explore:metric/focus/composition:detail@2024"))
	require.Contains(t, snapshot.Frames, document.FrameRef("explore:metric/focus/composition:detail"))
}

func TestHandlers_DynamicChildrenResolveAndCachePerPath(t *testing.T) {
	t.Parallel()
	handlers, executor, store := newTestHandlers(t, 0)
	spec := handlers.spec
	detail := &spec.Explorers[0].Branches[0].Perspectives[0].Nodes[1]
	detailPanel := panel.Table("detail-panel", "Detail", "detail-data").
		IDField("row_id").
		Columns(panel.TableColumn{Field: "value", Label: "Value"}).
		Build()
	detail.Panel = &detailPanel
	detail.Edges = nil
	detail.DynamicEdges = true
	detail.DynamicTargets = []string{"end"}
	leaf := action.Navigate("").WithFieldURL("url")
	detail.DynamicChildren = &explore.DynamicChildren{
		Key: action.FieldValue("child_id"), Label: action.FieldValue("child_label"),
		Target: sourcePtr(action.FieldValue("target")), Action: &leaf,
	}
	handlers.spec = spec
	executor.pathFrames = map[string]*frame.FrameSet{
		"detail-panel:root/2025/detail": dynamicFrames(t, "month-12", "December"),
		"detail-panel:root/2024/detail": dynamicFrames(t, "month-11", "November"),
	}
	doc := requestDocument(t, handlers, "/dash/document")

	first := queryLevel(t, handlers, QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "2025", "detail"}, Perspective: "composition"})
	second := queryLevel(t, handlers, QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "2024", "detail"}, Perspective: "composition"})
	require.Equal(t, []document.Column{
		{Name: "value", Type: document.ColumnNumber},
		{Name: "row_id", Type: document.ColumnString},
		{Name: "child_id", Type: document.ColumnString},
		{Name: "child_label", Type: document.ColumnString},
		{Name: "target", Type: document.ColumnString},
		{Name: "url", Type: document.ColumnString},
	}, first.Frames["explore:metric/focus/composition:detail"].Columns)
	require.Equal(t, document.NodeKey("month-12"), first.Frames["explore:metric/focus/composition:detail"].Children[0].Key)
	require.Equal(t, "December", first.Frames["explore:metric/focus/composition:detail"].Children[0].Label)
	require.Equal(t, document.NodeKey("metric/focus/composition/end"), first.Frames["explore:metric/focus/composition:detail"].Children[0].Target)
	require.Equal(t, document.NodeKey("month-11"), second.Frames["explore:metric/focus/composition:detail"].Children[0].Key)

	queryLevel(t, handlers, QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"root", "2025", "detail"}, Perspective: "composition"})
	require.Equal(t, 2, executor.callCount("detail-panel"))
	snapshot, err := store.Get(t.Context(), doc.SnapshotID)
	require.NoError(t, err)
	require.Equal(t, document.NodeKey("month-12"), snapshot.Frames["explore:metric/focus/composition:detail@2025"].Children[0].Key)
	require.Equal(t, document.NodeKey("month-11"), snapshot.Frames["explore:metric/focus/composition:detail@2024"].Children[0].Key)
}

func TestHandlers_QueryUsesFrozenScopeOverConflictingQueryParams(t *testing.T) {
	t.Parallel()
	handlers, executor, _ := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document?region=west")
	body := marshal(t, QueryRequest{
		SnapshotID:  doc.SnapshotID,
		Path:        document.NodePath{"detail"},
		Perspective: "composition",
	})
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query?region=EVIL", body))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	call := executor.lastCall("detail-panel")
	require.Equal(t, "west", call.request.Request.Get("region"))
	require.Equal(t, "west", call.request.Overrides["region"])
}

func TestHandlers_SnapshotGone(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlers(t, 0)
	body := marshal(t, QueryRequest{SnapshotID: "gone", Path: document.NodePath{"root"}, Perspective: "composition"})
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", body))
	require.Equal(t, http.StatusGone, recorder.Code)
	var response errorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, document.QueryErrorSnapshotGone, response.Error)

	recorder = httptest.NewRecorder()
	handlers.Export(recorder, httptest.NewRequest(http.MethodGet, "/dash/export?snapshot=gone", nil))
	require.Equal(t, http.StatusGone, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, document.QueryErrorSnapshotGone, response.Error)
}

func TestHandlers_ExpiredSnapshotReturnsGone(t *testing.T) {
	t.Parallel()
	store := document.NewMemoryStore(time.Millisecond, 32)
	handlers, _, _ := newTestHandlersWithStore(t, 0, store, nil)
	doc := requestDocument(t, handlers, "/dash/document")
	time.Sleep(2 * time.Millisecond)

	body := marshal(t, QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"root"}, Perspective: "composition"})
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", body))
	require.Equal(t, http.StatusGone, recorder.Code, recorder.Body.String())
	var response errorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, document.QueryErrorSnapshotGone, response.Error)
}

func TestHandlers_EvidenceIsLiveAndPaginated(t *testing.T) {
	t.Parallel()
	handlers, executor, store := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document?region=east")
	request := QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"evidence"}, Perspective: "evidence", Page: 3}

	first := queryLevel(t, handlers, request)
	second := queryLevel(t, handlers, request)
	require.Equal(t, &Page{Number: 3, Size: 17, HasNext: false}, first.Page)
	require.Equal(t, first.Page, second.Page)
	require.Equal(t, 2, executor.callCount("evidence-panel"))
	call := executor.lastCall("evidence-panel")
	require.Equal(t, "3", call.request.Request.Get(lensruntime.TablePaginationPageQuery))
	require.Equal(t, "17", call.request.Request.Get(lensruntime.TablePaginationLimitQuery))
	require.Equal(t, "evidence-panel", call.request.Request.Get(lensruntime.TablePaginationPanelQuery))
	require.Equal(t, "east", call.request.Overrides["region"])
	frame := first.Frames[document.FrameRef("explore:metric/focus/evidence:evidence")]
	require.Equal(t, []document.Column{
		{Name: "policy", Type: document.ColumnString},
		{Name: "amount", Type: document.ColumnNumber},
		{Name: "record_id", Type: document.ColumnString},
	}, frame.Columns)
	require.Equal(t, []any{"P-1", float64(1), "row-1"}, frame.Rows[0])
	snapshot, err := store.Get(t.Context(), doc.SnapshotID)
	require.NoError(t, err)
	require.NotContains(t, snapshot.Frames, document.FrameRef("explore:metric/focus/evidence:evidence"))
}

func TestHandlers_EvidenceHasNext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want bool
	}{
		{name: "exact final page", want: false},
		{name: "full page with following rows", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			handlers, executor, _ := newTestHandlers(t, 0)
			executor.pageFrames = map[string]map[int]*frame.FrameSet{
				"evidence-panel": {1: evidenceFramesWithRows(t, 17)},
			}
			executor.pageHasMore = map[string]map[int]bool{"evidence-panel": {1: tt.want}}
			doc := requestDocument(t, handlers, "/dash/document")

			response := queryLevel(t, handlers, QueryRequest{
				SnapshotID: doc.SnapshotID, Path: document.NodePath{"evidence"}, Perspective: "evidence", Page: 1,
			})

			require.Equal(t, &Page{Number: 1, Size: 17, HasNext: tt.want}, response.Page)
			require.Len(t, response.Frames[document.FrameRef("explore:metric/focus/evidence:evidence")].Rows, 17)
			require.Equal(t, 1, executor.callCount("evidence-panel"))
		})
	}
}

func TestEvidenceHasNextRejectsFullPageWithoutPaginationMetadata(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlers(t, 0)
	wire := document.Frame{Rows: make([][]any, handlers.pageSize)}
	_, err := handlers.evidenceHasNext(&lensruntime.PanelResult{}, &wire)
	require.ErrorContains(t, err, "requires authoritative table pagination metadata")
}

func TestHandlers_DocumentSkipsErroredExplorePanelAndQueryReportsIt(t *testing.T) {
	t.Parallel()
	observer := &recordingObserver{}
	handlers, executor, _ := newTestHandlersWithStore(t, 0, document.NewMemoryStore(time.Minute, 32), observer)
	rootErr := errors.New("root panel failed")
	detailErr := errors.New("detail panel failed")
	executor.panelErrs = map[string]error{"root-panel": rootErr, "detail-panel": detailErr}

	recorder := httptest.NewRecorder()
	handlers.Document(recorder, httptest.NewRequest(http.MethodGet, "/dash/document", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var doc document.DashboardDocument
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &doc))
	require.NotContains(t, doc.Frames, document.FrameRef("explore:metric/focus/composition:root"))
	require.Empty(t, observer.recorded())

	body := marshal(t, QueryRequest{
		SnapshotID:  doc.SnapshotID,
		Path:        document.NodePath{"detail"},
		Perspective: "composition",
	})
	recorder = httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", body))
	require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	recorded := observer.recorded()
	require.Len(t, recorded, 1)
	require.ErrorIs(t, recorded[0].err, detailErr)
}

func TestHandlers_ObserverReceivesWrappedExecutionError(t *testing.T) {
	t.Parallel()
	observer := &recordingObserver{}
	handlers, executor, _ := newTestHandlersWithStore(t, 0, document.NewMemoryStore(time.Minute, 32), observer)
	doc := requestDocument(t, handlers, "/dash/document")
	executionErr := errors.New("datasource unavailable")
	executor.executeErrs = map[string]error{"detail-panel": executionErr}

	body := marshal(t, QueryRequest{
		SnapshotID:  doc.SnapshotID,
		Path:        document.NodePath{"detail"},
		Perspective: "composition",
	})
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", body))
	require.Equal(t, http.StatusInternalServerError, recorder.Code, recorder.Body.String())
	recorded := observer.recorded()
	require.Len(t, recorded, 1)
	require.Equal(t, "lens/serve.writeExecutionError", recorded[0].op)
	require.ErrorIs(t, recorded[0].err, executionErr)
	require.ErrorContains(t, recorded[0].err, "lens/serve.executeLevel")
}

func TestHandlers_ConcurrentAppendExecutesAggregateOnce(t *testing.T) {
	t.Parallel()
	handlers, executor, store := newTestHandlers(t, 0)
	executor.delay = 25 * time.Millisecond
	doc := requestDocument(t, handlers, "/dash/document?region=north")
	request := QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"end"}, Perspective: "composition"}

	const workers = 12
	var wg sync.WaitGroup
	errorsFound := make(chan error, workers)
	payload, err := json.Marshal(request)
	require.NoError(t, err)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			recorder := httptest.NewRecorder()
			handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", bytes.NewReader(payload)))
			if recorder.Code != http.StatusOK {
				errorsFound <- errors.New(recorder.Body.String())
			}
		}()
	}
	wg.Wait()
	close(errorsFound)
	for err := range errorsFound {
		require.NoError(t, err)
	}
	require.Equal(t, 1, executor.callCount("end-panel"))
	snapshot, err := store.Get(t.Context(), doc.SnapshotID)
	require.NoError(t, err)
	require.Contains(t, snapshot.Frames, document.FrameRef("explore:metric/focus/composition:end"))
}

func TestHandlers_FirstCanceledCallerDoesNotAbortSharedExecution(t *testing.T) {
	t.Parallel()
	handlers, executor, _ := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document")
	executor.delay = 75 * time.Millisecond
	executor.started = make(chan struct{})
	payload, err := json.Marshal(QueryRequest{
		SnapshotID:  doc.SnapshotID,
		Path:        document.NodePath{"detail"},
		Perspective: "composition",
	})
	require.NoError(t, err)

	firstCtx, cancelFirst := context.WithCancel(t.Context())
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		request := httptest.NewRequest(http.MethodPost, "/dash/lens/query", bytes.NewReader(payload)).WithContext(firstCtx)
		handlers.Query(httptest.NewRecorder(), request)
	}()
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}

	canceled := make(chan struct{})
	timer := time.AfterFunc(10*time.Millisecond, func() {
		cancelFirst()
		close(canceled)
	})
	defer timer.Stop()
	second := httptest.NewRecorder()
	handlers.Query(second, httptest.NewRequest(http.MethodPost, "/dash/lens/query", bytes.NewReader(payload)))
	<-canceled
	require.Equal(t, http.StatusOK, second.Code, second.Body.String())
	select {
	case <-firstDone:
	case <-time.After(time.Second):
		t.Fatal("canceled waiter did not stop")
	}
	require.Equal(t, 1, executor.callCount("detail-panel"))
}

func TestHandlers_QueryCancellationStopsWaiter(t *testing.T) {
	t.Parallel()
	handlers, executor, _ := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document")
	handlers.workTimeout = 25 * time.Millisecond
	executor.cancelPanel = "detail-panel"
	executor.started = make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	request := httptest.NewRequest(http.MethodPost, "/dash/lens/query", marshal(t, QueryRequest{
		SnapshotID: doc.SnapshotID, Path: document.NodePath{"detail"}, Perspective: "composition",
	})).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		handlers.Query(httptest.NewRecorder(), request)
	}()
	select {
	case <-executor.started:
	case <-time.After(time.Second):
		t.Fatal("executor did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("query did not stop after cancellation")
	}
}

func TestHandlers_ExportUsesSnapshotFrames(t *testing.T) {
	t.Parallel()
	handlers, executor, _ := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document")
	recorder := httptest.NewRecorder()
	handlers.Export(recorder, httptest.NewRequest(http.MethodGet, "/dash/export?snapshot="+url.QueryEscape(doc.SnapshotID)+"&panel=host", nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", recorder.Header().Get("Content-Type"))
	require.True(t, bytes.HasPrefix(recorder.Body.Bytes(), []byte("PK")))
	require.Equal(t, 1, executor.callCount(""))
}

func TestNew_ValidatesConfig(t *testing.T) {
	t.Parallel()
	spec, frames := testDashboard(t)
	executor := &fakeExecutor{frames: frames}
	store := document.NewMemoryStore(time.Minute, 10)
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "executor", cfg: Config{Spec: spec, Snapshots: store}, want: "executor"},
		{name: "store", cfg: Config{Spec: spec, Engine: executor}, want: "snapshot store"},
		{name: "depth", cfg: Config{Spec: spec, Engine: executor, Snapshots: store, InlineDepth: -1}, want: "inline depth"},
		{name: "page size", cfg: Config{Spec: spec, Engine: executor, Snapshots: store, PageSize: -1}, want: "page size"},
		{name: "base path", cfg: Config{Spec: spec, Engine: executor, Snapshots: store, BasePath: "relative"}, want: "base path"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := New(test.cfg)
			require.ErrorContains(t, err, test.want)
		})
	}
}

func newTestHandlers(t *testing.T, inlineDepth int) (*Handlers, *fakeExecutor, document.SnapshotStore) {
	t.Helper()
	return newTestHandlersWithStore(t, inlineDepth, document.NewMemoryStore(time.Minute, 32), nil)
}

func newTestHandlersWithStore(
	t *testing.T,
	inlineDepth int,
	store document.SnapshotStore,
	observer Observer,
) (*Handlers, *fakeExecutor, document.SnapshotStore) {
	t.Helper()
	spec, frames := testDashboard(t)
	executor := &fakeExecutor{frames: frames}
	handlers, err := New(Config{
		Spec: spec, Engine: executor, Snapshots: store, BasePath: "/dash", InlineDepth: inlineDepth, PageSize: 17,
		Observer: observer,
		Request: func(r *http.Request) lensruntime.Request {
			locale := requestValue(r.URL.Query(), "locale", "en")
			return lensruntime.Request{Locale: locale, DataScope: "tenant:test", Request: r.URL.Query()}
		},
	})
	require.NoError(t, err)
	return handlers, executor, store
}

func testDashboard(t *testing.T) (lens.DashboardSpec, map[string]*frame.FrameSet) {
	t.Helper()
	frames := map[string]*frame.FrameSet{
		"host":           testFrames(t, "host", 100),
		"root-panel":     testFrames(t, "root", 80),
		"detail-panel":   testFrames(t, "detail", 60),
		"end-panel":      testFrames(t, "end", 40),
		"evidence-panel": evidenceFrames(t),
	}
	host := panel.Pie("host", "Premium", "host-data").IDField("id").Build()
	host.Export.EvidenceDatasets = []string{"evidence-data"}
	root := panel.Pie("root-panel", "Root", "root-data").IDField("id").Build()
	detail := panel.Pie("detail-panel", "Detail", "detail-data").IDField("id").Build()
	end := panel.Pie("end-panel", "End", "end-data").IDField("id").Terminal().Build()
	evidence := panel.Table("evidence-panel", "Evidence", "evidence-data").
		IDField("record_id").
		Columns(
			panel.TableColumn{Field: "policy", Label: "Policy"},
			panel.TableColumn{Field: "amount", Label: "Amount"},
		).
		Terminal().Build()
	explorer := explore.Spec{
		ID: "metric", HostPanelID: "host", Branches: []explore.Branch{{
			Key: "focus", Label: "Focus", DefaultPerspective: "composition", Perspectives: []explore.Perspective{
				{Key: "composition", Label: "Composition", Semantics: explore.SemanticsPartition, RootNode: "root", Nodes: []explore.Node{
					{Key: "root", Label: "Root", Panel: &root, Edges: []explore.Edge{{PointKey: "a", ToNode: "detail"}}},
					{Key: "detail", Label: "Detail", Panel: &detail, Edges: []explore.Edge{{PointKey: "b", ToNode: "end"}}},
					{Key: "end", Label: "End", Panel: &end},
				}},
				{Key: "evidence", Label: "Evidence", Semantics: explore.SemanticsEvidence, RootNode: "evidence", Nodes: []explore.Node{
					{Key: "evidence", Label: "Evidence", Panel: &evidence},
				}},
			},
		}},
	}
	spec := lens.DashboardSpec{
		ID: "dashboard", Title: "Dashboard", Rows: []lens.RowSpec{{Panels: []panel.Spec{host}}},
		Variables: []lens.VariableSpec{{Name: "region", Label: "Region", Kind: lens.VariableText, Default: "all"}},
		Datasets: []lens.DatasetSpec{
			staticDataset("host-data", frames["host"]), staticDataset("root-data", frames["root-panel"]),
			staticDataset("detail-data", frames["detail-panel"]), staticDataset("end-data", frames["end-panel"]),
			staticDataset("evidence-data", frames["evidence-panel"]),
		},
		Explorers: []explore.Spec{explorer},
	}
	require.NoError(t, lensruntime.Validate(spec))
	return spec, frames
}

func staticDataset(name string, frames *frame.FrameSet) lens.DatasetSpec {
	return lens.DatasetSpec{Name: name, Kind: lens.DatasetKindStatic, Static: frames}
}

func testFrames(t *testing.T, name string, value float64) *frame.FrameSet {
	t.Helper()
	primary, err := frame.New(name,
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{name}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{name}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{value}},
	)
	require.NoError(t, err)
	result, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	return result
}

func dynamicFrames(t *testing.T, key, label string) *frame.FrameSet {
	t.Helper()
	primary, err := frame.New("dynamic",
		frame.Field{Name: "row_id", Type: frame.FieldTypeString, Values: []any{"row-" + key}},
		frame.Field{Name: "child_id", Type: frame.FieldTypeString, Values: []any{key}},
		frame.Field{Name: "child_label", Type: frame.FieldTypeString, Values: []any{label}},
		frame.Field{Name: "target", Type: frame.FieldTypeString, Values: []any{"end"}},
		frame.Field{Name: "url", Type: frame.FieldTypeString, Values: []any{"/records/" + key}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{1.0}},
	)
	require.NoError(t, err)
	result, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	return result
}

func sourcePtr(source action.ValueSource) *action.ValueSource {
	return &source
}

func evidenceFrames(t *testing.T) *frame.FrameSet {
	t.Helper()
	return evidenceFramesWithRows(t, 1)
}

func evidenceFramesWithRows(t *testing.T, count int) *frame.FrameSet {
	t.Helper()
	ids := make([]any, count)
	policies := make([]any, count)
	amounts := make([]any, count)
	for index := range count {
		ids[index] = "row-" + strconv.Itoa(index+1)
		policies[index] = "P-" + strconv.Itoa(index+1)
		amounts[index] = float64(index + 1)
	}
	primary, err := frame.New("evidence",
		frame.Field{Name: "record_id", Type: frame.FieldTypeString, Values: ids},
		frame.Field{Name: "policy", Type: frame.FieldTypeString, Values: policies},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: amounts},
	)
	require.NoError(t, err)
	result, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	return result
}

func panelResult(spec panel.Spec, frames *frame.FrameSet, req lensruntime.Request) *lensruntime.PanelResult {
	return &lensruntime.PanelResult{Panel: spec, Frames: frames, Locale: req.Locale, Timezone: req.Timezone, Variables: req.Overrides, Request: req.Request}
}

func requestDocument(t *testing.T, handlers *Handlers, target string) document.DashboardDocument {
	t.Helper()
	recorder := httptest.NewRecorder()
	handlers.Document(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response document.DashboardDocument
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func requestPanel(t *testing.T, handlers *Handlers, snapshotID string, request PanelRequest, tenant string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(PanelBatchRequest{SnapshotID: snapshotID, Panels: []PanelRequest{{
		PanelID: request.PanelID, Recompute: request.Recompute, Search: request.Search, Sort: request.Sort, Page: request.Page,
	}}})
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	target := "/dash/lens/panel?tenant=" + url.QueryEscape(tenant)
	handlers.Panel(recorder, httptest.NewRequest(http.MethodPost, target, bytes.NewReader(payload)))
	if recorder.Code != http.StatusOK {
		return recorder
	}
	batch := decodePanelBatchStream(t, recorder.Body.Bytes())
	result := batch.Panels[request.PanelID]
	recorder = httptest.NewRecorder()
	if result.Error != nil {
		status := http.StatusInternalServerError
		if result.Error.Error == document.QueryErrorBadRequest {
			status = http.StatusBadRequest
			if strings.Contains(result.Error.Message, "recomputed recently") {
				status = http.StatusTooManyRequests
			}
		}
		writeJSON(recorder, status, result.Error)
		return recorder
	}
	writeJSON(recorder, http.StatusOK, PanelResponse{
		Frames: result.Frames, Calculation: *result.Calculation, Summary: result.Summary, Page: result.Page,
	})
	return recorder
}

func decodePanelBatchStream(t *testing.T, payload []byte) PanelBatchResponse {
	t.Helper()
	response := PanelBatchResponse{Panels: make(map[string]document.PanelBatchResult)}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	complete := false
	for {
		var event PanelBatchStreamEvent
		err := decoder.Decode(&event)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if event.Complete {
			complete = true
			continue
		}
		require.NotEmpty(t, event.PanelID)
		require.NotNil(t, event.Result)
		_, duplicate := response.Panels[event.PanelID]
		require.False(t, duplicate)
		response.Panels[event.PanelID] = *event.Result
	}
	require.True(t, complete, "panel stream must end with a completion event")
	return response
}

func queryLevel(t *testing.T, handlers *Handlers, request QueryRequest) QueryResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", marshal(t, request)))
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response QueryResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	return response
}

func marshal(t *testing.T, value any) *bytes.Reader {
	t.Helper()
	payload, err := json.Marshal(value)
	require.NoError(t, err)
	return bytes.NewReader(payload)
}

func requestValue(values url.Values, key, fallback string) string {
	value := values.Get(key)
	if value == "" {
		return fallback
	}
	return value
}

func cloneRuntimeRequest(req lensruntime.Request) lensruntime.Request {
	req.Request = cloneValues(req.Request)
	req.Overrides = cloneParams(req.Overrides)
	return req
}

func TestHandlers_BadRequestsAreJSON(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlers(t, 0)
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", bytes.NewBufferString("{}")))
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	var response errorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, document.QueryErrorBadRequest, response.Error)
	assert.NotEmpty(t, response.Message)
}

func TestHandlers_EnforceMethods(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlers(t, 0)
	tests := []struct {
		name    string
		method  string
		target  string
		handler http.HandlerFunc
	}{
		{name: "document", method: http.MethodPost, target: "/dash/document", handler: handlers.Document},
		{name: "query", method: http.MethodGet, target: "/dash/lens/query", handler: handlers.Query},
		{name: "export", method: http.MethodPost, target: "/dash/export", handler: handlers.Export},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			test.handler(recorder, httptest.NewRequest(test.method, test.target, nil))
			require.Equal(t, http.StatusMethodNotAllowed, recorder.Code, recorder.Body.String())
		})
	}
}

func TestHandlers_QueryRejectsOversizedBody(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlers(t, 0)
	body := strings.NewReader(`{"snapshotId":"` + strings.Repeat("x", maxQueryBodyBytes) + `"}`)
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", body))
	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "request body too large")
}

func TestHandlers_QueryRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlers(t, 0)
	body := strings.NewReader(`{"snapshotId":"snapshot","path":["root"],"unexpected":true}`)
	recorder := httptest.NewRecorder()
	handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", body))
	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	require.Contains(t, recorder.Body.String(), "unknown field")
}

func TestHandlers_QueryBoundsPathAndPage(t *testing.T) {
	t.Parallel()
	handlers, _, _ := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document")
	longPath := make(document.NodePath, maxQueryPathDepth+1)
	for index := range longPath {
		longPath[index] = document.NodeKey("x")
	}

	tests := []QueryRequest{
		{SnapshotID: doc.SnapshotID, Path: longPath, Perspective: "composition"},
		{SnapshotID: doc.SnapshotID, Path: document.NodePath{document.NodeKey(strings.Repeat("x", maxQueryPathEntry+1))}, Perspective: "composition"},
		{SnapshotID: doc.SnapshotID, Path: document.NodePath{"detail"}, Perspective: "composition", Page: lensruntime.MaxTablePage + 1},
		{SnapshotID: doc.SnapshotID, Path: document.NodePath{"detail"}, Perspective: "composition", IdlePrefetch: true},
	}
	for _, request := range tests {
		recorder := httptest.NewRecorder()
		handlers.Query(recorder, httptest.NewRequest(http.MethodPost, "/dash/lens/query", marshal(t, request)))
		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	}
}
