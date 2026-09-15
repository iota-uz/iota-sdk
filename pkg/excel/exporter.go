// Package excel provides this package.
package excel

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// decimalNumberRe matches a whole-cell plain decimal number (single dot, digits
// only) — e.g. "14773814.00". It deliberately excludes dates ("21.01.2026", two
// dots) and space/letter-formatted money ("14 773 814.00 UZS"), so those are
// left untouched by the comma conversion.
var decimalNumberRe = regexp.MustCompile(`^-?\d+\.\d+$`)

// toDecimalComma renders numeric values with a comma decimal separator so
// locale-neutral Excel (ru/uz) shows "14773814,00". Go floats become 2-dp comma
// strings; plain decimal-number strings (pgx numeric columns arrive as text to
// avoid precision loss) have their dot swapped. Everything else passes through.
func toDecimalComma(v interface{}) interface{} {
	switch f := v.(type) {
	case float64:
		return strings.Replace(strconv.FormatFloat(f, 'f', 2, 64), ".", ",", 1)
	case float32:
		return strings.Replace(strconv.FormatFloat(float64(f), 'f', 2, 32), ".", ",", 1)
	case string:
		if decimalNumberRe.MatchString(f) {
			return strings.Replace(f, ".", ",", 1)
		}
	}
	return v
}

// Exporter exports data to Excel format
type Exporter interface {
	Export(ctx context.Context, datasource DataSource) ([]byte, error)
	// ExportToWriter streams the export to w with flat memory (large exports).
	ExportToWriter(ctx context.Context, w io.Writer, datasource DataSource) error
}

// ExcelExporter implements Exporter using excelize
type ExcelExporter struct {
	options      *ExportOptions
	styleOptions *StyleOptions
}

// NewExcelExporter creates a new Excel exporter
func NewExcelExporter(opts *ExportOptions, styleOpts *StyleOptions) *ExcelExporter {
	if opts == nil {
		opts = DefaultOptions()
	}
	if styleOpts == nil {
		styleOpts = DefaultStyleOptions()
	}
	return &ExcelExporter{
		options:      opts,
		styleOptions: styleOpts,
	}
}

// Export exports data from the datasource to Excel format
func (e *ExcelExporter) Export(ctx context.Context, datasource DataSource) ([]byte, error) {
	f := excelize.NewFile()
	sheetName := datasource.GetSheetName()

	// Create sheet
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(index)

	// Delete default sheet if it exists
	_ = f.DeleteSheet("Sheet1")

	// Get headers
	headers := datasource.GetHeaders()
	if len(headers) == 0 {
		return nil, fmt.Errorf("no columns found in data source")
	}

	rowNum := 1

	// Write headers if enabled
	if e.options.IncludeHeaders {
		if err := e.writeHeaders(f, sheetName, headers); err != nil {
			return nil, fmt.Errorf("failed to write headers: %w", err)
		}
		rowNum++
	}

	// Get row iterator
	getRow, err := datasource.GetRows(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get rows: %w", err)
	}

	// Write data rows
	rowCount := 0
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		row, err := getRow()
		if err != nil {
			return nil, fmt.Errorf("failed to get row: %w", err)
		}
		if row == nil {
			break // No more rows
		}

		if e.options.MaxRows > 0 && rowCount >= e.options.MaxRows {
			break
		}

		if err := e.applyDataStyle(f, sheetName, rowNum, len(headers)); err != nil {
			return nil, fmt.Errorf("failed to style row %d: %w", rowNum, err)
		}
		if err := e.writeRow(f, sheetName, rowNum, row); err != nil {
			return nil, fmt.Errorf("failed to write row %d: %w", rowNum, err)
		}

		rowNum++
		rowCount++
	}

	if err := e.applyHeaderStyle(f, sheetName, len(headers)); err != nil {
		return nil, fmt.Errorf("failed to apply styles: %w", err)
	}

	// Auto-fit columns
	for i := 0; i < len(headers); i++ {
		col, _ := excelize.ColumnNumberToName(i + 1)
		_ = f.SetColWidth(sheetName, col, col, 15)
	}

	// Apply options
	if e.options.IncludeHeaders {
		if e.options.AutoFilter {
			endCol, _ := excelize.ColumnNumberToName(len(headers))
			_ = f.AutoFilter(sheetName, fmt.Sprintf("A1:%s1", endCol), nil)
		}

		if e.options.FreezeHeader {
			_ = f.SetPanes(sheetName, &excelize.Panes{
				Freeze:      true,
				Split:       false,
				XSplit:      0,
				YSplit:      1,
				TopLeftCell: "A2",
				ActivePane:  "bottomLeft",
			})
		}
	}

	// Get buffer
	buffer, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write to buffer: %w", err)
	}

	return buffer.Bytes(), nil
}

// ExportToWriter streams the datasource to w as an .xlsx using excelize's
// StreamWriter. Unlike Export (which materializes the whole workbook in memory
// and then re-walks every row to apply styles), StreamWriter spools rows to a
// temp file so peak memory stays flat regardless of row count, and styles are
// attached per cell at write time (no O(N) post-pass). Use this for large
// exports; Export remains for small exports that want richer styling.
//
// Trade-off vs Export: no alternate-row striping or per-cell borders
// (StreamWriter cannot style a range after the row is flushed). Header styling,
// column widths and numeric/date cell formats are preserved — so money stays a
// real number, not text.
func (e *ExcelExporter) ExportToWriter(ctx context.Context, w io.Writer, datasource DataSource) error {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheetName := datasource.GetSheetName()
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create sheet: %w", err)
	}
	f.SetActiveSheet(index)
	_ = f.DeleteSheet("Sheet1")

	headers := datasource.GetHeaders()
	if len(headers) == 0 {
		return fmt.Errorf("no columns found in data source")
	}

	sw, err := f.NewStreamWriter(sheetName)
	if err != nil {
		return fmt.Errorf("failed to create stream writer: %w", err)
	}

	// Column widths must be set before any row is written.
	for i := range headers {
		_ = sw.SetColWidth(i+1, i+1, 15)
	}

	// Pre-create reusable styles once (StreamWriter attaches them per cell).
	floatStyle, err := createNumberStyle(f, 2, e.options.FloatNumberFormat) // 0.00 by default
	if err != nil {
		return fmt.Errorf("failed to create numeric style: %w", err)
	}
	timeStyle, err := f.NewStyle(&excelize.Style{NumFmt: 22}) // m/d/yy h:mm
	if err != nil {
		return fmt.Errorf("failed to create datetime style: %w", err)
	}
	intStyle, err := f.NewStyle(&excelize.Style{NumFmt: 1}) // 0
	if err != nil {
		return fmt.Errorf("failed to create integer style: %w", err)
	}

	rowNum := 1
	if e.options.IncludeHeaders {
		var headerStyle int
		if e.styleOptions != nil && e.styleOptions.HeaderStyle != nil {
			if headerStyle, err = e.createStyle(f, e.styleOptions.HeaderStyle); err != nil {
				return fmt.Errorf("failed to create header style: %w", err)
			}
		}
		cells := make([]interface{}, len(headers))
		for i, h := range headers {
			cells[i] = excelize.Cell{StyleID: headerStyle, Value: h}
		}
		if err := sw.SetRow("A1", cells); err != nil {
			return fmt.Errorf("failed to write headers: %w", err)
		}
		rowNum++
	}

	getRow, err := datasource.GetRows(ctx)
	if err != nil {
		return fmt.Errorf("failed to get rows: %w", err)
	}

	rowCount := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		row, err := getRow()
		if err != nil {
			return fmt.Errorf("failed to get row: %w", err)
		}
		if row == nil {
			break
		}
		if e.options.MaxRows > 0 && rowCount >= e.options.MaxRows {
			break
		}

		cellRef, _ := excelize.CoordinatesToCellName(1, rowNum)
		cells := make([]interface{}, len(row))
		for i, v := range row {
			val := convertPgxValue(v)
			if e.options != nil && e.options.DecimalComma {
				val = toDecimalComma(val)
			}
			cells[i] = streamCell(val, floatStyle, timeStyle, intStyle)
		}
		if err := sw.SetRow(cellRef, cells); err != nil {
			return fmt.Errorf("failed to write row %d: %w", rowNum, err)
		}

		rowNum++
		rowCount++
	}

	if err := sw.Flush(); err != nil {
		return fmt.Errorf("failed to flush stream writer: %w", err)
	}

	if err := f.Write(w); err != nil {
		return fmt.Errorf("failed to write workbook: %w", err)
	}
	return nil
}

// streamCell wraps a normalized value in an excelize.Cell carrying the right
// number format so numeric/date cells are typed (not text). Strings and other
// types pass through unstyled.
func streamCell(v interface{}, floatStyle, timeStyle, intStyle int) interface{} {
	switch t := v.(type) {
	case time.Time:
		return excelize.Cell{StyleID: timeStyle, Value: t}
	case *time.Time:
		if t == nil {
			return nil
		}
		return excelize.Cell{StyleID: timeStyle, Value: *t}
	case float64, float32:
		return excelize.Cell{StyleID: floatStyle, Value: v}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return excelize.Cell{StyleID: intStyle, Value: v}
	default:
		return v
	}
}

// writeHeaders writes header row to the Excel file
func (e *ExcelExporter) writeHeaders(f *excelize.File, sheet string, headers []string) error {
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, header); err != nil {
			return err
		}
	}
	return nil
}

// writeRow writes a data row to the Excel file
func (e *ExcelExporter) writeRow(f *excelize.File, sheet string, rowNum int, row []interface{}) error {
	for i, value := range row {
		cell, _ := excelize.CoordinatesToCellName(i+1, rowNum)
		normalizedValue := convertPgxValue(value)

		// Format value based on type
		formattedValue := formatValue(normalizedValue, e.options)

		decimalComma := e.options != nil && e.options.DecimalComma
		if decimalComma {
			formattedValue = toDecimalComma(formattedValue)
		}

		if err := f.SetCellValue(sheet, cell, formattedValue); err != nil {
			return err
		}

		// Set number format for specific types
		switch normalizedValue.(type) {
		case time.Time, *time.Time:
			if err := applyCellNumFmt(f, sheet, cell, 22, ""); err != nil { // m/d/yy h:mm
				return err
			}
		case float64, float32:
			// In comma mode the float is now a text cell; a numeric NumFmt would
			// be a no-op, so skip it.
			if decimalComma {
				break
			}
			if err := applyCellNumFmt(f, sheet, cell, 2, e.options.FloatNumberFormat); err != nil { // 0.00 by default
				return err
			}
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			if err := applyCellNumFmt(f, sheet, cell, 1, ""); err != nil { // 0
				return err
			}
		}
	}
	return nil
}

func applyCellNumFmt(f *excelize.File, sheet, cell string, numFmt int, customNumFmt string) error {
	styleID, err := f.GetCellStyle(sheet, cell)
	if err != nil {
		return err
	}
	style, err := f.GetStyle(styleID)
	if err != nil {
		return err
	}
	setNumberFormat(style, numFmt, customNumFmt)
	styleID, err = f.NewStyle(style)
	if err != nil {
		return err
	}
	return f.SetCellStyle(sheet, cell, cell, styleID)
}

func createNumberStyle(f *excelize.File, numFmt int, customNumFmt string) (int, error) {
	style := &excelize.Style{}
	setNumberFormat(style, numFmt, customNumFmt)
	return f.NewStyle(style)
}

func setNumberFormat(style *excelize.Style, numFmt int, customNumFmt string) {
	style.NumFmt = numFmt
	style.CustomNumFmt = nil
	if customNumFmt != "" {
		style.NumFmt = 0
		style.CustomNumFmt = &customNumFmt
	}
}

func (e *ExcelExporter) applyHeaderStyle(f *excelize.File, sheet string, colCount int) error {
	if e.styleOptions == nil {
		return nil
	}

	// Apply header style
	if e.options.IncludeHeaders && e.styleOptions.HeaderStyle != nil {
		headerStyle, err := e.createStyle(f, e.styleOptions.HeaderStyle)
		if err != nil {
			return err
		}

		endCol, _ := excelize.ColumnNumberToName(colCount)
		if err := f.SetCellStyle(sheet, "A1", fmt.Sprintf("%s1", endCol), headerStyle); err != nil {
			return err
		}
	}

	return nil
}

func (e *ExcelExporter) applyDataStyle(f *excelize.File, sheet string, row, colCount int) error {
	if e.styleOptions == nil || (e.styleOptions.DataStyle == nil && !e.styleOptions.AlternateRow) {
		return nil
	}

	var cellStyle *CellStyle
	if e.styleOptions.AlternateRow && row%2 == 0 {
		cellStyle = &CellStyle{
			Fill: &FillStyle{Type: "pattern", Pattern: 1, Color: "#F5F5F5"},
		}
		if e.styleOptions.DataStyle != nil {
			cellStyle.Font = e.styleOptions.DataStyle.Font
			cellStyle.Alignment = e.styleOptions.DataStyle.Alignment
		}
	} else {
		cellStyle = e.styleOptions.DataStyle
	}
	if cellStyle == nil {
		return nil
	}

	styleID, err := e.createStyle(f, cellStyle)
	if err != nil {
		return err
	}
	endCol, _ := excelize.ColumnNumberToName(colCount)
	return f.SetCellStyle(
		sheet,
		fmt.Sprintf("A%d", row),
		fmt.Sprintf("%s%d", endCol, row),
		styleID,
	)
}

// createStyle creates an excelize style from CellStyle
func (e *ExcelExporter) createStyle(f *excelize.File, cellStyle *CellStyle) (int, error) {
	style := &excelize.Style{}

	if cellStyle.Font != nil {
		style.Font = &excelize.Font{
			Bold:   cellStyle.Font.Bold,
			Italic: cellStyle.Font.Italic,
			Size:   float64(cellStyle.Font.Size),
			Color:  cellStyle.Font.Color,
		}
	}

	if cellStyle.Fill != nil {
		style.Fill = excelize.Fill{
			Type:    cellStyle.Fill.Type,
			Pattern: cellStyle.Fill.Pattern,
			Color:   []string{cellStyle.Fill.Color},
		}
	}

	if cellStyle.Alignment != nil {
		style.Alignment = &excelize.Alignment{
			Horizontal: cellStyle.Alignment.Horizontal,
			Vertical:   cellStyle.Alignment.Vertical,
			WrapText:   cellStyle.Alignment.WrapText,
		}
	}

	return f.NewStyle(style)
}
