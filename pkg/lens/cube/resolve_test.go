package cube

import (
	"net/url"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	lensbuild "github.com/iota-uz/iota-sdk/pkg/lens/build"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
	"github.com/stretchr/testify/require"
)

func TestResolveOverrideDatasetInheritsCubeParamsAndFilters(t *testing.T) {
	t.Parallel()

	spec := New("insurance-sales", "Sales").
		SQL("primary", "insurance.contracts c").
		ParamLiteral("tenant_id", "tenant-1").
		Dimension("product", "Product").
		Column("c.product_id::text").
		Dimension("age_group", "Age Group").
		PanelKind(panel.KindHorizontalBar).
		Override(lens.DatasetSpec{
			Kind: lens.DatasetKindQuery,
			Query: &lens.QuerySpec{
				Text: "SELECT @tenant_id, @f_product, @f_age_group",
				Kind: datasource.QueryKindRaw,
				Params: map[string]lens.ParamValue{
					"custom": {Literal: "value"},
				},
			},
		}).
		Measure("total_policies", "Total Policies").
		Column("DISTINCT c.id").
		Count().
		DefaultDimension("product").
		Build()

	resolved, err := resolveDimensionDataset(spec, DrillContext{
		Filters: []DimensionFilter{{Dimension: "product", Value: "osago"}},
	}, spec.Dimensions[1])
	require.NoError(t, err)
	require.Equal(t, "cube_dim_age_group", resolved.Name)
	require.Len(t, resolved.Datasets, 1)
	require.Equal(t, "primary", resolved.Datasets[0].Source)
	require.NotNil(t, resolved.Datasets[0].Query)
	require.Equal(t, "tenant-1", resolved.Datasets[0].Query.Params["tenant_id"].Literal)
	require.Equal(t, []string{"osago"}, resolved.Datasets[0].Query.Params["f_product"].Literal)
	require.Contains(t, resolved.Datasets[0].Query.Params, "f_age_group")
	require.Nil(t, resolved.Datasets[0].Query.Params["f_age_group"].Literal)
	require.Equal(t, "value", resolved.Datasets[0].Query.Params["custom"].Literal)
}

func TestResolveMeasureOverrideDatasetInheritsCubeParamsAndFilters(t *testing.T) {
	t.Parallel()

	spec := New("insurance-sales", "Sales").
		SQL("primary", "insurance.contracts c").
		ParamLiteral("tenant_id", "tenant-1").
		Dimension("product", "Product").
		Column("c.product_id::text").
		Measure("total_policies", "Total Policies").
		Column("DISTINCT c.id").
		Count().
		Override(lens.DatasetSpec{
			Kind: lens.DatasetKindQuery,
			Query: &lens.QuerySpec{
				Text: "SELECT @tenant_id, @f_product",
				Kind: datasource.QueryKindRaw,
				Params: map[string]lens.ParamValue{
					"custom": {Literal: "value"},
				},
			},
		}).
		DefaultDimension("product").
		Build()

	resolved, err := resolveStatDatasets(spec, DrillContext{
		Filters: []DimensionFilter{{Dimension: "product", Value: "osago"}},
	})
	require.NoError(t, err)
	require.Len(t, resolved.Datasets, 1)
	require.Equal(t, "cube_stat_total_policies", resolved.Datasets[0].Name)
	require.Equal(t, "primary", resolved.Datasets[0].Source)
	require.NotNil(t, resolved.Datasets[0].Query)
	require.Equal(t, "tenant-1", resolved.Datasets[0].Query.Params["tenant_id"].Literal)
	require.Equal(t, []string{"osago"}, resolved.Datasets[0].Query.Params["f_product"].Literal)
	require.Equal(t, "value", resolved.Datasets[0].Query.Params["custom"].Literal)
	require.Equal(t, "cube_stat_total_policies", resolved.DatasetByMeasure["total_policies"])
}

func TestResolveMeasureOverrideBacksStatPanelOnly(t *testing.T) {
	t.Parallel()

	base, err := frame.FromRows("policies",
		frame.Row{"product": "OSAGO"},
		frame.Row{"product": "KASKO"},
	)
	require.NoError(t, err)
	exact, err := frame.FromRows("exact_total", frame.Row{"total_policies": 72451})
	require.NoError(t, err)

	spec := New("crm-sales", "Sales").
		Dataset(base).
		Dimension("product", "Product").
		Field("product").
		Measure("total_policies", "Total Policies").
		Count().
		Override(lens.DatasetSpec{
			Kind:   lens.DatasetKindStatic,
			Static: exact,
		}).
		DefaultDimension("product").
		Build()

	dashboard, err := Resolve(spec, DrillContext{}, "/crm/reports/sales")
	require.NoError(t, err)
	require.Len(t, dashboard.Rows, 2)
	strip := dashboard.Rows[0].Panels[0]
	require.Equal(t, panel.KindStatGroup, strip.Kind)
	require.Len(t, strip.Children, 1)
	require.Equal(t, "cube_stat_total_policies", strip.Children[0].Dataset)
	require.Equal(t, "total_policies", strip.Children[0].Fields.Value.Name())
	require.Equal(t, "cube_dim_product", dashboard.Rows[1].Panels[0].Dataset)

	names := make([]string, 0, len(dashboard.Datasets))
	for _, dataset := range dashboard.Datasets {
		names = append(names, dataset.Name)
	}
	require.Contains(t, names, "cube_base")
	require.Contains(t, names, "cube_stat_total_policies")
	require.Contains(t, names, "cube_dim_product")
	require.NotContains(t, names, "cube_stats")
}

func TestBuildDimensionPanelUsesBaseURLForFacetToggle(t *testing.T) {
	t.Parallel()

	spec := New("crm-sales", "Sales").
		Dataset(nil).
		Dimension("payment_method", "Payment Method").
		Field("payment_method").
		Leaf("/crm/reports/sales/drill/policies").
		Measure("total_policies", "Total Policies").
		Count().
		Build()

	panelSpec := buildDimensionPanel(spec, spec.Dimensions[0], dimensionDatasetResolution{Name: "cube_dim_payment_method"}, "/crm/reports/sales", 1, 0)
	require.NotNil(t, panelSpec.Action)
	require.Equal(t, action.KindCrossFilter, panelSpec.Action.Kind)
	require.Equal(t, "/crm/reports/sales", panelSpec.Action.URL)
}

func TestBuildDimensionPanelCarriesChoroplethConfig(t *testing.T) {
	t.Parallel()

	spec := New("regional-sales", "Regional sales").
		Dataset(nil).
		Dimension("region", "Region").
		Field("region_code").
		Choropleth(
			panel.GeoJSONSource{URL: "/maps/regions.geojson", MaxBytes: 200_000},
			"shapeISO",
			"shapeName",
		).MapAttribution("© Example Maps").
		Measure("premium", "Premium").
		Field("premium").
		Sum().
		Build()
	dim := spec.Dimensions[0]

	got := buildDimensionPanel(
		spec,
		dim,
		dimensionDatasetResolution{Name: "cube_dim_region"},
		"/sales",
		1,
		0,
	)

	require.Equal(t, panel.KindMap, got.Kind)
	require.Equal(t, panel.Ref("filter_value"), got.Fields.ID)
	require.Equal(t, "/maps/regions.geojson", got.Map.Source.URL)
	require.Equal(t, "shapeISO", got.Map.FeatureProperty)
	require.Equal(t, "shapeName", got.Map.LabelProperty)
	require.Equal(t, "© Example Maps", got.Map.Attribution)
	require.Equal(t, action.KindCrossFilter, got.Action.Kind)
}

func TestBuildStatPanelsPreserveMeasureAction(t *testing.T) {
	t.Parallel()

	measureAction := action.Navigate("/crm/reports/sales/drill/policies").WithPreservedQuery()
	spec := New("crm-sales", "Sales").
		Dataset(nil).
		Dimension("payment_method", "Payment Method").
		Field("payment_method").
		Measure("total_policies", "Total Policies").
		Count().
		Action(measureAction).
		Build()

	panels := buildStatPanelsCompared(spec, map[string]string{"total_policies": "cube_stats"}, false)
	require.Len(t, panels, 1)
	require.NotNil(t, panels[0].Action)
	require.Equal(t, action.KindNavigate, panels[0].Action.Kind)
	require.Equal(t, "/crm/reports/sales/drill/policies", panels[0].Action.URL)
	require.True(t, panels[0].Action.PreserveQuery)
}

func TestResolveComparisonBuildsPairedDatasetsAndDeltaPresentation(t *testing.T) {
	t.Parallel()

	spec := New("sales", "Sales").
		SQL("primary", "policies p").
		ParamVariable("period_param", "period").
		Variable(lensbuild.DateRangeVariable("period", "Period", 30*24*time.Hour)).
		Variable(lensbuild.CompareVariable("compare", "Compare", "period")).
		Dimension("region", "Region").Column("p.region").
		Measure("premium", "Premium").Column("p.premium").Sum().InvertTrend().
		Build()

	compared, err := Resolve(spec, ParseDrillContext(url.Values{"compare": []string{"previous_period"}}), "/sales")
	require.NoError(t, err)
	datasets := make(map[string]lens.DatasetSpec, len(compared.Datasets))
	for _, dataset := range compared.Datasets {
		datasets[dataset.Name] = dataset
	}
	require.Contains(t, datasets, "cube_stats_comparison")
	require.Contains(t, datasets, "cube_stats_compared")
	require.Equal(t, "compare", datasets["cube_stats_comparison"].TimeRangeVariable)
	require.Equal(t, []string{"cube_stats", "cube_stats_comparison"}, datasets["cube_stats_compared"].DependsOn)
	require.Contains(t, datasets, "cube_dim_region_comparison")
	require.Contains(t, datasets, "cube_dim_region_compared")
	statTransforms := datasets["cube_stats_compared"].Transforms
	require.True(t, statTransforms[2].Formula.AbsoluteRight, "percent change must use the absolute baseline")

	require.Len(t, compared.Rows, 2)
	stat := compared.Rows[0].Panels[0].Children[0]
	require.Equal(t, panel.Ref("premium_delta"), stat.Trend.AbsoluteField)
	require.Equal(t, panel.Ref("premium_delta_percent"), stat.Trend.PercentField)
	require.True(t, stat.Trend.Invert)
	require.True(t, stat.Terminal)
	chart := compared.Rows[1].Panels[0]
	require.Equal(t, panel.Ref("previous_premium"), chart.Fields.Previous)
	require.Equal(t, action.KindCrossFilter, chart.Action.Kind)

	off, err := Resolve(spec, ParseDrillContext(url.Values{}), "/sales")
	require.NoError(t, err)
	for _, dataset := range off.Datasets {
		require.NotContains(t, dataset.Name, comparisonSuffix)
		require.NotContains(t, dataset.Name, "_compared")
	}
	require.Nil(t, off.Rows[0].Panels[0].Children[0].Trend)
}

func TestResolveComparisonRanksTopNFromPrimaryPeriod(t *testing.T) {
	t.Parallel()

	spec := New("sales", "Sales").
		SQL("primary", "policies p").
		ParamVariable("period_param", "period").
		Variable(lensbuild.DateRangeVariable("period", "Period", 30*24*time.Hour)).
		Variable(lensbuild.CompareVariable("compare", "Compare", "period")).
		Dimension("region", "Region").Column("p.region").
		Transforms(transform.TopN("premium", 2, "Other")).
		Measure("premium", "Premium").Column("p.premium").Sum().
		Build()

	compared, err := Resolve(spec, ParseDrillContext(url.Values{"compare": []string{"previous_period"}}), "/sales")
	require.NoError(t, err)
	datasets := make(map[string]lens.DatasetSpec, len(compared.Datasets))
	for _, dataset := range compared.Datasets {
		datasets[dataset.Name] = dataset
	}
	comparison := datasets["cube_dim_region_comparison"]
	require.Contains(t, comparison.DependsOn, "cube_dim_region")
	require.Len(t, comparison.Transforms, 2)
	topN := comparison.Transforms[0].TopN
	require.NotNil(t, topN)
	require.Equal(t, "cube_dim_region", topN.RankByDataset)
	require.Equal(t, []string{"filter_value", "label"}, topN.KeyFields)
}

func TestCubeRejectsDimensionKindsWhoseRequiredShapeItCannotProduce(t *testing.T) {
	t.Parallel()
	for _, kind := range []panel.Kind{panel.KindBoxPlot, panel.KindHeatmap} {
		spec := New("sales", "Sales").Dataset(nil).
			Dimension("region", "Region").Field("region").PanelKind(kind).
			Measure("premium", "Premium").Field("premium").Sum().Build()
		err := spec.Validate()
		require.ErrorContains(t, err, "cube dimensions do not provide its required fields")
	}
}

func TestResolveComparisonRejectsMalformedCustomRangeAndUsesDeclaredRequestKeys(t *testing.T) {
	t.Parallel()

	comparison := lensbuild.CompareVariable("compare", "Compare", "period")
	comparison.RequestKeys = []string{"comparison_mode", "baseline_from", "baseline_to"}
	spec := New("sales", "Sales").
		SQL("primary", "policies p").
		ParamVariable("period_param", "period").
		Variable(lensbuild.DateRangeVariable("period", "Period", 30*24*time.Hour)).
		Variable(comparison).
		Dimension("region", "Region").Column("p.region").
		Measure("premium", "Premium").Column("p.premium").Sum().
		Build()

	for name, values := range map[string]url.Values{
		"missing boundaries": {"comparison_mode": []string{"custom"}},
		"invalid start":      {"comparison_mode": []string{"custom"}, "baseline_from": []string{"not-a-date"}, "baseline_to": []string{"2026-02-28"}},
		"reversed range":     {"comparison_mode": []string{"custom"}, "baseline_from": []string{"2026-03-01"}, "baseline_to": []string{"2026-02-28"}},
	} {
		t.Run(name, func(t *testing.T) {
			resolved, err := Resolve(spec, ParseDrillContext(values), "/sales")
			require.NoError(t, err)
			for _, dataset := range resolved.Datasets {
				require.NotContains(t, dataset.Name, comparisonSuffix)
				require.NotContains(t, dataset.Name, "_compared")
			}
		})
	}

	valid, err := Resolve(spec, ParseDrillContext(url.Values{
		"comparison_mode": []string{"custom"},
		"baseline_from":   []string{"2026-02-01"},
		"baseline_to":     []string{"2026-02-28"},
	}), "/sales")
	require.NoError(t, err)
	require.Contains(t, datasetNames(valid.Datasets), "cube_stats_comparison")
}

func datasetNames(datasets []lens.DatasetSpec) map[string]struct{} {
	names := make(map[string]struct{}, len(datasets))
	for _, dataset := range datasets {
		names[dataset.Name] = struct{}{}
	}
	return names
}

func TestBuildComparedTableShowsBeforeAfterAndDelta(t *testing.T) {
	t.Parallel()

	spec := New("sales", "Sales").Dataset(nil).
		Dimension("region", "Region").Field("region").PanelKind(panel.KindTable).
		Measure("premium", "Premium").Field("premium").Sum().Build()
	dim := spec.Dimensions[0]
	got := buildDimensionPanel(spec, dim, dimensionDatasetResolution{
		Name: "cube_dim_region_compared", Compared: true,
	}, "/sales", 1, 0)

	require.Equal(t, panel.KindTable, got.Kind)
	require.Equal(t, []string{"Before", "After", "Δ"}, []string{
		got.Columns[1].Label, got.Columns[2].Label, got.Columns[3].Label,
	})
	require.Equal(t, panel.TableCellDelta, got.Columns[3].Cell.Kind)
	require.Equal(t, panel.Ref("premium_delta_percent"), got.Columns[3].Cell.PercentField)
}

func TestResolveDimensionDataset_TransformResolutionScenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spec     CubeSpec
		assertFn func(t *testing.T, resolved dimensionDatasetResolution)
	}{
		{
			name: "sql dimensions wrap transforms with additive metadata",
			spec: New("insurance-sales", "Sales").
				SQL("primary", "insurance.contracts c").
				Dimension("agency", "Agency").
				Column("COALESCE(c.agency_id::text, '')").
				Transforms(transform.TopN("total_policies", 10, "Other")).
				Measure("total_policies", "Total Policies").
				Column("DISTINCT c.id").
				Count().
				Build(),
			assertFn: func(t *testing.T, resolved dimensionDatasetResolution) {
				t.Helper()
				require.Equal(t, "cube_dim_agency", resolved.Name)
				require.Len(t, resolved.Datasets, 2)
				require.Equal(t, lens.DatasetKindQuery, resolved.Datasets[0].Kind)
				require.Equal(t, "cube_dim_agency_source", resolved.Datasets[0].Name)
				require.Equal(t, lens.DatasetKindTransform, resolved.Datasets[1].Kind)
				require.Equal(t, "cube_dim_agency", resolved.Datasets[1].Name)
				require.Equal(t, []string{"cube_dim_agency_source"}, resolved.Datasets[1].DependsOn)
				require.Len(t, resolved.Datasets[1].Transforms, 1)
				require.Equal(t, "Other", resolved.Datasets[1].Transforms[0].TopN.Other)
				require.True(t, resolved.Datasets[1].Transforms[0].TopN.AdditiveFields["total_policies"])
			},
		},
		{
			name: "override dimensions wrap transforms",
			spec: New("insurance-sales", "Sales").
				SQL("primary", "insurance.contracts c").
				Dimension("agency", "Agency").
				Override(lens.DatasetSpec{
					Kind: lens.DatasetKindQuery,
					Query: &lens.QuerySpec{
						Text: "SELECT c.agency_id::text AS filter_value, c.agency_name AS label, COUNT(*) AS total_policies FROM insurance.contracts c GROUP BY 1, 2",
					},
				}).
				Transforms(transform.TopN("total_policies", 10, "Other")).
				Measure("total_policies", "Total Policies").
				Column("DISTINCT c.id").
				Count().
				Build(),
			assertFn: func(t *testing.T, resolved dimensionDatasetResolution) {
				t.Helper()
				require.Equal(t, "cube_dim_agency", resolved.Name)
				require.Len(t, resolved.Datasets, 2)
				require.Equal(t, "cube_dim_agency_source", resolved.Datasets[0].Name)
				require.Equal(t, lens.DatasetKindTransform, resolved.Datasets[1].Kind)
				require.Equal(t, []string{"cube_dim_agency_source"}, resolved.Datasets[1].DependsOn)
				require.Len(t, resolved.Datasets[1].Transforms, 1)
				require.Equal(t, "Other", resolved.Datasets[1].Transforms[0].TopN.Other)
				require.True(t, resolved.Datasets[1].Transforms[0].TopN.AdditiveFields["total_policies"])
			},
		},
		{
			name: "override dimensions carry color value availability",
			spec: New("insurance-sales", "Sales").
				SQL("primary", "insurance.contracts c").
				Dimension("agency", "Agency").
				Override(lens.DatasetSpec{
					Kind: lens.DatasetKindQuery,
					Query: &lens.QuerySpec{
						Text: "SELECT c.agency_id::text AS filter_value, c.agency_name AS label, c.brand_color AS color_value, COUNT(*) AS total_policies FROM insurance.contracts c GROUP BY 1, 2, 3",
					},
				}).
				ColorField("color_value").
				Transforms(transform.TopN("total_policies", 10, "Other")).
				Measure("total_policies", "Total Policies").
				Column("DISTINCT c.id").
				Count().
				Build(),
			assertFn: func(t *testing.T, resolved dimensionDatasetResolution) {
				t.Helper()
				require.True(t, resolved.HasColorValue)
				require.Len(t, resolved.Datasets, 2)
				require.Equal(t, "cube_dim_agency_source", resolved.Datasets[0].Name)
				require.Equal(t, lens.DatasetKindTransform, resolved.Datasets[1].Kind)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := resolveDimensionDataset(tc.spec, DrillContext{}, tc.spec.Dimensions[0])
			require.NoError(t, err)
			tc.assertFn(t, resolved)
		})
	}
}

func TestResolveDimensionDataset_DatasetColorFieldCollisionKeepsFilterValue(t *testing.T) {
	t.Parallel()

	data, err := frame.FromRows("contracts",
		frame.Row{"agency_name": "Alpha", "total_policies": 10.0},
		frame.Row{"agency_name": "Beta", "total_policies": 8.0},
	)
	require.NoError(t, err)

	spec := New("insurance-sales", "Sales").
		Dataset(data).
		Dimension("agency", "Agency").
		Field("agency_name").
		ColorField("agency_name").
		Transforms(transform.TopN("total_policies", 10, "Other")).
		Measure("total_policies", "Total Policies").
		Field("total_policies").
		Sum().
		Build()

	resolved, err := resolveDimensionDataset(spec, DrillContext{}, spec.Dimensions[0])
	require.NoError(t, err)
	require.False(t, resolved.HasColorValue)
	require.Len(t, resolved.Datasets, 1)
	require.NotEmpty(t, resolved.Datasets[0].Transforms)

	lookup := resolved.Datasets[0].Transforms[2].Lookup
	require.NotNil(t, lookup)
	require.Equal(t, map[string]string{"agency_name": "filter_value"}, lookup.Fields)
}
