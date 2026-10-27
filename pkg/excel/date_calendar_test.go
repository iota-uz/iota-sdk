package excel_test

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
