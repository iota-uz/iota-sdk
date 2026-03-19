package cube

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/stretchr/testify/require"
)

func TestMeasureBuilder_ActionClonesNestedState(t *testing.T) {
	t.Parallel()

	spec := action.Navigate("/crm/reports/sales").
		WithPreservedQuery()
	spec.Params = []action.Param{{
		Name: "product",
		Source: action.ValueSource{
			Kind: action.SourceField,
			Name: "product_id",
		},
	}}
	spec.Payload = map[string]action.ValueSource{
		"active_only": {
			Kind: action.SourceVariable,
			Name: "active_only",
		},
	}
	spec.Drill = &action.DrillSpec{
		Dimension: "product",
		Value:     action.FieldValue("filter_value"),
	}

	cubeSpec := New("crm-sales", "Sales").
		Dataset(nil).
		Dimension("product", "Product").
		Field("product_id").
		Measure("total_policies", "Total Policies").
		Count().
		Action(spec).
		Build()

	spec.Params[0].Name = "region"
	spec.Payload["active_only"] = action.ValueSource{Kind: action.SourceLiteral, Value: false}
	spec.Drill.Dimension = "region"

	stored := cubeSpec.Measures[0].Action
	require.NotNil(t, stored)
	require.Equal(t, "product", stored.Params[0].Name)
	require.Equal(t, "active_only", stored.Payload["active_only"].Name)
	require.Equal(t, action.SourceVariable, stored.Payload["active_only"].Kind)
	require.NotNil(t, stored.Drill)
	require.Equal(t, "product", stored.Drill.Dimension)
}
