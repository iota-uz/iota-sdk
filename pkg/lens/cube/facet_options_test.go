package cube

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/stretchr/testify/require"
)

func TestSQLFacetOptionsQueryPreservesParamValues(t *testing.T) {
	t.Parallel()

	spec := New("sales", "Sales").
		SQL("primary", "contracts c").
		ParamVariable("tenant_id", "tenant").
		Dimension("region", "Region").
		Column("c.region_id::text").
		LabelColumn("c.region_name").
		Measure("total", "Total").
		Column("*").
		Count().
		Build()

	text, params := sqlFacetOptionsQuery(spec, DrillContext{
		Filters: []DimensionFilter{{Dimension: "product", Value: "osago"}},
	}, spec.Dimensions[0], "tash")

	require.Contains(t, text, "c.region_id::text AS value")
	require.Equal(t, "tenant", params["tenant_id"].Variable)
	require.Equal(t, "%tash%", params["facet_search"].Literal)
}

func TestSQLFacetOptionsQueryUsesOverrideDataset(t *testing.T) {
	t.Parallel()

	spec := New("sales", "Sales").
		SQL("primary", "contracts c").
		ParamLiteral("tenant_id", "tenant-1").
		Dimension("age_group", "Age Group").
		Override(lens.DatasetSpec{
			Kind: lens.DatasetKindQuery,
			Query: &lens.QuerySpec{
				Text: "SELECT '30-34' AS filter_value, '30-34' AS label, COUNT(*) AS total FROM contracts c WHERE c.tenant_id = @tenant_id",
				Kind: datasource.QueryKindRaw,
			},
		}).
		Measure("total", "Total").
		Column("*").
		Count().
		Build()

	text, params := sqlFacetOptionsQuery(spec, DrillContext{}, spec.Dimensions[0], "30")

	require.Contains(t, text, "FROM (\nSELECT '30-34' AS filter_value")
	require.Contains(t, text, "total::int AS count")
	require.Equal(t, "tenant-1", params["tenant_id"].Literal)
	require.Equal(t, "%30%", params["facet_search"].Literal)
}

func TestResolveFacetOptionsPassesParamValuesToLookup(t *testing.T) {
	t.Parallel()

	spec := New("sales", "Sales").
		SQL("primary", "contracts c").
		ParamVariable("tenant_id", "tenant").
		Dimension("region", "Region").
		Column("c.region_id::text").
		Measure("total", "Total").
		Column("*").
		Count().
		Build()

	_, err := ResolveFacetOptions(context.Background(), spec, DrillContext{}, "region", "", 10,
		func(_ context.Context, _ string, params map[string]lens.ParamValue, _ int) ([]lens.DrillFacetOptionMeta, error) {
			require.Equal(t, "tenant", params["tenant_id"].Variable)
			return nil, nil
		},
	)
	require.NoError(t, err)
}
