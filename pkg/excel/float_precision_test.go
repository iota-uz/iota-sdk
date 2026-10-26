package excel_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// Merely seeing a number cell would miss Excel rounding it when opened. The
// stored value must satisfy the same 15-digit contract as integer/SQL decimals.
func TestWorkbookFloatPrecisionMatchesIntegerContract(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmtName(false, streaming), func(t *testing.T) {
			for _, tc := range []struct {
				value any
				raw   string
			}{
				{float64(0.1), "0.1"},
				{float64(0.29), "0.29"},
				{float64(0.125), "0.125"},
				{float64(1.23456789012345), "1.23456789012345"},
				{float32(0.1), "0.1"},
				{float32(0.29), "0.29"},
				{float32(1.0 / 3), "0.33333334"},
			} {
				f := exportWorkbook(t, nil, excel.NewSliceDataSource([]string{"value"}, [][]any{{tc.value}}), streaming)
				raw, err := f.GetCellValue("Sheet1", "A2", excelize.Options{RawCellValue: true})
				require.NoError(t, err)
				assert.Equal(t, tc.raw, raw)
				typ, err := f.GetCellType("Sheet1", "A2")
				require.NoError(t, err)
				assert.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
			}
			for _, comma := range []bool{false, true} {
				for _, value := range []float64{1234567890123456, 1.234567890123456, 1.0 / 3} {
					opts := excel.DefaultOptions()
					opts.DecimalComma = comma
					exporter := excel.NewExcelExporter(opts, nil)
					source := excel.NewSliceDataSource([]string{"value"}, [][]any{{0.29}, {excel.Cell{Value: value, NumberFormat: "#,##0.00"}}})
					if streaming {
						var output bytes.Buffer
						err := exporter.ExportToWriter(context.Background(), &output, source)
						require.Error(t, err)
						assert.Empty(t, output.Bytes())
					} else {
						output, err := exporter.Export(context.Background(), source)
						require.Error(t, err)
						assert.Empty(t, output)
					}
				}
			}
		})
	}
}
