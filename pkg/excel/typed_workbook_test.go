package excel_test

import (
	"bytes"
	"context"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/excel"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// A formatted string can look exactly like a date or amount; types and formulas
// on the saved workbook prevent that false green.
func TestWorkbookValuesRemainTypedWithDefaultAndCommaOptions(t *testing.T) {
	for _, comma := range []bool{false, true} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmtName(comma, streaming), func(t *testing.T) {
				opts := excel.DefaultOptions()
				opts.DecimalComma = comma
				opts.FloatNumberFormat = "#,##0.00"
				instant := time.Date(2026, 10, 26, 23, 45, 12, 0, time.FixedZone("UZT", 5*3600))
				var absent *time.Time
				source := excel.NewSliceDataSource([]string{"amount", "time", "identifier", "absent", "zero"}, [][]any{
					{1234.50, instant, "000123456789012345678", absent, time.Time{}},
					{-34.25, instant.Add(time.Hour), "001.20", absent, time.Time{}},
				})
				f := exportWorkbook(t, opts, source, streaming)
				for _, cell := range []string{"A2", "A3", "B2", "B3"} {
					typ, err := f.GetCellType("Sheet1", cell)
					require.NoError(t, err)
					assert.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset, excelize.CellTypeDate}, typ, cell)
				}
				require.NoError(t, f.SetCellFormula("Sheet1", "A4", "SUM(A2:A3)"))
				total, err := f.CalcCellValue("Sheet1", "A4")
				require.NoError(t, err)
				assert.Equal(t, "1200.25", total)
				raw, err := f.GetCellValue("Sheet1", "B2", excelize.Options{RawCellValue: true})
				require.NoError(t, err)
				assert.NotContains(t, raw, "2026")
				for cell, expected := range map[string]string{"C2": "000123456789012345678", "C3": "001.20", "D2": "", "E2": ""} {
					value, err := f.GetCellValue("Sheet1", cell)
					require.NoError(t, err)
					assert.Equal(t, expected, value, cell)
				}
			})
		}
	}
}

// Checking just formatted output would miss a string date or a string amount.
func TestWorkbookExplicitCellFormatsPreserveValuesAndStyles(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmtName(false, streaming), func(t *testing.T) {
			instant := time.Date(2026, 10, 26, 23, 45, 12, 0, time.FixedZone("UZT", 5*3600))
			date := excel.Date(instant)
			date.NumberFormat = "dd.mm.yyyy"
			datetime := excel.DateTime(instant)
			datetime.NumberFormat = "dd.mm.yyyy hh:mm:ss"
			data := [][]any{{
				excel.Cell{Value: 1234.5, NumberFormat: "#,##0.00"},
				excel.Cell{Value: 0.2, NumberFormat: "0.00%"},
				excel.Cell{Value: 12850.12345678, NumberFormat: "0.00000000"},
				date, datetime, excel.Date(time.Time{}), excel.DateTime(time.Time{}),
				excel.Cell{Value: "001.20", NumberFormat: "0.00"},
				excel.Cell{Value: "=1+1", NumberFormat: "0.00"},
				excel.Cell{Value: true},
				excel.Cell{Value: excel.Formula{Expression: "A2*B2"}, NumberFormat: "#,##0.00"},
			}}
			f := exportWorkbook(t, excel.DefaultOptions(), excel.NewSliceDataSource(
				[]string{"money", "percentage", "rate", "date", "datetime", "absent_date", "absent_time", "identifier", "text", "bool", "formula"}, data), streaming)
			headerStyleID, err := f.GetCellStyle("Sheet1", "A1")
			require.NoError(t, err)
			headerStyle, err := f.GetStyle(headerStyleID)
			require.NoError(t, err)
			require.NotNil(t, headerStyle.Font)
			assert.True(t, headerStyle.Font.Bold)
			assert.Contains(t, headerStyle.Fill.Color, "E0E0E0")
			for cell, expected := range map[string]string{"A2": "1,234.50", "B2": "20.00%", "C2": "12850.12345678", "D2": "26.10.2026", "E2": "26.10.2026 23:45:12", "F2": "", "G2": "", "H2": "001.20", "I2": "=1+1"} {
				value, err := f.GetCellValue("Sheet1", cell)
				require.NoError(t, err)
				assert.Equal(t, expected, value, cell)
			}
			for _, cell := range []string{"A2", "B2", "C2", "D2", "E2"} {
				typ, err := f.GetCellType("Sheet1", cell)
				require.NoError(t, err)
				assert.NotContains(t, []excelize.CellType{excelize.CellTypeSharedString, excelize.CellTypeInlineString}, typ, cell)
				styleID, err := f.GetCellStyle("Sheet1", cell)
				require.NoError(t, err)
				style, err := f.GetStyle(styleID)
				require.NoError(t, err)
				assert.Contains(t, style.Fill.Color, "F5F5F5", cell)
				require.NotNil(t, style.CustomNumFmt)
			}
			for cell, expected := range map[string]time.Time{"D2": time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), "E2": time.Date(2026, 10, 26, 23, 45, 12, 0, time.UTC)} {
				raw, err := f.GetCellValue("Sheet1", cell, excelize.Options{RawCellValue: true})
				require.NoError(t, err)
				serial, err := strconv.ParseFloat(raw, 64)
				require.NoError(t, err)
				decoded, err := excelize.ExcelDateToTime(serial, false)
				require.NoError(t, err)
				assert.WithinDuration(t, expected, decoded, time.Millisecond, cell)
			}
			formula, err := f.GetCellFormula("Sheet1", "I2")
			require.NoError(t, err)
			assert.Empty(t, formula)
			result, err := f.CalcCellValue("Sheet1", "K2")
			require.NoError(t, err)
			assert.Equal(t, "246.90", result)
		})
	}
}

func TestCSVCellUnwrapsFormattedValuesWithoutLosingFormulaSafety(t *testing.T) {
	// A struct's default rendering would silently break shared XLSX/CSV sources.
	assert.Equal(t, "-12.50", excel.CSVCell(excel.Cell{Value: -12.5, NumberFormat: "#,##0.00"}))
	assert.Equal(t, "'=1+1", excel.CSVCell(excel.Cell{Value: "=1+1", NumberFormat: "0.00"}))
	assert.Equal(t, "=SUM(A1:A2)", excel.CSVCell(excel.Cell{Value: excel.Formula{Expression: "SUM(A1:A2)"}}))
	assert.Empty(t, excel.CSVCell(excel.Date(time.Time{})))
}

func TestWorkbookNativeDateTimeFormatIsExplicit(t *testing.T) {
	// Legacy Go layouts must not be silently ignored or stringify the date.
	for _, streaming := range []bool{false, true} {
		t.Run(fmtName(false, streaming), func(t *testing.T) {
			value := time.Date(2026, 10, 26, 23, 45, 12, 0, time.UTC)
			opts := excel.DefaultOptions()
			opts.DateTimeFormat = "02.01.2006 15:04"
			source := func() excel.DataSource { return excel.NewSliceDataSource([]string{"time"}, [][]any{{value}}) }
			exporter := excel.NewExcelExporter(opts, nil)
			if streaming {
				var output bytes.Buffer
				require.Error(t, exporter.ExportToWriter(context.Background(), &output, source()))
				assert.Empty(t, output.Bytes())
			} else {
				data, err := exporter.Export(context.Background(), source())
				require.Error(t, err)
				assert.Empty(t, data)
			}
			opts.DateTimeNumberFormat = "dd.mm.yyyy hh:mm"
			f := exportWorkbook(t, opts, source(), streaming)
			formatted, err := f.GetCellValue("Sheet1", "A2")
			require.NoError(t, err)
			assert.Equal(t, "26.10.2026 23:45", formatted)
		})
	}
}

func TestWorkbookRejectsUnsupportedDatesAndNonfiniteNumbers(t *testing.T) {
	// An error without checking the output could hide a corrupt partial file.
	values := []any{
		time.Date(1800, 1, 1, 0, 0, 0, 0, time.UTC),
		math.NaN(), math.Inf(1), math.Inf(-1),
		int64(9007199254740993), uint64(18446744073709551615),
	}
	for _, text := range []string{"NaN", "Infinity", "-Infinity"} {
		var numeric pgtype.Numeric
		require.NoError(t, numeric.Scan(text))
		values = append(values, numeric)
	}
	for _, streaming := range []bool{false, true} {
		for _, value := range values {
			exporter := excel.NewExcelExporter(nil, nil)
			source := excel.NewSliceDataSource([]string{"value"}, [][]any{{1}, {value}})
			if streaming {
				var output bytes.Buffer
				require.Error(t, exporter.ExportToWriter(context.Background(), &output, source))
				assert.Empty(t, output.Bytes())
			} else {
				output, err := exporter.Export(context.Background(), source)
				require.Error(t, err)
				assert.Empty(t, output)
			}
		}
	}
}

// Numeric's SQL type, not a numeric-looking string, establishes its semantics.
func TestWorkbookNativeNumericIsTypedOrRefusedWithoutRounding(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmtName(false, streaming), func(t *testing.T) {
			var numeric pgtype.Numeric
			require.NoError(t, numeric.Scan("2387743666.1250"))
			f := exportWorkbook(t, nil, excel.NewSliceDataSource([]string{"amount"}, [][]any{{numeric}}), streaming)
			typ, err := f.GetCellType("Sheet1", "A2")
			require.NoError(t, err)
			assert.Contains(t, []excelize.CellType{excelize.CellTypeNumber, excelize.CellTypeUnset}, typ)
			require.NoError(t, f.SetCellFormula("Sheet1", "A3", "SUM(A2:A2)"))
			total, err := f.CalcCellValue("Sheet1", "A3")
			require.NoError(t, err)
			assert.Equal(t, "2387743666.125", total)
			require.NoError(t, numeric.Scan("1234567890123456.78"))
			exporter := excel.NewExcelExporter(nil, nil)
			source := excel.NewSliceDataSource([]string{"amount"}, [][]any{{numeric}})
			if streaming {
				var output bytes.Buffer
				err = exporter.ExportToWriter(context.Background(), &output, source)
				assert.Empty(t, output.Bytes())
			} else {
				_, err = exporter.Export(context.Background(), source)
			}
			require.Error(t, err)
		})
	}
}

func fmtName(comma, streaming bool) string {
	name := "default"
	if comma {
		name = "comma"
	}
	if streaming {
		return name + "/streaming"
	}
	return name + "/buffered"
}

func exportWorkbook(t *testing.T, opts *excel.ExportOptions, source excel.DataSource, streaming bool) *excelize.File {
	t.Helper()
	exporter := excel.NewExcelExporter(opts, nil)
	var data []byte
	if streaming {
		var buf bytes.Buffer
		require.NoError(t, exporter.ExportToWriter(context.Background(), &buf, source))
		data = buf.Bytes()
	} else {
		var err error
		data, err = exporter.Export(context.Background(), source)
		require.NoError(t, err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	return f
}
