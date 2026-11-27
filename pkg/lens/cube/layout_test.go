package cube_test

import (
	"context"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

func samplePoliciesFrameSet(t *testing.T) *frame.FrameSet {
	t.Helper()
	fs, err := frame.FromRows("policies",
		frame.Row{"agent_type": "individual", "agency_id": "a1", "agency_name": "Alpha", "issued_at": time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), "premium": 500.0},
		frame.Row{"agent_type": "legal", "agency_id": "a2", "agency_name": "Beta", "issued_at": time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC), "premium": 800.0},
		frame.Row{"agent_type": "legal", "agency_id": "a3", "agency_name": "Gamma", "issued_at": time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), "premium": 300.0},
		frame.Row{"agent_type": "individual", "agency_id": "a4", "agency_name": "Delta", "issued_at": time.Date(2026, 2, 18, 0, 0, 0, 0, time.UTC), "premium": 150.0},
		frame.Row{"agent_type": "legal", "agency_id": "a5", "agency_name": "Epsilon", "issued_at": time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC), "premium": 50.0},
	)
	require.NoError(t, err)
	return fs
}

func resolveTestPortfolioURL(_ context.Context, drill cube.DrillContext, _ map[string]any) string {
	values := drill.WithValues(drill.Request)
	delete(values, cube.QueryLeaf)
	if len(values) == 0 {
		return "/portfolio/policies"
	}
	return "/portfolio/policies?" + values.Encode()
}

func TestDeclarativeSQLCube_MultiLevelDrillAndLeaf(t *testing.T) {
	t.Parallel()

	spec := cube.New("agent_program", "Agent Program").
		SQL("primary", "insurance.policies pol").
		Join("agency", "LEFT JOIN insurance.agencies a ON a.id = pol.agency_id").
		ParamLiteral("tenant_id", "tenant-1").
		ParamVariable("range_start", "range_start").
		ParamVariable("range_end", "range_end").
		Dimension("agent_type", "Тип агента").
		Column("CASE WHEN a.legal_form = 'individual' THEN 'individual' ELSE 'legal' END").
		LabelMap(map[string]string{"individual": "Физлица", "legal": "Юрлица"}).
		RequiresJoin("agency").
		AsFacet().
		Done().
		Dimension("agency", "Агент").
		Column("a.id::text").
		LabelColumn("a.name").
		RequiresJoin("agency").
		AsFacet().
		Done().
		Dimension("period", "Период").
		TimeBucket("pol.issued_at", "month").
		Done().
		Measure("premium", "Премия").
		Column("pol.premium_uzs").
		Sum().
		Money("UZS").
		Done().
		Layout(
			cube.KPIRow("premium"),
			cube.SplitRow(
				cube.Chart("by_type", panel.KindDonut).Dimension("agent_type").Measure("premium"),
				cube.Chart("trend", panel.KindLine).Dimension("period").Series("agent_type").Measure("premium"),
			),
			cube.Chart("top_agencies", panel.KindBar).Dimension("agency").Measure("premium").TopN(15).WithRemainder(),
		).
		DrillPath("agent_type", "agency").
		LeafSpec(cube.LeafSpec{
			Table:        cube.LeafTable("policies_leaf", "Полисы"),
			ExternalLink: resolveTestPortfolioURL,
		}).
		Build()

	require.NoError(t, spec.Validate())
	require.Len(t, spec.FacetDimensions(), 2)

	// Depth 0: root with period query params.
	rootQuery := url.Values{"range-start": {"2026-01-01"}, "range-end": {"2026-03-31"}}
	rootDrill := cube.ParseDrillContext(rootQuery)
	require.False(t, rootDrill.IsLeaf(spec))

	dash, err := cube.ResolveWithContext(context.Background(), spec, rootDrill, "/analytics/agent-program")
	require.NoError(t, err)
	require.Len(t, dash.Rows, 3)

	// KPI stat card must carry a valid drill action preserving period.
	kpiPanel, ok := lens.FindPanel(dash, "stat_premium")
	require.True(t, ok)
	require.False(t, kpiPanel.Terminal)
	require.NotNil(t, kpiPanel.Action)
	require.Equal(t, action.KindNavigate, kpiPanel.Action.Kind)
	require.True(t, kpiPanel.Action.PreserveQuery)
	require.Contains(t, kpiPanel.Action.URL, "_leaf=1")
	require.Contains(t, kpiPanel.Action.URL, "range-start=2026-01-01")

	// Donut panel drills on agent_type -> next group-by agency.
	byType, ok := lens.FindPanel(dash, "by_type")
	require.True(t, ok)
	require.Equal(t, panel.KindDonut, byType.Kind)
	require.Equal(t, 6, byType.Span)
	require.NotNil(t, byType.Action)
	require.Equal(t, action.KindCubeDrill, byType.Action.Kind)
	require.True(t, byType.Action.PreserveQuery)
	require.Equal(t, "agent_type", byType.Action.Drill.Dimension)
	require.Equal(t, "agency", byType.Action.Drill.GroupBy)

	// Line chart with Series drills on period and encodes Series field.
	trend, ok := lens.FindPanel(dash, "trend")
	require.True(t, ok)
	require.Equal(t, panel.KindTimeSeries, trend.Kind)
	require.Equal(t, 6, trend.Span)
	require.Equal(t, panel.Ref("series"), trend.Fields.Series)
	require.NotNil(t, trend.Action)
	require.Equal(t, action.KindCubeDrill, trend.Action.Kind)
	require.Equal(t, "period", trend.Action.Drill.Dimension)
	require.True(t, trend.Action.PreserveQuery)

	// Bar chart with TopN(15).WithRemainder() spans 12 and emits ExpandableTopN transform.
	topAgencies, ok := lens.FindPanel(dash, "top_agencies")
	require.True(t, ok)
	require.Equal(t, panel.KindBar, topAgencies.Kind)
	require.Equal(t, 12, topAgencies.Span)
	require.NotNil(t, topAgencies.Action)
	require.Equal(t, action.KindCubeDrill, topAgencies.Action.Kind)
	require.Equal(t, "agency", topAgencies.Action.Drill.Dimension)

	// Verify generated SQL for LabelMap CASE, TimeBucket, Series, and TopN.
	dsByName := map[string]lens.DatasetSpec{}
	for _, ds := range dash.Datasets {
		dsByName[ds.Name] = ds
	}
	require.Contains(t, dsByName["cube_chart_by_type"].Query.Text, "WHEN 'individual' THEN 'Физлица'")
	require.Contains(t, dsByName["cube_chart_by_type"].Query.Text, "WHEN 'legal' THEN 'Юрлица'")
	require.Contains(t, dsByName["cube_chart_trend"].Query.Text, "to_char(date_trunc('month', pol.issued_at), 'YYYY-MM')")
	require.Contains(t, dsByName["cube_chart_trend"].Query.Text, "AS series")
	require.Contains(t, dsByName["cube_chart_trend"].Query.Text, "ORDER BY filter_value ASC, series ASC")
	topDS := dsByName["cube_chart_top_agencies"]
	require.Equal(t, lens.DatasetKindTransform, topDS.Kind)
	require.Len(t, topDS.Transforms, 1)
	require.Equal(t, transform.KindTopN, topDS.Transforms[0].Kind)
	require.True(t, topDS.Transforms[0].TopN.Expandable)
	require.Equal(t, 15, topDS.Transforms[0].TopN.N)
	require.Equal(t, "Other", topDS.Transforms[0].TopN.Other)

	// Depth 2: both DrillPath dimensions filtered -> Leaf table + ExternalLink.
	leafQuery := url.Values{
		cube.QueryFilter: {"agent_type:legal", "agency:a2"},
		"range-start":    {"2026-01-01"},
		"range-end":      {"2026-03-31"},
	}
	leafDrill := cube.ParseDrillContext(leafQuery)
	require.True(t, leafDrill.IsLeaf(spec))

	leafDash, err := cube.ResolveWithContext(context.Background(), spec, leafDrill, "/analytics/agent-program")
	require.NoError(t, err)
	leafPanel, ok := lens.FindPanel(leafDash, "policies_leaf")
	require.True(t, ok)
	require.Equal(t, panel.KindTable, leafPanel.Kind)
	require.NotNil(t, leafPanel.Action)
	require.Equal(t, action.KindNavigate, leafPanel.Action.Kind)
	require.Contains(t, leafPanel.Action.URL, "/portfolio/policies?")
	require.Contains(t, leafPanel.Action.URL, "_f=agent_type%3Alegal")
	require.Contains(t, leafPanel.Action.URL, "_f=agency%3Aa2")
	require.Contains(t, leafPanel.Action.URL, "range-start=2026-01-01")

	// KPI at leaf links directly to ExternalLink preserving active filters and period.
	leafKPI, ok := lens.FindPanel(leafDash, "stat_premium")
	require.True(t, ok)
	require.NotNil(t, leafKPI.Action)
	require.Contains(t, leafKPI.Action.URL, "/portfolio/policies?")
	require.Contains(t, leafKPI.Action.URL, "range-end=2026-03-31")
}

func TestDeclarativeDatasetCube_EndToEndExecutionAndDocumentValidation(t *testing.T) {
	t.Parallel()

	// Would be falsely green if runtime execution or document validation were skipped.
	base := samplePoliciesFrameSet(t)
	spec := cube.New("agent_program", "Agent Program").
		Dataset(base).
		Dimension("agent_type", "Тип агента").
		Field("agent_type").
		LabelMap(map[string]string{"individual": "Физлица", "legal": "Юрлица"}).
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
			cube.Chart("top_agencies", panel.KindBar).Dimension("agency").Measure("premium").TopN(2).WithRemainder("Прочие"),
		).
		DrillPath("agent_type", "agency").
		LeafSpec(cube.LeafSpec{
			Table:        cube.LeafTable("policies_leaf", "Полисы"),
			ExternalLink: resolveTestPortfolioURL,
		}).
		Build()

	rt := lensruntime.New(lensruntime.Options{})

	// 1. Root level execution & full document.Build validation.
	rootDash, err := cube.ResolveWithContext(context.Background(), spec, cube.DrillContext{}, "/analytics/agent-program")
	require.NoError(t, err)
	result, err := rt.Execute(context.Background(), rootDash, lensruntime.Request{DataScope: "tenant-1"}, lensruntime.DashboardScope())
	require.NoError(t, err)

	doc, err := document.Build(rootDash, result, document.BuildOptions{Locale: "ru"})
	require.NoError(t, err)
	require.NoError(t, doc.Validate())

	// Verify LabelMap applied on by_type frame.
	byTypeRows := result.Panel("by_type").Frames.Primary().Rows()
	labels := map[string]float64{}
	for _, row := range byTypeRows {
		labels[row["label"].(string)] = row["premium"].(float64)
	}
	require.InDelta(t, 1150.0, labels["Юрлица"], 1e-6)
	require.InDelta(t, 650.0, labels["Физлица"], 1e-6)

	// Verify TimeBucket + Series on trend frame (chronological order).
	trendRows := result.Panel("trend").Frames.Primary().Rows()
	require.Len(t, trendRows, 4)
	require.Equal(t, "2026-01", trendRows[0]["filter_value"])
	require.Equal(t, "2026-02", trendRows[len(trendRows)-1]["filter_value"])
	seriesNames := map[string]bool{}
	for _, row := range trendRows {
		seriesNames[row["series"].(string)] = true
	}
	require.True(t, seriesNames["Физлица"])
	require.True(t, seriesNames["Юрлица"])

	// Verify TopN(2).WithRemainder("Прочие") tags rows beyond top 2.
	topRows := result.Panel("top_agencies").Frames.Primary().Rows()
	require.Len(t, topRows, 5)
	require.Nil(t, topRows[0][transform.TopNGroupField])
	require.Nil(t, topRows[1][transform.TopNGroupField])
	require.Equal(t, "Прочие", topRows[2][transform.TopNGroupField])
	require.Equal(t, "Прочие", topRows[4][transform.TopNGroupField])

	// 2. TimeBucket filter narrowing (_f=period:2026-02).
	febDrill := cube.ParseDrillContext(url.Values{cube.QueryFilter: {"period:2026-02"}})
	febDash, err := cube.ResolveWithContext(context.Background(), spec, febDrill, "/analytics/agent-program")
	require.NoError(t, err)
	febResult, err := rt.Execute(context.Background(), febDash, lensruntime.Request{DataScope: "tenant-1"}, lensruntime.DashboardScope())
	require.NoError(t, err)
	febStatRows := febResult.Panel("stat_premium").Frames.Primary().Rows()
	require.Len(t, febStatRows, 1)
	require.InDelta(t, 500.0, febStatRows[0]["premium"].(float64), 1e-6)

	// 3. Leaf level execution & document validation.
	leafDrill := cube.ParseDrillContext(url.Values{cube.QueryFilter: {"agent_type:legal", "agency:a2"}})
	leafDash, err := cube.ResolveWithContext(context.Background(), spec, leafDrill, "/analytics/agent-program")
	require.NoError(t, err)
	leafResult, err := rt.Execute(context.Background(), leafDash, lensruntime.Request{DataScope: "tenant-1"}, lensruntime.DashboardScope())
	require.NoError(t, err)
	leafDoc, err := document.Build(leafDash, leafResult, document.BuildOptions{Locale: "ru"})
	require.NoError(t, err)
	require.NoError(t, leafDoc.Validate())

	leafRows := leafResult.Panel("policies_leaf").Frames.Primary().Rows()
	require.Len(t, leafRows, 1)
	require.Equal(t, "Beta", leafRows[0]["agency_name"])

	// 4. Facet options & DrillFilters use LabelMap.
	opts, err := cube.ResolveFacetOptions(context.Background(), spec, cube.DrillContext{}, "agent_type", "", 10, nil)
	require.NoError(t, err)
	require.Len(t, opts, 2)
	require.Equal(t, "Юрлица", opts[0].Label)
	require.Equal(t, "legal", opts[0].Value)

	resolvedFilters, err := cube.ResolveDrillFilters(context.Background(), spec, leafDrill, nil)
	require.NoError(t, err)
	require.Len(t, resolvedFilters, 2)
	require.Equal(t, "Юрлица", resolvedFilters[0].Display)
	require.Equal(t, "Beta", resolvedFilters[1].Display)
}

func TestDeclarativeNamedDataSourceCube_ParityWithDatasetMode(t *testing.T) {
	t.Parallel()

	// Would be falsely green if NamedDataSource did not execute through the cube pipeline.
	named := datasource.NewNamedDataSource()
	require.NoError(t, named.Register("agent_policies", func(_ context.Context, _ datasource.QueryRequest) (*frame.FrameSet, error) {
		return samplePoliciesFrameSet(t), nil
	}))

	spec := cube.New("agent_named", "Agent Named").
		Named("domain", "agent_policies").
		Dimension("agent_type", "Тип агента").
		Field("agent_type").
		LabelMap(map[string]string{"individual": "Физлица", "legal": "Юрлица"}).
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
			cube.Chart("trend", panel.KindLine).Dimension("period").Series("agent_type").Measure("premium"),
		).
		DrillPath("agent_type").
		LeafSpec(cube.LeafSpec{
			Table:        cube.LeafTable("policies_leaf", "Полисы"),
			ExternalLink: resolveTestPortfolioURL,
		}).
		Build()

	rt := lensruntime.New(lensruntime.Options{})
	req := lensruntime.Request{
		DataScope:            "tenant-1",
		DataSources:          map[string]datasource.DataSource{"domain": named},
		DataSourceIdentities: map[string]string{"domain": "domain:v1"},
	}

	dash, err := cube.ResolveWithContext(context.Background(), spec, cube.DrillContext{}, "/analytics/agent-named")
	require.NoError(t, err)
	res, err := rt.Execute(context.Background(), dash, req, lensruntime.DashboardScope())
	require.NoError(t, err)
	require.InDelta(t, 1800.0, res.Panel("stat_premium").Frames.Primary().Rows()[0]["premium"].(float64), 1e-6)
	require.Len(t, res.Panel("trend").Frames.Primary().Rows(), 4)

	// Drill to leaf via agent_type.
	leafDrill := cube.ParseDrillContext(url.Values{cube.QueryFilter: {"agent_type:individual"}})
	require.True(t, leafDrill.IsLeaf(spec))
	leafDash, err := cube.ResolveWithContext(context.Background(), spec, leafDrill, "/analytics/agent-named")
	require.NoError(t, err)
	leafRes, err := rt.Execute(context.Background(), leafDash, req, lensruntime.DashboardScope())
	require.NoError(t, err)
	require.Len(t, leafRes.Panel("policies_leaf").Frames.Primary().Rows(), 2)
}

func TestTimeBucketBoundsAndDrillNormalization(t *testing.T) {
	t.Parallel()

	// Would be falsely green if TimeBucketBounds returned wrong end boundaries.
	start, end, ok := cube.TimeBucketBounds("month", "2026-02")
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), start)
	require.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), end)

	qStart, qEnd, ok := cube.TimeBucketBounds("quarter", "2026-Q2")
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), qStart)
	require.Equal(t, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), qEnd)

	yStart, yEnd, ok := cube.TimeBucketBounds("year", "2026")
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), yStart)
	require.Equal(t, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), yEnd)

	wStart, wEnd, ok := cube.TimeBucketBounds("week", "2026-03-02")
	require.True(t, ok)
	require.Equal(t, time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), wStart)
	require.Equal(t, time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), wEnd)

	// Empty filter value normalization.
	normalized := cube.ParseDrillContextNormalized(url.Values{
		cube.QueryFilter: {"agency:", "agent_type:legal"},
		cube.QueryLeaf:   {"1"},
	}, "")
	require.True(t, normalized.Leaf)
	require.True(t, normalized.ContainsDimension("agency"))
	require.Equal(t, cube.UnknownDrillValue, normalized.Filters[0].Value)
	require.Equal(t, "1", normalized.Encode().Get(cube.QueryLeaf))
}

func TestCubeSpecValidation_RejectsInvalidLayoutAndDrillPath(t *testing.T) {
	t.Parallel()

	// Would be falsely green if Validate did not enforce layout/drill constraints.
	base := samplePoliciesFrameSet(t)

	badDrill := cube.New("c1", "C1").
		Dataset(base).
		Dimension("agent_type", "Type").Field("agent_type").Done().
		Measure("premium", "Premium").Field("premium").Sum().Done().
		DrillPath("nonexistent").
		Build()
	require.ErrorContains(t, badDrill.Validate(), "unknown dimension")

	badSeries := cube.New("c2", "C2").
		Dataset(base).
		Dimension("agent_type", "Type").Field("agent_type").Done().
		Measure("premium", "Premium").Field("premium").Sum().Done().
		Layout(cube.Chart("ch1", panel.KindLine).Dimension("agent_type").Series("agent_type").Measure("premium")).
		Build()
	require.ErrorContains(t, badSeries.Validate(), "must differ")

	badGranularity := cube.New("c3", "C3").
		Dataset(base).
		Dimension("period", "Period").TimeBucket("issued_at", "decade").Done().
		Measure("premium", "Premium").Field("premium").Sum().Done().
		Build()
	require.ErrorContains(t, badGranularity.Validate(), "unsupported granularity")
}

func TestLeafStringCompatibility_AllReceivers(t *testing.T) {
	t.Parallel()

	// Would be falsely green if string-form Leaf did not compile or populate Leaf.URL on all builders.
	fromBuilder := cube.New("b1", "B1").
		SQL("primary", "t").
		Dimension("d", "D").Column("d").Done().
		Measure("m", "M").Column("m").Sum().Done().
		Leaf("/records/from-builder").
		Build()
	require.Equal(t, "/records/from-builder", fromBuilder.Leaf.URL)

	fromDim := cube.New("b2", "B2").
		SQL("primary", "t").
		Measure("m", "M").Column("m").Sum().Done().
		Dimension("d", "D").Column("d").
		Leaf("/records/from-dim").
		Build()
	require.Equal(t, "/records/from-dim", fromDim.Leaf.URL)

	fromMeasure := cube.New("b3", "B3").
		SQL("primary", "t").
		Dimension("d", "D").Column("d").Done().
		Measure("m", "M").Column("m").Sum().
		Leaf("/records/from-measure").
		Build()
	require.Equal(t, "/records/from-measure", fromMeasure.Leaf.URL)
}

func TestUnknownDrillSentinel_MatchesEmptyAndNullInSQLAndDataset(t *testing.T) {
	t.Parallel()

	// Would be falsely green if __unknown__ failed to match empty/NULL rows or relaxed non-sentinel matching.
	sqlSpec := cube.New("sql_unknown", "SQL Unknown").
		SQL("primary", "insurance.contracts c").
		Dimension("product", "Product").Column("c.product_code").Done().
		Measure("policies", "Policies").Column("c.id").Count().Done().
		Build()

	unknownDrill := cube.ParseDrillContextNormalized(url.Values{cube.QueryFilter: {"product:"}}, "")
	unknownDash, err := cube.ResolveWithContext(context.Background(), sqlSpec, unknownDrill, "/sales")
	require.NoError(t, err)
	var statsDS lens.DatasetSpec
	for _, ds := range unknownDash.Datasets {
		if ds.Name == "cube_stats" {
			statsDS = ds
			break
		}
	}
	require.NotNil(t, statsDS.Query)
	require.Contains(t, statsDS.Query.Text, "((c.product_code) IS NULL OR (c.product_code)::text = ANY(@f_product))")
	require.Equal(t, []string{"", cube.UnknownDrillValue}, statsDS.Query.Params["f_product"].Literal)

	exactDrill := cube.ParseDrillContextNormalized(url.Values{cube.QueryFilter: {"product:osago"}}, "")
	exactDash, err := cube.ResolveWithContext(context.Background(), sqlSpec, exactDrill, "/sales")
	require.NoError(t, err)
	for _, ds := range exactDash.Datasets {
		if ds.Name == "cube_stats" {
			statsDS = ds
			break
		}
	}
	require.Contains(t, statsDS.Query.Text, "c.product_code = ANY(@f_product)")
	require.NotContains(t, statsDS.Query.Text, "IS NULL")
	require.Equal(t, []string{"osago"}, statsDS.Query.Params["f_product"].Literal)

	// Dataset mode: rows with "", nil, and literal "__unknown__" all match; "osago" does not.
	dsFrames, err := frame.FromRows("contracts",
		frame.Row{"product_code": "", "region": "tashkent", "premium": 100.0},
		frame.Row{"product_code": nil, "region": "samarkand", "premium": 200.0},
		frame.Row{"product_code": cube.UnknownDrillValue, "region": "bukhara", "premium": 50.0},
		frame.Row{"product_code": "osago", "region": "tashkent", "premium": 900.0},
	)
	require.NoError(t, err)
	dsSpec := cube.New("ds_unknown", "DS Unknown").
		Dataset(dsFrames).
		Dimension("product", "Product").Field("product_code").AsFacet().Done().
		Dimension("region", "Region").Field("region").AsFacet().Done().
		Measure("premium", "Premium").Field("premium").Sum().Done().
		Build()

	rt := lensruntime.New(lensruntime.Options{})
	dsUnknownDash, err := cube.ResolveWithContext(context.Background(), dsSpec, unknownDrill, "/sales")
	require.NoError(t, err)
	resUnknown, err := rt.Execute(context.Background(), dsUnknownDash, lensruntime.Request{DataScope: "t1"}, lensruntime.DashboardScope())
	require.NoError(t, err)
	require.InDelta(t, 350.0, resUnknown.Panel("stat_premium").Frames.Primary().Rows()[0]["premium"].(float64), 1e-6)

	// Exact non-sentinel filter must exclude empty and NULL rows.
	dsExactDash, err := cube.ResolveWithContext(context.Background(), dsSpec, exactDrill, "/sales")
	require.NoError(t, err)
	resExact, err := rt.Execute(context.Background(), dsExactDash, lensruntime.Request{DataScope: "t1"}, lensruntime.DashboardScope())
	require.NoError(t, err)
	require.InDelta(t, 900.0, resExact.Panel("stat_premium").Frames.Primary().Rows()[0]["premium"].(float64), 1e-6)

	// Facet cross-filtering with __unknown__ on product should see all three unknown rows' regions.
	regionOpts, err := cube.ResolveFacetOptions(context.Background(), dsSpec, unknownDrill, "region", "", 10, nil)
	require.NoError(t, err)
	require.Len(t, regionOpts, 3)
}

func TestResolveFacetOptions_NamedModeSearchAndMonthOnlyTimeBucket(t *testing.T) {
	t.Parallel()

	// Would be falsely green if Named mode skipped search filtering or pre-truncated before filtering.
	namedSpec := cube.New("named_facets", "Named Facets").
		Named("domain", "policies_named").
		Dimension("agent_type", "Type").
		Field("agent_type").
		LabelMap(map[string]string{"individual": "Individual", "legal": "Legal Entity", "broker": "Broker"}).
		AsFacet().
		Done().
		Measure("premium", "Premium").Field("premium").Sum().Done().
		Build()

	var capturedLimit int
	lookup := func(_ context.Context, _ string, _ map[string]lens.ParamValue, maxRows int) ([]lens.DrillFacetOptionMeta, error) {
		capturedLimit = maxRows
		return []lens.DrillFacetOptionMeta{
			{Value: "individual", Label: "Individual", Count: 10},
			{Value: "broker", Label: "Broker", Count: 5},
			{Value: "legal", Label: "Legal Entity", Count: 3},
		}, nil
	}
	filtered, err := cube.ResolveFacetOptions(context.Background(), namedSpec, cube.DrillContext{}, "agent_type", "LEGAL", 1, lookup)
	require.NoError(t, err)
	require.Equal(t, 0, capturedLimit, "lookup should not pre-truncate before search filtering")
	require.Len(t, filtered, 1)
	require.Equal(t, "legal", filtered[0].Value)

	// Month-only string ("YYYY-MM") must normalize to "YYYY" under year granularity (matching chart bucket).
	monthFrames, err := frame.FromRows("monthly",
		frame.Row{"ym": "2026-02", "premium": 100.0},
		frame.Row{"ym": "2026-05", "premium": 200.0},
	)
	require.NoError(t, err)
	yearSpec := cube.New("year_tb", "Year TB").
		Dataset(monthFrames).
		Dimension("period", "Year").TimeBucket("ym", "year").AsFacet().Done().
		Measure("premium", "Premium").Field("premium").Sum().Done().
		Build()
	yearOpts, err := cube.ResolveFacetOptions(context.Background(), yearSpec, cube.DrillContext{}, "period", "", 10, nil)
	require.NoError(t, err)
	require.Len(t, yearOpts, 1)
	require.Equal(t, "2026", yearOpts[0].Value)
	require.Equal(t, 2, yearOpts[0].Count)
}
