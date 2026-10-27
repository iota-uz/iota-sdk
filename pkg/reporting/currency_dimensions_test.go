package reporting

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func monetaryDimensionDataset() Dataset {
	d := fixtureDataset("fixture")
	d.Dimensions = append(d.Dimensions,
		Field{Name: "limit_amount", Type: "number", CurrencyDimension: "limit_currency"},
		Field{Name: "limit_currency", Type: "string"},
	)
	return d
}

// Characterizes the new dimension binding; setting the currency in the plan
// manually would miss the compiler's mandatory currency insertion.
func TestMonetaryDimensionKeepsItsCurrencyWithoutBecomingAnAdditiveMeasure(t *testing.T) {
	for _, mode := range []string{"detail", "aggregate"} {
		t.Run(mode, func(t *testing.T) {
			catalog, err := NewCatalog(monetaryDimensionDataset())
			require.NoError(t, err)
			plan := fixturePlan()
			plan.Mode = mode
			plan.Dimensions = []Dimension{{Name: "limit_amount"}}
			compiled, err := catalog.Compile(plan)
			require.NoError(t, err)
			require.Equal(t, []Dimension{{Name: "limit_amount"}, {Name: "currency"}, {Name: "limit_currency"}}, compiled.Plan.Dimensions)
			require.Len(t, plan.Dimensions, 1)
			require.Len(t, compiled.Measures, 1)
			require.Equal(t, "limit_currency", compiled.Columns[0].CurrencyDimension)
			require.NotContains(t, compiled.SQL, `SUM("limit_amount")`)
			if mode == "aggregate" {
				require.Contains(t, compiled.SQL, `GROUP BY "limit_amount", "currency", "limit_currency"`)
			}
			encoded, err := json.Marshal(compiled.Definition)
			require.NoError(t, err)
			var definition Dataset
			require.NoError(t, json.Unmarshal(encoded, &definition))
			require.Equal(t, "limit_currency", definition.Dimensions[3].CurrencyDimension)
			definition.Dimensions[3].CurrencyDimension = ""
			original, err := catalog.Dataset("fixture")
			require.NoError(t, err)
			require.Equal(t, "limit_currency", original.Dimensions[3].CurrencyDimension)
			recompiled, err := catalog.Compile(compiled.Plan)
			require.NoError(t, err)
			require.Equal(t, compiled.Fingerprint, recompiled.Fingerprint)
		})
	}
}

func TestCatalogRejectsInvalidDimensionCurrencyBindings(t *testing.T) {
	// Accepting a reference to a measure or another numeric field would create
	// an unresolved or chained unit contract, even if the names look valid.
	for _, mutate := range []func(*Dataset){
		func(d *Dataset) { d.Dimensions[3].CurrencyDimension = "missing" },
		func(d *Dataset) { d.Dimensions[3].CurrencyDimension = "premium" },
		func(d *Dataset) { d.Dimensions[3].CurrencyDimension = "issued" },
		func(d *Dataset) { d.Dimensions[3].CurrencyDimension = "limit_amount" },
		func(d *Dataset) { d.Dimensions[4].Type = "number" },
		func(d *Dataset) { d.Dimensions[4].CurrencyDimension = "currency" },
		func(d *Dataset) { d.Dimensions[3].Type = "string" },
		func(d *Dataset) { d.Measures[0].Field.CurrencyDimension = "currency" },
	} {
		definition := monetaryDimensionDataset()
		mutate(&definition)
		_, err := NewCatalog(definition)
		require.ErrorIs(t, err, ErrDefinition)
	}
	catalog, err := NewCatalog(monetaryDimensionDataset())
	require.NoError(t, err)
	plan := fixturePlan()
	plan.Dimensions = []Dimension{{Name: "limit_amount"}, {Name: "limit_currency", Bucket: "month"}}
	_, err = catalog.Compile(plan)
	require.ErrorIs(t, err, ErrPlan)
}

func TestReportColumnBudgetIncludesAutomaticCurrenciesAndMeasures(t *testing.T) {
	// Checking only input dimensions could allow hidden currency expansion to
	// exceed the bounded output or could keep full-width portfolios unselectable.
	definition := monetaryDimensionDataset()
	definition.Measures[0].CurrencyDimension = ""
	for i := 0; i < 65; i++ {
		definition.Dimensions = append(definition.Dimensions, Field{Name: fmt.Sprintf("field_%02d", i), Type: "string"})
	}
	catalog, err := NewCatalog(definition)
	require.NoError(t, err)
	plan := fixturePlan()
	plan.Mode = "detail"
	plan.Dimensions = nil
	for i := 0; i < 40; i++ {
		plan.Dimensions = append(plan.Dimensions, Dimension{Name: fmt.Sprintf("field_%02d", i)})
	}
	compiled, err := catalog.Compile(plan)
	require.NoError(t, err)
	require.Len(t, compiled.Columns, 41)
	for i := 40; i < 63; i++ {
		plan.Dimensions = append(plan.Dimensions, Dimension{Name: fmt.Sprintf("field_%02d", i)})
	}
	compiled, err = catalog.Compile(plan)
	require.NoError(t, err)
	require.Len(t, compiled.Columns, MaxReportColumns)
	plan.Dimensions[0] = Dimension{Name: "limit_amount"}
	_, err = catalog.Compile(plan)
	require.ErrorIs(t, err, ErrPlan, "mandatory currency is inside the output budget")
	plan.Dimensions[0] = Dimension{Name: "field_00"}
	plan.Dimensions = append(plan.Dimensions, Dimension{Name: "field_63"})
	_, err = catalog.Compile(plan)
	require.ErrorIs(t, err, ErrPlan, "measures are inside the output budget")
}
