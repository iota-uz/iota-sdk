package excel_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// Reconstructing midnight in a timezone with a midnight DST gap can silently
// move the date backwards. The workbook must retain the input calendar day.
func TestWorkbookDateKeepsCalendarAcrossMidnightDSTGap(t *testing.T) {
	zone, err := time.LoadLocation("America/Sao_Paulo")
	require.NoError(t, err)
	value := time.Date(2018, time.November, 4, 12, 0, 0, 0, zone)
	for _, streaming := range []bool{false, true} {
		t.Run(fmtName(false, streaming), func(t *testing.T) {
			f := exportWorkbook(t, nil, excel.NewSliceDataSource([]string{"date"}, [][]any{{excel.Date(value)}}), streaming)
			date, err := f.GetCellValue("Sheet1", "A2")
			require.NoError(t, err)
			assert.Equal(t, "2018-11-04", date)
		})
	}
}

// False green if the row count stays below Excel's 65,430 style limit: six date
// columns over 11,000 rows exceed it when each styled date allocates a style.
func TestBufferedWorkbookDatesDoNotExhaustExcelStyles(t *testing.T) {
	const rows = 11000
	stamp := time.Date(2026, time.June, 30, 15, 4, 5, 0, time.UTC)
	data := make([][]any, rows)
	for i := range data {
		data[i] = []any{stamp, stamp, stamp, stamp, stamp, stamp}
	}
	headers := []string{"a", "b", "c", "d", "e", "f"}
	f := exportWorkbook(t, nil, excel.NewSliceDataSource(headers, data), false)
	shown, err := f.GetCellValue("Sheet1", fmt.Sprintf("F%d", rows+1))
	require.NoError(t, err)
	assert.Equal(t, "2026-06-30 15:04:05", shown)
	cellType, err := f.GetCellType("Sheet1", fmt.Sprintf("F%d", rows+1))
	require.NoError(t, err)
	assert.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, cellType, "a date stays a native Excel number")
}
