package excel

import (
	"context"
	"fmt"
	"time"

	"github.com/xuri/excelize/v2"
)

// Exporter exports data to Excel format
type Exporter interface {
	Export(ctx context.Context, datasource DataSource) ([]byte, error)
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

		if err := e.writeRow(f, sheetName, rowNum, row); err != nil {
			return nil, fmt.Errorf("failed to write row %d: %w", rowNum, err)
		}

		rowNum++
		rowCount++
	}

	// Apply styling
	if err := e.applyStyles(f, sheetName, len(headers), rowNum-1); err != nil {
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

		// Format value based on type
		formattedValue := formatValue(value, e.options)

		if err := f.SetCellValue(sheet, cell, formattedValue); err != nil {
			return err
		}

		// Set number format for specific types
		switch v := value.(type) {
		case time.Time, *time.Time:
			style, _ := f.NewStyle(&excelize.Style{
				NumFmt: 22, // m/d/yy h:mm
			})
			_ = f.SetCellStyle(sheet, cell, cell, style)
		case float64, float32:
			style, _ := f.NewStyle(&excelize.Style{
				NumFmt: 2, // 0.00
			})
			_ = f.SetCellStyle(sheet, cell, cell, style)
		case int, int64, int32:
			if v != nil {
				style, _ := f.NewStyle(&excelize.Style{
					NumFmt: 1, // 0
				})
				_ = f.SetCellStyle(sheet, cell, cell, style)
			}
		}
	}
	return nil
}

// applyStyles applies styling to the Excel file
func (e *ExcelExporter) applyStyles(f *excelize.File, sheet string, colCount, rowCount int) error {
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

	// Apply data style and alternate row coloring
	if e.styleOptions.DataStyle != nil || e.styleOptions.AlternateRow {
		startRow := 1
		if e.options.IncludeHeaders {
			startRow = 2
		}

		for row := startRow; row <= rowCount; row++ {
			var style int
			var err error

			if e.styleOptions.AlternateRow && row%2 == 0 {
				// Create alternate row style
				altStyle := &CellStyle{
					Font:      e.styleOptions.DataStyle.Font,
					Alignment: e.styleOptions.DataStyle.Alignment,
					Fill: &FillStyle{
						Type:    "pattern",
						Pattern: 1,
						Color:   "#F5F5F5",
					},
				}
				style, err = e.createStyle(f, altStyle)
			} else if e.styleOptions.DataStyle != nil {
				style, err = e.createStyle(f, e.styleOptions.DataStyle)
			}

			if err != nil {
				return err
			}

			if style > 0 {
				endCol, _ := excelize.ColumnNumberToName(colCount)
				startCell := fmt.Sprintf("A%d", row)
				endCell := fmt.Sprintf("%s%d", endCol, row)
				if err := f.SetCellStyle(sheet, startCell, endCell, style); err != nil {
					return err
				}
			}
		}
	}

	return nil
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
