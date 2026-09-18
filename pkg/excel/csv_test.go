package excel

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// Pins which leading characters make a spreadsheet evaluate a cell. Falsely
// green if every value were prefixed: the numeric, formatted-number, phone and
// placeholder cases must come back unchanged.
func TestNeutralizeFormula(t *testing.T) {
	t.Parallel()

	cases := []struct{ in, want string }{
		{`=HYPERLINK("http://x","y")`, `'=HYPERLINK("http://x","y")`},
		{"@SUM(A1:A2)", "'@SUM(A1:A2)"},
		{"+1+cmd|' /C calc'!A0", "'+1+cmd|' /C calc'!A0"},
		{"-2+3", "'-2+3"},
		{"-1-1", "'-1-1"},
		{"-(1)", "'-(1)"},
		{"-SUM(1)", "'-SUM(1)"},
		{"\t=1", "'\t=1"},
		{"\r=1", "'\r=1"},
		{"+998901234567", "+998901234567"},
		{"+998 (90) 123-45-67", "+998 (90) 123-45-67"},
		{"-5.00", "-5.00"},
		{"-1 234,56", "-1 234,56"},
		{"-1 234,56", "-1 234,56"},
		{"-", "-"},
		{"", ""},
		{"Plain Name", "Plain Name"},
		{"a=b", "a=b"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, NeutralizeFormula(tc.in), "input %q", tc.in)
	}
}

// Only the explicit Formula type is written as a formula; the same text as a
// plain string is data. Falsely green if Formula were neutralized too: its
// cell must start with '='.
func TestCSVCell(t *testing.T) {
	t.Parallel()

	var nilFormula *Formula
	assert.Equal(t, "=SUM(A1:A2)", CSVCell(Formula{Expression: "SUM(A1:A2)"}))
	assert.Equal(t, "=SUM(A1:A2)", CSVCell(&Formula{Expression: "=SUM(A1:A2)"}))
	assert.Equal(t, "'=SUM(A1:A2)", CSVCell("=SUM(A1:A2)"))
	assert.Empty(t, CSVCell(nilFormula))
	assert.Empty(t, CSVCell(nil))
	assert.Equal(t, "-12.50", CSVCell(-12.5))
	assert.Equal(t, "7", CSVCell(7))
	assert.Equal(t, "'@x", CSVCell([]byte("@x")))
}

// End to end through a DataSource: BOM, headers, neutralized user text,
// formulas kept, decimal comma and date format applied, MaxRows honoured.
func TestCSVExporter_ExportToWriter(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 19, 10, 30, 0, 0, time.UTC)
	ds := NewSliceDataSource([]string{"Name", "Amount", "At", "Check"}, [][]interface{}{
		{"=1+2 Nodir", 1234.5, at, Formula{Expression: "B2*2"}},
		{"+998 90 123 45 67", "14773814.00", nil, nil},
		{"never written", 0.0, nil, nil},
	})
	exporter := NewCSVExporter(&CSVOptions{
		IncludeHeaders: true,
		IncludeBOM:     true,
		DecimalComma:   true,
		DateTimeFormat: "02.01.2006 15:04",
		MaxRows:        2,
	})

	var buf bytes.Buffer
	require.NoError(t, exporter.ExportToWriter(context.Background(), &buf, ds))
	require.True(t, bytes.HasPrefix(buf.Bytes(), utf8BOM), "UTF-8 BOM first")

	records, err := csv.NewReader(bytes.NewReader(buf.Bytes()[len(utf8BOM):])).ReadAll()
	require.NoError(t, err)
	assert.Equal(t, [][]string{
		{"Name", "Amount", "At", "Check"},
		{"'=1+2 Nodir", "1234,50", "19.09.2026 10:30", "=B2*2"},
		{"+998 90 123 45 67", "14773814,00", "", ""},
	}, records)
}

// The delimiter option reaches the csv.Writer; the default writer adds no BOM
// when asked not to.
func TestCSVWriter_CommaAndNoBOM(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	w, err := NewCSVWriter(&buf, &CSVOptions{Comma: ';'})
	require.NoError(t, err)
	require.NoError(t, w.WriteHeader([]string{"A", "-B"}))
	require.NoError(t, w.WriteRow([]interface{}{"x", "@y"}))
	require.NoError(t, w.Flush())
	assert.Equal(t, "A;'-B\nx;'@y\n", buf.String())
}

// XLSX: a plain string that looks like a formula is stored as text in both
// export paths, and only a Formula value becomes a formula cell. Falsely
// green if the workbook were read back through a formula-evaluating reader:
// GetCellFormula reports the stored cell type directly.
func TestExcelExporter_OnlyFormulaTypeIsAFormula(t *testing.T) {
	t.Parallel()

	ds := func() DataSource {
		return NewSliceDataSource([]string{"Text", "Formula"}, [][]interface{}{
			{"=1+2", Formula{Expression: "=1+2"}},
		})
	}
	exporter := NewExcelExporter(DefaultOptions(), nil)

	buffered, err := exporter.Export(context.Background(), ds())
	require.NoError(t, err)
	var streamed bytes.Buffer
	require.NoError(t, exporter.ExportToWriter(context.Background(), &streamed, ds()))

	for name, data := range map[string][]byte{"Export": buffered, "ExportToWriter": streamed.Bytes()} {
		f, err := excelize.OpenReader(bytes.NewReader(data))
		require.NoError(t, err, name)
		sheet := f.GetSheetName(0)

		textFormula, err := f.GetCellFormula(sheet, "A2")
		require.NoError(t, err, name)
		assert.Empty(t, textFormula, "%s: plain string must not be a formula", name)
		text, err := f.GetCellValue(sheet, "A2")
		require.NoError(t, err, name)
		assert.Equal(t, "=1+2", text, name)

		formula, err := f.GetCellFormula(sheet, "B2")
		require.NoError(t, err, name)
		assert.Equal(t, "1+2", formula, name)
		require.NoError(t, f.Close())
	}
}

// limitedSource serves rows until limit and fails any fetch past it.
type limitedSource struct{ limit int }

func (s limitedSource) GetHeaders() []string { return []string{"N"} }
func (s limitedSource) GetSheetName() string { return "Sheet1" }
func (s limitedSource) GetRows(context.Context) (func() ([]interface{}, error), error) {
	fetched := 0
	return func() ([]interface{}, error) {
		fetched++
		if fetched > s.limit {
			return nil, errors.New("fetched past MaxRows")
		}
		return []interface{}{fetched}, nil
	}, nil
}

// MaxRows caps what the exporters ask the source for, not only what they
// write: a source that fails on the fetch after the limit must still export.
// Falsely green if the source stopped by itself (returned nil) at the limit —
// it errors instead.
func TestExporters_MaxRowsStopsBeforeFetchingPastLimit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	src := limitedSource{limit: 2}
	xlsx := NewExcelExporter(&ExportOptions{IncludeHeaders: true, MaxRows: 2}, nil)
	csvExporter := NewCSVExporter(&CSVOptions{IncludeHeaders: true, MaxRows: 2})

	_, err := xlsx.Export(ctx, src)
	require.NoError(t, err, "ExcelExporter.Export")
	require.NoError(t, xlsx.ExportToWriter(ctx, &bytes.Buffer{}, src), "ExcelExporter.ExportToWriter")

	var buf bytes.Buffer
	require.NoError(t, csvExporter.ExportToWriter(ctx, &buf, src), "CSVExporter.ExportToWriter")
	assert.Equal(t, "N\n1\n2\n", buf.String())
}
