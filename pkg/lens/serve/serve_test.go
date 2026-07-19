package serve

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
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

type fakeExecutor struct {
	mu          sync.Mutex
	calls       []executorCall
	started     chan struct{}
	cancelPanel string
	delay       time.Duration
	frames      map[string]*frame.FrameSet
	executeErrs map[string]error
	panelErrs   map[string]error
	startOnce   sync.Once
}

func (f *fakeExecutor) Execute(ctx context.Context, spec lens.DashboardSpec, req lensruntime.Request, scope lensruntime.Scope) (*lensruntime.DashboardResult, error) {
	panelID := ""
	if len(scope.PanelIDs) > 0 {
		panelID = scope.PanelIDs[0]
	}
	f.mu.Lock()
	f.calls = append(f.calls, executorCall{panelID: panelID, request: cloneRuntimeRequest(req)})
	f.mu.Unlock()
	if f.started != nil && panelID != "" {
		f.startOnce.Do(func() { close(f.started) })
	}
	if err := f.executeErrs[panelID]; err != nil {
		return nil, err
	}
	if f.cancelPanel != "" && panelID == f.cancelPanel {
		<-ctx.Done()
		return nil, ctx.Err()
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
	result.Panels[panelID] = panelResult(target, f.frames[panelID], req)
	result.Panels[panelID].Error = f.panelErrs[panelID]
	return result, nil
}

type observedError struct {
	op  string
	err error
}

type recordingObserver struct {
	mu     sync.Mutex
	errors []observedError
}

func (o *recordingObserver) OnError(_ context.Context, op string, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.errors = append(o.errors, observedError{op: op, err: err})
}

func (o *recordingObserver) recorded() []observedError {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]observedError(nil), o.errors...)
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
	require.Equal(t, "snapshot_gone", response.Error)

	recorder = httptest.NewRecorder()
	handlers.Export(recorder, httptest.NewRequest(http.MethodGet, "/dash/export?snapshot=gone", nil))
	require.Equal(t, http.StatusGone, recorder.Code)
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "snapshot_gone", response.Error)
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
	require.Equal(t, "snapshot_gone", response.Error)
}

func TestHandlers_EvidenceIsLiveAndPaginated(t *testing.T) {
	t.Parallel()
	handlers, executor, store := newTestHandlers(t, 0)
	doc := requestDocument(t, handlers, "/dash/document?region=east")
	request := QueryRequest{SnapshotID: doc.SnapshotID, Path: document.NodePath{"evidence"}, Perspective: "evidence", Page: 3}

	first := queryLevel(t, handlers, request)
	second := queryLevel(t, handlers, request)
	require.Equal(t, &Page{Number: 3, Size: 17}, first.Page)
	require.Equal(t, first.Page, second.Page)
	require.Equal(t, 2, executor.callCount("evidence-panel"))
	call := executor.lastCall("evidence-panel")
	require.Equal(t, "3", call.request.Request.Get(lensruntime.TablePaginationPageQuery))
	require.Equal(t, "17", call.request.Request.Get(lensruntime.TablePaginationLimitQuery))
	require.Equal(t, "evidence-panel", call.request.Request.Get(lensruntime.TablePaginationPanelQuery))
	require.Equal(t, "east", call.request.Overrides["region"])
	snapshot, err := store.Get(t.Context(), doc.SnapshotID)
	require.NoError(t, err)
	require.NotContains(t, snapshot.Frames, document.FrameRef("explore:metric/focus/evidence:evidence"))
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
	end := panel.Pie("end-panel", "End", "end-data").IDField("id").Build()
	evidence := panel.Table("evidence-panel", "Evidence", "evidence-data").Build()
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

func evidenceFrames(t *testing.T) *frame.FrameSet {
	t.Helper()
	primary, err := frame.New("evidence",
		frame.Field{Name: "policy", Type: frame.FieldTypeString, Values: []any{"P-1"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{25.0}},
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
	assert.Equal(t, "bad_request", response.Error)
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
