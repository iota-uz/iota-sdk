package reporting

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	bichatsql "github.com/iota-uz/iota-sdk/pkg/bichat/sql"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// False-green risk: checking conversion alone misses the workbook boundary
// where an unsupported pgx TIME was silently serialized as text.
func TestReportExportPreservesNativeAndStringTimeCells(t *testing.T) {
	t.Parallel()
	r := &bichatsql.QueryResult{Columns: []string{"native_time", "string_time", "null_time"}, ColumnTypes: []string{"date", "date", "date"}, RowCount: 1, Rows: [][]any{{pgtype.Time{Valid: true, Microseconds: int64(12 * time.Hour / time.Microsecond)}, "12:00:00", pgtype.Time{}}}}
	dir := t.TempDir()
	out, err := NewReportExportTool(&reportResultExecutor{r}, dir, "/exports").CallStructured(context.Background(), `{"sql":"SELECT fixture"}`)
	require.NoError(t, err)
	f, err := excelize.OpenFile(filepath.Join(dir, out.Artifacts[0].Name))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	for _, cell := range []string{"A2", "B2"} {
		typ, err := f.GetCellType("Sheet1", cell)
		require.NoError(t, err)
		require.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
		value, err := f.GetCellValue("Sheet1", cell, excelize.Options{RawCellValue: true})
		require.NoError(t, err)
		fraction, err := strconv.ParseFloat(value, 64)
		require.NoError(t, err)
		require.InDelta(t, 0.5, fraction, 0.00000001)
	}
	value, err := f.GetCellValue("Sheet1", "C2")
	require.NoError(t, err)
	require.Empty(t, value)
	require.IsType(t, pgtype.Time{}, r.Rows[0][0], "export must not mutate the source result")
}
