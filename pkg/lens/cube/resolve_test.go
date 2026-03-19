package cube

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
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
	require.Equal(t, "osago", resolved.Datasets[0].Query.Params["f_product"].Literal)
	require.Contains(t, resolved.Datasets[0].Query.Params, "f_age_group")
	require.Nil(t, resolved.Datasets[0].Query.Params["f_age_group"].Literal)
	require.Equal(t, "value", resolved.Datasets[0].Query.Params["custom"].Literal)
}

func TestBuildDimensionPanelUsesLeafURLForTerminalDrill(t *testing.T) {
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
	require.Equal(t, action.KindCubeDrill, panelSpec.Action.Kind)
	require.Equal(t, "/crm/reports/sales/drill/policies", panelSpec.Action.URL)
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

	panels := buildStatPanels(spec, "cube_stats")
	require.Len(t, panels, 1)
	require.NotNil(t, panels[0].Action)
	require.Equal(t, action.KindNavigate, panels[0].Action.Kind)
	require.Equal(t, "/crm/reports/sales/drill/policies", panels[0].Action.URL)
	require.True(t, panels[0].Action.PreserveQuery)
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

func TestBuildDimensionPanelUsesFilterValueWhenColorValueIsUnavailable(t *testing.T) {
	t.Parallel()

	spec := New("insurance-sales", "Sales").
		Dataset(nil).
		Dimension("agency", "Agency").
		Field("agency_id").
		ColorScale("AGENCY").
		Measure("total_policies", "Total Policies").
		Count().
		Build()

	panelSpec := buildDimensionPanel(spec, spec.Dimensions[0], dimensionDatasetResolution{Name: "cube_dim_agency"}, "/crm/reports/sales", 1, 0)
	require.Equal(t, panel.Ref("filter_value"), panelSpec.ColorField)
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
		ColorScale("AGENCY").
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
