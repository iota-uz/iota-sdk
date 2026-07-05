package compile

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
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
