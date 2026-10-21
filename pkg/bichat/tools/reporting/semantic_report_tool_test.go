package reporting

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/iota-uz/iota-sdk/pkg/bichat/types"
	"github.com/iota-uz/iota-sdk/pkg/reporting"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// False-green risk: CallStructured alone misses the formatter boundary that
// previously wrapped discovery under an unexpected output key for the model.
func TestReportDiscoveryFormatsUsableDefinitionWithoutSQL(t *testing.T) {
	t.Parallel()
	tool := NewReportCatalogTool(testReportCatalog(t))
	text, err := tool.Call(context.Background(), `{"dataset_id":"fixture"}`)
	require.NoError(t, err)
	var output struct {
		Definition reporting.Dataset `json:"definition"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &output))
	require.Equal(t, "fixture", output.Definition.ID)
	require.Empty(t, output.Definition.SourceSQL, "source SQL must not be disclosed")
	require.Len(t, output.Definition.Measures, 1)
	text, err = tool.Call(context.Background(), `{"search":"fixture","limit":1}`)
	require.NoError(t, err)
	var search struct {
		Datasets []map[string]any `json:"datasets"`
		Total    int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &search))
	require.Equal(t, 1, search.Total)
	require.Len(t, search.Datasets, 1)
	require.NotContains(t, search.Datasets[0], "measures", "search returns summaries, not entire definitions")
}

// False-green risk: SUM's numeric result looks valid even when NULL source
// values were skipped; only the same-statement diagnostics reveal omissions.
func TestSemanticAggregateChecksMissingValuesAndPopulationBeforeExport(t *testing.T) {
	t.Parallel()
	for _, missing := range []string{"1", "0"} {
		dir := t.TempDir()
		r := &bichatsql.QueryResult{Columns: []string{"premium_currency", "written_premium", "_report_population_count", "_report_null_written_premium"}, ColumnTypes: []string{"string", "number", "number", "number"}, RowCount: 1, Rows: [][]any{{"UZS", "12.3", "2", missing}}}
		out, err := NewSemanticReportTool(testReportCatalog(t), &reportResultExecutor{r}, dir, "/exports").CallStructured(context.Background(), `{"dataset":"fixture","version":"v1","mode":"aggregate","measures":["written_premium"],"dimensions":[],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`)
		if missing == "1" {
			require.ErrorIs(t, err, ErrReportMissingMeasure)
			require.Nil(t, out)
			files, err := os.ReadDir(dir)
			require.NoError(t, err)
			require.Empty(t, files)
			continue
		}
		require.NoError(t, err)
		contract := out.Payload.(types.JSONPayload).Output.(map[string]any)["report_contract"].(map[string]any)
		require.Equal(t, "2", contract["population_record_count"])
		require.Equal(t, 1, contract["row_count"], "one group represents two source records")
		file, err := excelize.OpenFile(filepath.Join(dir, out.Artifacts[0].Name))
		require.NoError(t, err)
		rows, err := file.GetRows("Sheet1")
		require.NoError(t, err)
		require.NoError(t, file.Close())
		require.Len(t, rows, 2)
		require.Len(t, rows[0], 2, "internal diagnostics must not appear as business columns")
	}
}

// False-green risk: aggregate diagnostics cannot detect a NULL detail measure;
// asserting only an error would miss a wrong classification or partial export.
func TestSemanticDetailRejectsMissingMeasureBeforeExport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := &bichatsql.QueryResult{Columns: []string{"premium_currency", "written_premium"}, ColumnTypes: []string{"string", "number"}, RowCount: 1, Rows: [][]any{{"UZS", nil}}}
	out, err := NewSemanticReportTool(testReportCatalog(t), &reportResultExecutor{r}, dir, "/exports").CallStructured(context.Background(), `{"dataset":"fixture","version":"v1","mode":"detail","measures":["written_premium"],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`)
	require.ErrorIs(t, err, ErrReportMissingMeasure)
	require.Nil(t, out)
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, files)
}

// False-green risk: rejecting a malformed plan after execution still touches
// the DB. Reject each incompatible plan before any executor call or file.
func TestSemanticReportRejectsInvalidPlansBeforeExecution(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		input string
		want  error
	}{
		{`{"dataset":"fixture","version":"old","mode":"aggregate","measures":["written_premium"],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`, reporting.ErrVersion},
		{`{"dataset":"fixture","version":"v1","mode":"aggregate","measures":["cash_receipts"],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`, reporting.ErrPlan},
		{`{"dataset":"fixture","version":"v1","mode":"detail","measures":["written_premium"],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"},"limit":200}`, reporting.ErrPlan},
	} {
		dir := t.TempDir()
		_, err := NewSemanticReportTool(testReportCatalog(t), forbiddenReportExecutor{}, dir, "/exports").CallStructured(context.Background(), tc.input)
		require.ErrorIs(t, err, tc.want)
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, files)
	}
}

// False-green risk: a non-empty currency-only fixture misses null metadata
// or changed result columns that make a seemingly successful workbook invalid.
func TestSemanticReportRejectsResultContractDriftBeforeExport(t *testing.T) {
	t.Parallel()
	for _, r := range []*bichatsql.QueryResult{
		{Columns: []string{"premium_currency", "written_premium"}, ColumnTypes: []string{"string", "number"}, Rows: [][]any{{nil, "10"}}, RowCount: 1},
		{Columns: []string{"premium_currency", "cash"}, ColumnTypes: []string{"string", "number"}, Rows: [][]any{{"UZS", "10"}}, RowCount: 1},
	} {
		dir := t.TempDir()
		_, err := NewSemanticReportTool(testReportCatalog(t), &reportResultExecutor{r}, dir, "/exports").CallStructured(context.Background(), `{"dataset":"fixture","version":"v1","mode":"detail","measures":["written_premium"],"dimensions":[],"period":{"start":"2026-01-01","end_exclusive":"2026-07-01"}}`)
		require.ErrorIs(t, err, ErrReportShape)
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, files)
	}
}
