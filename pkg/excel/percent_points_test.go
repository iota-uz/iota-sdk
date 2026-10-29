package excel_test

import (
	"bytes"
	"context"
	"math"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// Checking display alone could hide a rounded or text fraction. These tests
// characterize the new percentage-point representation helper in saved files.
func TestWorkbookPercentPointsPreservesDecimalFraction(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmtName(false, streaming), func(t *testing.T) {
			source := excel.NewSliceDataSource([]string{"rate"}, [][]any{
				{excel.PercentPoints(20.2)}, {excel.PercentPoints(-1.25)},
				{excel.PercentPoints(0)}, {excel.PercentPoints(100)},
				{excel.PercentPoints(0.0049)}, {excel.PercentPoints(-0.0049)},
				{excel.PercentPoints(0.005)}, {excel.PercentPoints(-0.005)},
			})
			f := exportWorkbook(t, nil, source, streaming)
			for cell, expected := range map[string][2]string{
				"A2": {"0.202", "20.20%"}, "A3": {"-0.0125", "-1.25%"},
				"A4": {"0", "0.00%"}, "A5": {"1", "100.00%"},
				"A6": {"0", "0.00%"}, "A7": {"0", "0.00%"},
				"A8": {"0.0001", "0.01%"}, "A9": {"-0.0001", "-0.01%"},
			} {
				raw, err := f.GetCellValue("Sheet1", cell, excelize.Options{RawCellValue: true})
				require.NoError(t, err)
				assert.Equal(t, expected[0], raw, cell)
				shown, err := f.GetCellValue("Sheet1", cell)
				require.NoError(t, err)
				assert.Equal(t, expected[1], shown, cell)
				typ, err := f.GetCellType("Sheet1", cell)
				require.NoError(t, err)
				assert.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
			}
			require.NoError(t, f.SetCellFormula("Sheet1", "B2", "A2*100"))
			points, err := f.CalcCellValue("Sheet1", "B2")
			require.NoError(t, err)
			assert.Equal(t, "20.2", points)
			for _, invalid := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 1e-323} {
				exporter := excel.NewExcelExporter(nil, nil)
				source := excel.NewSliceDataSource([]string{"rate"}, [][]any{{excel.PercentPoints(invalid)}})
				if streaming {
					var out bytes.Buffer
					require.Error(t, exporter.ExportToWriter(context.Background(), &out, source))
					assert.Empty(t, out.Bytes())
				} else {
					out, err := exporter.Export(context.Background(), source)
					require.Error(t, err)
					assert.Empty(t, out)
				}
			}
		})
	}
}

// False green: checking only the displayed percentage misses the value used by formulas.
func TestWorkbookCalculatedPercentPoints(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmtName(false, streaming), func(t *testing.T) {
			source := excel.NewSliceDataSource([]string{"rate"}, [][]any{
				{excel.PercentPoints(49.931155338590564)},
				{excel.PercentPoints(-12.345)},
			})
			f := exportWorkbook(t, nil, source, streaming)
			for cell, expected := range map[string]string{"A2": "0.4993", "A3": "-0.1235"} {
				raw, err := f.GetCellValue("Sheet1", cell, excelize.Options{RawCellValue: true})
				require.NoError(t, err)
				assert.Equal(t, expected, raw)
				typ, err := f.GetCellType("Sheet1", cell)
				require.NoError(t, err)
				assert.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
			}
		})
	}
}
