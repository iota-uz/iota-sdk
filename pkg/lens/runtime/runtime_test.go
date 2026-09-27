package runtime

import (
	"context"
	"math"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubDataSource struct {
	calls     atomic.Int32
	lastClass atomic.Value
}

func execute(ctx context.Context, spec lens.DashboardSpec, req Request) (*DashboardResult, error) {
	if req.DataScope == "" {
		req.DataScope = "test"
	}
	if req.DataSourceIdentities == nil {
		req.DataSourceIdentities = map[string]string{}
	}
	for name := range req.DataSources {
		req.DataSourceIdentities[name] = "test:" + name
	}
	return New(Options{}).Execute(ctx, spec, req, DashboardScope())
}

func TestExecute_RejectsIncompleteCacheIdentity(t *testing.T) {
	t.Parallel()
	spec := lensbuild.Dashboard("identity", "Identity", lensbuild.Row(panel.Bar("chart", "Chart", "data").Terminal().Build())).Datasets(lensbuild.QueryDataset("data", "primary", "select 1")).Build()
	ds := &stubDataSource{}
	_, err := New(Options{}).Execute(context.Background(), spec, Request{DataSources: map[string]datasource.DataSource{"primary": ds}}, DashboardScope())
	require.ErrorContains(t, err, "data scope is required")
	_, err = New(Options{}).Execute(context.Background(), spec, Request{DataScope: "tenant:a", DataSources: map[string]datasource.DataSource{"primary": ds}}, DashboardScope())
	require.ErrorContains(t, err, `datasource identity for "primary" is required`)
}

func TestExecute_FailsClosedForUnserializableIdentity(t *testing.T) {
	t.Parallel()
	frames := mustFrameSet(t, "static")
	spec := lensbuild.Dashboard("identity", "Identity", lensbuild.Row(panel.Table("table", "Table", "data").Terminal().Build())).Datasets(lensbuild.StaticDataset("data", frames)).Build()
	spec.Variables = []lens.VariableSpec{{Name: "invalid", Default: "valid"}}
	_, err := New(Options{}).Execute(context.Background(), spec, Request{DataScope: "tenant:a", Overrides: map[string]any{"invalid": func() {}}}, DashboardScope())
	require.Error(t, err)
}

func (s *stubDataSource) Run(_ context.Context, req datasource.QueryRequest) (*frame.FrameSet, error) {
	s.calls.Add(1)
	s.lastClass.Store(req.ExecutionClass)
	fr, err := frame.New(req.Source,
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"a", "b"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{1.0, 2.0}},
	)
	if err != nil {
		return nil, err
	}
	return frame.NewFrameSet(fr)
}

func TestRunForwardsSchedulerExecutionClassWithoutChangingQueryIdentity(t *testing.T) {
	t.Parallel()
	ds := &stubDataSource{}
	spec := lensbuild.Dashboard("class", "Class",
		lensbuild.Row(panel.Bar("chart", "Chart", "data").Terminal().Build()),
	).Datasets(lensbuild.QueryDataset("data", "primary", "select 1")).Build()

	_, err := execute(t.Context(), spec, Request{
		ExecutionClass: datasource.ExecutionClassRootCritical,
		DataSources:    map[string]datasource.DataSource{"primary": ds},
	})
	require.NoError(t, err)
	require.Equal(t, datasource.ExecutionClassRootCritical, ds.lastClass.Load())

	base := datasource.QueryRequest{Source: "primary", Text: "select 1", ExecutionClass: datasource.ExecutionClassRootCritical}
	promoted := base
	promoted.ExecutionClass = datasource.ExecutionClassInteractiveCritical
	require.Equal(t, queryCacheKey(base), queryCacheKey(promoted), "priority promotion must join the same semantic query")
}

func (s *stubDataSource) Capabilities() datasource.CapabilitySet {
	return datasource.CapabilitySet{datasource.CapabilityParameterizedQueries: true}
}

func TestRunReusesDatasetAcrossPanels(t *testing.T) {
	t.Parallel()

	ds := &stubDataSource{}
	spec := lensbuild.Dashboard("shared", "Shared Dataset",
		lensbuild.Row(
			panel.Bar("p1", "Panel 1", "shared-data").Terminal().Build(),
			panel.Table("p2", "Panel 2", "shared-data").Terminal().Build(),
		),
	).Datasets(
		lensbuild.QueryDataset("shared-data", "primary", "select 1"),
	).Build()

	result, err := execute(context.Background(), spec, Request{
		DataSources: map[string]datasource.DataSource{"primary": ds},
	})
	require.NoError(t, err)
	require.Len(t, result.Panels, 2)
	require.Equal(t, int32(1), ds.calls.Load())
}

func TestApplyFrameOutputMetadataPromotesDeferredTotals(t *testing.T) {
	t.Parallel()

	placeholder := 5.0
	actual := 210.0
	fr, err := frame.New("expenses",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Acquisition", "Operations"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{150.0, 60.0}},
	)
	require.NoError(t, err)
	fr.Meta.AuthoritativeTotal = &actual
	fr.Meta.SeriesTotals = map[string]float64{"actual": 210, "plan": 240}
	frames, err := frame.NewFrameSet(fr)
	require.NoError(t, err)

	spec := panel.MultiRingDonut("expenses", "Expenses", "expenses",
		panel.RadialRing{Key: "actual", Total: placeholder},
		panel.RadialRing{Key: "plan", Total: placeholder},
	).TotalBadgeValue(placeholder).Terminal().Build()
	result := applyFrameOutputMetadata(spec, frames)

	require.NotNil(t, result.TotalBadgeValue)
	require.InDelta(t, actual, *result.TotalBadgeValue, 1e-9)
	require.InDelta(t, 210.0, result.Radial.Rings[0].Total, 1e-9)
	require.InDelta(t, 240.0, result.Radial.Rings[1].Total, 1e-9)
	require.InDelta(t, placeholder, spec.Radial.Rings[0].Total, 1e-9, "the frozen structural panel must not be mutated")
}

func TestRunSanitizesInternalPaginationParamsAndPreservesPath(t *testing.T) {
	t.Parallel()

	ds := &stubDataSource{}
	spec := lensbuild.Dashboard("shared", "Shared Dataset",
		lensbuild.Row(
			panel.Table("p1", "Panel 1", "shared-data").Terminal().Build(),
		),
	).Datasets(
		lensbuild.QueryDataset("shared-data", "primary", "select 1"),
	).Build()

	result, err := execute(context.Background(), spec, Request{
		Path: "/reports/drill/contracts",
		Request: url.Values{
			TablePaginationPanelQuery: []string{"p1"},
			TablePaginationPageQuery:  []string{"2"},
			TablePaginationLimitQuery: []string{"50"},
			"issue_at_from":           []string{"2026-03-01"},
		},
		DataSources: map[string]datasource.DataSource{"primary": ds},
	})
	require.NoError(t, err)
	require.Equal(t, "/reports/drill/contracts", result.RequestPath)
	require.NotContains(t, result.Request, TablePaginationPanelQuery)
	panelResult := result.Panels["p1"]
	require.NotNil(t, panelResult)
	require.Equal(t, "/reports/drill/contracts", panelResult.RequestPath)
	require.NotContains(t, panelResult.Request, TablePaginationPageQuery)
	require.Equal(t, "2026-03-01", panelResult.Request.Get("issue_at_from"))
}

func TestTablePaginationHelpers(t *testing.T) {
	t.Parallel()

	values := url.Values{
		TablePaginationPanelQuery: []string{"contracts-table"},
		TablePaginationPageQuery:  []string{"3"},
		TablePaginationLimitQuery: []string{"75"},
	}

	require.True(t, IsTableChunkRequest(values, "contracts-table"))
	require.False(t, IsTableChunkRequest(values, "policies-table"))
	require.Equal(t, 3, tablePage(values, DefaultTablePage))
	require.Equal(t, 75, tablePerPage(values, DefaultTablePerPage))
	pageState := ParseTablePageState(values, DefaultTablePerPage)
	require.Equal(t, 3, pageState.Page)
	require.Equal(t, 75, pageState.PerPage)
	require.Equal(t, 150, pageState.Offset)
	require.Equal(t, []string{"contracts-table"}, TableChunkScope(values, "contracts-table").PanelIDs)
	require.Empty(t, TableChunkScope(values, "policies-table").PanelIDs)
	require.Equal(t, DefaultTablePage, tablePage(url.Values{}, DefaultTablePage))
	require.Equal(t, DefaultTablePerPage, tablePerPage(url.Values{}, DefaultTablePerPage))
	overflow := ParseTablePageState(url.Values{
		TablePaginationPageQuery:  []string{"9223372036854775807"},
		TablePaginationLimitQuery: []string{"9223372036854775807"},
	}, DefaultTablePerPage)
	require.GreaterOrEqual(t, overflow.Offset, 0)
	require.LessOrEqual(t, overflow.Page, MaxTablePage)
}

func TestApplyTablePagination(t *testing.T) {
	t.Parallel()

	result := &Result{
		Panels: map[string]*PanelResult{
			"contracts-table": {Panel: panel.Spec{ID: "contracts-table"}},
		},
	}

	ApplyTablePagination(result, "contracts-table", 50, 25, 25, 120)

	panelResult := result.Panel("contracts-table")
	require.NotNil(t, panelResult)
	require.NotNil(t, panelResult.TablePagination)
	require.Equal(t, 3, panelResult.TablePagination.Page)
	require.Equal(t, 25, panelResult.TablePagination.PerPage)
	require.True(t, panelResult.TablePagination.HasMore)
}

func TestPlan_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		spec   lens.DashboardSpec
		assert func(t *testing.T, plan ExecutionPlan)
	}{
		{
			name: "includes only required datasets",
			spec: lensbuild.Dashboard("planned", "Planned",
				lensbuild.Row(
					panel.Bar("sales", "Sales", "daily_sales").Terminal().Build(),
				),
			).Datasets(
				lensbuild.StaticDataset("source_lookup", mustFrameSet(t, "source_lookup")),
				lens.DatasetSpec{
					Name:       "daily_sales",
					Kind:       lens.DatasetKindTransform,
					DependsOn:  []string{"raw_sales", "source_lookup"},
					Transforms: nil,
				},
				lensbuild.QueryDataset("raw_sales", "primary", "select 1"),
				lensbuild.StaticDataset("unused_dataset", mustFrameSet(t, "unused_dataset")),
			).Build(),
			assert: func(t *testing.T, plan ExecutionPlan) {
				t.Helper()
				assert.Len(t, plan.DatasetStages, 2)
				assert.ElementsMatch(t, []string{"raw_sales", "source_lookup"}, plan.DatasetStages[0].Datasets)
				assert.Equal(t, []string{"daily_sales"}, plan.DatasetStages[1].Datasets)
				assert.NotContains(t, plan.DatasetStages[0].Datasets, "unused_dataset")
				assert.Equal(t, []string{"sales"}, plan.Panels)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := Plan(tt.spec, DashboardScope())
			require.NoError(t, err)
			tt.assert(t, plan)
		})
	}
}

func TestPlan_ExportScopeMaterializesEvidenceWithoutSlowingDashboardScope(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("audit", "Audit",
		lensbuild.Row(panel.Bar("sales", "Sales", "sales").Export("/export", "panel_evidence").Terminal().Build()),
	).Datasets(
		lensbuild.QueryDataset("sales", "primary", "select 1"),
		lensbuild.QueryDataset("panel_evidence", "primary", "select 2"),
		lensbuild.QueryDataset("dashboard_evidence", "primary", "select 3"),
	).Export("/export", "audit").Build()
	spec.Export.EvidenceDatasets = []string{"dashboard_evidence"}

	dashboardPlan, err := Plan(spec, DashboardScope())
	require.NoError(t, err)
	require.Len(t, dashboardPlan.DatasetStages, 1)
	assert.Equal(t, []string{"sales"}, dashboardPlan.DatasetStages[0].Datasets)

	exportPlan, err := Plan(spec, DashboardExportScope())
	require.NoError(t, err)
	require.Len(t, exportPlan.DatasetStages, 1)
	assert.ElementsMatch(t, []string{"sales", "panel_evidence", "dashboard_evidence"}, exportPlan.DatasetStages[0].Datasets)

	panelExportPlan, err := Plan(spec, PanelExportScope("sales"))
	require.NoError(t, err)
	require.Len(t, panelExportPlan.DatasetStages, 1)
	assert.ElementsMatch(t, []string{"sales", "panel_evidence"}, panelExportPlan.DatasetStages[0].Datasets)
	assert.NotContains(t, panelExportPlan.DatasetStages[0].Datasets, "dashboard_evidence")
}

func TestRun_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		spec   lens.DashboardSpec
		assert func(t *testing.T, result *Result, ds *stubDataSource)
	}{
		{
			name: "skips unused datasets",
			spec: lensbuild.Dashboard("planned", "Planned",
				lensbuild.Row(
					panel.Bar("sales", "Sales", "shared-data").Terminal().Build(),
				),
			).Datasets(
				lensbuild.QueryDataset("shared-data", "primary", "select 1"),
				lensbuild.QueryDataset("unused-data", "primary", "select 2"),
			).Build(),
			assert: func(t *testing.T, result *Result, ds *stubDataSource) {
				t.Helper()
				assert.Equal(t, int32(1), ds.calls.Load())
				assert.Contains(t, result.Datasets, "shared-data")
				assert.NotContains(t, result.Datasets, "unused-data")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := &stubDataSource{}
			result, err := execute(context.Background(), tt.spec, Request{
				DataSources: map[string]datasource.DataSource{"primary": ds},
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.assert(t, result, ds)
		})
	}
}

func TestValidateRejectsDatasetCycles(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("cycle", "Cycle").Datasets(
		lens.DatasetSpec{Name: "a", Kind: lens.DatasetKindTransform, DependsOn: []string{"b"}},
		lens.DatasetSpec{Name: "b", Kind: lens.DatasetKindTransform, DependsOn: []string{"a"}},
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestDateRangeVariableSupportsAllTimeAndDefaults(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lensbuild.DateRangeVariable("range", "Range", 24*time.Hour),
	).Build()

	defaults, err := resolveVariables(spec.Variables, Request{Request: url.Values{}})
	require.NoError(t, err)
	defaultRange := defaults["range"].(lens.DateRangeValue)
	require.Equal(t, "default", defaultRange.Mode)
	require.NotNil(t, defaultRange.Start)
	require.NotNil(t, defaultRange.End)

	allTime, err := resolveVariables(spec.Variables, Request{Request: url.Values{"range": []string{"all"}}})
	require.NoError(t, err)
	allRange := allTime["range"].(lens.DateRangeValue)
	require.Equal(t, "all", allRange.Mode)
}

func TestDateRangeVariableUsesStartAndEndRequestKeysWhenModeKeyIsPresent(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lensbuild.DateRangeVariable("range", "Range", 24*time.Hour),
	).Build()

	values := url.Values{
		"range":       []string{"bounded"},
		"range-start": []string{"2026-03-01"},
		"range-end":   []string{"2026-03-15"},
	}

	resolved, err := resolveVariables(spec.Variables, Request{Request: values})
	require.NoError(t, err)

	bounded := resolved["range"].(lens.DateRangeValue)
	require.Equal(t, "bounded", bounded.Mode)
	require.NotNil(t, bounded.Start)
	require.NotNil(t, bounded.End)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), bounded.Start.UTC())
	assert.Equal(t, time.Date(2026, 3, 15, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC), bounded.End.UTC())
}

func TestCompareVariableResolvesEveryModeAgainstItsDateRange(t *testing.T) {
	t.Parallel()

	variables := []lens.VariableSpec{
		lensbuild.CompareVariable("compare", "Compare", "range"),
		lensbuild.DateRangeVariable("range", "Range", 24*time.Hour),
	}
	base := func() url.Values {
		return url.Values{
			"range":       []string{"bounded"},
			"range-start": []string{"2026-03-10"},
			"range-end":   []string{"2026-03-15"},
		}
	}
	assertRange := func(mode lens.CompareMode, start, end string) {
		request := base()
		request.Set("compare", string(mode))
		resolved, err := resolveVariables(variables, Request{Request: request})
		require.NoError(t, err)
		value := resolved["compare"].(lens.CompareValue)
		require.Equal(t, mode, value.Mode)
		require.Equal(t, start, value.Range.Start.Format("2006-01-02"))
		require.Equal(t, end, value.Range.End.Format("2006-01-02"))
	}

	assertRange(lens.ComparePreviousPeriod, "2026-03-04", "2026-03-09")
	assertRange(lens.CompareYearAgo, "2025-03-10", "2025-03-15")

	custom := base()
	custom.Set("compare", string(lens.CompareCustom))
	custom.Set("compare-start", "2026-02-01")
	custom.Set("compare-end", "2026-02-28")
	resolved, err := resolveVariables(variables, Request{Request: custom})
	require.NoError(t, err)
	require.Equal(t, lens.CompareCustom, resolved["compare"].(lens.CompareValue).Mode)

	off, err := resolveVariables(variables, Request{Request: base()})
	require.NoError(t, err)
	require.Equal(t, lens.CompareOff, off["compare"].(lens.CompareValue).Mode)
}

func TestCompareVariableUsesCalendarPeriodsAndRequestTimezone(t *testing.T) {
	t.Parallel()

	variables := []lens.VariableSpec{
		lensbuild.DateRangeVariable("range", "Range", 24*time.Hour),
		lensbuild.CompareVariable("compare", "Compare", "range"),
	}

	previous, err := resolveVariables(variables, Request{Timezone: "Asia/Tashkent", Request: url.Values{
		"range": []string{"bounded"}, "range-start": []string{"2026-03-01"}, "range-end": []string{"2026-03-31"},
		"compare": []string{"previous_period"},
	}})
	require.NoError(t, err)
	current := previous["range"].(lens.DateRangeValue)
	baseline := previous["compare"].(lens.CompareValue).Range
	require.Equal(t, "Asia/Tashkent", current.Start.Location().String())
	require.Equal(t, "2026-02-01T00:00:00+05:00", current.Start.AddDate(0, -1, 0).Format(time.RFC3339))
	require.Equal(t, "2026-02-01", baseline.Start.Format("2006-01-02"))
	require.Equal(t, "2026-02-28", baseline.End.Format("2006-01-02"))

	yearAgo, err := resolveVariables(variables, Request{Timezone: "Asia/Tashkent", Request: url.Values{
		"range": []string{"bounded"}, "range-start": []string{"2028-02-01"}, "range-end": []string{"2028-02-29"},
		"compare": []string{"year_ago"},
	}})
	require.NoError(t, err)
	baseline = yearAgo["compare"].(lens.CompareValue).Range
	require.Equal(t, "2027-02-01", baseline.Start.Format("2006-01-02"))
	require.Equal(t, "2027-02-28", baseline.End.Format("2006-01-02"))

	custom, err := resolveVariables(variables, Request{Timezone: "Asia/Tashkent", Request: url.Values{
		"range": []string{"bounded"}, "range-start": []string{"2026-03-01"}, "range-end": []string{"2026-03-31"},
		"compare": []string{"custom"}, "compare-start": []string{"2026-01-01"}, "compare-end": []string{"2026-01-31"},
	}})
	require.NoError(t, err)
	baseline = custom["compare"].(lens.CompareValue).Range
	require.Equal(t, "Asia/Tashkent", baseline.Start.Location().String())
	require.Equal(t, "2026-01-01T00:00:00+05:00", baseline.Start.Format(time.RFC3339))
	require.Equal(t, "2026-01-31T23:59:59+05:00", baseline.End.Format(time.RFC3339))
}

func TestValidateRejectsDuplicatePanels(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("duplicates", "Duplicates",
		lensbuild.Row(
			panel.Bar("same", "Panel 1", "dataset-a").
				LabelField("label").
				ValueField("value").
				Terminal().Build(),
			panel.Bar("same", "Panel 2", "dataset-b").
				LabelField("label").
				ValueField("value").
				Terminal().Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset-a", mustFrameSet(t, "dataset-a")),
		lensbuild.StaticDataset("dataset-b", mustFrameSet(t, "dataset-b")),
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestValidateRejectsDuplicateDatasets(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("duplicate-datasets", "Duplicate Datasets",
		lensbuild.Row(
			panel.Bar("panel-a", "Panel 1", "dataset-a").
				LabelField("label").
				ValueField("value").
				Terminal().Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset-a", mustFrameSet(t, "dataset-a")),
		lensbuild.StaticDataset("dataset-a", mustFrameSet(t, "dataset-b")),
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestValidateRejectsMissingStaticFramesAndQuerySpec(t *testing.T) {
	t.Parallel()

	staticErr := Validate(lensbuild.Dashboard("static", "Static").Datasets(
		lens.DatasetSpec{Name: "missing-static", Kind: lens.DatasetKindStatic},
	).Build())
	require.Error(t, staticErr)

	queryErr := Validate(lensbuild.Dashboard("query", "Query").Datasets(
		lens.DatasetSpec{Name: "missing-query", Kind: lens.DatasetKindQuery, Source: "primary"},
	).Build())
	require.Error(t, queryErr)
}

func TestValidateRejectsUnsupportedComparisonAlignment(t *testing.T) {
	t.Parallel()

	dataset := lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset"))
	dataset.ComparisonAlignment = lens.ComparisonAlignment("position-ish")
	err := Validate(lensbuild.Dashboard("comparison", "Comparison").Datasets(dataset).Build())

	require.ErrorContains(t, err, `dataset dataset has unsupported comparison alignment "position-ish"`)
}

func TestValidateRejectsMissingActionFieldSource(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("actions", "Actions",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(action.Navigate("/contracts", action.FieldParam("source", ""))).
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	).Build()

	err := Validate(spec)
	require.Error(t, err)
}

func TestValidateAcceptsActionURLFieldSource(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("actions", "Actions",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(action.Navigate("").WithFieldURL("label")).
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	).Build()

	require.NoError(t, Validate(spec))
}

func TestValidateAcceptsOpenDrawerAction(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("actions", "Actions",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(action.OpenDrawer("/analytics/drill/lens/document", action.FieldParam("metric", "label"))).
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	).Build()

	require.NoError(t, Validate(spec))

	metricSpec := lensbuild.Dashboard("metric-action", "Metric action",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(action.OpenDrawerMetric(action.FieldValue("label"), action.FieldParam("product", "label"))).
				Build(),
		),
	).Datasets(lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset"))).Build()
	require.NoError(t, Validate(metricSpec))
}

func TestValidateRejectsDrawerMetricKeyOnNonDrawerAction(t *testing.T) {
	t.Parallel()
	actionSpec := action.Navigate("/records")
	drawerKey := action.LiteralValue("loss-ratio")
	actionSpec.DrawerKey = &drawerKey
	spec := lensbuild.Dashboard("bad-action", "Bad action", lensbuild.Row(
		panel.Bar("sales", "Sales", "dataset").LabelField("label").ValueField("value").Action(actionSpec).Build(),
	)).Datasets(lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset"))).Build()
	require.ErrorContains(t, Validate(spec), "metric key is only valid for a drawer")
}

func TestValidateRejectsEmptyActionURLFieldSource(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("actions", "Actions",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(action.Navigate("").WithFieldURL("")).
				Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	).Build()

	err := Validate(spec)
	require.ErrorContains(t, err, "action value url requires source name")
}

func TestValidate_AcceptsKeyedPieDrillTree(t *testing.T) {
	t.Parallel()

	spec := drillTreeDashboard(t, panel.KindPie, panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children: []panel.DrillNode{
			{Key: "direct", Label: "Direct", Value: 70, Action: actionSpec(action.Navigate("/portfolio"))},
			{Key: "reinsurance", Label: "Reinsurance", Value: 30, Action: actionSpec(action.HtmxSwap("/reinsurance", "#drawer"))},
		},
	}}}, frame.Row{"id": "earned", "label": "Earned", "value": 100.0})

	require.NoError(t, Validate(spec))
	result, err := execute(context.Background(), spec, Request{})
	require.NoError(t, err)
	require.NoError(t, result.Panels["drill"].Error)
}

func TestValidate_RejectsInvalidDrillTrees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		kind    panel.Kind
		tree    panel.DrillTree
		mutate  func(*panel.Spec)
		wantErr string
	}{
		{name: "unsupported panel kind", kind: panel.KindBar, tree: validDrillTree(), wantErr: `unsupported for kind "bar"`},
		{name: "empty branches", kind: panel.KindPie, tree: panel.DrillTree{}, wantErr: "requires at least one branch"},
		{name: "blank branch key", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 1}}}}}, wantErr: "requires trigger key"},
		{name: "duplicate branch key", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 1}}}, {TriggerKey: "earned", Label: "Other", Children: []panel.DrillNode{{Key: "other", Label: "Other", Value: 1}}}}}, wantErr: `duplicate branch key "earned"`},
		{name: "branch without children", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned"}}}, wantErr: "requires children"},
		{name: "duplicate sibling key", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 1}, {Key: "direct", Label: "Again", Value: 2}}}}}, wantErr: `duplicate child key "direct"`},
		{name: "negative value", kind: panel.KindDonut, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: -1}}}}}, wantErr: "finite nonnegative value"},
		{name: "nonfinite value", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: math.Inf(1)}}}}}, wantErr: "finite nonnegative value"},
		{name: "overflowed sibling total", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: math.MaxFloat64}, {Key: "reinsurance", Label: "Reinsurance", Value: math.MaxFloat64}}}}}, wantErr: "requires finite total"},
		{name: "children and action", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 1, Action: actionSpec(action.Navigate("/portfolio")), Children: []panel.DrillNode{{Key: "q1", Label: "Q1", Value: 1}}}}}}}, wantErr: "cannot have both children and action"},
		{name: "cube action", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 1, Action: actionSpec(action.CubeDrill("/portfolio", "source"))}}}}}, wantErr: `unsupported kind "cube_drill"`},
		{name: "field action source", kind: panel.KindPie, tree: panel.DrillTree{Branches: []panel.DrillBranch{{TriggerKey: "earned", Label: "Earned", Children: []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 1, Action: actionSpec(action.Navigate("").WithFieldURL("url"))}}}}}, wantErr: "cannot use a field source"},
		{name: "missing id mapping", kind: panel.KindPie, tree: validDrillTree(), mutate: func(spec *panel.Spec) { spec.Fields.ID = "" }, wantErr: "requires id field"},
		{name: "invalid expanded span", kind: panel.KindPie, tree: panel.DrillTree{ExpandedSpan: 13, Branches: validDrillTree().Branches}, wantErr: "expanded span must be between 1 and 12"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			built := panel.Pie("drill", "Drill", "dataset").LabelField("label").ValueField("value").DrillTree(test.tree).Build()
			built.Kind = test.kind
			if test.mutate != nil {
				test.mutate(&built)
			}
			spec := lensbuild.Dashboard("drill", "Drill", lensbuild.Row(built)).Datasets(
				lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
			).Build()
			err := Validate(spec)
			require.ErrorContains(t, err, test.wantErr)
		})
	}
}

func TestValidate_RejectsInvalidActionKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		action  action.Spec
		wantErr string
	}{
		{name: "blank parameter name", action: action.Navigate("/contracts", action.LiteralParam(" ", "active")), wantErr: "action parameter name cannot be blank"},
		{name: "parameter name with surrounding whitespace", action: action.Navigate("/contracts", action.LiteralParam(" status ", "active")), wantErr: "action parameter name \" status \" has surrounding whitespace"},
		{name: "blank payload key", action: action.Spec{Kind: action.KindEmitEvent, Event: "selected", Payload: map[string]action.ValueSource{"": action.LiteralValue("active")}}, wantErr: "action payload key cannot be blank"},
		{name: "payload key with surrounding whitespace", action: action.Spec{Kind: action.KindEmitEvent, Event: "selected", Payload: map[string]action.ValueSource{" status ": action.LiteralValue("active")}}, wantErr: "action payload key \" status \" has surrounding whitespace"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			panelSpec := panel.Bar("sales", "Sales", "dataset").
				LabelField("label").
				ValueField("value").
				Action(test.action).
				Build()
			spec := lensbuild.Dashboard("actions", "Actions", lensbuild.Row(panelSpec)).Datasets(
				lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
			).Build()
			require.ErrorContains(t, Validate(spec), test.wantErr)
		})
	}
}

func TestExecuteRejectsDrillTreeWithoutUniqueMatchingDatasetID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		rows    []frame.Row
		wantErr string
	}{
		{name: "missing id field", rows: []frame.Row{{"label": "Earned", "value": 100.0}}, wantErr: `missing id field "id"`},
		{name: "missing branch key", rows: []frame.Row{{"id": "unearned", "label": "Unearned", "value": 100.0}}, wantErr: `branch key "earned" is missing`},
		{name: "duplicate branch key", rows: []frame.Row{{"id": "earned", "label": "Earned", "value": 60.0}, {"id": "earned", "label": "Earned", "value": 40.0}}, wantErr: `id field "id" has duplicate key "earned"`},
		{name: "duplicate nonbranch key", rows: []frame.Row{{"id": "earned", "label": "Earned", "value": 60.0}, {"id": "unearned", "label": "Unearned", "value": 20.0}, {"id": "unearned", "label": "Unearned", "value": 20.0}}, wantErr: `id field "id" has duplicate key "unearned"`},
		{name: "id with surrounding whitespace", rows: []frame.Row{{"id": " earned ", "label": "Earned", "value": 100.0}}, wantErr: `requires a nonblank string`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			spec := drillTreeDashboard(t, panel.KindPie, validDrillTree(), test.rows...)
			result, err := execute(context.Background(), spec, Request{})
			require.NoError(t, err)
			require.ErrorContains(t, result.Panels["drill"].Error, test.wantErr)
		})
	}
}

func validDrillTree() panel.DrillTree {
	return panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children:   []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 100}},
	}}}
}

func drillTreeDashboard(t *testing.T, kind panel.Kind, tree panel.DrillTree, rows ...frame.Row) lens.DashboardSpec {
	t.Helper()
	set, err := frame.FromRows("dataset", rows...)
	require.NoError(t, err)
	chart := panel.Pie("drill", "Drill", "dataset").
		LabelField("label").
		ValueField("value").
		DrillTree(tree).
		Build()
	chart.Kind = kind
	return lensbuild.Dashboard("drill", "Drill", lensbuild.Row(chart)).Datasets(
		lensbuild.StaticDataset("dataset", set),
	).Build()
}

func actionSpec(spec action.Spec) *action.Spec {
	return &spec
}

func TestExecuteMarksMissingPanelFieldsAsPanelError(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("frames", "Frames",
		lensbuild.Row(
			panel.Bar("sales", "Sales", "dataset").
				LabelField("missing_label").
				ValueField("value").
				Terminal().Build(),
		),
	).Datasets(
		lensbuild.StaticDataset("dataset", mustFrameSet(t, "dataset")),
	).Build()

	result, err := execute(context.Background(), spec, Request{})
	require.NoError(t, err)
	require.Error(t, result.Panels["sales"].Error)
	require.Contains(t, result.Panels["sales"].Error.Error(), "missing field")
}

func TestResolveVariablesPreservesAllMultiSelectValues(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{
			Name:    "products",
			Label:   "Products",
			Kind:    lens.VariableMultiSelect,
			Default: []string{"default"},
		},
	).Build()

	values, err := resolveVariables(spec.Variables, Request{
		Request: url.Values{"products": []string{"osago", "travel"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"osago", "travel"}, values["products"])
}

func TestResolveVariablesSplitsCommaSeparatedMultiSelectValues(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{
			Name:    "products",
			Label:   "Products",
			Kind:    lens.VariableMultiSelect,
			Default: []string{"default"},
		},
	).Build()

	values, err := resolveVariables(spec.Variables, Request{
		Request: url.Values{"products": []string{"osago, travel", "kasko"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"osago", "travel", "kasko"}, values["products"])
}

func TestResolveVariablesParsesNumberValues(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{Name: "limit", Label: "Limit", Kind: lens.VariableNumber, Default: 10.0},
	).Build()

	values, err := resolveVariables(spec.Variables, Request{
		Request: url.Values{"limit": []string{"25.5"}},
	})
	require.NoError(t, err)
	require.InDelta(t, 25.5, values["limit"].(float64), 0.001)
}

func TestResolveVariablesUsesToggleDefaultWhenRequestMissing(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("variables", "Variables").Variables(
		lens.VariableSpec{Name: "active_only", Label: "Active Only", Kind: lens.VariableToggle, Default: true},
	).Build()

	values, err := resolveVariables(spec.Variables, Request{Request: url.Values{}})
	require.NoError(t, err)
	require.Equal(t, true, values["active_only"])
}

func TestValidateAllowsUngroupedTimeSeriesPanels(t *testing.T) {
	t.Parallel()

	spec := lensbuild.Dashboard("sales", "Sales").Datasets(
		lensbuild.StaticDataset("daily_sales", mustFrameSet(t, "daily_sales")),
	).Build()
	spec.Rows = []lens.RowSpec{
		lensbuild.Row(
			panel.TimeSeries("daily", "Daily Sales", "daily_sales").
				CategoryField("category").
				ValueField("value").
				Build(),
		),
	}

	require.NoError(t, Validate(spec))
}

func TestTemporalOverlayFieldsValidateAfterTransforms(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("daily",
		frame.Row{"category": "2026-07-01", "value": 3.0},
		frame.Row{"category": "2026-07-02", "value": 6.0},
		frame.Row{"category": "2026-07-03", "value": 9.0},
	)
	require.NoError(t, err)
	chart := panel.TimeSeries("daily", "Daily", "daily").
		CategoryField("category").ValueField("value").
		Transforms(transform.MovingAverage("value", "sma_3", 3), transform.LinearRegression("value", "regression")).
		MovingAverage(3, panel.FieldRef("sma_3"), "SMA 3").
		Regression(panel.FieldRef("regression"), "Trend").Terminal().Build()
	spec := lensbuild.Dashboard("daily", "Daily", lensbuild.Row(chart)).Datasets(lensbuild.StaticDataset("daily", set)).Build()

	result, err := execute(context.Background(), spec, Request{})
	require.NoError(t, err)
	require.NoError(t, result.Panels["daily"].Error)
	require.InDelta(t, 6.0, result.Panels["daily"].Frames.Primary().MustField("sma_3").Values[2], 1e-9)
}

func TestTemporalOverlayRejectsUnsupportedSMAWindow(t *testing.T) {
	t.Parallel()

	primary, err := frame.New("daily",
		frame.Field{Name: "category", Type: frame.FieldTypeString, Values: []any{"2026-07-01"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{3.0}},
		frame.Field{Name: "sma", Type: frame.FieldTypeNumber, Values: []any{3.0}},
	)
	require.NoError(t, err)
	spec := panel.TimeSeries("daily", "Daily", "daily").
		CategoryField("category").ValueField("value").
		MovingAverage(5, panel.FieldRef("sma"), "SMA 5").Terminal().Build()

	require.ErrorContains(t, validateRequiredPanelFields(spec, primary), "3, 7, or 12")
}

func TestDistributionPanelsValidateExplicitFrameContracts(t *testing.T) {
	t.Parallel()

	set, err := frame.FromRows("distribution", frame.Row{
		"bucket": "0–10", "count": 12.0, "product": "OSAGO", "region": "Tashkent",
		"min": 1.0, "q1": 3.0, "median": 5.0, "q3": 8.0, "max": 20.0,
	})
	require.NoError(t, err)
	spec := lensbuild.Dashboard("distribution", "Distribution", lensbuild.Row(
		panel.Histogram("hist", "Histogram", "distribution").CategoryField("bucket").ValueField("count").Terminal().Build(),
		panel.BoxPlot("box", "Box plot", "distribution").CategoryField("product").BoxFields("min", "q1", "median", "q3", "max").Terminal().Build(),
		panel.Heatmap("heat", "Heatmap", "distribution").CategoryField("region").SeriesField("product").ValueField("count").Terminal().Build(),
	)).Datasets(lensbuild.StaticDataset("distribution", set)).Build()

	require.NoError(t, Validate(spec))
	result, err := execute(context.Background(), spec, Request{})
	require.NoError(t, err)
	for _, panelID := range []string{"hist", "box", "heat"} {
		require.NoError(t, result.Panels[panelID].Error)
	}
}

func mustFrameSet(t *testing.T, name string) *frame.FrameSet {
	t.Helper()

	set, err := frame.FromRows(name, frame.Row{
		"label": "row",
		"value": 1.0,
	})
	require.NoError(t, err)
	return set
}
