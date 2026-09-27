package excel

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
)

// utf8BOM lets Excel on Windows open UTF-8 CSV (Cyrillic, Uzbek) correctly.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// CSVOptions configures CSV output.
type CSVOptions struct {
	// IncludeHeaders writes the data source headers as the first record
	// (CSVExporter only; CSVWriter callers write headers explicitly).
	IncludeHeaders bool
	// IncludeBOM starts the output with a UTF-8 byte order mark.
	IncludeBOM bool
	// Comma is the field delimiter; zero means ','.
	Comma rune
	// DecimalComma renders floats and plain decimal strings with a comma
	// decimal separator ("14773814,00") for locale-neutral ru/uz spreadsheets,
	// the same conversion ExportOptions.DecimalComma applies to XLSX.
	DecimalComma bool
	// DateTimeFormat formats time.Time values; empty means time.Time.String().
	DateTimeFormat string
	// MaxRows limits the number of data rows (0 = no limit; CSVExporter only).
	MaxRows int
}

// DefaultCSVOptions returns headers on, a UTF-8 BOM and ',' as delimiter.
func DefaultCSVOptions() *CSVOptions {
	return &CSVOptions{IncludeHeaders: true, IncludeBOM: true, Comma: ','}
}

// CSVWriter writes spreadsheet-safe CSV records. Every plain string cell goes
// through NeutralizeFormula, so user text can never open as a formula; only a
// Formula value is written as one. Use it for streaming exports that do not
// fit a DataSource; CSVExporter wraps it for DataSource exports.
type CSVWriter struct {
	w    *csv.Writer
	opts CSVOptions
}

// NewCSVWriter starts CSV output on w, writing the BOM when opts asks for it.
// A nil opts means DefaultCSVOptions.
func NewCSVWriter(w io.Writer, opts *CSVOptions) (*CSVWriter, error) {
	if opts == nil {
		opts = DefaultCSVOptions()
	}
	if opts.IncludeBOM {
		if _, err := w.Write(utf8BOM); err != nil {
			return nil, fmt.Errorf("failed to write BOM: %w", err)
		}
	}
	cw := csv.NewWriter(w)
	if opts.Comma != 0 {
		cw.Comma = opts.Comma
	}
	return &CSVWriter{w: cw, opts: *opts}, nil
}

// WriteHeader writes a header record; header text is neutralized like data.
func (c *CSVWriter) WriteHeader(headers []string) error {
	record := make([]string, len(headers))
	for i, h := range headers {
		record[i] = NeutralizeFormula(h)
	}
	return c.w.Write(record)
}

// WriteRow renders values like CSVCell, with this writer's options, and
// writes them as one record.
func (c *CSVWriter) WriteRow(values []interface{}) error {
	record := make([]string, len(values))
	for i, v := range values {
		record[i] = c.cell(v)
	}
	return c.w.Write(record)
}

// Flush writes buffered records and reports any write error so far.
func (c *CSVWriter) Flush() error {
	c.w.Flush()
	return c.w.Error()
}

// CSVCell renders one export value as CSV text with default options: a
// Formula becomes "=<expression>", floats keep two decimals, nil is empty,
// and every other value is formatted with %v and neutralized.
func CSVCell(v interface{}) string {
	return (&CSVWriter{opts: CSVOptions{}}).cell(v)
}

// cell renders one value with the writer's options; see CSVCell.
func (c *CSVWriter) cell(v interface{}) string {
	value := convertPgxValue(v)
	if c.opts.DecimalComma {
		value = toDecimalComma(value)
	}
	switch t := value.(type) {
	case nil:
		return ""
	case Formula:
		return "=" + formulaExpression(t)
	case *Formula:
		if t == nil {
			return ""
		}
		return "=" + formulaExpression(*t)
	case float64:
		return strconv.FormatFloat(t, 'f', 2, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', 2, 32)
	case time.Time:
		return NeutralizeFormula(c.time(t))
	case *time.Time:
		if t == nil {
			return ""
		}
		return NeutralizeFormula(c.time(*t))
	case string:
		return NeutralizeFormula(t)
	default:
		return NeutralizeFormula(fmt.Sprintf("%v", t))
	}
}

// time formats t with DateTimeFormat, or time.Time.String() when unset.
func (c *CSVWriter) time(t time.Time) string {
	if c.opts.DateTimeFormat != "" {
		return t.Format(c.opts.DateTimeFormat)
	}
	return t.String()
}

// CSVExporter exports a DataSource as spreadsheet-safe CSV. It implements
// Exporter, so it can stand wherever an ExcelExporter does.
type CSVExporter struct {
	options *CSVOptions
}

var _ Exporter = (*CSVExporter)(nil)

// NewCSVExporter creates a CSV exporter; nil opts means DefaultCSVOptions.
func NewCSVExporter(opts *CSVOptions) *CSVExporter {
	if opts == nil {
		opts = DefaultCSVOptions()
	}
	return &CSVExporter{options: opts}
}

// Export renders the whole data source into memory.
func (e *CSVExporter) Export(ctx context.Context, datasource DataSource) ([]byte, error) {
	var buf bytes.Buffer
	if err := e.ExportToWriter(ctx, &buf, datasource); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ExportToWriter streams the data source to w row by row.
func (e *CSVExporter) ExportToWriter(ctx context.Context, w io.Writer, datasource DataSource) error {
	cw, err := NewCSVWriter(w, e.options)
	if err != nil {
		return err
	}
	if e.options.IncludeHeaders {
		if err := cw.WriteHeader(datasource.GetHeaders()); err != nil {
			return fmt.Errorf("failed to write headers: %w", err)
		}
	}

	getRow, err := datasource.GetRows(ctx)
	if err != nil {
		return fmt.Errorf("failed to get rows: %w", err)
	}
	for rowCount := 0; ; rowCount++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if e.options.MaxRows > 0 && rowCount >= e.options.MaxRows {
			break
		}
		row, err := getRow()
		if err != nil {
			return fmt.Errorf("failed to get row: %w", err)
		}
		if row == nil {
			break
		}
		if err := cw.WriteRow(row); err != nil {
			return fmt.Errorf("failed to write row %d: %w", rowCount+1, err)
		}
	}
	if err := cw.Flush(); err != nil {
		return fmt.Errorf("failed to flush CSV: %w", err)
	}
	return nil
}
