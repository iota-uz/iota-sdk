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
	require.Equal(t, golden(t, "generated_explore.json"), string(payload)+"\n")
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
	rootPanel := panel.Pie("explore-root", "Root", "premium").IDField("id").Build()
	detailPanel := panel.Pie("explore-detail", "Detail", "premium").IDField("id").Build()
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
				Columns(panel.TableColumn{Field: panel.FieldRef("label"), Label: "Label", Action: &htmx}).Build(),
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

	_, ok = convertAction(action.HtmxSwap("/drill/loss", "#drawer"), false)
	require.False(t, ok)
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

	withTotal := panel.Pie("with-total", "With total", "totals").TotalBadgeValue(125.5).Build()
	withoutTotal := panel.Pie("without-total", "Without total", "totals").Build()
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

func TestBuild_TableProjectsColumnsAndCarriesMetadata(t *testing.T) {
	t.Parallel()
	primary, err := frame.New("profitability",
		frame.Field{Name: "id", Type: frame.FieldTypeString, Values: []any{"row-1"}},
		frame.Field{Name: "group", Type: frame.FieldTypeString, Values: []any{"Retail"}},
		frame.Field{Name: "amount", Type: frame.FieldTypeNumber, Values: []any{1250.0}},
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
				panel.TableColumn{Field: "amount", Label: "Заработанная премия", Align: "right", Formatter: &money, Cell: &panel.TableCellSpec{Kind: panel.TableCellBar}},
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
		{Field: "amount", Label: "Заработанная премия", Align: TableAlignRight, Cell: TableCell{Kind: TableCellBar}},
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
	require.Equal(t, FieldFormat{Kind: FormatPercent, Precision: PrecisionOf(1), DecimalSeparator: "."}, wirePanel.Format["delta_pct"])

	wireFrame := doc.Frames[wirePanel.Frame]
	require.Equal(t, []Column{
		{Name: "group", Type: ColumnString},
		{Name: "amount", Type: ColumnNumber},
		{Name: "delta", Type: ColumnNumber},
		{Name: "id", Type: ColumnString},
		{Name: "delta_pct", Type: ColumnNumber},
		{Name: "earned_premium_url", Type: ColumnString},
	}, wireFrame.Columns)
	require.Equal(t, [][]any{{"Retail", 1250.0, -50.0, "row-1", -4.0, "/analytics/premium?signed=token"}}, wireFrame.Rows)
	require.NotContains(t, columnNames(wireFrame.Columns), "action_url")
	require.NotContains(t, columnNames(wireFrame.Columns), "renderer_internal")
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
			).Build(),
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
		lensbuild.Row(panel.Table("plain", "Plain", "rows").Build()),
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
		lensbuild.Row(panel.Pie("p", "P", "rows").Format(compact).Build()),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "ru", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "ru"})
	require.NoError(t, err)
	require.Equal(t, FieldFormat{
		Kind: FormatMoney, Currency: "UZS", Precision: PrecisionOf(2), Compact: true, DecimalSeparator: ".",
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
		panel.Stat("ratio-a", "Ratio A", "rows").Status("ОЦЕНКА", panel.StatusWarning).Colors("#2f56d9").Build(),
		panel.Stat("ratio-b", "Ratio B", "rows").Build(),
	).Build()
	tabs := panel.Tabs("result", "Result").Span(12).Children(
		panel.Stat("cash", "Cash result", "rows").Build(),
		panel.Table("underwriting", "Underwriting result", "rows").Build(),
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
		require.NotNil(t, item.Group)
		require.Equal(t, LayoutGroupMetrics, item.Group.Kind)
		require.Equal(t, "ratios", item.Group.ID)
		require.Equal(t, "By earned premium", item.Group.Label)
		require.Equal(t, LayoutGroupColumns, item.Group.Layout)
		require.Equal(t, 12, item.Group.Span)
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
	require.Equal(t, LayoutGroupTabs, tabItems[0].Group.Kind)
	require.Equal(t, "Cash result", tabItems[0].Group.Tab)
	require.Equal(t, "Underwriting result", tabItems[1].Group.Tab)
	require.Equal(t, "result", tabItems[1].Group.ID)
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
		Build()
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
		Build()
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
		lensbuild.Row(panel.Stat("s", "S", "rows").Format(percent).Build()),
	).Datasets(lensbuild.StaticDataset("rows", frames)).Build()
	executed, err := runtime.New(runtime.Options{}).Execute(
		context.Background(), spec, runtime.Request{Locale: "ru", DataScope: "tenant:1"}, runtime.DashboardScope(),
	)
	require.NoError(t, err)

	doc, err := Build(spec, executed, BuildOptions{SnapshotID: "s", GeneratedAt: time.Unix(1, 0), Locale: "ru"})
	require.NoError(t, err)
	// The Go renderer prints "47.1%"; the wire format carries the same
	// separator so the runtime does not drift to "47,1 %".
	require.Equal(t, FieldFormat{Kind: FormatPercent, Precision: PrecisionOf(1), DecimalSeparator: "."}, doc.Panels[0].Format["value"])
}
