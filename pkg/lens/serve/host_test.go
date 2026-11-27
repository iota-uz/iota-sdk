package serve_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/engine"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/lens/serve"
)

type countingExecutor struct {
	base            engine.Executor
	calls           atomic.Int32
	recomputeSeen   atomic.Int32
	failNextRequest atomic.Bool
}

func (c *countingExecutor) Execute(ctx context.Context, spec lens.DashboardSpec, req lensruntime.Request, scope lensruntime.Scope) (*lensruntime.DashboardResult, error) {
	c.calls.Add(1)
	if serve.RecomputeRequested(ctx) {
		c.recomputeSeen.Add(1)
	}
	if c.failNextRequest.CompareAndSwap(true, false) {
		return nil, errors.New("injected execution failure")
	}
	return c.base.Execute(ctx, spec, req, scope)
}

func hostTestFrameSet(t *testing.T) *frame.FrameSet {
	t.Helper()
	fs, err := frame.FromRows("policies",
		frame.Row{"agent_type": "individual", "agency_id": "a1", "agency_name": "Alpha", "issued_at": time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), "premium": 400.0},
		frame.Row{"agent_type": "legal", "agency_id": "a2", "agency_name": "Beta", "issued_at": time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC), "premium": 900.0},
		frame.Row{"agent_type": "legal", "agency_id": "a3", "agency_name": "Gamma", "issued_at": time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC), "premium": 200.0},
		frame.Row{"agent_type": cube.UnknownDrillValue, "agency_id": "a4", "agency_name": "Unknown", "issued_at": time.Date(2026, 2, 20, 0, 0, 0, 0, time.UTC), "premium": 100.0},
	)
	require.NoError(t, err)
	return fs
}

func buildTestCubeSpec(t *testing.T) cube.CubeSpec {
	t.Helper()
	return cube.New("agent_program", "Agent Program").
		Dataset(hostTestFrameSet(t)).
		Dimension("agent_type", "Тип агента").
		Field("agent_type").
		LabelMap(map[string]string{
			"individual":           "Физлица",
			"legal":                "Юрлица",
			cube.UnknownDrillValue: "Не указано",
		}).
		AsFacet().
		Done().
		Dimension("agency", "Агент").
		Field("agency_id").
		LabelField("agency_name").
		AsFacet().
		Done().
		Dimension("period", "Период").
		TimeBucket("issued_at", "month").
		Done().
		Measure("premium", "Премия").
		Field("premium").
		Sum().
		Money("UZS").
		Done().
		Layout(
			cube.KPIRow("premium"),
			cube.SplitRow(
				cube.Chart("by_type", panel.KindDonut).Dimension("agent_type").Measure("premium"),
				cube.Chart("trend", panel.KindLine).Dimension("period").Series("agent_type").Measure("premium"),
			),
			cube.Chart("top_agencies", panel.KindBar).Dimension("agency").Measure("premium").TopN(2).WithRemainder(),
		).
		DrillPath("agent_type", "agency").
		LeafSpec(cube.LeafSpec{
			Table: cube.LeafTable("policies_leaf", "Полисы"),
			ExternalLink: func(_ context.Context, drill cube.DrillContext, _ map[string]any) string {
				return "/portfolio/policies?" + drill.Encode().Encode()
			},
		}).
		Build()
}

func TestNewHost_Validation(t *testing.T) {
	t.Parallel()

	// Would be falsely green if NewHost accepted invalid configs.
	rt := lensruntime.New(lensruntime.Options{})
	eng := engine.New(rt)

	_, err := serve.NewHost(serve.HostConfig{})
	require.ErrorContains(t, err, "executor is required")

	_, err = serve.NewHost(serve.HostConfig{Executor: eng.Executor()})
	require.ErrorContains(t, err, "exactly one of ResolveCube or ResolveDashboard")

	_, err = serve.NewHost(serve.HostConfig{
		Executor: eng.Executor(),
		ResolveCube: func(context.Context, *http.Request, cube.DrillContext) (serve.ResolvedCube, error) {
			return serve.ResolvedCube{}, nil
		},
		ResolveDashboard: func(context.Context, *http.Request) (serve.ResolvedDashboard, error) {
			return serve.ResolvedDashboard{}, nil
		},
	})
	require.ErrorContains(t, err, "exactly one of ResolveCube or ResolveDashboard")
}

func TestHost_CubeMode_CachingFacetsToggleLeafAndExport(t *testing.T) {
	t.Parallel()

	// Would be falsely green if Host rebuilt documents on cache hits or ignored export/facet options.
	rt := lensruntime.New(lensruntime.Options{})
	exec := &countingExecutor{base: engine.New(rt).Executor()}
	budget := resource.NewBudget(64 << 20)
	var observedErrors atomic.Int32

	host, err := serve.NewHost(serve.HostConfig{
		BasePath:         "/analytics/agent-program/lens",
		Executor:         exec,
		ExecutionTimeout: 30 * time.Second,
		Cache: serve.CacheOptions{
			TTL:        10 * time.Minute,
			MaxEntries: 32,
			Budget:     budget,
		},
		Export: serve.ExportOptions{
			VisibleTableColumnsOnly: true,
			OmitLineage:             true,
			PanelExportFields: func(context.Context) map[string]string {
				return map[string]string{"label": "Категория", "premium": "Премия"}
			},
		},
		Observer: serve.ObserverFunc(func(_ context.Context, _ string, _ error) {
			observedErrors.Add(1)
		}),
		ResolveCube: func(_ context.Context, r *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			scope := r.Header.Get("X-Tenant")
			if scope == "" {
				scope = "tenant-1"
			}
			return serve.ResolvedCube{
				Spec: buildTestCubeSpec(t),
				Request: lensruntime.Request{
					DataScope: scope,
					Locale:    "ru",
				},
			}, nil
		},
	})
	require.NoError(t, err)
	defer host.Close()

	router := mux.NewRouter()
	host.Mount(router.PathPrefix("/analytics/agent-program/lens").Subrouter())

	// 1. Parameter-keyed cache reuse on identical query.
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/document?range-start=2026-01-01&range-end=2026-03-31", nil))
	require.Equal(t, http.StatusOK, rec1.Code)
	var doc1 document.DashboardDocument
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &doc1))
	require.Equal(t, "/analytics/agent-program/lens/query", doc1.Endpoints.Query)
	require.Equal(t, "/analytics/agent-program/lens/export", doc1.Endpoints.Export)
	require.Equal(t, "/analytics/agent-program/lens/release", doc1.Endpoints.Release)
	require.Len(t, doc1.Filters, 2)
	require.Equal(t, document.FilterKindFacet, doc1.Filters[0].Kind)
	require.Contains(t, doc1.Filters[0].Facet.OptionsEndpoint, "/analytics/agent-program/lens/facet-options?")
	require.Equal(t, int32(1), exec.calls.Load())

	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/document?range-end=2026-03-31&range-start=2026-01-01", nil))
	require.Equal(t, http.StatusOK, rec2.Code)
	var doc2 document.DashboardDocument
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &doc2))
	require.Equal(t, doc1.SnapshotID, doc2.SnapshotID)
	require.Equal(t, int32(1), exec.calls.Load(), "second request with identical params must hit cache")
	require.Equal(t, 1, host.Stats().Entries)

	// 2. Empty drill filter value normalization ("_f=agent_type:" -> "__unknown__").
	recUnknown := httptest.NewRecorder()
	router.ServeHTTP(recUnknown, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/document?_f=agent_type:", nil))
	require.Equal(t, http.StatusOK, recUnknown.Code)
	var docUnknown document.DashboardDocument
	require.NoError(t, json.Unmarshal(recUnknown.Body.Bytes(), &docUnknown))
	require.Len(t, docUnknown.ActiveFilters, 1)
	require.Equal(t, cube.UnknownDrillValue, docUnknown.ActiveFilters[0].Value)
	require.Contains(t, docUnknown.ActiveFilters[0].Label, "Не указано")

	// 3. Facet options endpoint returns mapped labels and rejects non-facet dimension.
	recFacet := httptest.NewRecorder()
	router.ServeHTTP(recFacet, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/facet-options?_facet=agent_type&range-start=2026-01-01", nil))
	require.Equal(t, http.StatusOK, recFacet.Code)
	var facetResp document.FacetOptionsResponse
	require.NoError(t, json.Unmarshal(recFacet.Body.Bytes(), &facetResp))
	require.GreaterOrEqual(t, len(facetResp.Options), 2)
	require.Equal(t, "Юрлица", facetResp.Options[0].Label)
	require.Contains(t, facetResp.Options[0].ToggleURL, "range-start=2026-01-01")
	require.Contains(t, facetResp.Options[0].ToggleURL, "_f=agent_type%3Alegal")

	recNonFacet := httptest.NewRecorder()
	router.ServeHTTP(recNonFacet, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/facet-options?_facet=period", nil))
	require.Equal(t, http.StatusBadRequest, recNonFacet.Code)

	// 4. Toggle-filter adds, removes, preserves period, and blocks open redirects.
	recToggleAdd := httptest.NewRecorder()
	router.ServeHTTP(recToggleAdd, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/toggle-filter?lens_filter_target=/analytics/agent-program&lens_filter_dimension=agent_type&lens_filter_value=legal&range-start=2026-01-01&_lpage=2", nil))
	require.Equal(t, http.StatusFound, recToggleAdd.Code)
	locAdd := recToggleAdd.Header().Get("Location")
	require.Contains(t, locAdd, "/analytics/agent-program?")
	require.Contains(t, locAdd, "_f=agent_type%3Alegal")
	require.Contains(t, locAdd, "range-start=2026-01-01")
	require.NotContains(t, locAdd, "_lpage=")

	recToggleRemove := httptest.NewRecorder()
	router.ServeHTTP(recToggleRemove, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/toggle-filter?lens_filter_target=/analytics/agent-program&lens_filter_dimension=agent_type&lens_filter_value=legal&_f=agent_type:legal&range-start=2026-01-01", nil))
	require.Equal(t, http.StatusFound, recToggleRemove.Code)
	locRemove := recToggleRemove.Header().Get("Location")
	require.NotContains(t, locRemove, "_f=agent_type")
	require.Contains(t, locRemove, "range-start=2026-01-01")

	recOpenRedirect := httptest.NewRecorder()
	router.ServeHTTP(recOpenRedirect, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/toggle-filter?lens_filter_target=https://evil.example&lens_filter_dimension=agent_type&lens_filter_value=legal", nil))
	require.Equal(t, http.StatusBadRequest, recOpenRedirect.Code)

	// 5. Leaf drill-down document + XLSX export with VisibleTableColumnsOnly & OmitLineage.
	recLeaf := httptest.NewRecorder()
	router.ServeHTTP(recLeaf, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/document?_f=agent_type:legal&_f=agency:a2", nil))
	require.Equal(t, http.StatusOK, recLeaf.Code)
	var leafDoc document.DashboardDocument
	require.NoError(t, json.Unmarshal(recLeaf.Body.Bytes(), &leafDoc))
	var foundLeafPanel bool
	for _, p := range leafDoc.Panels {
		if p.ID == "policies_leaf" {
			foundLeafPanel = true
			require.Equal(t, document.PanelKindTable, p.Kind)
		}
	}
	require.True(t, foundLeafPanel)

	recExport := httptest.NewRecorder()
	router.ServeHTTP(recExport, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/export?snapshot="+url.QueryEscape(leafDoc.SnapshotID), nil))
	require.Equal(t, http.StatusOK, recExport.Code)
	wb, err := excelize.OpenReader(bytes.NewReader(recExport.Body.Bytes()))
	require.NoError(t, err)
	defer func() { _ = wb.Close() }()
	// OmitLineage=true drops the "Источники"/"Sources" sheet.
	for _, sheet := range wb.GetSheetList() {
		require.NotEqual(t, "Источники", sheet)
		require.NotEqual(t, "Sources", sheet)
	}

	// Cross-tenant scope isolation returns 410 Gone on export.
	crossReq := httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/export?snapshot="+url.QueryEscape(leafDoc.SnapshotID), nil)
	crossReq.Header.Set("X-Tenant", "tenant-other")
	recCross := httptest.NewRecorder()
	router.ServeHTTP(recCross, crossReq)
	require.Equal(t, http.StatusGone, recCross.Code)

	// 6. Release endpoint returns 204.
	releaseBody, err := json.Marshal(document.ReleaseRequest{SnapshotID: doc1.SnapshotID})
	require.NoError(t, err)
	recRelease := httptest.NewRecorder()
	router.ServeHTTP(recRelease, httptest.NewRequest(http.MethodPost, "/analytics/agent-program/lens/release", bytes.NewReader(releaseBody)))
	require.Equal(t, http.StatusNoContent, recRelease.Code)

	// 7. Observer receives internal execution error.
	exec.failNextRequest.Store(true)
	recErr := httptest.NewRecorder()
	router.ServeHTTP(recErr, httptest.NewRequest(http.MethodGet, "/analytics/agent-program/lens/document?_f=agent_type:individual", nil))
	require.Equal(t, http.StatusInternalServerError, recErr.Code)
	require.GreaterOrEqual(t, observedErrors.Load(), int32(1))
}

func TestHost_ProgressiveRecomputeCooldownWaitsGracefully(t *testing.T) {
	t.Parallel()

	// Would be falsely green if recompute failed with cooldown error instead of waiting gracefully.
	rt := lensruntime.New(lensruntime.Options{})
	exec := &countingExecutor{base: engine.New(rt).Executor()}

	host, err := serve.NewHost(serve.HostConfig{
		BasePath:    "/analytics/progressive/lens",
		Executor:    exec,
		Progressive: true,
		Cache: serve.CacheOptions{
			RecomputeCooldown: 80 * time.Millisecond,
		},
		ResolveCube: func(_ context.Context, _ *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			return serve.ResolvedCube{
				Spec: buildTestCubeSpec(t),
				Request: lensruntime.Request{
					DataScope: "tenant-1",
					Locale:    "en",
				},
			}, nil
		},
	})
	require.NoError(t, err)
	defer host.Close()

	handler := host.Handler()

	// Step 1: Fetch progressive document.
	recDoc := httptest.NewRecorder()
	handler.ServeHTTP(recDoc, httptest.NewRequest(http.MethodGet, "/analytics/progressive/lens/document", nil))
	require.Equal(t, http.StatusOK, recDoc.Code)
	var doc document.DashboardDocument
	require.NoError(t, json.Unmarshal(recDoc.Body.Bytes(), &doc))
	require.Equal(t, "/analytics/progressive/lens/panel", doc.Endpoints.Panel)

	// Step 2: Materialize panel initially.
	started := time.Now()
	panelReq, err := json.Marshal(document.PanelBatchRequest{
		SnapshotID: doc.SnapshotID,
		Panels:     []document.PanelRequest{{PanelID: "by_type"}},
	})
	require.NoError(t, err)
	recPanel1 := httptest.NewRecorder()
	handler.ServeHTTP(recPanel1, httptest.NewRequest(http.MethodPost, "/analytics/progressive/lens/panel", bytes.NewReader(panelReq)))
	require.Equal(t, http.StatusOK, recPanel1.Code)
	require.Equal(t, "identity", recPanel1.Header().Get("Content-Encoding"))

	// Step 3: Immediately trigger Recompute=true within the 80ms cooldown.
	recomputeReq, err := json.Marshal(document.PanelBatchRequest{
		SnapshotID: doc.SnapshotID,
		Panels:     []document.PanelRequest{{PanelID: "by_type", Recompute: true}},
	})
	require.NoError(t, err)
	recPanel2 := httptest.NewRecorder()
	handler.ServeHTTP(recPanel2, httptest.NewRequest(http.MethodPost, "/analytics/progressive/lens/panel", bytes.NewReader(recomputeReq)))
	require.Equal(t, http.StatusOK, recPanel2.Code)
	require.GreaterOrEqual(t, time.Since(started), 75*time.Millisecond)
	require.GreaterOrEqual(t, exec.recomputeSeen.Load(), int32(1))

	// Decode NDJSON stream and assert no panel error occurred.
	dec := json.NewDecoder(recPanel2.Body)
	var sawPanel, sawComplete bool
	for dec.More() {
		var evt document.PanelBatchStreamEvent
		require.NoError(t, dec.Decode(&evt))
		if evt.Complete {
			sawComplete = true
			continue
		}
		if evt.PanelID == "by_type" {
			sawPanel = true
			require.NotNil(t, evt.Result)
			require.Nil(t, evt.Result.Error, "recompute must wait out cooldown instead of failing panel")
		}
	}
	require.True(t, sawPanel)
	require.True(t, sawComplete)

	// Step 4: Progressive export materializes remaining panels and succeeds.
	recExport := httptest.NewRecorder()
	handler.ServeHTTP(recExport, httptest.NewRequest(http.MethodGet, "/analytics/progressive/lens/export?snapshot="+url.QueryEscape(doc.SnapshotID), nil))
	require.Equal(t, http.StatusOK, recExport.Code)
}

type stubHostExplorationLoader struct {
	fn func(context.Context, lensruntime.ExplorationLoadRequest) (lensruntime.ExplorationDefinition, error)
}

func (l stubHostExplorationLoader) LoadExploration(
	ctx context.Context,
	req lensruntime.ExplorationLoadRequest,
) (lensruntime.ExplorationDefinition, error) {
	return l.fn(ctx, req)
}

type stubHostExplorationExecutor struct {
	calls atomic.Int32
}

func (s *stubHostExplorationExecutor) ExecuteExploration(
	ctx context.Context,
	_ lens.DashboardSpec,
	loader lensruntime.ExplorationLoader,
	req lensruntime.ExplorationLoadRequest,
	_ lensruntime.Request,
) (*lensruntime.ExplorationResult, error) {
	s.calls.Add(1)
	def, err := loader.LoadExploration(ctx, req)
	if err != nil {
		return nil, err
	}
	var targetPanel panel.Spec
	for _, row := range def.Dashboard.Rows {
		for _, p := range row.Panels {
			if p.ID == def.PanelID {
				targetPanel = p
			}
		}
	}
	var frames *frame.FrameSet
	for _, ds := range def.Dashboard.Datasets {
		if ds.Name == targetPanel.Dataset {
			frames = ds.Static
		}
	}
	return &lensruntime.ExplorationResult{
		Panel: &lensruntime.PanelResult{
			Panel:  targetPanel,
			Frames: frames,
		},
	}, nil
}

func TestHost_DashboardMode_ExplorationDrawerAndExportAuth(t *testing.T) {
	t.Parallel()

	// Would be falsely green if Host did not wire ResolveDashboard, Exploration, Drawer, and Export.Authorize.
	rt := lensruntime.New(lensruntime.Options{})
	exploreExec := &stubHostExplorationExecutor{}

	rootFrames, err := frame.FromRows("root", frame.Row{"id": "2026", "label": "2026", "value": 100.0})
	require.NoError(t, err)
	detailFrames, err := frame.FromRows("detail", frame.Row{"id": "osago", "label": "OSAGO", "value": 60.0})
	require.NoError(t, err)

	detailPanel := panel.Bar("detail_panel", "Detail", "detail_ds").
		IDField("id").LabelField("label").ValueField("value").Terminal().Build()

	dashSpec := lens.DashboardSpec{
		ID:    "custom_dash",
		Title: "Custom Dashboard",
		Datasets: []lens.DatasetSpec{
			{Name: "root_ds", Kind: lens.DatasetKindStatic, Static: rootFrames},
			{Name: "detail_ds", Kind: lens.DatasetKindStatic, Static: detailFrames},
		},
		Rows: []lens.RowSpec{{
			Panels: []panel.Spec{
				panel.Stat("headline", "Headline", "root_ds").
					ValueField("value").
					Action(action.Explore("main", "premium")).
					Build(),
			},
		}},
		Explorers: []explore.Spec{{
			ID:          "main",
			HostPanelID: "headline",
			Branches: []explore.Branch{{
				Key:                "premium",
				Label:              "Premium",
				DefaultPerspective: "by_product",
				Perspectives: []explore.Perspective{{
					Key:       "by_product",
					Label:     "By product",
					Semantics: explore.SemanticsPartition,
					RootNode:  "products",
					Nodes: []explore.Node{{
						Key:   "products",
						Label: "Products",
						Panel: &detailPanel,
					}},
				}},
			}},
		}},
	}
	require.NoError(t, lensruntime.Validate(dashSpec))

	host, err := serve.NewHost(serve.HostConfig{
		BasePath:    "/analytics/custom/lens",
		Executor:    engine.New(rt).Executor(),
		Exploration: exploreExec,
		InlineDepth: 0,
		ResolveLoader: func(_ context.Context, _ lensruntime.ExplorationLoadRequest, _ lensruntime.Request) (lensruntime.ExplorationLoader, error) {
			return stubHostExplorationLoader{
				fn: func(_ context.Context, _ lensruntime.ExplorationLoadRequest) (lensruntime.ExplorationDefinition, error) {
					return lensruntime.ExplorationDefinition{
						PanelID: detailPanel.ID,
						Dashboard: lens.DashboardSpec{
							ID:    "detail_dash",
							Title: "Detail",
							Datasets: []lens.DatasetSpec{
								{Name: "detail_ds", Kind: lens.DatasetKindStatic, Static: detailFrames},
							},
							Rows: []lens.RowSpec{{Panels: []panel.Spec{detailPanel}}},
						},
					}, nil
				},
			}, nil
		},
		DrawerResolver: func(_ *http.Request, req document.DrawerResolveRequest, _ lensruntime.Request) (string, error) {
			return "/analytics/custom/drawer/" + req.MetricKey, nil
		},
		Export: serve.ExportOptions{
			Authorize: func(r *http.Request, _, _ string) error {
				if r.Header.Get("X-Allow-Export") != "true" {
					return errors.New("denied")
				}
				return nil
			},
		},
		ResolveDashboard: func(_ context.Context, _ *http.Request) (serve.ResolvedDashboard, error) {
			return serve.ResolvedDashboard{
				Spec: dashSpec,
				Request: lensruntime.Request{
					DataScope: "tenant-1",
					Locale:    "en",
				},
			}, nil
		},
	})
	require.NoError(t, err)
	defer host.Close()

	handler := host.Handler()

	// 1. Fetch dashboard document.
	recDoc := httptest.NewRecorder()
	handler.ServeHTTP(recDoc, httptest.NewRequest(http.MethodGet, "/analytics/custom/lens/document", nil))
	require.Equal(t, http.StatusOK, recDoc.Code)
	var doc document.DashboardDocument
	require.NoError(t, json.Unmarshal(recDoc.Body.Bytes(), &doc))
	require.Equal(t, "/analytics/custom/lens/drawer", doc.Endpoints.Drawer)

	// 2. Query exploration level.
	qBody, err := json.Marshal(document.QueryRequest{
		SnapshotID:  doc.SnapshotID,
		Path:        document.NodePath{"products"},
		Perspective: "by_product",
	})
	require.NoError(t, err)
	recQuery := httptest.NewRecorder()
	handler.ServeHTTP(recQuery, httptest.NewRequest(http.MethodPost, "/analytics/custom/lens/query", bytes.NewReader(qBody)))
	require.Equal(t, http.StatusOK, recQuery.Code, recQuery.Body.String())
	require.Equal(t, int32(1), exploreExec.calls.Load())

	// 3. Resolve drawer URL.
	dBody, err := json.Marshal(document.DrawerResolveRequest{
		SnapshotID: doc.SnapshotID,
		MetricKey:  "premium_total",
	})
	require.NoError(t, err)
	recDrawer := httptest.NewRecorder()
	handler.ServeHTTP(recDrawer, httptest.NewRequest(http.MethodPost, "/analytics/custom/lens/drawer", bytes.NewReader(dBody)))
	require.Equal(t, http.StatusOK, recDrawer.Code)
	var drawerResp document.DrawerResolveResponse
	require.NoError(t, json.Unmarshal(recDrawer.Body.Bytes(), &drawerResp))
	require.Contains(t, drawerResp.URL, "/analytics/custom/drawer/premium_total")
	require.Contains(t, drawerResp.URL, "_lens_snapshot="+url.QueryEscape(doc.SnapshotID))

	// 4. Export authorization enforced (403 without header, 200 with header).
	recDenied := httptest.NewRecorder()
	handler.ServeHTTP(recDenied, httptest.NewRequest(http.MethodGet, "/analytics/custom/lens/export?snapshot="+url.QueryEscape(doc.SnapshotID), nil))
	require.Equal(t, http.StatusForbidden, recDenied.Code)

	allowReq := httptest.NewRequest(http.MethodGet, "/analytics/custom/lens/export?snapshot="+url.QueryEscape(doc.SnapshotID), nil)
	allowReq.Header.Set("X-Allow-Export", "true")
	recAllowed := httptest.NewRecorder()
	handler.ServeHTTP(recAllowed, allowReq)
	require.Equal(t, http.StatusOK, recAllowed.Code)

	// 5. Cube-only endpoints are not mounted (404) in dashboard mode.
	recFacet404 := httptest.NewRecorder()
	handler.ServeHTTP(recFacet404, httptest.NewRequest(http.MethodGet, "/analytics/custom/lens/facet-options?_facet=x", nil))
	require.Equal(t, http.StatusNotFound, recFacet404.Code)
}

type stubSQLDataSource struct {
	queryFunc func(ctx context.Context, req datasource.QueryRequest) (*frame.FrameSet, error)
}

func (s *stubSQLDataSource) Run(ctx context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
	return s.queryFunc(ctx, req)
}

func (s *stubSQLDataSource) Capabilities() datasource.CapabilitySet {
	return datasource.CapabilitySet{datasource.CapabilityParameterizedQueries: true}
}

func TestHost_SQLCubeMode_FacetOptionsAndValidation(t *testing.T) {
	t.Parallel()

	// Would be falsely green if NewHost accepted invalid configs or if SQL facet-options skipped the cube datasource.
	rt := lensruntime.New(lensruntime.Options{})
	exec := engine.New(rt).Executor()

	_, err := serve.NewHost(serve.HostConfig{BasePath: "/x"})
	require.Error(t, err, "missing executor must fail")
	_, err = serve.NewHost(serve.HostConfig{BasePath: "/x", Executor: exec})
	require.Error(t, err, "missing resolver must fail")
	_, err = serve.NewHost(serve.HostConfig{
		BasePath: "/x",
		Executor: exec,
		ResolveCube: func(_ context.Context, _ *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			return serve.ResolvedCube{}, nil
		},
		ResolveDashboard: func(_ context.Context, _ *http.Request) (serve.ResolvedDashboard, error) {
			return serve.ResolvedDashboard{}, nil
		},
	})
	require.Error(t, err, "configuring both ResolveCube and ResolveDashboard must fail")

	var capturedQuery datasource.QueryRequest
	sqlDS := &stubSQLDataSource{
		queryFunc: func(_ context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
			capturedQuery = req
			return frame.FromRows("facet",
				frame.Row{"value": "osago", "label": "OSAGO Auto", "count": 42.0},
			)
		},
	}

	sqlCube := cube.New("sql_sales", "SQL Sales").
		SQL("primary", "insurance.contracts c").
		Dimension("product", "Product").
		Column("c.product_code").
		LabelMap(map[string]string{"osago": "OSAGO Auto", "kasko": "KASKO Hull"}).
		AsFacet().
		Done().
		Measure("policies", "Policies").
		Column("c.id").
		Count().
		Done().
		Layout(
			cube.KPIRow("policies"),
			cube.SplitRow(cube.Chart("by_product", panel.KindDonut).Dimension("product").Measure("policies")),
		).
		DrillPath("product").
		LeafSpec(cube.LeafSpec{
			ExternalLink: func(_ context.Context, drill cube.DrillContext, _ map[string]any) string {
				return "/contracts?" + drill.Encode().Encode()
			},
		}).
		Build()

	host, err := serve.NewHost(serve.HostConfig{
		BasePath: "/sql/lens",
		Executor: exec,
		ResolveCube: func(_ context.Context, _ *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			return serve.ResolvedCube{
				Spec:     sqlCube,
				PagePath: "/sql",
				Request: lensruntime.Request{
					DataScope: "tenant-sql",
					DataSources: map[string]datasource.DataSource{
						"primary": sqlDS,
					},
				},
			}, nil
		},
	})
	require.NoError(t, err)
	defer host.Close()

	router := mux.NewRouter()
	host.Mount(router.PathPrefix("/sql/lens").Subrouter())

	recFacet := httptest.NewRecorder()
	router.ServeHTTP(recFacet, httptest.NewRequest(http.MethodGet, "/sql/lens/facet-options?_facet=product&_facet_search=osago", nil))
	require.Equal(t, http.StatusOK, recFacet.Code, recFacet.Body.String())

	var facetResp document.FacetOptionsResponse
	require.NoError(t, json.Unmarshal(recFacet.Body.Bytes(), &facetResp))
	require.Contains(t, capturedQuery.Text, "OSAGO Auto", "SQL facet query should apply LabelMap CASE")
	require.Equal(t, "%osago%", capturedQuery.Params["facet_search"])
	require.Len(t, facetResp.Options, 1)
	require.Equal(t, "osago", facetResp.Options[0].Value)
	require.Equal(t, "OSAGO Auto", facetResp.Options[0].Label)
	require.Equal(t, 42, facetResp.Options[0].Count)
}

type recordingPanelExecutor struct {
	base             engine.Executor
	mu               sync.Mutex
	recomputeByPanel map[string]bool
	calls            atomic.Int32
	delay            time.Duration
}

func (e *recordingPanelExecutor) Execute(ctx context.Context, spec lens.DashboardSpec, req lensruntime.Request, scope lensruntime.Scope) (*lensruntime.DashboardResult, error) {
	e.calls.Add(1)
	if len(scope.PanelIDs) > 0 {
		e.mu.Lock()
		if e.recomputeByPanel == nil {
			e.recomputeByPanel = map[string]bool{}
		}
		for _, panelID := range scope.PanelIDs {
			e.recomputeByPanel[panelID] = serve.RecomputeRequested(ctx)
		}
		e.mu.Unlock()
	}
	if e.delay > 0 && len(scope.PanelIDs) > 0 {
		timer := time.NewTimer(e.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return e.base.Execute(ctx, spec, req, scope)
}

func TestHost_PanelRecomputeScopingAndRequestResolverContext(t *testing.T) {
	t.Parallel()

	// Would be falsely green if non-recompute siblings inherited RecomputeRequested or Request resolver saw false.
	rt := lensruntime.New(lensruntime.Options{})
	exec := &recordingPanelExecutor{base: engine.New(rt).Executor()}

	var reqResolverMu sync.Mutex
	var reqResolverSawRecompute []bool

	host, err := serve.NewHost(serve.HostConfig{
		BasePath:    "/analytics/recompute/lens",
		Executor:    exec,
		Progressive: true,
		Cache:       serve.CacheOptions{RecomputeCooldown: -1},
		Request: func(r *http.Request) lensruntime.Request {
			reqResolverMu.Lock()
			reqResolverSawRecompute = append(reqResolverSawRecompute, serve.RecomputeRequested(r.Context()))
			reqResolverMu.Unlock()
			return lensruntime.Request{DataScope: "tenant-1", Locale: "en"}
		},
		ResolveCube: func(_ context.Context, _ *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			return serve.ResolvedCube{
				Spec:    buildTestCubeSpec(t),
				Request: lensruntime.Request{DataScope: "tenant-1", Locale: "en"},
			}, nil
		},
	})
	require.NoError(t, err)
	defer host.Close()

	handler := host.Handler()
	recDoc := httptest.NewRecorder()
	handler.ServeHTTP(recDoc, httptest.NewRequest(http.MethodGet, "/analytics/recompute/lens/document", nil))
	require.Equal(t, http.StatusOK, recDoc.Code)
	var doc document.DashboardDocument
	require.NoError(t, json.Unmarshal(recDoc.Body.Bytes(), &doc))

	// Ordinary panel load: Request resolver must see RecomputeRequested == false.
	ordinaryBody, err := json.Marshal(document.PanelBatchRequest{
		SnapshotID: doc.SnapshotID,
		Panels:     []document.PanelRequest{{PanelID: "top_agencies", Recompute: false}},
	})
	require.NoError(t, err)
	recOrd := httptest.NewRecorder()
	handler.ServeHTTP(recOrd, httptest.NewRequest(http.MethodPost, "/analytics/recompute/lens/panel", bytes.NewReader(ordinaryBody)))
	require.Equal(t, http.StatusOK, recOrd.Code)

	reqResolverMu.Lock()
	require.Equal(t, []bool{false}, reqResolverSawRecompute)
	reqResolverSawRecompute = nil
	reqResolverMu.Unlock()

	// Mixed batch: by_type has Recompute=true, trend has Recompute=false (uncached).
	mixedBody, err := json.Marshal(document.PanelBatchRequest{
		SnapshotID: doc.SnapshotID,
		Panels: []document.PanelRequest{
			{PanelID: "by_type", Recompute: true},
			{PanelID: "trend", Recompute: false},
		},
	})
	require.NoError(t, err)
	recMixed := httptest.NewRecorder()
	handler.ServeHTTP(recMixed, httptest.NewRequest(http.MethodPost, "/analytics/recompute/lens/panel", bytes.NewReader(mixedBody)))
	require.Equal(t, http.StatusOK, recMixed.Code)

	reqResolverMu.Lock()
	require.Equal(t, []bool{true}, reqResolverSawRecompute, "HostConfig.Request must observe recompute context")
	reqResolverMu.Unlock()

	exec.mu.Lock()
	require.True(t, exec.recomputeByPanel["by_type"], "recompute panel job must see RecomputeRequested=true")
	require.False(t, exec.recomputeByPanel["trend"], "non-recompute sibling must see RecomputeRequested=false")
	exec.mu.Unlock()
}

func TestHost_ExportAdmissionBeforeMaterializationAndDeadline(t *testing.T) {
	t.Parallel()

	// Would be falsely green if BuildLimiter saturation still ran panels or Export.Timeout ignored panel materialization.
	rt := lensruntime.New(lensruntime.Options{})
	exec := &recordingPanelExecutor{base: engine.New(rt).Executor(), delay: 120 * time.Millisecond}
	limiter := serve.NewReportBuildLimiter(1, 0)

	host, err := serve.NewHost(serve.HostConfig{
		BasePath:         "/analytics/exp/lens",
		Executor:         exec,
		ExecutionTimeout: 5 * time.Second,
		Progressive:      true,
		Cache: serve.CacheOptions{
			BuildLimiter: limiter,
		},
		Export: serve.ExportOptions{
			Timeout: 40 * time.Millisecond,
		},
		ResolveCube: func(_ context.Context, _ *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			return serve.ResolvedCube{
				Spec:    buildTestCubeSpec(t),
				Request: lensruntime.Request{DataScope: "tenant-1", Locale: "en"},
			}, nil
		},
	})
	require.NoError(t, err)
	defer host.Close()

	handler := host.Handler()
	recDoc := httptest.NewRecorder()
	handler.ServeHTTP(recDoc, httptest.NewRequest(http.MethodGet, "/analytics/exp/lens/document", nil))
	require.Equal(t, http.StatusOK, recDoc.Code)
	var doc document.DashboardDocument
	require.NoError(t, json.Unmarshal(recDoc.Body.Bytes(), &doc))
	callsAfterDoc := exec.calls.Load()

	// 1. Saturate BuildLimiter: export must be rejected BEFORE submitting any panel jobs.
	release, err := limiter.Acquire(context.Background())
	require.NoError(t, err)
	recBusy := httptest.NewRecorder()
	handler.ServeHTTP(recBusy, httptest.NewRequest(http.MethodGet, "/analytics/exp/lens/export?snapshot="+url.QueryEscape(doc.SnapshotID), nil))
	release()
	require.Equal(t, http.StatusServiceUnavailable, recBusy.Code)
	require.Equal(t, callsAfterDoc, exec.calls.Load(), "rejected export must not submit progressive panel jobs")

	// 2. With capacity available, slow panel materialization exceeding Export.Timeout must return 504 Gateway Timeout.
	recTimeout := httptest.NewRecorder()
	handler.ServeHTTP(recTimeout, httptest.NewRequest(http.MethodGet, "/analytics/exp/lens/export?snapshot="+url.QueryEscape(doc.SnapshotID), nil))
	require.Equal(t, http.StatusGatewayTimeout, recTimeout.Code)
}

func TestProjectPanelExportFields_PreservesTemporalOverlays(t *testing.T) {
	t.Parallel()

	// Would be falsely green if temporal overlay columns (e.g. sma_7) were dropped or unshown columns kept.
	fs, err := frame.FromRows("series",
		frame.Row{"period": "2026-01", "premium": 100.0, "sma_7": 95.0, "reg": 98.0, "internal_id": "secret-1"},
	)
	require.NoError(t, err)

	p := panel.TimeSeries("trend", "Premium Trend", "series").
		CategoryField("period").
		ValueField("premium").
		MovingAverage(7, panel.Ref("sma_7"), "7d SMA").
		Regression(panel.Ref("reg"), "Trendline").
		Build()

	result := &lensruntime.DashboardResult{
		Panels: map[string]*lensruntime.PanelResult{
			"trend": {Panel: p, Frames: fs},
		},
	}

	require.NoError(t, serve.ProjectPanelExportFields(result, map[string]string{
		"period":  "Period",
		"premium": "Premium",
	}))

	fields := result.Panels["trend"].Frames.Primary().Fields
	fieldLabels := map[string]string{}
	for _, f := range fields {
		fieldLabels[f.Name] = f.Labels["default"]
	}
	require.Equal(t, "Period", fieldLabels["period"])
	require.Equal(t, "Premium", fieldLabels["premium"])
	require.Equal(t, "7d SMA", fieldLabels["sma_7"], "moving average overlay column must be preserved")
	require.Equal(t, "Trendline", fieldLabels["reg"], "regression overlay column must be preserved")
	_, hasInternal := fieldLabels["internal_id"]
	require.False(t, hasInternal, "unshown unnamed columns must remain excluded")
}

func TestHost_ToggleFilterBlocksEncodedOpenRedirectsAndCacheKeyPreservesValueOrder(t *testing.T) {
	t.Parallel()

	// Would be falsely green if encoded backslash/slash bypassed redirect validation or repeated param order collided in cache.
	rt := lensruntime.New(lensruntime.Options{})
	exec := &countingExecutor{base: engine.New(rt).Executor()}

	host, err := serve.NewHost(serve.HostConfig{
		BasePath: "/analytics/sec/lens",
		Executor: exec,
		ResolveCube: func(_ context.Context, r *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			metric := r.URL.Query().Get("metric")
			spec := buildTestCubeSpec(t)
			if metric != "" {
				spec.Title = "Metric " + metric
			}
			return serve.ResolvedCube{
				Spec:    spec,
				Request: lensruntime.Request{DataScope: "tenant-1", Locale: "en"},
			}, nil
		},
	})
	require.NoError(t, err)
	defer host.Close()

	handler := host.Handler()

	for _, badTarget := range []string{"/%5Cevil.com", "/%2Fevil.com", "//evil.com", "/analytics/sec/lens/toggle-filter"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/analytics/sec/lens/toggle-filter?lens_filter_target="+badTarget+"&lens_filter_dimension=agent_type&lens_filter_value=legal", nil))
		require.Equal(t, http.StatusBadRequest, rec.Code, "target %q must be rejected", badTarget)
	}

	// Repeated query value order must produce distinct cache entries.
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, httptest.NewRequest(http.MethodGet, "/analytics/sec/lens/document?metric=claims&metric=premium", nil))
	require.Equal(t, http.StatusOK, recA.Code)
	var docA document.DashboardDocument
	require.NoError(t, json.Unmarshal(recA.Body.Bytes(), &docA))

	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, httptest.NewRequest(http.MethodGet, "/analytics/sec/lens/document?metric=premium&metric=claims", nil))
	require.Equal(t, http.StatusOK, recB.Code)
	var docB document.DashboardDocument
	require.NoError(t, json.Unmarshal(recB.Body.Bytes(), &docB))

	require.NotEqual(t, docA.SnapshotID, docB.SnapshotID, "reordered repeated values must not share cache key")
	require.Equal(t, "Metric claims", docA.Meta.Title)
	require.Equal(t, "Metric premium", docB.Meta.Title)
}

func TestHost_FacetOptionsResolvesDateRangeUntilAndNamedDrillParams(t *testing.T) {
	t.Parallel()

	// Would be falsely green if RangeUntil bound nil from browser range-start/range-end or Named mode dropped f_<dim>.
	rt := lensruntime.New(lensruntime.Options{})
	exec := engine.New(rt).Executor()

	periodVar := lens.VariableSpec{
		Name:        "period",
		Kind:        lens.VariableDateRange,
		RequestKeys: []string{"range-start", "range-end"},
	}

	// 1. SQL mode: browser range-start/range-end resolves RangeStart & RangeUntil (first instant after end date).
	var sqlCaptured datasource.QueryRequest
	sqlDS := &stubSQLDataSource{
		queryFunc: func(_ context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
			sqlCaptured = req
			return frame.FromRows("facet", frame.Row{"value": "osago", "label": "OSAGO", "count": 5.0})
		},
	}
	sqlSpec := cube.New("sql_range", "SQL Range").
		SQL("primary", "insurance.contracts c").
		Variable(periodVar).
		Dimension("product", "Product").Column("c.product_code").AsFacet().Done().
		Measure("policies", "Policies").Column("c.id").Count().Done().
		Build()
	sqlSpec.Params["from"] = lens.RangeParam("period", lens.RangeStart)
	sqlSpec.Params["until"] = lens.RangeParam("period", lens.RangeUntil)

	var useOverride atomic.Bool
	overrideStart := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	overrideEnd := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)

	sqlHost, err := serve.NewHost(serve.HostConfig{
		BasePath: "/sql-range/lens",
		Executor: exec,
		ResolveCube: func(_ context.Context, _ *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			req := lensruntime.Request{
				DataScope:   "t1",
				Timezone:    "UTC",
				DataSources: map[string]datasource.DataSource{"primary": sqlDS},
			}
			if useOverride.Load() {
				req.Overrides = map[string]any{
					"period": lens.DateRangeValue{Mode: "custom", Start: &overrideStart, End: &overrideEnd},
				}
			}
			return serve.ResolvedCube{Spec: sqlSpec, PagePath: "/sql-range", Request: req}, nil
		},
	})
	require.NoError(t, err)
	defer sqlHost.Close()

	recSQL := httptest.NewRecorder()
	sqlHost.Handler().ServeHTTP(recSQL, httptest.NewRequest(http.MethodGet, "/sql-range/lens/facet-options?_facet=product&range-start=2026-01-01&range-end=2026-01-31", nil))
	require.Equal(t, http.StatusOK, recSQL.Code, recSQL.Body.String())
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), sqlCaptured.Params["from"])
	require.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), sqlCaptured.Params["until"], "RangeUntil must be exclusive first instant after 2026-01-31")

	// Explicit typed override still takes precedence over query params.
	useOverride.Store(true)
	recSQLOverride := httptest.NewRecorder()
	sqlHost.Handler().ServeHTTP(recSQLOverride, httptest.NewRequest(http.MethodGet, "/sql-range/lens/facet-options?_facet=product&range-start=2026-01-01&range-end=2026-01-31", nil))
	require.Equal(t, http.StatusOK, recSQLOverride.Code, recSQLOverride.Body.String())
	require.Equal(t, overrideStart, sqlCaptured.Params["from"])
	require.Equal(t, overrideEnd.Add(time.Nanosecond), sqlCaptured.Params["until"])

	// 2. Named mode: binds active cross-dimension drill filters (f_region), binds facet's own f_product key, and resolves RangeUntil.
	var namedCaptured datasource.QueryRequest
	namedDS := datasource.NewNamedDataSource()
	require.NoError(t, namedDS.Register("policies_named", func(_ context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
		namedCaptured = req
		return frame.FromRows("policies",
			frame.Row{"region": "tashkent", "product": "osago", "premium": 100.0},
			frame.Row{"region": "tashkent", "product": "kasko", "premium": 200.0},
		)
	}))
	namedSpec := cube.New("named_range", "Named Range").
		Named("domain", "policies_named").
		Variable(periodVar).
		Dimension("region", "Region").Field("region").AsFacet().Done().
		Dimension("product", "Product").Field("product").AsFacet().Done().
		Measure("premium", "Premium").Field("premium").Sum().Done().
		Build()
	namedSpec.Params["until"] = lens.RangeParam("period", lens.RangeUntil)

	namedHost, err := serve.NewHost(serve.HostConfig{
		BasePath: "/named-range/lens",
		Executor: exec,
		ResolveCube: func(_ context.Context, _ *http.Request, _ cube.DrillContext) (serve.ResolvedCube, error) {
			return serve.ResolvedCube{
				Spec:     namedSpec,
				PagePath: "/named-range",
				Request: lensruntime.Request{
					DataScope:   "t1",
					Timezone:    "UTC",
					DataSources: map[string]datasource.DataSource{"domain": namedDS},
				},
			}, nil
		},
	})
	require.NoError(t, err)
	defer namedHost.Close()

	recNamed := httptest.NewRecorder()
	namedHost.Handler().ServeHTTP(recNamed, httptest.NewRequest(http.MethodGet, "/named-range/lens/facet-options?_facet=product&_f=region:tashkent&_f=product:osago&range-start=2026-03-01&range-end=2026-03-31", nil))
	require.Equal(t, http.StatusOK, recNamed.Code, recNamed.Body.String())
	require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), namedCaptured.Params["until"])
	require.Equal(t, []string{"tashkent"}, namedCaptured.Params["f_region"], "active cross-dimension drill filter must be bound for Named query")
	_, hasOwnDimKey := namedCaptured.Params["f_product"]
	require.True(t, hasOwnDimKey, "facet dimension param key should be explicitly bound")
	require.Nil(t, namedCaptured.Params["f_product"], "facet's own dimension param should bind nil so unselected options remain visible")
}
