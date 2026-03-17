package cube

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
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
	require.Equal(t, "primary", resolved.Source)
	require.NotNil(t, resolved.Query)
	require.Equal(t, "tenant-1", resolved.Query.Params["tenant_id"].Literal)
	require.Equal(t, "osago", resolved.Query.Params["f_product"].Literal)
	require.Contains(t, resolved.Query.Params, "f_age_group")
	require.Nil(t, resolved.Query.Params["f_age_group"].Literal)
	require.Equal(t, "value", resolved.Query.Params["custom"].Literal)
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
