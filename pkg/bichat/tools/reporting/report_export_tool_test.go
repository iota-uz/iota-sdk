package reporting

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/reporting"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// False-green risk: checking JSON row_count alone misses a truncated workbook
// and numeric strings. Read the actual saved XLSX and its last data row.
func TestReportExportWritesAllRowsAndTypedCells(t *testing.T) {
	r := &bichatsql.QueryResult{Columns: []string{"identifier", "premium", "issued"}, ColumnTypes: []string{"string", "number", "date"}, RowCount: 29463}
	for i := 0; i < r.RowCount; i++ {
		r.Rows = append(r.Rows, []any{"000123", "1234.56000000000000000000", "2026-06-30T00:00:00Z"})
	}
	dir := t.TempDir()
	tool := NewReportExportTool(NewReportExecutor(&reportResultExecutor{r}), dir, "/exports")
	out, err := tool.CallStructured(context.Background(), `{"sql":"SELECT original_query","filename":"../../full.xlsx"}`)
	require.NoError(t, err)
	require.Len(t, out.Artifacts, 1)
	file, err := excelize.OpenFile(filepath.Join(dir, out.Artifacts[0].Name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	rows, err := file.GetRows("Sheet1")
	require.NoError(t, err)
	require.Len(t, rows, 29464)
	typ, err := file.GetCellType("Sheet1", "B29464")
	require.NoError(t, err)
	require.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
	typ, err = file.GetCellType("Sheet1", "C29464")
	require.NoError(t, err)
	require.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset, excelize.CellTypeDate}, typ)
	value, err := file.GetCellValue("Sheet1", "A29464")
	require.NoError(t, err)
	require.Equal(t, "000123", value)
	value, err = file.GetCellValue("Sheet1", "B29464")
	require.NoError(t, err)
	require.Equal(t, "1234.56", value)
	require.Equal(t, "1234.56000000000000000000", r.Rows[0][1], "conversion must not mutate source data")
}

// False-green risk: a tool error with an already-published partial file still
// reproduces the incident. Verify that no artifact and no file are produced.
func TestReportExportRefusesIncompleteAndUnsafeNumbers(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    *bichatsql.QueryResult
		want error
	}{
		{"truncated", &bichatsql.QueryResult{Truncated: true}, ErrIncompleteReport},
		{"precision", &bichatsql.QueryResult{Columns: []string{"amount"}, ColumnTypes: []string{"number"}, Rows: [][]any{{"1234567890123456.78"}}, RowCount: 1}, ErrExcelPrecision},
		{"shape", &bichatsql.QueryResult{Columns: []string{"amount"}, Rows: [][]any{{"1"}}, RowCount: 1}, ErrReportShape},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tool := NewReportExportTool(NewReportExecutor(&reportResultExecutor{tc.r}), dir, "/exports")
			out, err := tool.CallStructured(context.Background(), `{"sql":"SELECT 1"}`)
			require.ErrorIs(t, err, tc.want)
			require.Nil(t, out)
			files, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Empty(t, files)
		})
	}
}

// False-green risk: recomputing only a separate aggregate would leave the XLSX
// population independent. Both manifest and file must come from these rows.
func TestSemanticReportManifestUsesExportedPopulation(t *testing.T) {
	t.Parallel()
	r := &bichatsql.QueryResult{Columns: []string{"premium_currency", "written_premium"}, ColumnTypes: []string{"string", "number"}, Rows: [][]any{{"UZS", "0.10"}, {"UZS", "0.20"}, {"USD", "5.01"}}, RowCount: 3}
	dir := t.TempDir()
	tool := NewSemanticReportTool(testReportCatalog(t), NewReportExecutor(&reportResultExecutor{r}), dir, "/exports")
	out, err := tool.CallStructured(context.Background(), `{"dataset":"fixture","version":"v1","mode":"detail","measures":["written_premium"],"dimensions":[],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`)
	require.NoError(t, err)
	payload := out.Payload.(types.JSONPayload).Output.(map[string]any)
	contract := payload["report_contract"].(map[string]any)
	require.Equal(t, map[string]map[string]string{"written_premium": {"UZS": "0.3", "USD": "5.01"}}, contract["totals_by_measure_and_unit"])
	file, err := excelize.OpenFile(filepath.Join(dir, out.Artifacts[0].Name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	rows, err := file.GetRows("Sheet1")
	require.NoError(t, err)
	require.Len(t, rows, 4)
	contractRows, err := file.GetRows("Report contract")
	require.NoError(t, err)
	var savedTotals map[string]map[string]string
	for _, row := range contractRows {
		if row[0] == "totals_by_measure_and_unit" {
			require.NoError(t, json.Unmarshal([]byte(row[1]), &savedTotals))
		}
	}
	require.Equal(t, contract["totals_by_measure_and_unit"], savedTotals)
}

// False-green risk: an error after querying or exporting can already expose an
// invented financial report. The source below panics if it is called.
func TestSemanticUnsupportedDefinitionDoesNotQuery(t *testing.T) {
	t.Parallel()
	tool := NewSemanticReportTool(testReportCatalog(t), forbiddenReportExecutor{}, t.TempDir(), "/exports")
	_, err := tool.CallStructured(context.Background(), `{"dataset":"financial_form_2","version":"v1","mode":"detail","measures":["written_premium"],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`)
	require.ErrorIs(t, err, reporting.ErrUnknownDataset)
}

type forbiddenReportExecutor struct{}

func testReportCatalog(t *testing.T) *reporting.Catalog {
	t.Helper()
	c, err := reporting.NewCatalog(reporting.Dataset{ID: "fixture", Version: "v1", SourceSQL: "SELECT fixture", Grain: "one fixture", Population: "all fixture records", DateBasis: "issued", Timezone: "Asia/Tashkent", PeriodParameterType: "date",
		Dimensions: []reporting.Field{{Name: "issued", Type: "date"}, {Name: "premium_currency", Type: "string"}},
		Measures:   []reporting.Measure{{Field: reporting.Field{Name: "written_premium", Type: "number"}, Aggregation: "sum", Unit: "major currency units", CurrencyDimension: "premium_currency"}},
	})
	require.NoError(t, err)
	return c
}

func (forbiddenReportExecutor) ExecuteQuery(context.Context, string, []any, time.Duration) (*bichatsql.QueryResult, error) {
	panic("unsupported report queried database")
}

// False-green risk: previewing 200 rows is harmless only if the download still
// contains the complete source. Inspect both result payload and workbook.
func TestReportTableLimitsPreviewOnly(t *testing.T) {
	r := &bichatsql.QueryResult{Columns: []string{"amount"}, ColumnTypes: []string{"number"}, RowCount: 301}
	for i := 0; i < 301; i++ {
		r.Rows = append(r.Rows, []any{"12.34"})
	}
	dir := t.TempDir()
	tool := NewReportTableTool(NewReportExecutor(&reportResultExecutor{r}), dir, "/exports")
	out, err := tool.CallStructured(context.Background(), `{"sql":"SELECT 1"}`)
	require.NoError(t, err)
	payload := out.Payload.(types.JSONPayload).Output.(map[string]any)
	require.Len(t, payload["rows"], 200)
	data, err := json.Marshal(payload["export"])
	require.NoError(t, err)
	var exp struct {
		Filename string `json:"filename"`
		Rows     int    `json:"row_count"`
	}
	require.NoError(t, json.Unmarshal(data, &exp))
	require.Equal(t, 301, exp.Rows)
	file, err := excelize.OpenFile(filepath.Join(dir, exp.Filename))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, file.Close()) })
	rows, err := file.GetRows("Sheet1")
	require.NoError(t, err)
	require.Len(t, rows, 302)
}
