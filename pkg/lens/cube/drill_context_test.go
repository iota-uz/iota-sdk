package cube

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDrillContextPreservesRepeatedFilterValues(t *testing.T) {
	t.Parallel()

	ctx := ParseDrillContext(url.Values{
		QueryFilter: []string{
			"region:tashkent,with-comma",
			"region:samarkand",
			"product:osago",
		},
		QueryGroupBy: []string{"region"},
	})

	require.Equal(t, []DimensionFilter{
		{Dimension: "region", Value: "tashkent,with-comma", Values: []string{"tashkent,with-comma", "samarkand"}},
		{Dimension: "product", Value: "osago", Values: []string{"osago"}},
	}, ctx.Filters)
	require.Equal(t, "region", ctx.GroupBy)

	encoded := ctx.Encode()
	require.Equal(t, []string{"region:tashkent,with-comma", "region:samarkand", "product:osago"}, encoded[QueryFilter])
	require.Equal(t, "region", encoded.Get(QueryGroupBy))
}

func TestDrillContextWithValuesPreservesPassthroughAndRepeatedFilters(t *testing.T) {
	t.Parallel()

	values := DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "region", Values: []string{"tashkent", "samarkand"}},
		},
		GroupBy: "product",
	}.WithValues(url.Values{
		"ActualRangeStart": []string{"2026-05-29"},
		"ActualRangeEnd":   []string{"2026-06-27"},
		QueryFilter:        []string{"region:old"},
		QueryFacet:         []string{"region"},
	})

	require.Equal(t, []string{"region:tashkent", "region:samarkand"}, values[QueryFilter])
	require.Equal(t, "product", values.Get(QueryGroupBy))
	require.Equal(t, "2026-05-29", values.Get("ActualRangeStart"))
	require.NotContains(t, values, QueryFacet)
}

func TestDrillContextIsLeafWhenAllDimensionsFiltered(t *testing.T) {
	t.Parallel()

	spec := CubeSpec{
		Dimensions: []DimensionSpec{
			{Name: "product"},
			{Name: "region"},
		},
	}

	require.False(t, DrillContext{}.IsLeaf(spec))
	require.False(t, DrillContext{
		Filters: []DimensionFilter{{Dimension: "product", Value: "osago"}},
	}.IsLeaf(spec))
	require.True(t, DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "product", Value: "osago"},
			{Dimension: "region", Value: "tashkent"},
		},
	}.IsLeaf(spec))
}
