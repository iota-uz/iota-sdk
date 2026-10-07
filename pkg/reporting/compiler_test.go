package reporting

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func fixtureDataset(id string) Dataset {
	return Dataset{ID: id, Version: "v1", Description: "Fixture written premium", SourceSQL: "SELECT fixture", Grain: "one record", Population: "registered records", DateBasis: "issued", Timezone: "Asia/Tashkent", PeriodParameterType: "timestamp",
		Dimensions: []Field{{Name: "issued", Type: "date"}, {Name: "currency", Type: "string"}, {Name: "product", Type: "string"}},
		Measures:   []Measure{{Field: Field{Name: "premium", Type: "number"}, Aggregation: "sum", Unit: "major currency units", CurrencyDimension: "currency"}, {Field: Field{Name: "records", Type: "number"}, Aggregation: "count", Unit: "records"}},
	}
}
func fixturePlan() Plan {
	return Plan{Dataset: "fixture", Version: "v1", Mode: "aggregate", Period: Period{Start: "2026-01-01", End: "2026-07-01"}, Measures: []string{"premium"}, Dimensions: []Dimension{{Name: "issued", Bucket: "month"}}}
}

// False-green risk: asserting a manually supplied currency dimension hides a
// compiler that sums mixed currencies whenever the model forgets it.
func TestCompilerEnforcesCurrencyAndPeriod(t *testing.T) {
	t.Parallel()
	c, err := NewCatalog(fixtureDataset("fixture"))
	require.NoError(t, err)
	p := fixturePlan()
	compiled, err := c.Compile(p)
	require.NoError(t, err)
	require.Len(t, compiled.Plan.Dimensions, 2)
	require.Equal(t, "currency", compiled.Plan.Dimensions[1].Name)
	require.Len(t, p.Dimensions, 1, "normalization must not mutate the input")
	encoded, err := json.Marshal(compiled.Args)
	require.NoError(t, err)
	require.JSONEq(t, `["2026-01-01T00:00:00+05:00","2026-07-01T00:00:00+05:00"]`, string(encoded))
	second, err := c.Compile(compiled.Plan)
	require.NoError(t, err)
	require.Equal(t, compiled.Fingerprint, second.Fingerprint, "recorded plans must compile identically")
}

// False-green risk: compiling only registered names misses unsupported
// semantics, stale versions, injected identifiers and invalid operators.
func TestCompilerRejectsUnregisteredAndIncompatibleSemantics(t *testing.T) {
	t.Parallel()
	c, err := NewCatalog(fixtureDataset("fixture"))
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		change func(*Plan)
		want   error
	}{
		{"unknown", func(p *Plan) { p.Dataset = "financial_form_2" }, ErrUnknownDataset},
		{"stale", func(p *Plan) { p.Version = "old" }, ErrVersion},
		{"ratio", func(p *Plan) { p.Measures = []string{"loss_ratio"} }, ErrPlan},
		{"injection", func(p *Plan) { p.Dimensions = []Dimension{{Name: `product"; DROP TABLE policies;--`}} }, ErrPlan},
		{"duplicate", func(p *Plan) { p.Measures = []string{"premium", "premium"} }, ErrPlan},
		{"detail bucket", func(p *Plan) { p.Mode = "detail" }, ErrPlan},
		{"detail count", func(p *Plan) { p.Mode = "detail"; p.Dimensions = nil; p.Measures = []string{"records"} }, ErrPlan},
		{"reversed period", func(p *Plan) { p.Period.End = p.Period.Start }, ErrPlan},
		{"invalid date", func(p *Plan) { p.Period.Start = "2026-02-30" }, ErrPlan},
		{"metric filter", func(p *Plan) { p.Filters = []Filter{{Field: "premium", Operator: "gt", Values: []any{0}}} }, ErrPlan},
		{"null equality", func(p *Plan) { p.Filters = []Filter{{Field: "product", Operator: "eq", Values: []any{nil}}} }, ErrPlan},
		{"unknown order", func(p *Plan) { p.Order = []Order{{Field: "not_selected"}} }, ErrPlan},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := fixturePlan()
			tc.change(&p)
			_, err := c.Compile(p)
			require.ErrorIs(t, err, tc.want)
		})
	}
}

// False-green risk: permissive JSON decoding silently ignores SQL, joins or
// LIMIT supplied by the model and gives it a misleading successful artifact.
func TestPlanParsingRejectsExtraInstructions(t *testing.T) {
	t.Parallel()
	for _, input := range []string{`{"sql":"SELECT *"}`, `{"period":{"start":"2026-01-01","end_exclusive":"2026-07-01","timezone":"UTC"}}`, `{} {}`, `{"limit":200}`} {
		_, err := ParsePlan(input)
		require.ErrorIs(t, err, ErrPlan)
	}
}

// False-green risk: a two-source catalog never demonstrates bounded discovery
// or reusable compilation when hundreds of definitions are registered.
func TestCatalogScalesWithoutReportSpecificCompiler(t *testing.T) {
	t.Parallel()
	definitions := []Dataset{}
	for i := 0; i < 500; i++ {
		definitions = append(definitions, fixtureDataset(fmt.Sprintf("dataset_%03d", i)))
	}
	c, err := NewCatalog(definitions...)
	require.NoError(t, err)
	hits, total, err := c.Search("premium", 490, 5)
	require.NoError(t, err)
	require.Equal(t, 500, total)
	require.Len(t, hits, 5)
	for _, d := range hits {
		p := fixturePlan()
		p.Dataset = d.ID
		_, err := c.Compile(p)
		require.NoError(t, err)
	}
	_, _, err = c.Search("", 0, 501)
	require.ErrorIs(t, err, ErrPlan)
	// Discovery callers cannot corrupt the catalog's currency contract.
	hits[0].Measures[0].CurrencyDimension = ""
	p := fixturePlan()
	p.Dataset = hits[0].ID
	compiled, err := c.Compile(p)
	require.NoError(t, err)
	require.Len(t, compiled.Plan.Dimensions, 2)
	encoded, err := json.Marshal(hits)
	require.NoError(t, err)
	var public []map[string]any
	require.NoError(t, json.Unmarshal(encoded, &public))
	for _, d := range public {
		require.NotContains(t, d, "SourceSQL")
	}
}
