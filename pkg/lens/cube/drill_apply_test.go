package cube

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyDrillFilters_CallsMatchingApplier(t *testing.T) {
	t.Parallel()

	spec := CubeSpec{
		ID: "test",
		Dimensions: []DimensionSpec{
			{Name: "product", Label: "Product", Column: "p.id"},
			{Name: "region", Label: "Region", Column: "r.id"},
		},
		Measures: []MeasureSpec{{Name: "count", Label: "Count", Column: "*", Aggregation: AggregationCount}},
	}
	ctx := DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "product", Value: "osago"},
			{Dimension: "region", Value: "tashkent"},
		},
	}

	var gotProduct, gotRegion string
	appliers := map[string]DrillApplier{
		"product": func(v string) { gotProduct = v },
		"region":  func(v string) { gotRegion = v },
	}

	ApplyDrillFilters(spec, ctx, appliers)

	assert.Equal(t, "osago", gotProduct)
	assert.Equal(t, "tashkent", gotRegion)
}

func TestApplyDrillFilters_SkipsUnknownDimension(t *testing.T) {
	t.Parallel()

	spec := CubeSpec{
		ID: "test",
		Dimensions: []DimensionSpec{
			{Name: "product", Label: "Product", Column: "p.id"},
		},
		Measures: []MeasureSpec{{Name: "count", Label: "Count", Column: "*", Aggregation: AggregationCount}},
	}
	ctx := DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "nonexistent", Value: "foo"},
		},
	}

	called := false
	appliers := map[string]DrillApplier{
		"nonexistent": func(v string) { called = true },
	}

	ApplyDrillFilters(spec, ctx, appliers)
	assert.False(t, called, "should not call applier for dimension not in spec")
}

func TestApplyDrillFilters_WarnsOnMissingApplier(t *testing.T) {
	t.Parallel()

	spec := CubeSpec{
		ID: "test",
		Dimensions: []DimensionSpec{
			{Name: "product", Label: "Product", Column: "p.id"},
			{Name: "region", Label: "Region", Column: "r.id"},
		},
		Measures: []MeasureSpec{{Name: "count", Label: "Count", Column: "*", Aggregation: AggregationCount}},
	}
	ctx := DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "product", Value: "osago"},
			{Dimension: "region", Value: "tashkent"},
		},
	}

	var gotProduct string
	appliers := map[string]DrillApplier{
		"product": func(v string) { gotProduct = v },
		// "region" intentionally missing — should warn but not panic
	}

	// Should not panic, should apply product, should skip region
	ApplyDrillFilters(spec, ctx, appliers)
	assert.Equal(t, "osago", gotProduct)
}

func TestApplyDrillFilters_EmptyFilters(t *testing.T) {
	t.Parallel()

	spec := CubeSpec{
		ID: "test",
		Dimensions: []DimensionSpec{
			{Name: "product", Label: "Product", Column: "p.id"},
		},
		Measures: []MeasureSpec{{Name: "count", Label: "Count", Column: "*", Aggregation: AggregationCount}},
	}
	ctx := DrillContext{}

	called := false
	appliers := map[string]DrillApplier{
		"product": func(v string) { called = true },
	}

	ApplyDrillFilters(spec, ctx, appliers)
	assert.False(t, called, "no filters means no appliers called")
}

func TestApplyDrillFilters_NilAppliers(t *testing.T) {
	t.Parallel()

	spec := CubeSpec{
		ID: "test",
		Dimensions: []DimensionSpec{
			{Name: "product", Label: "Product", Column: "p.id"},
		},
		Measures: []MeasureSpec{{Name: "count", Label: "Count", Column: "*", Aggregation: AggregationCount}},
	}
	ctx := DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "product", Value: "osago"},
		},
	}

	// nil appliers map — should not panic
	ApplyDrillFilters(spec, ctx, nil)
}

func TestApplyDrillFilters_MultipleValuesForSameDimension(t *testing.T) {
	t.Parallel()

	spec := CubeSpec{
		ID: "test",
		Dimensions: []DimensionSpec{
			{Name: "product", Label: "Product", Column: "p.id"},
		},
		Measures: []MeasureSpec{{Name: "count", Label: "Count", Column: "*", Aggregation: AggregationCount}},
	}
	ctx := DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "product", Value: "osago"},
			{Dimension: "product", Value: "kasko"},
		},
	}

	var values []string
	appliers := map[string]DrillApplier{
		"product": func(v string) { values = append(values, v) },
	}

	ApplyDrillFilters(spec, ctx, appliers)
	assert.Equal(t, []string{"osago", "kasko"}, values)
}
