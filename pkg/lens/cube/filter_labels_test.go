package cube

import (
	"context"
	"testing"

	lens "github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/stretchr/testify/require"
)

func TestResolveDrillFiltersUsesDatasetLabels(t *testing.T) {
	t.Parallel()

	builder := frame.NewBuilder("crm_policies").
		String("product_code", frame.RoleDimension).
		String("product_label", frame.RoleDimension).
		String("payment_method_code", frame.RoleDimension).
		String("payment_method_label", frame.RoleDimension).
		Number("total_policies", frame.RoleMetric)
	require.NoError(t, builder.Append(frame.Row{
		"product_code":         "osago",
		"product_label":        "OSAGO",
		"payment_method_code":  "cash",
		"payment_method_label": "Cash",
		"total_policies":       1.0,
	}))
	data, err := builder.FrameSet()
	require.NoError(t, err)

	spec := CubeSpec{
		ID:       "crm-sales-report",
		Title:    "CRM",
		DataMode: DataModeDataset,
		Data:     data,
		Dimensions: []DimensionSpec{
			{Name: "product", Label: "Product", Field: "product_code", LabelField: "product_label"},
			{Name: "payment_method", Label: "Payment Method", Field: "payment_method_code", LabelField: "payment_method_label"},
		},
		Measures: []MeasureSpec{
			{Name: "total_policies", Label: "Policies", Field: "total_policies"},
		},
	}

	filters, err := ResolveDrillFilters(context.Background(), spec, DrillContext{
		Filters: []DimensionFilter{
			{Dimension: "product", Value: "osago"},
			{Dimension: "payment_method", Value: "cash"},
		},
	}, nil)
	require.NoError(t, err)
	require.Equal(t, []lens.DrillFilterMeta{
		{Dimension: "product", Value: "osago", Display: "OSAGO"},
		{Dimension: "payment_method", Value: "cash", Display: "Cash"},
	}, filters)
}
