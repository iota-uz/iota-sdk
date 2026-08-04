package document

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

func TestBuild_ExistingExploreSpec(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	doc, err := Build(spec, result, BuildOptions{
		SnapshotID: "snapshot-generated", GeneratedAt: time.Date(2026, time.July, 19, 10, 0, 0, 0, time.UTC),
		Locale: "en", InlineDepth: 1,
	})
	require.NoError(t, err)
	require.Equal(t, ContractVersion, doc.Version)
	require.Len(t, doc.Panels, 1)
	require.Equal(t, SemanticsPartition, doc.Panels[0].Semantics)
	require.NotNil(t, doc.Panels[0].DrillRoot)
	require.Contains(t, doc.Drill.Edges, *doc.Panels[0].DrillRoot)
	require.Len(t, doc.Perspectives, 1)
	require.Equal(t, NodeKey("metric/focus/composition/root"), doc.Perspectives[0].Root)
	require.Empty(t, doc.Drill.Edges["metric"].Label)
	require.Equal(t, "Focus", doc.Drill.Edges["metric/focus"].Label)
	require.Equal(t, "Root", doc.Drill.Edges["metric/focus/composition/root"].Label)
	require.Empty(t, doc.Drill.Edges["metric/focus/composition/root"].Children[0].Label)

	payload, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)
	requireGolden(t, "generated_explore.json", string(payload)+"\n")
}

func TestBuild_DeferredPanelsKeepLayoutAndStructuralValidation(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Total"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	stat := panel.Stat("deferred", "Deferred", "rows").ValueField("value").Terminal().Build()
	spec := lensbuild.Dashboard("shell", "Shell", lensbuild.Row(stat)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	shell, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.ShellScope(),
	)
	require.NoError(t, err)
	doc, err := Build(spec, shell, BuildOptions{
		SnapshotID: "deferred", GeneratedAt: time.Unix(1, 0).UTC(), Locale: "en",
		Endpoints: Endpoints{Panel: "/lens/panel"}, DeferPanels: true,
	})
	require.NoError(t, err)
	require.Len(t, doc.Panels, 1)
	require.True(t, doc.Panels[0].Deferred)
	require.Equal(t, "value", doc.Panels[0].Encoding.Value)
	require.NotContains(t, doc.Frames, FrameRef("panel:deferred"))
	require.NoError(t, doc.Validate())

	doc.Panels[0].Status = &PanelStatus{Label: "Bad", Tone: StatusTone("invalid")}
	require.ErrorContains(t, doc.Validate(), "unsupported status tone")
}

func TestBuild_RadialPanelsCarryExplicitGeometry(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("radial",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"north", "south", "north", "south"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"North", "South", "North", "South"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"actual", "actual", "plan", "plan"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{60.0, 40.0, 55.0, 45.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	partition := panel.MultiRingDonut("mix", "Mix", "radial",
		panel.RadialRing{Key: "actual", Label: "Actual", Order: 1, Total: 100},
		panel.RadialRing{Key: "plan", Label: "Plan", Order: 2, Total: 100},
	).IDField("id").SeriesField("series").RadialTolerance(0.01).Terminal().Build()
	spec := lensbuild.Dashboard("radial", "Radial", lensbuild.Row(partition)).
		Datasets(lensbuild.StaticDataset("radial", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{
		SnapshotID: "radial", GeneratedAt: time.Unix(1, 0).UTC(), Locale: "en",
	})
	require.NoError(t, err)
	require.NoError(t, doc.Validate())
	require.Len(t, doc.Panels, 1)
	wire := doc.Panels[0]
	require.Equal(t, PanelKindRadial, wire.Kind)
	require.Equal(t, SemanticsPartition, wire.Semantics)
	require.Equal(t, "series", wire.Encoding.Series)
	require.Equal(t, &RadialConfig{
		Mode: RadialModePartition,
		Rings: []RadialRing{
			{Key: "actual", Label: "Actual", Order: 1, Total: 100},
			{Key: "plan", Label: "Plan", Order: 2, Total: 100},
		},
		Tolerance: 0.01,
	}, wire.Radial)
	require.Equal(t, &Presentation{Legend: LegendBelow, SliceLabels: SliceLabelsPercent}, wire.Presentation)

	// A deferred panel has no rows yet, so a declared ring total can only be the
	// layout skeleton's placeholder. Left in place it made every slice print its
	// own amount as a percentage of that placeholder; zero tells the runtime to
	// reconcile against the rows it is actually given.
	deferredDoc, err := Build(spec, executed, BuildOptions{
		SnapshotID: "radial-deferred", GeneratedAt: time.Unix(1, 0).UTC(), Locale: "en", DeferPanels: true,
	})
	require.NoError(t, err)
	require.Len(t, deferredDoc.Panels, 1)
	require.NotNil(t, deferredDoc.Panels[0].Radial)
	for _, ring := range deferredDoc.Panels[0].Radial.Rings {
		require.Zero(t, ring.Total, ring.Key)
	}
}

func TestBuildPresentation_ShowLegendFallsBackToBelow(t *testing.T) {
	t.Parallel()

	require.Equal(t, &Presentation{Legend: LegendBelow}, buildPresentation(panel.Spec{ShowLegend: true}))
}

func TestBuildPresentation_CarriesOptInDataLabels(t *testing.T) {
	t.Parallel()

	require.Equal(t, &Presentation{DataLabels: true}, buildPresentation(panel.Spec{
		Presentation: panel.PresentationHints{DataLabels: true},
	}))
}

func TestBuild_GaugeUsesWireKind(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("gauge", frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{68.4}})
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	spec := lensbuild.Dashboard("gauge", "Gauge", lensbuild.Row(
		panel.Gauge("budget", "Budget", "gauge").ValueField("value").Terminal().Build(),
	)).Datasets(lensbuild.StaticDataset("gauge", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "gauge", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Equal(t, PanelKindGauge, doc.Panels[0].Kind)
	require.NoError(t, doc.Validate())
}

func TestBuild_DistributionPanelsCarryExplicitWireEncodings(t *testing.T) {
	t.Parallel()
	set, err := frame.FromRows("distribution", frame.Row{
		"bucket": "0–10", "count": 12.0, "product": "OSAGO", "region": "Tashkent",
		"min": 1.0, "q1": 3.0, "median": 5.0, "q3": 8.0, "max": 20.0,
	})
	require.NoError(t, err)
	numberFormat := format.Count()
	spec := lensbuild.Dashboard("distribution", "Distribution", lensbuild.Row(
		panel.Histogram("hist", "Histogram", "distribution").CategoryField("bucket").ValueField("count").Terminal().Build(),
		panel.BoxPlot("box", "Box plot", "distribution").CategoryField("product").
			BoxFields("min", "q1", "median", "q3", "max").Format(numberFormat).Terminal().Build(),
		panel.Heatmap("heat", "Heatmap", "distribution").CategoryField("region").SeriesField("product").ValueField("count").Terminal().Build(),
	)).Datasets(lensbuild.StaticDataset("distribution", set)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "distribution", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Equal(t, []PanelKind{PanelKindHistogram, PanelKindBoxPlot, PanelKindHeatmap}, []PanelKind{
		doc.Panels[0].Kind, doc.Panels[1].Kind, doc.Panels[2].Kind,
	})
	require.Equal(t, Encoding{Category: "product", Lower: "min", Q1: "q1", Median: "median", Q3: "q3", Upper: "max"}, doc.Panels[1].Encoding)
	require.Contains(t, doc.Panels[1].Format, "median")
	require.NoError(t, doc.Validate())
}

func TestBuild_ChoroplethCarriesBoundedGeometryAndJoin(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("regions",
		frame.Field{Name: "code", Type: frame.FieldTypeString, Values: []any{"north"}},
		frame.Field{Name: "name", Type: frame.FieldTypeString, Values: []any{"North"}},
		frame.Field{Name: "premium", Type: frame.FieldTypeNumber, Values: []any{42.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	geometry := &panel.GeoJSONFeatureCollection{Type: "FeatureCollection", Features: []panel.GeoJSONFeature{{
		Type: "Feature", Properties: map[string]any{"code": "north", "name": "North", "name_ru": "Север"},
		Geometry: map[string]any{"type": "Polygon", "coordinates": []any{[]any{}}},
	}}}
	spec := lensbuild.Dashboard("map", "Map", lensbuild.Row(
		panel.Choropleth("regions", "Regions", "regions", panel.GeoJSONSource{Inline: geometry}, "code").
			MapLabelProperty("name").MapLabelProperties(map[string]string{"en": "name", "ru": "name_ru"}).
			MapAttribution("© Example Maps").ComparisonUnsupported().
			IDField("code").LabelField("name").ValueField("premium").Terminal().Build(),
	)).Datasets(lensbuild.StaticDataset("regions", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "map", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Equal(t, PanelKindMap, doc.Panels[0].Kind)
	require.Equal(t, "code", doc.Panels[0].Map.FeatureProperty)
	require.Equal(t, "name", doc.Panels[0].Map.LabelProperty)
	require.Equal(t, map[string]string{"en": "name", "ru": "name_ru"}, doc.Panels[0].Map.LabelProperties)
	require.Equal(t, "© Example Maps", doc.Panels[0].Map.Attribution)
	require.True(t, doc.Panels[0].ComparisonUnsupported)
	require.Equal(t, "north", doc.Panels[0].Map.Source.Inline.Features[0].Properties["code"])
	require.NoError(t, doc.Validate())
}

func TestBuild_DocumentHeaderAndDrawerSuppression(t *testing.T) {
	t.Parallel()

	spec, result := executeExploreDashboard(t)
	header := &DocumentHeader{Title: "Board view", Subtitle: "FY 2026"}
	doc, err := Build(spec, result, BuildOptions{
		SnapshotID: "header", GeneratedAt: time.Unix(1, 0), Header: header,
	})
	require.NoError(t, err)
	require.Equal(t, header, doc.Header)

	drawerDoc, err := Build(spec, result, BuildOptions{
		SnapshotID: "drawer-header", GeneratedAt: time.Unix(1, 0), Header: header,
		Drawer: &DrawerHeader{Title: "Details", Size: DrawerSizeWide},
	})
	require.NoError(t, err)
	require.Nil(t, drawerDoc.Header)
	require.Empty(t, drawerDoc.Meta.Title)
}

func TestBuild_NodeKeysIgnoreLabelsAndDefinitionOrder(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	first, err := Build(spec, result, BuildOptions{SnapshotID: "one", GeneratedAt: time.Unix(1, 0), InlineDepth: 1})
	require.NoError(t, err)

	view := &spec.Explorers[0].Branches[0].Perspectives[0]
	view.Label = "Localized label"
	for index := range view.Nodes {
		view.Nodes[index].Label = "Localized " + view.Nodes[index].Key
	}
	slices.Reverse(view.Nodes)
	second, err := Build(spec, result, BuildOptions{SnapshotID: "two", GeneratedAt: time.Unix(2, 0), InlineDepth: 1})
	require.NoError(t, err)

	firstKeys := maps.Keys(first.Drill.Edges)
	secondKeys := maps.Keys(second.Drill.Edges)
	require.ElementsMatch(t, slices.Collect(firstKeys), slices.Collect(secondKeys))
	for index := range first.Perspectives {
		require.Equal(t, first.Perspectives[index].Root, second.Perspectives[index].Root)
	}
}

func TestBuild_ReusesRuntimeExploreValidation(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	spec.Explorers[0].Branches[0].Perspectives[0].Semantics = "unsupported"
	_, err := Build(spec, result, BuildOptions{SnapshotID: "invalid"})
	require.ErrorContains(t, err, "unsupported semantics")
}

func TestBuild_InlineDepthIncludesOnlyMaterializedAggregateLevels(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	view := &spec.Explorers[0].Branches[0].Perspectives[0]
	rootPanel := panel.Pie("explore-root", "Root", "premium").IDField("id").Terminal().Build()
	detailPanel := panel.Pie("explore-detail", "Detail", "premium").IDField("id").Terminal().Build()
	view.Nodes[0].Load = nil
	view.Nodes[0].Panel = &rootPanel
	view.Nodes[1].Load = nil
	view.Nodes[1].Panel = &detailPanel
	result.Panels[rootPanel.ID] = &runtime.PanelResult{Panel: rootPanel, Frames: result.Panels["host"].Frames}
	result.Panels[detailPanel.ID] = &runtime.PanelResult{Panel: detailPanel, Frames: result.Panels["host"].Frames}

	doc, err := Build(spec, result, BuildOptions{SnapshotID: "inline", GeneratedAt: time.Unix(1, 0), InlineDepth: 0})
	require.NoError(t, err)
	require.Equal(t, FrameRef("explore:metric/focus/composition:root"), doc.Drill.Edges["metric/focus/composition/root"].Frame)
	require.Empty(t, doc.Drill.Edges["metric/focus/composition/detail"].Frame)
	require.NotContains(t, doc.Frames, FrameRef("explore:metric/focus/composition:detail"))
}

func TestBuild_StaticDrillTreeBecomesReactDrillGraph(t *testing.T) {
	t.Parallel()

	primary, err := frame.New("broker_receivables",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"broker-a", "broker-b"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Broker A", "Broker B"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{100.0, 50.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	tree := panel.DrillTree{ExpandedSpan: 8, Branches: []panel.DrillBranch{
		{
			TriggerKey: "broker-a",
			Label:      "Broker A",
			Children: []panel.DrillNode{{
				Key: "proxy", Label: "Management estimate", Value: 100,
				Children: []panel.DrillNode{{Key: "USD", Label: "USD", Value: 100}},
			}},
		},
		{
			TriggerKey: "broker-b",
			Label:      "Broker B",
			Children:   []panel.DrillNode{{Key: "scheduled", Label: "Schedule", Value: 50}},
		},
	}}
	chart := panel.Pie("brokers", "Broker receivables", "broker_receivables").
		IDField("id").
		LabelField("label").
		ValueField("amount").
		DrillTree(tree).
		FocusCanvas().
		Build()
	spec := lensbuild.Dashboard("underwriting", "Underwriting", lensbuild.Row(chart)).
		Datasets(lensbuild.StaticDataset("broker_receivables", frames)).
		Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "static-tree", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Len(t, doc.Panels, 1)
	require.NotNil(t, doc.Panels[0].DrillRoot)
	root := doc.Drill.Edges[*doc.Panels[0].DrillRoot]
	require.Equal(t, FocusModeCanvas, root.Presentation.Focus)
	require.Len(t, root.Children, 2)
	require.Equal(t, NodeKey("broker-a"), root.Children[0].Key)
	require.NotEmpty(t, root.Children[0].Target)

	brokerLevel := doc.Drill.Edges[root.Children[0].Target]
	require.Equal(t, "Broker A", brokerLevel.Label)
	require.Equal(t, &Encoding{ID: "id", Label: "label", Value: "amount"}, brokerLevel.Encoding)
	require.Len(t, doc.Frames[brokerLevel.Frame].Rows, 1)
	require.NotEmpty(t, brokerLevel.Children[0].Target)
	currencyLevel := doc.Drill.Edges[brokerLevel.Children[0].Target]
	require.Equal(t, "Management estimate", currencyLevel.Label)
	require.Equal(t, "USD", doc.Frames[currencyLevel.Frame].Rows[0][1])
}

// TestBuild_SparklineTargetAndFocusCanvas covers the focus-canvas panel
// additions end to end: a stat's Go-side sparkline reaches the wire, a
// coverage panel carries its bullet target marker, and FocusCanvas surfaces as
// Presentation.Focus="canvas".
func TestBuild_SparklineTargetAndFocusCanvas(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"a", "b"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Known", "Buffer"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{58.21, 118.4}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	stat := panel.Stat("kpi", "KPI", "rows").SparklineColored([]float64{1, 2, 3}, "#2563eb").Terminal().Build()
	coverage := panel.SegmentBar("coverage", "Coverage", "rows").Target(58.21, "Known liabilities").Terminal().Build()
	host := panel.Donut("host", "Host", "rows").IDField("id").FocusCanvas().Terminal().Build()
	spec := lensbuild.Dashboard("focus", "Focus", lensbuild.Row(stat, coverage, host)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "focus", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	byID := map[string]Panel{}
	for _, wirePanel := range doc.Panels {
		byID[wirePanel.ID] = wirePanel
	}
	require.Equal(t, &Sparkline{Values: []float64{1, 2, 3}, Color: "#2563eb"}, byID["kpi"].Sparkline)
	require.Equal(t, &PanelTarget{Value: 58.21, Label: "Known liabilities"}, byID["coverage"].Target)
	require.NotNil(t, byID["host"].Presentation)
	require.Equal(t, FocusModeCanvas, byID["host"].Presentation.Focus)
	require.Nil(t, byID["kpi"].Target)
	require.Nil(t, byID["coverage"].Sparkline)
}

// TestBuild_FocusCanvasReachesExplorerRootLevel pins the chrome-activation
// contract: a host panel's `.FocusCanvas()` must surface on the explorer's
// drill ROOT level, because the React runtime (isFocusCanvas) reads the mode
// from the root level, not from the panel.
func TestBuild_FocusCanvasReachesExplorerRootLevel(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	spec.Rows[0].Panels[0].Presentation.FocusCanvas = true

	doc, err := Build(spec, result, BuildOptions{SnapshotID: "focus-root", GeneratedAt: time.Unix(1, 0), InlineDepth: 1})
	require.NoError(t, err)
	require.NotNil(t, doc.Panels[0].Presentation)
	require.Equal(t, FocusModeCanvas, doc.Panels[0].Presentation.Focus)
	root := doc.Drill.Edges["metric"]
	require.NotNil(t, root.Presentation)
	require.Equal(t, FocusModeCanvas, root.Presentation.Focus)
	branch := doc.Drill.Edges["metric/focus"]
	require.Equal(t, "metric/focus/composition", branch.DefaultPerspective)
}

// TestBuild_ExplorerLevelViewPresentationStatusAndSourceData pins the key
// unlock of the focus canvas: a drill level's declared visualization,
// presentation hints, quality status, and audit table all survive
// buildExplorer instead of being dropped with the node's panel kind.
func TestBuild_ExplorerLevelViewPresentationStatusAndSourceData(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	view := &spec.Explorers[0].Branches[0].Perspectives[0]
	money := format.Money("UZS", 0)
	rootPanel := panel.Pie("explore-root", "Root", "premium").IDField("id").Terminal().Build()
	sourceTable := panel.Table("root-source", "Source rows", "premium").IDField("id").Columns(
		panel.TableColumn{Field: "label", Label: "Label"},
		panel.TableColumn{Field: "value", Label: "Value", Formatter: &money},
	).Terminal().Build()
	view.Nodes[0] = explore.PanelNode("root", "Root", rootPanel, explore.ToNode("a", "detail")).
		WithView(panel.KindHorizontalBar).
		WithPresentation(panel.PresentationHints{Waterfall: true}).
		WithStatus("ПРОКСИ", panel.StatusWarning).
		WithSourceData("Исходные данные", sourceTable)
	result.Panels[rootPanel.ID] = &runtime.PanelResult{Panel: rootPanel, Frames: result.Panels["host"].Frames}
	result.Panels[sourceTable.ID] = &runtime.PanelResult{Panel: sourceTable, Frames: result.Panels["host"].Frames}

	doc, err := Build(spec, result, BuildOptions{SnapshotID: "focus-level", GeneratedAt: time.Unix(1, 0), InlineDepth: 1})
	require.NoError(t, err)
	level := doc.Drill.Edges["metric/focus/composition/root"]
	require.Equal(t, PanelKindHBar, level.View)
	require.NotNil(t, level.Presentation)
	require.Equal(t, BridgeLayoutWaterfall, level.Presentation.BridgeLayout)
	require.Equal(t, &PanelStatus{Label: "ПРОКСИ", Tone: StatusToneWarning}, level.Status)
	require.NotNil(t, level.Source)
	require.Equal(t, "Исходные данные", level.Source.Label)
	require.Equal(t, FrameRef("source:metric/focus/composition:root"), level.Source.Frame)
	require.Contains(t, doc.Frames, level.Source.Frame)
	require.Equal(t, []string{"label", "value", "id"}, columnNames(doc.Frames[level.Source.Frame].Columns))
	require.Len(t, level.Source.Columns, 2)
	require.Equal(t, FormatMoney, level.Source.Format["value"].Kind)
	// The detail level declares none of the additions and stays bare.
	detail := doc.Drill.Edges["metric/focus/composition/detail"]
	require.Empty(t, detail.View)
	require.Nil(t, detail.Presentation)
	require.Nil(t, detail.Status)
	require.Nil(t, detail.Source)

	// A source table whose execution is absent from the runtime result drops
	// the declaration, mirroring the inline level frame guard.
	delete(result.Panels, sourceTable.ID)
	doc, err = Build(spec, result, BuildOptions{SnapshotID: "no-source", GeneratedAt: time.Unix(1, 0), InlineDepth: 1})
	require.NoError(t, err)
	require.Nil(t, doc.Drill.Edges["metric/focus/composition/root"].Source)
}

// TestBuild_ExplorerLevelViewRejectsNonWireKinds pins the build-time guard: a
// level view that has no wire representation fails the build with the node's
// address instead of emitting an invalid document.
func TestBuild_ExplorerLevelViewRejectsNonWireKinds(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	view := &spec.Explorers[0].Branches[0].Perspectives[0]
	view.Nodes[0] = view.Nodes[0].WithView(panel.KindTabs)
	_, err := Build(spec, result, BuildOptions{SnapshotID: "bad-view", GeneratedAt: time.Unix(1, 0), InlineDepth: 1})
	require.ErrorContains(t, err, "node root view")
	require.ErrorContains(t, err, "unsupported document panel kind")
}

func TestBuild_TableSemanticsRequiresLeafActionForEvidence(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"tx-1"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	leaf := action.Navigate("/records/{id}", action.FieldParam("id", "id"))
	htmx := action.HtmxSwap("/drill", "#drawer")
	spec := lensbuild.Dashboard("overview", "Overview",
		lensbuild.Row(
			panel.Table("evidence-table", "Evidence", "rows").IDField("id").
				Columns(panel.TableColumn{Field: panel.FieldRef("label"), Label: "Label", Action: &leaf}).Build(),
			// An aggregate matrix: its only interaction is a renderer-local
			// HTMX drawer, which never becomes a wire action.
			panel.Table("matrix-table", "Matrix", "rows").IDField("id").
				Columns(panel.TableColumn{Field: panel.FieldRef("label"), Label: "Label", Action: &htmx}).Terminal().Build(),
		),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope())
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(0, 0).UTC(), Locale: "en"})
	require.NoError(t, err)
	semantics := map[string]Semantics{}
	for _, p := range doc.Panels {
		semantics[p.ID] = p.Semantics
	}
	require.Equal(t, SemanticsEvidence, semantics["evidence-table"])
	require.Equal(t, SemanticsSeries, semantics["matrix-table"])
}

func TestConvertAction_PreservesDrawerAndDropsHTMX(t *testing.T) {
	t.Parallel()

	drawer, ok := convertAction(action.OpenDrawer("/drill/loss/lens/document"), false)
	require.True(t, ok)
	require.Equal(t, ActionOpenDrawer, drawer.Kind)
	require.Equal(t, "/drill/loss/lens/document", drawer.URLTemplate)
	metricDrawer, ok := convertAction(action.OpenDrawerMetric(action.FieldValue("metric_key")), false)
	require.True(t, ok)
	require.Equal(t, ActionOpenDrawer, metricDrawer.Kind)
	require.NotNil(t, metricDrawer.DrawerKey)
	require.Equal(t, ValueSourceField, metricDrawer.DrawerKey.Kind)
	require.Equal(t, "metric_key", metricDrawer.DrawerKey.Name)
	require.Empty(t, metricDrawer.URLTemplate)

	_, ok = convertAction(action.HtmxSwap("/drill/loss", "#drawer"), false)
	require.False(t, ok)
}

func TestBuildTrendCarriesPercentagePointUnit(t *testing.T) {
	t.Parallel()
	trend := buildTrend(panel.Stat("ratio", "Ratio", "ratio").
		AutoTrend("delta", "delta_percent", "Comparison", false).
		TrendAbsoluteDeltaUnit(panel.TrendDeltaPercentagePoints).
		Build())
	require.Equal(t, TrendDeltaPercentagePoints, trend.AbsoluteDeltaUnit)
}

func TestBuild_PanelTotalBadgeValue(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("totals",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Paid"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{75.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	withTotal := panel.Pie("with-total", "With total", "totals").TotalBadgeValue(125.5).Terminal().Build()
	withoutTotal := panel.Pie("without-total", "Without total", "totals").Terminal().Build()
	spec := lensbuild.Dashboard("totals", "Totals", lensbuild.Row(withTotal, withoutTotal)).
		Datasets(lensbuild.StaticDataset("totals", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "totals", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Len(t, doc.Panels, 2)
	require.InDelta(t, 125.5, *doc.Panels[0].Total, 1e-9)
	require.Nil(t, doc.Panels[1].Total)

	payload, err := json.Marshal(doc.Panels)
	require.NoError(t, err)
	var wirePanels []map[string]any
	require.NoError(t, json.Unmarshal(payload, &wirePanels))
	total, ok := wirePanels[0]["total"].(float64)
	require.True(t, ok)
	require.InDelta(t, 125.5, total, 1e-9)
	require.NotContains(t, wirePanels[1], "total")
}

func TestBuild_PreservesLogarithmicValueAxis(t *testing.T) {
	t.Parallel()

	primary, err := frame.New("products",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Large", "Medium", "Small"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{1000.0, 10.0, 1.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	spec := lensbuild.Dashboard("sales", "Sales",
		lensbuild.Row(
			panel.HorizontalBar("products", "Products", "products").
				LogarithmicValueAxis(10).
				Terminal().Build(),
		),
	).Datasets(lensbuild.StaticDataset("products", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "log-axis", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Len(t, doc.Panels, 1)
	require.Equal(t, &ValueAxis{Scale: AxisScaleLogarithmic, LogBase: 10}, doc.Panels[0].ValueAxis)
}

func TestBuild_TableProjectsColumnsAndCarriesMetadata(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("profitability",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"row-1"}},
		frame.Field{Name: "group", Type: frame.FieldTypeString, Values: []any{"Retail"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{1250.0}},
		frame.Field{Name: "sample_count", Type: frame.FieldTypeNumber, Values: []any{2.0}},
		frame.Field{Name: "delta", Type: frame.FieldTypeNumber, Values: []any{-50.0}},
		frame.Field{Name: "delta_pct", Type: frame.FieldTypeNumber, Values: []any{-4.0}},
		frame.Field{Name: "earned_premium_url", Type: frame.FieldTypeString, Values: []any{"/analytics/premium?signed=token"}},
		frame.Field{Name: "action_url", Type: frame.FieldTypeString, Values: []any{"/analytics/drawer?signed=token"}},
		frame.Field{Name: "renderer_internal", Type: frame.FieldTypeString, Values: []any{"must-not-leak"}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	money := format.Money("UZS", 0)
	navigate := action.Navigate("").WithFieldURL("earned_premium_url")
	htmx := action.HtmxSwap("", "#drawer").WithFieldURL("action_url")
	spec := lensbuild.Dashboard("profitability", "Profitability",
		lensbuild.Row(
			panel.Table("profitability-table", "Profitability", "profitability").IDField("id").Columns(
				panel.TableColumn{Field: "group", Label: "Группа", Action: &htmx},
				panel.TableColumn{Field: "amount", Label: "Заработанная премия", Align: "right", Formatter: &money, Cell: &panel.TableCellSpec{Kind: panel.TableCellBar}, Heat: true, SampleSizeField: "sample_count", MinSampleSize: 5},
				panel.TableColumn{Field: "delta", Label: "Изменение", Align: "right", Cell: &panel.TableCellSpec{Kind: panel.TableCellDelta, PercentField: "delta_pct"}, Action: &navigate},
			).Build(),
		),
	).Datasets(lensbuild.StaticDataset("profitability", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(context.Background(), spec, runtime.Request{Locale: "ru", DataScope: "tenant:1"}, runtime.DashboardScope())
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "projection", GeneratedAt: time.Unix(1, 0), Locale: "ru"})
	require.NoError(t, err)
	require.Len(t, doc.Panels, 1)
	wirePanel := doc.Panels[0]
	require.Equal(t, SemanticsEvidence, wirePanel.Semantics)
	require.Empty(t, wirePanel.Actions)
	require.Equal(t, []TableColumn{
		{Field: "group", Label: "Группа", Cell: TableCell{Kind: TableCellPlain}},
		{Field: "amount", Label: "Заработанная премия", Align: TableAlignRight, Cell: TableCell{Kind: TableCellBar}, Heat: true, SampleSizeField: "sample_count", MinSampleSize: 5},
		{
			Field: "delta", Label: "Изменение", Align: TableAlignRight,
			Cell: TableCell{Kind: TableCellDelta, SecondaryField: "delta_pct"},
			Action: &Action{
				Kind: ActionNavigateToLeaf, Method: "GET", URLSource: &Source{Kind: ValueSourceField, Name: "earned_premium_url"},
				Params: []ActionParam{}, Payload: map[string]Source{},
			},
		},
	}, wirePanel.Columns)
	// A spec that asks for whole units must reach the wire as precision 0, not
	// as an absent field: an absent precision means "locale default", which is
	// how a money headline used to pick up three decimals.
	require.Equal(t, FieldFormat{
		Kind: FormatMoney, Currency: "UZS", Precision: PrecisionOf(0), Symbol: "so’m",
	}, wirePanel.Format["amount"])
	// Delta secondaries carry percent-unit values, so the wire format defaults
	// to percent when the column declares no formatter of its own.
	require.Equal(t, FieldFormat{Kind: FormatPercent, Precision: PrecisionOf(1)}, wirePanel.Format["delta_pct"])

	wireFrame := doc.Frames[wirePanel.Frame]
	require.Equal(t, []Column{
		{Name: "group", Type: ColumnString},
		{Name: "amount", Type: ColumnNumber},
		{Name: "delta", Type: ColumnNumber},
		{Name: "id", Type: ColumnString},
		{Name: "sample_count", Type: ColumnNumber},
		{Name: "delta_pct", Type: ColumnNumber},
		{Name: "earned_premium_url", Type: ColumnString},
	}, wireFrame.Columns)
	require.Equal(t, [][]any{{"Retail", 1250.0, -50.0, "row-1", 2.0, -4.0, "/analytics/premium?signed=token"}}, wireFrame.Rows)
	require.NotContains(t, columnNames(wireFrame.Columns), "action_url")
	require.NotContains(t, columnNames(wireFrame.Columns), "renderer_internal")
}

// TestBuild_SingleLevelContainersUseGroupsChain pins that even a single-level
// container is represented by the authoritative Groups chain.
func TestBuild_SingleLevelContainersUseGroupsChain(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	group := panel.StatGroup("ratios", "By earned premium").Span(12).Children(
		panel.Stat("ratio-a", "Ratio A", "rows").Terminal().Build(),
	).Build()
	tabs := panel.Tabs("result", "Result").Span(12).Children(
		panel.Stat("cash", "Cash result", "rows").Terminal().Build(),
	).Build()
	spec := lensbuild.Dashboard("groups", "Groups", lensbuild.Row(group), lensbuild.Row(tabs)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)

	require.Len(t, doc.Layout.Rows[0].Panels[0].Groups, 1)
	require.Len(t, doc.Layout.Rows[1].Panels[0].Groups, 1)
}

// TestBuild_ContainerDescriptionBecomesGroupCaption pins that a container's
// Description survives the wire. A container owns no Panel of its own, so
// before LayoutGroup.Caption existed the text was silently dropped — a
// dashboard could set it, pass validation, and render nothing.
func TestBuild_ContainerDescriptionBecomesGroupCaption(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	group := panel.StatGroup("ratios", "By written premium").Span(12).
		Description("  Diagnostic basis, not a competing verdict.  ").
		Children(panel.Stat("ratio-a", "Ratio A", "rows").Terminal().Build()).Build()
	tabs := panel.Tabs("result", "Result").Span(12).
		Description("Cash and underwriting views of the same period.").
		Children(panel.Stat("cash", "Cash result", "rows").Terminal().Build()).Build()
	spec := lensbuild.Dashboard("groups", "Groups", lensbuild.Row(group), lensbuild.Row(tabs)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)

	require.Equal(t, "Diagnostic basis, not a competing verdict.", doc.Layout.Rows[0].Panels[0].Groups[0].Caption)
	require.Equal(t, "Cash and underwriting views of the same period.", doc.Layout.Rows[1].Panels[0].Groups[0].Caption)
}

// TestBuild_MetricKinds covers all three metric panel kinds end to end: kind
// mapping, semantics inference (flow=reconciliation, hierarchy=series,
// relationship=series for association), encoding of the new Share/Confidence/
// Availability roles, per-element action conversion (including an htmx stage
// action being dropped), hierarchy depth derivation from Parent, and a nested
// Tabs-in-Tabs / StatGroup-inside-Tabs group chain.
func TestBuild_MetricKinds(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{
			"in", "add", "sub", "result", "root", "mid", "leaf", "unalloc", "src", "tgt",
		}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{
			100.0, 20.0, -5.0, 115.0, 100.0, 60.0, 0.0, 40.0, 50.0, 50.0,
		}},
		frame.Field{Name: "share", Type: frame.FieldTypeNumber, Values: []any{
			1.0, 0.2, 0.05, 1.0, 1.0, 0.6, 0.0, 0.4, 0.5, 0.5,
		}},
		frame.Field{Name: "confidence", Type: frame.FieldTypeString, Values: []any{
			"verified", "calculated", "calculated", "verified", "verified", "calculated", "verified", "proxy", "verified", "verified",
		}},
		frame.Field{Name: "availability", Type: frame.FieldTypeString, Values: []any{
			"available", "available", "available", "available", "available", "available", "available", "available", "available", "available",
		}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{
			"Input", "Add", "Subtract", "Result", "Root", "Mid", "Leaf", "Unallocated", "Source", "Target",
		}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	navigate := action.Navigate("/flow/result")
	htmx := action.HtmxSwap("/flow/add", "#drawer")
	flow := panel.MetricFlow("flow", "Flow", "rows",
		panel.FlowStage{Key: "in", Label: "Input", Role: panel.FlowRoleInput},
		panel.FlowStage{
			Key: "add", Label: "Add", Role: panel.FlowRoleAdd,
			Confidence: panel.ConfidenceCalculated, Availability: panel.AvailabilityAvailable, Action: &htmx,
		},
		panel.FlowStage{Key: "sub", Label: "Subtract", Role: panel.FlowRoleSubtract, Caption: "movement"},
		panel.FlowStage{Key: "result", Label: "Result", Role: panel.FlowRoleResult, Action: &navigate},
	).FlowReconcile(0.5).Confidence(panel.ConfidenceVerified).Availability(panel.AvailabilityAvailable).Build()

	hierarchy := panel.MetricHierarchy("hierarchy", "Hierarchy", "rows",
		panel.HierarchyRow{Key: "root", Label: "Root"},
		panel.HierarchyRow{Key: "mid", Label: "Mid", Parent: "root"},
		panel.HierarchyRow{Key: "leaf", Label: "Leaf", Parent: "mid"},
		panel.HierarchyRow{Key: "unalloc", Label: "Unallocated", Parent: "mid", Unallocated: true},
		panel.HierarchyRow{Key: "ghost", Label: "Ghost (missing frame key)", Parent: "root"},
	).HierarchyReconcile(0.1).Terminal().Build()

	relationship := panel.MetricRelationship("relationship", "Relationship", "rows", panel.RelationshipSpec{
		Source: panel.RelationshipEnd{Key: "src", Label: "Source"},
		Target: panel.RelationshipEnd{Key: "tgt", Label: "Target"},
		Type:   panel.RelationshipAssociation,
	}).Terminal().Build()

	statA := panel.Stat("stat-a", "Stat A", "rows").Terminal().Build()
	statB := panel.Stat("stat-b", "Stat B", "rows").Terminal().Build()
	stockGroup := panel.StatGroup("stock-group", "Stock ratios").Span(12).Children(statA, statB).Build()

	innerA := panel.Stat("inner-a", "Inner A", "rows").Terminal().Build()
	innerB := panel.Stat("inner-b", "Inner B", "rows").Terminal().Build()
	movementDetail := panel.Tabs("movement-detail", "Detail").Span(12).Children(innerA, innerB).Build()

	composition := panel.Tabs("composition", "Composition").Span(12).Children(stockGroup, movementDetail).Build()

	spec := lensbuild.Dashboard("metric-kinds", "Metric kinds",
		lensbuild.Row(flow, hierarchy, relationship),
		lensbuild.Row(composition),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{
		SnapshotID: "metric-kinds", GeneratedAt: time.Unix(1, 0).UTC(), Locale: "en",
	})
	require.NoError(t, err)
	require.NoError(t, doc.Validate())

	byID := map[string]Panel{}
	for _, wirePanel := range doc.Panels {
		byID[wirePanel.ID] = wirePanel
	}

	// Kind mapping + panel-level quality defaults.
	require.Equal(t, PanelKindMetricFlow, byID["flow"].Kind)
	require.Equal(t, PanelKindMetricHierarchy, byID["hierarchy"].Kind)
	require.Equal(t, PanelKindMetricRelationship, byID["relationship"].Kind)
	require.Equal(t, ConfidenceVerified, byID["flow"].Confidence)
	require.Equal(t, AvailabilityAvailable, byID["flow"].Availability)

	// Semantics inference: flow always reconciliation; hierarchy stays series
	// (see build.go's documented deviation on validatePartitionFrame's
	// non-negative invariant); relationship is series for association.
	require.Equal(t, SemanticsReconciliation, byID["flow"].Semantics)
	require.Equal(t, SemanticsSeries, byID["hierarchy"].Semantics)
	require.Equal(t, SemanticsSeries, byID["relationship"].Semantics)

	// Encodings carry the new Share/Confidence/Availability roles.
	require.Equal(t, "share", byID["flow"].Encoding.Share)
	require.Equal(t, "confidence", byID["flow"].Encoding.Confidence)
	require.Equal(t, "availability", byID["flow"].Encoding.Availability)

	// MetricFlow stage action conversion: htmx is dropped, navigate survives.
	require.NotNil(t, byID["flow"].MetricFlow)
	stagesByKey := map[string]MetricFlowStage{}
	for _, stage := range byID["flow"].MetricFlow.Stages {
		stagesByKey[stage.Key] = stage
	}
	require.Nil(t, stagesByKey["add"].Action, "htmx stage action must be dropped, not carried to the wire")
	require.NotNil(t, stagesByKey["result"].Action)
	require.Equal(t, ActionNavigateToLeaf, stagesByKey["result"].Action.Kind)
	require.Equal(t, "movement", stagesByKey["sub"].Caption)
	require.NotNil(t, byID["flow"].MetricFlow.Reconcile)
	require.InDelta(t, 0.5, byID["flow"].MetricFlow.Reconcile.Tolerance, 1e-9)

	// MetricHierarchy depth derivation from Parent (root=0), including a row
	// whose key has no matching frame row (a structural declaration is valid;
	// resolving it against the frame is a render-time, not a build-time,
	// concern).
	require.NotNil(t, byID["hierarchy"].MetricHierarchy)
	rowsByKey := map[string]MetricHierarchyRow{}
	for _, row := range byID["hierarchy"].MetricHierarchy.Rows {
		rowsByKey[row.Key] = row
	}
	require.Equal(t, 0, rowsByKey["root"].Depth)
	require.Equal(t, 1, rowsByKey["mid"].Depth)
	require.Equal(t, 2, rowsByKey["leaf"].Depth)
	require.Equal(t, 2, rowsByKey["unalloc"].Depth)
	require.Equal(t, 1, rowsByKey["ghost"].Depth)
	require.True(t, rowsByKey["unalloc"].Unallocated)
	require.NotNil(t, byID["hierarchy"].MetricHierarchy.Reconcile)
	require.InDelta(t, 0.1, byID["hierarchy"].MetricHierarchy.Reconcile.Tolerance, 1e-9)

	// MetricRelationship: empty Direction defaults to bidirectional.
	require.NotNil(t, byID["relationship"].MetricRelationship)
	require.Equal(t, MetricRelationshipAssociation, byID["relationship"].MetricRelationship.Type)
	require.Equal(t, MetricRelationshipBidirectional, byID["relationship"].MetricRelationship.Direction)

	// Nested group chains: StatGroup-inside-Tabs and Tabs-in-Tabs each produce
	// a two-level Groups chain.
	items := map[string]LayoutItem{}
	for _, layoutRow := range doc.Layout.Rows {
		for _, item := range layoutRow.Panels {
			items[item.PanelID] = item
		}
	}
	// Top-level metric panels are not inside any container: no group at all.
	require.Nil(t, items["flow"].Groups)

	statAItem := items["stat-a"]
	require.Len(t, statAItem.Groups, 2)
	require.Equal(t, "composition", statAItem.Groups[0].ID)
	require.Equal(t, LayoutGroupTabs, statAItem.Groups[0].Kind)
	require.Equal(t, "Stock ratios", statAItem.Groups[0].Tab)
	require.Equal(t, "stock-group", statAItem.Groups[1].ID)
	require.Equal(t, LayoutGroupMetrics, statAItem.Groups[1].Kind)

	innerAItem := items["inner-a"]
	require.Len(t, innerAItem.Groups, 2)
	require.Equal(t, "composition", innerAItem.Groups[0].ID)
	require.Equal(t, "Detail", innerAItem.Groups[0].Tab)
	require.Equal(t, "movement-detail", innerAItem.Groups[1].ID)
	require.Equal(t, LayoutGroupTabs, innerAItem.Groups[1].Kind)
	require.Equal(t, "Inner A", innerAItem.Groups[1].Tab)

	innerBItem := items["inner-b"]
	require.Equal(t, "Inner B", innerBItem.Groups[1].Tab)

	payload, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)
	requireGolden(t, "metric_kinds.json", string(payload)+"\n")
}

func columnNames(columns []Column) []string {
	names := make([]string, len(columns))
	for index, column := range columns {
		names[index] = column.Name
	}
	return names
}

func executeExploreDashboard(t *testing.T) (lens.DashboardSpec, *runtime.Result) {
	t.Helper()
	primary, err := frame.New("premium",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"a", "b"}},
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha", "Beta"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{60.0, 40.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)
	explorerSpec, err := explore.New("metric", "host",
		explore.NewBranch("focus", "Focus", "composition",
			explore.NewPerspective("composition", "Composition", explore.SemanticsPartition, "root",
				explore.LazyNode("root", "Root", "/explore", explore.ToNode("a", "detail")),
				explore.LazyNode("detail", "Detail", "/explore", explore.ToAction("leaf", action.Navigate("/policies/{policyId}", action.LiteralParam("policyId", "selected")))),
			),
		),
	).Build()
	require.NoError(t, err)
	spec := lensbuild.Dashboard("overview", "Overview",
		lensbuild.Row(panel.Pie("host", "Premium", "premium").IDField("id").Build()),
	).Datasets(lensbuild.StaticDataset("premium", frames)).Explorers(explorerSpec).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope())
	require.NoError(t, err)
	return spec, executed
}

func TestBuild_ExplicitDeltaFormatterBeatsPercentDefault(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"row-1"}},
		frame.Field{Name: "delta", Type: frame.FieldTypeNumber, Values: []any{-50.0}},
		frame.Field{Name: "delta_pct", Type: frame.FieldTypeNumber, Values: []any{-4.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	explicit := format.Money("UZS", 2)
	spec := lensbuild.Dashboard("rows", "Rows",
		lensbuild.Row(
			panel.Table("t", "T", "rows").Columns(
				panel.TableColumn{Field: "delta", Label: "Delta", Cell: &panel.TableCellSpec{Kind: panel.TableCellDelta, PercentField: "delta_pct"}},
				panel.TableColumn{Field: "delta_pct", Label: "Delta %", Formatter: &explicit},
			).Terminal().Build(),
		),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Equal(t, FieldFormat{Kind: FormatMoney, Currency: "UZS", Precision: PrecisionOf(2), Symbol: "so’m"}, doc.Panels[0].Format["delta_pct"])
}

func TestBuild_TableWithoutColumnsKeepsEveryField(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	spec := lensbuild.Dashboard("rows", "Rows",
		lensbuild.Row(panel.Table("plain", "Plain", "rows").Terminal().Build()),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	wireFrame := doc.Frames[doc.Panels[0].Frame]
	require.Equal(t, []string{"label", "value"}, columnNames(wireFrame.Columns))
	require.Equal(t, [][]any{{"Alpha", 10.0}}, wireFrame.Rows)
}

func TestBuild_CompactFormatterPinsSeparator(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{9_364_442_607.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	compact := format.MoneyCompact("UZS")
	spec := lensbuild.Dashboard("rows", "Rows",
		lensbuild.Row(panel.Pie("p", "P", "rows").Format(compact).Terminal().Build()),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "ru", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "ru"})
	require.NoError(t, err)
	require.Equal(t, FieldFormat{
		Kind: FormatMoney, Currency: "UZS", Precision: PrecisionOf(2), Compact: true,
	}, doc.Panels[0].Format["value"])
}

func TestBuild_StatGroupAndTabsBecomeLayoutGroups(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{10.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	group := panel.StatGroup("ratios", "By earned premium").Span(12).Layout(panel.GroupColumns).Children(
		panel.Stat("ratio-a", "Ratio A", "rows").Status("ОЦЕНКА", panel.StatusWarning).Colors("#2f56d9").Terminal().Build(),
		panel.Stat("ratio-b", "Ratio B", "rows").Terminal().Build(),
	).Build()
	tabs := panel.Tabs("result", "Result").Span(12).Children(
		panel.Stat("cash", "Cash result", "rows").Terminal().Build(),
		panel.Table("underwriting", "Underwriting result", "rows").Terminal().Build(),
	).Build()
	spec := lensbuild.Dashboard("groups", "Groups", lensbuild.Row(group), lensbuild.Row(tabs)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)

	metrics := doc.Layout.Rows[0].Panels
	require.Len(t, metrics, 2)
	for _, item := range metrics {
		require.Len(t, item.Groups, 1)
		require.Equal(t, LayoutGroupMetrics, item.Groups[0].Kind)
		require.Equal(t, "ratios", item.Groups[0].ID)
		require.Equal(t, "By earned premium", item.Groups[0].Label)
		require.Equal(t, LayoutGroupColumns, item.Groups[0].Layout)
		require.Equal(t, 12, item.Groups[0].Span)
	}

	byID := map[string]Panel{}
	for _, wirePanel := range doc.Panels {
		byID[wirePanel.ID] = wirePanel
	}
	require.Equal(t, &PanelStatus{Label: "ОЦЕНКА", Tone: StatusToneWarning}, byID["ratio-a"].Status)
	require.Equal(t, "#2f56d9", byID["ratio-a"].Accent)
	require.Nil(t, byID["ratio-b"].Status)

	tabItems := doc.Layout.Rows[1].Panels
	require.Len(t, tabItems, 2)
	require.Equal(t, LayoutGroupTabs, tabItems[0].Groups[0].Kind)
	require.Equal(t, "Cash result", tabItems[0].Groups[0].Tab)
	require.Equal(t, "Underwriting result", tabItems[1].Groups[0].Tab)
	require.Equal(t, "result", tabItems[1].Groups[0].ID)
}

func TestBuild_SegmentBarBecomesCoverage(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Within reserve", "Above reserve"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{5.0, 0.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	headline := 5.0
	segment := panel.SegmentBar("payouts", "Claim payouts", "rows").
		Description("ALL CLAIMS COVERED BY RESERVE").
		Presentation(panel.PresentationHints{HideTotalBadge: true}).
		Terminal().Build()
	segment.HeadlineValue = &headline
	spec := lensbuild.Dashboard("coverage", "Coverage", lensbuild.Row(segment)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	wirePanel := doc.Panels[0]
	require.Equal(t, PanelKindCoverage, wirePanel.Kind)
	require.Equal(t, SemanticsPartition, wirePanel.Semantics)
	require.Equal(t, "ALL CLAIMS COVERED BY RESERVE", wirePanel.Caption)
	require.NotNil(t, wirePanel.Headline)
	require.InDelta(t, 5.0, *wirePanel.Headline, 1e-9)
	require.Equal(t, TotalBadgeNone, wirePanel.Presentation.TotalBadge)
}

func TestConvertPresentationCarriesProducerOwnedScaleAndMapPolicy(t *testing.T) {
	t.Parallel()

	presentation := convertPresentation(panel.PresentationHints{
		ColorByRank: true, ValueSpreadThreshold: 100,
	})
	require.NotNil(t, presentation)
	require.Equal(t, ColorByRank, presentation.ColorBy)
	require.InDelta(t, 100.0, presentation.ValueSpreadThreshold, 1e-9)
}

func TestBuild_CascadeCarriesWaterfallLayout(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("bridge",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Opening", "Closing"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{235.0, 56.98}},
		frame.Field{Name: "cut", Type: frame.FieldTypeNumber, Values: []any{0.0, 178.02}},
		frame.Field{Name: "cutLabel", Type: frame.FieldTypeString, Values: []any{"", "Net movement"}},
		frame.Field{Name: "final", Type: frame.FieldTypeBoolean, Values: []any{false, true}},
		frame.Field{Name: "annotation", Type: frame.FieldTypeString, Values: []any{"", "12 above threshold"}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	waterfall := panel.Cascade("reserve-movement", "Reserve movement", "bridge").
		Waterfall().
		AnnotationField("annotation").
		Terminal().Build()
	spec := lensbuild.Dashboard("waterfall", "Waterfall", lensbuild.Row(waterfall)).
		Datasets(lensbuild.StaticDataset("bridge", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.NotNil(t, doc.Panels[0].Presentation)
	require.Equal(t, BridgeLayoutWaterfall, doc.Panels[0].Presentation.BridgeLayout)
	require.Equal(t, "annotation", doc.Panels[0].Encoding.Annotation)
}

// A partition's colors must be reachable both by panel-scoped index and by the
// slice's own category name: chart renderers that only know a slice by its name
// would otherwise fall back to their built-in palette and ignore the spec.
func TestBuild_PanelColorsPublishIndexAndLabelSeriesKeys(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "segment", Type: frame.FieldTypeString, Values: []any{"Earned", "Unearned"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{7.0, 3.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	pie := panel.Pie("premium", "Premium", "rows").
		LabelField("segment").ValueField("amount").
		Colors("#2563eb", "#d97706").
		Terminal().Build()
	spec := lensbuild.Dashboard("premium-dash", "Premium", lensbuild.Row(pie)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Equal(t, "#2563eb", doc.Theme.Series["premium:0"])
	require.Equal(t, "#d97706", doc.Theme.Series["premium:1"])
	require.Equal(t, "#2563eb", doc.Theme.Series["Earned"])
	require.Equal(t, "#d97706", doc.Theme.Series["Unearned"])
}

// A series panel positions its colors by series, not by row: the n-th color is
// the n-th distinct series, and its rows are one per (category, series) pair.
// Reading the row label there published the category values as series colors
// and left the series themselves without an alias, so a renderer resolving a
// color by series name found nothing and fell back to its own palette.
func TestBuild_SeriesPanelColorsPublishSeriesNameKeys(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "month", Type: frame.FieldTypeString, Values: []any{"2025-01", "2025-01", "2025-02", "2025-02"}},
		frame.Field{Name: "series", Type: frame.FieldTypeString, Values: []any{"Claimed", "Paid", "Claimed", "Paid"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{9.0, 1.0, 8.0, 2.0}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	bar := panel.StackedBar("claimed", "Claimed", "rows").
		CategoryField("month").SeriesField("series").ValueField("amount").
		Colors("#2563eb", "#d97706").
		Terminal().Build()
	spec := lensbuild.Dashboard("claims-dash", "Claims", lensbuild.Row(bar)).
		Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "en", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "en"})
	require.NoError(t, err)
	require.Equal(t, "#2563eb", doc.Theme.Series["claimed:0"])
	require.Equal(t, "#d97706", doc.Theme.Series["claimed:1"])
	require.Equal(t, "#2563eb", doc.Theme.Series["Claimed"])
	require.Equal(t, "#d97706", doc.Theme.Series["Paid"])
	// A month is a position on the axis, not a series, and must not claim a
	// color that another panel's category of the same name would then inherit.
	require.NotContains(t, doc.Theme.Series, "2025-01")
	require.NotContains(t, doc.Theme.Series, "2025-02")
}

func TestBuild_PercentFormatPinsSeparator(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("rows",
		frame.Field{Name: "label", Type: frame.FieldTypeString, Values: []any{"Alpha"}},
		frame.Field{Name: "value", Type: frame.FieldTypeNumber, Values: []any{47.14}},
	)
	require.NoError(t, err)
	frames, err := frame.NewFrameSet(primary)
	require.NoError(t, err)

	percent := format.Percent(1)
	spec := lensbuild.Dashboard("rows", "Rows",
		lensbuild.Row(panel.Stat("s", "S", "rows").Format(percent).Terminal().Build()),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "ru", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "ru"})
	require.NoError(t, err)
	// The server formatter prints "47.1%"; the wire format carries the same
	// separator so the runtime does not drift to "47,1 %".
	require.Equal(t, FieldFormat{Kind: FormatPercent, Precision: PrecisionOf(1)}, doc.Panels[0].Format["value"])
}

// TestBuild_LazyExploreLevelsCarryDeclaredEncoding pins the contract that made
// document-mode expansion of lazy levels renderable: a level beyond the inline
// depth has no frame yet, but its declared field mapping must still ship, or
// the client falls back to the host panel's encoding and draws the fetched
// frame as empty.
func TestBuild_LazyExploreLevelsCarryDeclaredEncoding(t *testing.T) {
	t.Parallel()
	spec, result := executeExploreDashboard(t)
	view := &spec.Explorers[0].Branches[0].Perspectives[0]
	rootPanel := panel.Pie("explore-root", "Root", "premium").IDField("id").Terminal().Build()
	detailPanel := panel.Pie("explore-detail", "Detail", "premium").
		IDField("id").LabelField("label").ValueField("value").Terminal().Build()
	view.Nodes[0].Load = nil
	view.Nodes[0].Panel = &rootPanel
	view.Nodes[1].Load = nil
	view.Nodes[1].Panel = &detailPanel
	result.Panels[rootPanel.ID] = &runtime.PanelResult{Panel: rootPanel, Frames: result.Panels["host"].Frames}
	result.Panels[detailPanel.ID] = &runtime.PanelResult{Panel: detailPanel, Frames: result.Panels["host"].Frames}

	doc, err := Build(spec, result, BuildOptions{SnapshotID: "lazy", GeneratedAt: time.Unix(1, 0), InlineDepth: 0})
	require.NoError(t, err)

	detail := doc.Drill.Edges["metric/focus/composition/detail"]
	require.Empty(t, detail.Frame, "the lazy level still has no inline frame")
	require.NotNil(t, detail.Encoding, "a lazy level with a panel must declare its encoding")
	require.Equal(t, "id", detail.Encoding.ID)
	require.Equal(t, "label", detail.Encoding.Label)
	require.Equal(t, "value", detail.Encoding.Value)
	require.NoError(t, doc.Validate(), "encoding without a frame must stay valid")
}
