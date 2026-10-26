package reporting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/reporting"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

const dimensionCurrencyPlan = `{"dataset":"fixture","version":"v1","mode":"detail","measures":["written_premium"],"dimensions":[{"name":"limit_amount"}],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`

func dimensionCurrencyCatalog(t *testing.T) *reporting.Catalog {
	t.Helper()
	definition, err := testReportCatalog(t).Dataset("fixture")
	require.NoError(t, err)
	definition.Dimensions = append(definition.Dimensions,
		reporting.Field{Name: "limit_amount", Type: "number", CurrencyDimension: "limit_currency"},
		reporting.Field{Name: "limit_currency", Type: "string"},
	)
	catalog, err := reporting.NewCatalog(definition)
	require.NoError(t, err)
	return catalog
}

// A premium's currency cannot substitute for a different dimension's currency.
// These are new capability tests, including unavailable dimensions and zero.
func TestSemanticMonetaryDimensionValidatesCurrencyAndRetainsBindingInWorkbook(t *testing.T) {
	for _, tc := range []struct {
		name     string
		amount   any
		currency any
		wantErr  bool
	}{
		{"present", "100.25", "USD", false},
		{"unavailable", nil, nil, false},
		{"zero is a value", "0", nil, true},
		{"missing", "100.25", nil, true},
		{"blank", "100.25", " ", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			result := &bichatsql.QueryResult{
				Columns:     []string{"limit_amount", "premium_currency", "limit_currency", "written_premium"},
				ColumnTypes: []string{"number", "string", "string", "number"},
				RowCount:    1, Rows: [][]any{{tc.amount, "UZS", tc.currency, "12.3"}},
			}
			out, err := NewSemanticReportTool(dimensionCurrencyCatalog(t), &reportResultExecutor{result}, dir, "/exports").CallStructured(context.Background(), dimensionCurrencyPlan)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrReportShape)
				require.Nil(t, out)
				files, err := os.ReadDir(dir)
				require.NoError(t, err)
				require.Empty(t, files)
				return
			}
			require.NoError(t, err)
			contract := out.Payload.(types.JSONPayload).Output.(map[string]any)["report_contract"].(map[string]any)
			totals := contract["totals_by_measure_and_unit"].(map[string]map[string]string)
			require.Equal(t, map[string]map[string]string{"written_premium": {"UZS": "12.3"}}, totals)
			workbook, err := excelize.OpenFile(filepath.Join(dir, out.Artifacts[0].Name))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, workbook.Close()) })
			manifestRows, err := workbook.GetRows("Report contract")
			require.NoError(t, err)
			var definition reporting.Dataset
			for _, row := range manifestRows {
				if len(row) == 2 && row[0] == "definition" {
					require.NoError(t, json.Unmarshal([]byte(row[1]), &definition))
				}
			}
			bindingFound := false
			for _, field := range definition.Dimensions {
				if field.Name == "limit_amount" {
					require.Equal(t, "limit_currency", field.CurrencyDimension)
					bindingFound = true
				}
			}
			require.True(t, bindingFound)
			if tc.amount != nil {
				typ, err := workbook.GetCellType("Sheet1", "A2")
				require.NoError(t, err)
				require.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
			}
		})
	}
}

func TestSemanticMonetaryDimensionRefusesMissingCurrencyColumn(t *testing.T) {
	// Checking only values would miss an executor omitting the paired column.
	dir := t.TempDir()
	r := &bichatsql.QueryResult{Columns: []string{"limit_amount", "premium_currency", "written_premium"}, ColumnTypes: []string{"number", "string", "number"}, Rows: [][]any{{"100", "UZS", "12.3"}}, RowCount: 1}
	_, err := NewSemanticReportTool(dimensionCurrencyCatalog(t), &reportResultExecutor{r}, dir, "/exports").CallStructured(context.Background(), dimensionCurrencyPlan)
	require.ErrorIs(t, err, ErrReportShape)
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestSemanticReportExportsMoreThanFortyColumnsWithoutDroppingFields(t *testing.T) {
	// A compiler-only assertion cannot establish that the artifact retains all
	// selected fields after the executor, exporter and manifest boundaries.
	definition, err := testReportCatalog(t).Dataset("fixture")
	require.NoError(t, err)
	plan := reporting.Plan{Dataset: "fixture", Version: "v1", Mode: "detail", Period: reporting.Period{Start: "2026-01-01", End: "2026-07-01"}, Measures: []string{"written_premium"}}
	result := &bichatsql.QueryResult{RowCount: 1}
	row := []any{}
	for i := 0; i < 40; i++ {
		name := fmt.Sprintf("field_%02d", i)
		definition.Dimensions = append(definition.Dimensions, reporting.Field{Name: name, Type: "string"})
		plan.Dimensions = append(plan.Dimensions, reporting.Dimension{Name: name})
		result.Columns = append(result.Columns, name)
		result.ColumnTypes = append(result.ColumnTypes, "string")
		row = append(row, fmt.Sprintf("value_%02d", i))
	}
	result.Columns = append(result.Columns, "premium_currency", "written_premium")
	result.ColumnTypes = append(result.ColumnTypes, "string", "number")
	result.Rows = [][]any{append(row, "UZS", "12.3")}
	catalog, err := reporting.NewCatalog(definition)
	require.NoError(t, err)
	encoded, err := json.Marshal(plan)
	require.NoError(t, err)
	dir := t.TempDir()
	tool := NewSemanticReportTool(catalog, &reportResultExecutor{result}, dir, "/exports")
	properties := tool.Parameters()["properties"].(map[string]any)
	require.Equal(t, reporting.MaxReportColumns, properties["dimensions"].(map[string]any)["maxItems"])
	out, err := tool.CallStructured(context.Background(), string(encoded))
	require.NoError(t, err)
	workbook, err := excelize.OpenFile(filepath.Join(dir, out.Artifacts[0].Name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, workbook.Close()) })
	rows, err := workbook.GetRows("Sheet1")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.Len(t, rows[0], 42)
	require.Equal(t, "field_39", rows[0][39])
	require.Equal(t, "value_39", rows[1][39])
}
