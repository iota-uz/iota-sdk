package compile

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
	"github.com/stretchr/testify/require"
)

func TestDocumentCompilesSemanticDatasetMode(t *testing.T) {
	t.Parallel()

	baseBuilder := frame.NewBuilder("base").
		String("product_code", frame.RoleDimension).
		String("product_label", frame.RoleDimension).
		Number("premium", frame.RoleMetric)
	err := baseBuilder.Append(frame.Row{
		"product_code":  "osago",
		"product_label": "OSAGO",
		"premium":       125000,
	})
	require.NoError(t, err)
	base, err := baseBuilder.FrameSet()
	require.NoError(t, err)

	doc := lensspec.Document{
		Version:     lensspec.DocumentVersion,
		ID:          "semantic-report",
		Title:       lensspec.LiteralText("Semantic"),
		Description: lensspec.LiteralText("Dataset mode"),
		DataMode:    cube.DataModeDataset,
		DataRef:     "base_dataset",
		Dimensions: []lensspec.DimensionSpec{
			{Name: "product", Label: lensspec.LiteralText("Product"), Field: "product_code", LabelField: "product_label", PanelKind: panel.KindBar},
		},
		Measures: []lensspec.MeasureSpec{
			{Name: "total_revenue", Label: lensspec.LiteralText("Revenue"), Field: "premium", Aggregation: cube.AggregationSum},
		},
		DefaultDimension: "product",
	}

	compiled, err := Document(doc, Options{
		Locale: "en",
		Values: map[string]any{"base_dataset": base},
	})
	require.NoError(t, err)
	require.NotNil(t, compiled.Semantic)
	require.Equal(t, "semantic-report", compiled.Spec.ID)
	require.NotEmpty(t, compiled.Spec.Rows)
	require.NotEmpty(t, compiled.Spec.Datasets)
}

func TestDocumentCompilesMeasureOverrideStaticRef(t *testing.T) {
	t.Parallel()

	base, err := frame.FromRows("base", frame.Row{"product_code": "osago"})
	require.NoError(t, err)
	exact, err := frame.FromRows("exact", frame.Row{"total_policies": 72451})
	require.NoError(t, err)

	doc := lensspec.Document{
		Version:     lensspec.DocumentVersion,
		ID:          "semantic-report",
		Title:       lensspec.LiteralText("Semantic"),
		Description: lensspec.LiteralText("Dataset mode"),
		DataMode:    cube.DataModeDataset,
		DataRef:     "base_dataset",
		Dimensions: []lensspec.DimensionSpec{
			{Name: "product", Label: lensspec.LiteralText("Product"), Field: "product_code", PanelKind: panel.KindBar},
		},
		Measures: []lensspec.MeasureSpec{
			{
				Name:        "total_policies",
				Label:       lensspec.LiteralText("Policies"),
				Aggregation: cube.AggregationCount,
				Override: &lensspec.DatasetSpec{
					Kind:      lens.DatasetKindStatic,
					StaticRef: "exact_total_policies",
				},
			},
		},
		DefaultDimension: "product",
	}

	compiled, err := Document(doc, Options{
		Locale: "en",
		Values: map[string]any{
			"base_dataset":         base,
			"exact_total_policies": exact,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, compiled.Semantic)
	require.NotNil(t, compiled.Semantic.Measures[0].Override)
	require.Equal(t, "cube_stat_total_policies", compiled.Spec.Rows[0].Panels[0].Dataset)
}

func TestDocumentCompilesManualStaticDashboard(t *testing.T) {
	t.Parallel()

	statsBuilder := frame.NewBuilder("stats").Number("value", frame.RoleMetric)
	err := statsBuilder.Append(frame.Row{"value": 7})
	require.NoError(t, err)
	stats, err := statsBuilder.FrameSet()
	require.NoError(t, err)

	doc := lensspec.Document{
		Version:     lensspec.DocumentVersion,
		ID:          "manual-report",
		Title:       lensspec.LiteralText("Manual"),
		Description: lensspec.LiteralText("Static"),
		Datasets: []lensspec.DatasetSpec{
			{Name: "stats", Kind: lens.DatasetKindStatic, StaticRef: "stats_dataset"},
		},
		Rows: []lensspec.RowSpec{
			{
				Panels: []lensspec.PanelSpec{
					{
						ID:      "total",
						Title:   lensspec.LiteralText("Total"),
						Kind:    panel.KindStat,
						Dataset: "stats",
						Span:    12,
						Fields:  lensspec.FieldMappingSpec{Value: "value"},
					},
				},
			},
		},
	}

	compiled, err := Document(doc, Options{
		Locale: "en",
		Values: map[string]any{"stats_dataset": stats},
	})
	require.NoError(t, err)
	require.Nil(t, compiled.Semantic)
	require.Len(t, compiled.Spec.Datasets, 1)
	require.Len(t, compiled.Spec.Rows, 1)
	require.Equal(t, "stats", compiled.Spec.Datasets[0].Name)
	require.Equal(t, "total", compiled.Spec.Rows[0].Panels[0].ID)
}

func TestCompilePanelPreservesDrillTree(t *testing.T) {
	t.Parallel()

	tree := panel.DrillTree{Branches: []panel.DrillBranch{{
		TriggerKey: "earned",
		Label:      "Earned",
		Children:   []panel.DrillNode{{Key: "direct", Label: "Direct", Value: 100}},
	}}}
	item := lensspec.Pie("premium", "Premium", "premium_dataset").
		IDField("metric_key").
		DrillTree(tree).
		Build()

	compiled, err := compilePanel(item, Options{})
	require.NoError(t, err)
	require.Equal(t, panel.Ref("metric_key"), compiled.Fields.ID)
	require.Equal(t, &tree, compiled.DrillTree)
}

func TestCompilePanelPreservesRadialContract(t *testing.T) {
	t.Parallel()

	rings := []panel.RadialRing{
		{Key: "actual", Label: "Actual", Order: 1, Total: 100},
		{Key: "plan", Label: "Plan", Order: 2, Total: 100},
	}
	item := lensspec.MultiRingDonut("mix", "Mix", "mix_dataset", rings...).
		IDField("category_key").
		SeriesField("ring_key").
		RadialTolerance(0.01).
		Build()

	compiled, err := compilePanel(item, Options{})
	require.NoError(t, err)
	require.Equal(t, panel.KindRadial, compiled.Kind)
	require.Equal(t, panel.Ref("category_key"), compiled.Fields.ID)
	require.Equal(t, panel.Ref("ring_key"), compiled.Fields.Series)
	require.Equal(t, &panel.RadialSpec{
		Mode: panel.RadialPartition, Rings: rings, Tolerance: 0.01,
	}, compiled.Radial)
}

// Presentation hints and the rich table-column treatments are producer-side
// contract: a document that declares them must reach panel.Spec unchanged, or
// the wire document builder can never see them.
func TestCompilePanelPreservesPresentationAndColumnTreatments(t *testing.T) {
	t.Parallel()

	hints := panel.PresentationHints{
		LegendBelow: true, SliceLabelsPercent: true, TotalBadgeInPlot: true,
		FillPlot: true, BarWidthPx: 55, ColorByCategory: true,
	}
	item := lensspec.Table("products", "Products", "products_dataset").
		Presentation(hints).
		Columns(
			lensspec.Column("product", "Product").Clamp(2),
			lensspec.Column("result", "Result").Underline(),
			lensspec.Column("delta", "Delta").Delta("delta_pct").Stacked(),
			lensspec.Column("premium", "Premium").Pill(),
		).
		Build()

	compiled, err := compilePanel(item, Options{})
	require.NoError(t, err)
	require.Equal(t, hints, compiled.Presentation)
	require.Equal(t, 2, compiled.Columns[0].ClampLines)
	require.Equal(t, panel.TableCellUnderline, compiled.Columns[1].Cell.Kind)
	require.True(t, compiled.Columns[2].Cell.Stacked)
	require.Equal(t, panel.Ref("delta_pct"), compiled.Columns[2].Cell.PercentField)
	require.Equal(t, "pill", compiled.Columns[3].Affordance)
}

// TestCompilePanelRoundTripsMetricKinds pins the compilePanel contract for the
// three metric panel kinds: every new field (stages/rows/relationship, the
// reconcile tolerances, panel-level quality defaults, and the share/confidence/
// availability field roles) must reach panel.Spec unchanged.
func TestCompilePanelRoundTripsMetricKinds(t *testing.T) {
	t.Parallel()

	flowAction := action.Navigate("/flow/{key}", action.FieldParam("key", "id"))
	flowItem := lensspec.MetricFlow("flow", "Flow", "flow_dataset",
		panel.FlowStage{Key: "start", Label: "Start", Role: panel.FlowRoleInput},
		panel.FlowStage{
			Key: "add", Label: "Add", Role: panel.FlowRoleAdd, Caption: "note",
			Confidence: panel.ConfidenceCalculated, Availability: panel.AvailabilityAvailable,
			Action: &flowAction,
		},
		panel.FlowStage{Key: "end", Label: "End", Role: panel.FlowRoleResult},
	).
		FlowReconcile(0.5).
		Confidence(panel.ConfidenceVerified).
		Availability(panel.AvailabilityAvailable).
		ShareField("share_col").
		ConfidenceField("confidence_col").
		AvailabilityField("availability_col").
		Build()

	compiledFlow, err := compilePanel(flowItem, Options{})
	require.NoError(t, err)
	require.Equal(t, flowItem.FlowStages, compiledFlow.FlowStages)
	require.Equal(t, flowItem.FlowReconcile, compiledFlow.FlowReconcile)
	require.Equal(t, panel.ConfidenceVerified, compiledFlow.Confidence)
	require.Equal(t, panel.AvailabilityAvailable, compiledFlow.Availability)
	require.Equal(t, panel.Ref("share_col"), compiledFlow.Fields.Share)
	require.Equal(t, panel.Ref("confidence_col"), compiledFlow.Fields.Confidence)
	require.Equal(t, panel.Ref("availability_col"), compiledFlow.Fields.Availability)

	hierarchyItem := lensspec.MetricHierarchy("hierarchy", "Hierarchy", "hierarchy_dataset",
		panel.HierarchyRow{Key: "root", Label: "Root"},
		panel.HierarchyRow{Key: "child", Label: "Child", Parent: "root", Unallocated: true},
	).
		HierarchyReconcile(1.5).
		Build()

	compiledHierarchy, err := compilePanel(hierarchyItem, Options{})
	require.NoError(t, err)
	require.Equal(t, hierarchyItem.HierarchyRows, compiledHierarchy.HierarchyRows)
	require.Equal(t, hierarchyItem.HierarchyReconcile, compiledHierarchy.HierarchyReconcile)

	relationshipItem := lensspec.MetricRelationship("relationship", "Relationship", "relationship_dataset", panel.RelationshipSpec{
		Source: panel.RelationshipEnd{Key: "a", Label: "A"},
		Target: panel.RelationshipEnd{Key: "b", Label: "B"},
		Type:   panel.RelationshipAssociation,
	}).Build()

	compiledRelationship, err := compilePanel(relationshipItem, Options{})
	require.NoError(t, err)
	require.Equal(t, relationshipItem.Relationship, compiledRelationship.Relationship)
}

// A stat_group document compiles through to a validated dashboard spec: the
// group itself carries no dataset, children keep theirs, and the new stat v2
// fields (status, sparkline, trend.invert, groupLayout) pass through 1:1.
func TestDocumentCompilesStatGroupWithStatV2Fields(t *testing.T) {
	t.Parallel()

	statsBuilder := frame.NewBuilder("stats").Number("value", frame.RoleMetric)
	err := statsBuilder.Append(frame.Row{"value": 7})
	require.NoError(t, err)
	stats, err := statsBuilder.FrameSet()
	require.NoError(t, err)

	child := lensspec.Stat("kpi-a", "Premium", "stats").
		Status("ON TRACK", panel.StatusPositive).
		SparklineColored([]float64{1, 2, 3}, "#2563eb").
		TrendWithInvert(-4.2, "vs LY", true).
		Target(100, "Plan").
		Build()
	group := lensspec.StatGroup("kpi-group", "KPIs", child).
		Layout(panel.GroupRows).
		Build()

	doc := lensspec.Document{
		Version: lensspec.DocumentVersion,
		ID:      "stat-group-report",
		Title:   lensspec.LiteralText("Stat group"),
		Datasets: []lensspec.DatasetSpec{
			{Name: "stats", Kind: lens.DatasetKindStatic, StaticRef: "stats_dataset"},
		},
		Rows: []lensspec.RowSpec{{Panels: []lensspec.PanelSpec{group}}},
	}

	compiled, err := Document(doc, Options{
		Locale: "en",
		Values: map[string]any{"stats_dataset": stats},
	})
	require.NoError(t, err)
	require.Len(t, compiled.Spec.Rows, 1)

	compiledGroup := compiled.Spec.Rows[0].Panels[0]
	require.Equal(t, panel.KindStatGroup, compiledGroup.Kind)
	require.True(t, compiledGroup.Kind.IsContainer())
	require.Equal(t, panel.GroupRows, compiledGroup.GroupLayout)
	require.Empty(t, compiledGroup.Dataset)
	require.Len(t, compiledGroup.Children, 1)

	compiledChild := compiledGroup.Children[0]
	require.Equal(t, "stats", compiledChild.Dataset)
	require.NotNil(t, compiledChild.Status)
	require.Equal(t, "ON TRACK", compiledChild.Status.Label)
	require.Equal(t, panel.StatusPositive, compiledChild.Status.Tone)
	require.NotNil(t, compiledChild.Sparkline)
	require.Equal(t, []float64{1, 2, 3}, compiledChild.Sparkline.Values)
	require.Equal(t, "#2563eb", compiledChild.Sparkline.Color)
	require.Equal(t, &panel.TargetSpec{Value: 100, Label: "Plan"}, compiledChild.Target)
	require.NotNil(t, compiledChild.Trend)
	require.True(t, compiledChild.Trend.Invert)
	require.InDelta(t, -4.2, compiledChild.Trend.Percent, 0.0001)

	// Runtime validation treats the group as a container and its children as
	// stat leaves (dataset required on children, none on the group).
	require.NoError(t, runtime.Validate(compiled.Spec))
}

// A stat_group child without a dataset fails validation like any stat leaf.
func TestValidateRejectsStatGroupChildWithoutDataset(t *testing.T) {
	t.Parallel()

	group := panel.StatGroup("kpi-group", "KPIs",
		panel.Spec{ID: "kpi-a", Title: "A", Kind: panel.KindStat, Fields: panel.FieldMapping{Value: panel.DefaultValueField}},
	).Build()

	err := runtime.Validate(lens.DashboardSpec{
		ID:    "d",
		Title: "d",
		Rows:  []lens.RowSpec{{Panels: []panel.Spec{group}}},
	})
	require.ErrorContains(t, err, "kpi-a is missing dataset")
}

func TestDocumentRejectsHeadingRowWithPanels(t *testing.T) {
	t.Parallel()

	doc := lensspec.Document{
		Version: lensspec.DocumentVersion,
		ID:      "manual-report",
		Title:   lensspec.LiteralText("Manual"),
		Rows: []lensspec.RowSpec{
			{
				Heading: lensspec.LiteralText("Summary"),
				Panels: []lensspec.PanelSpec{
					{
						ID:   "total",
						Kind: panel.KindStat,
					},
				},
			},
		},
	}

	_, err := Document(doc, Options{Locale: "en"})
	require.Error(t, err)
	require.ErrorContains(t, err, `row heading "Summary" cannot be combined with panels`)
}

// A row anchor is what makes a section addressable from elsewhere on the page,
// so it has to survive the compile step that everything else about the row
// goes through.
func TestDocument_RowAnchorIsPreserved(t *testing.T) {
	t.Parallel()

	doc := lensspec.Document{
		Version: lensspec.DocumentVersion,
		ID:      "manual-report",
		Title:   lensspec.LiteralText("Manual"),
		Rows: []lensspec.RowSpec{
			{Heading: lensspec.LiteralText("Result"), Anchor: "manual-report-result"},
			{Panels: []lensspec.PanelSpec{{ID: "total", Kind: panel.KindStat}}},
		},
	}

	compiled, err := Document(doc, Options{Locale: "en"})
	require.NoError(t, err)
	require.Len(t, compiled.Spec.Rows, 2)
	require.Equal(t, "manual-report-result", compiled.Spec.Rows[0].Anchor)
	require.Empty(t, compiled.Spec.Rows[1].Anchor)
}

func TestDocumentTreatsBlankHeadingAsPanelRow(t *testing.T) {
	t.Parallel()

	doc := lensspec.Document{
		Version: lensspec.DocumentVersion,
		ID:      "manual-report",
		Title:   lensspec.LiteralText("Manual"),
		Rows: []lensspec.RowSpec{
			{
				Heading: lensspec.LiteralText("   "),
				Panels: []lensspec.PanelSpec{
					{
						ID:   "total",
						Kind: panel.KindStat,
					},
				},
			},
		},
	}

	compiled, err := Document(doc, Options{Locale: "en"})
	require.NoError(t, err)
	require.Len(t, compiled.Spec.Rows, 1)
	require.Empty(t, compiled.Spec.Rows[0].Heading)
	require.Len(t, compiled.Spec.Rows[0].Panels, 1)
}

func TestResolveTransformSpecsFailsWhenFillValueRefCannotBeResolved(t *testing.T) {
	t.Parallel()

	_, err := resolveTransformSpecs([]transform.Spec{
		{
			FillMissing: &transform.FillMissingConfig{
				FillValue: map[string]any{"$ref": "missing_value"},
			},
		},
	}, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing_value")
}

func TestResolveTransformSpecsFailsWhenPredicateRefCannotBeResolved(t *testing.T) {
	t.Parallel()

	_, err := resolveTransformSpecs([]transform.Spec{
		{
			Predicates: []transform.Predicate{
				{Field: "status", Op: "=", Value: map[string]any{"$ref": "missing_status"}},
			},
		},
	}, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing_status")
}

func TestDocumentCompilesVariableComponentOverride(t *testing.T) {
	t.Parallel()

	doc := lensspec.Document{
		Version: lensspec.DocumentVersion,
		ID:      "filter-components",
		Title:   lensspec.LiteralText("Filter Components"),
		Variables: []lensspec.VariableSpec{
			{
				Name:      "product",
				Label:     lensspec.LiteralText("Product"),
				Kind:      lens.VariableSingleSelect,
				Component: string(lens.VariableComponentTextInput),
			},
		},
		Rows: []lensspec.RowSpec{},
	}

	compiled, err := Document(doc, Options{Locale: "en"})
	require.NoError(t, err)
	require.Len(t, compiled.Spec.Variables, 1)
	require.Equal(t, lens.VariableComponentTextInput, compiled.Spec.Variables[0].Component)
}
