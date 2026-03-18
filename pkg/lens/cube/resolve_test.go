package cube

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
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

	panelSpec := buildDimensionPanel(spec, spec.Dimensions[0], "cube_dim_payment_method", "/crm/reports/sales", 1, 0)
	require.NotNil(t, panelSpec.Action)
	require.Equal(t, action.KindCubeDrill, panelSpec.Action.Kind)
	require.Equal(t, "/crm/reports/sales/drill/policies", panelSpec.Action.URL)
}

func TestResolveDimensionDatasetWrapsSQLDimensionsWithTransforms(t *testing.T) {
	t.Parallel()

	spec := New("insurance-sales", "Sales").
		SQL("primary", "insurance.contracts c").
		Dimension("agency", "Agency").
		Column("COALESCE(c.agency_id::text, '')").
		Transforms(transform.TopN("total_policies", 10, "Other")).
		Measure("total_policies", "Total Policies").
		Column("DISTINCT c.id").
		Count().
		Build()

	resolved, err := resolveDimensionDataset(spec, DrillContext{}, spec.Dimensions[0])
	require.NoError(t, err)
	require.Equal(t, "cube_dim_agency", resolved.Name)
	require.Len(t, resolved.Datasets, 2)
	require.Equal(t, lens.DatasetKindQuery, resolved.Datasets[0].Kind)
	require.Equal(t, "cube_dim_agency_source", resolved.Datasets[0].Name)
	require.Equal(t, lens.DatasetKindTransform, resolved.Datasets[1].Kind)
	require.Equal(t, "cube_dim_agency", resolved.Datasets[1].Name)
	require.Equal(t, []string{"cube_dim_agency_source"}, resolved.Datasets[1].DependsOn)
	require.Len(t, resolved.Datasets[1].Transforms, 1)
	require.Equal(t, "Other", resolved.Datasets[1].Transforms[0].TopN.Other)
}
