package export

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	"github.com/xuri/excelize/v2"
)

// EvidenceSource describes one export-only sheet without retaining its rows.
// Fields must contain metadata only (no Values). Rows is called synchronously,
// once for each selected source, never in parallel. It must close any cursor
// with defer, honor ctx, and return the first yield error. Rows may reuse its
// row slice after yield returns; the exporter never retains that slice.
type EvidenceSource struct {
	Fields []frame.Field
	Meta   frame.FrameMeta
	Rows   func(context.Context, func([]any) error) error
}

// SpoolOptions bounds request-owned export work independently of cached Lens
// payloads. Budget incrementally reserves conservative buffer capacity per
// streamed sheet until the workbook closes, not the size of all logical rows.
// Directory selects the parent of a private temporary directory. Zero limits
// use 256 MiB combined spool/archive, Excel's row bound, and 1 MiB per row.
type SpoolOptions struct {
	Directory   string
	MaxBytes    int64
	MaxRows     int
	MaxRowBytes int64
	Budget      *resource.Budget
}

var exportWorkingBudget = resource.NewBudget(resource.DefaultLimit)

type exportWork struct {
	dir          string
	options      SpoolOptions
	xmlBytes     int64
	reservations []*resource.Reservation
}

func newExportWork(ctx context.Context, options *SpoolOptions) (*exportWork, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts := SpoolOptions{}
	if options != nil {
		opts = *options
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 256 << 20
	}
	if opts.MaxRows <= 0 || opts.MaxRows > 1048575 {
		opts.MaxRows = 1048575
	}
	if opts.MaxRowBytes <= 0 {
		opts.MaxRowBytes = 1 << 20
	}
	if opts.Budget == nil {
		opts.Budget = exportWorkingBudget
	}
	dir, err := os.MkdirTemp(opts.Directory, "lens-export-")
	if err != nil {
		return nil, err
	}
	return &exportWork{dir: dir, options: opts, xmlBytes: 1 << 20}, nil
}

func (w *exportWork) close() {
	for _, reservation := range w.reservations {
		reservation.Release()
	}
	_ = os.RemoveAll(w.dir)
}

func (w *exportWork) chargeRow(values []any) error {
	if _, err := resource.Estimate(values, w.options.MaxRowBytes); err != nil {
		return err
	}
	// Inline XML can expand a Unicode scalar to an entity. Charge the actual
	// escaped text bound plus ample cell/row markup before StreamWriter writes.
	size := int64(128 + 192*len(values))
	for _, value := range values {
		text := fmt.Sprint(value)
		if utf8.RuneCountInString(text) > 32767 {
			return fmt.Errorf("export cell exceeds Excel text limit")
		}
		size += int64(len(text) + 5*strings.Count(text, "&") + 5*strings.Count(text, "<") + 5*strings.Count(text, ">") + 5*strings.Count(text, "\"") + 5*strings.Count(text, "'") + 5*strings.Count(text, "\r") + 7*strings.Count(text, "_x"))
		for _, character := range text {
			if character < 0x20 && character != '\t' && character != '\n' && character != '\r' {
				size += 6
			}
		}
	}
	if size > w.options.MaxBytes-w.xmlBytes {
		return resource.NewLimitError("export spool bytes", w.xmlBytes+size, w.options.MaxBytes)
	}
	w.xmlBytes += size
	return nil
}

func (w *exportWork) writeEvidence(ctx context.Context, book *workbook, sheet string, source EvidenceSource, spec exportmeta.Spec) error {
	if source.Rows == nil || len(source.Fields) == 0 || len(source.Fields) > 16384 {
		return fmt.Errorf("invalid evidence schema")
	}
	schemaBytes, err := resource.Estimate(source.Fields, w.options.MaxRowBytes)
	if err != nil {
		return err
	}
	for _, field := range source.Fields {
		if len(field.Values) != 0 {
			return fmt.Errorf("stream evidence schema must not retain values")
		}
	}
	sheetBytes, reserved := int64(0), int64(0)
	charge := func(values []any) error {
		before := w.xmlBytes
		if err := w.chargeRow(values); err != nil {
			return err
		}
		sheetBytes += w.xmlBytes - before
		const chunk = int64(1 << 20)
		target := schemaBytes + w.options.MaxRowBytes + min(2*((sheetBytes+chunk-1)/chunk)*chunk, 2*((16<<20)+w.options.MaxRowBytes))
		if target > reserved {
			reservation, err := w.options.Budget.Reserve(target - reserved)
			if err != nil {
				return err
			}
			w.reservations = append(w.reservations, reservation)
			reserved = target
		}
		return nil
	}
	if err := charge(nil); err != nil {
		return err
	}
	stream, err := book.file.NewStreamWriter(sheet)
	if err != nil {
		return err
	}
	if err := stream.SetColWidth(1, len(source.Fields), 20); err != nil {
		return err
	}
	headerRow := 1
	if source.Meta.Title != "" {
		headerRow++
	}
	if spec.FreezeHeader {
		if err := stream.SetPanes(&excelize.Panes{Freeze: true, YSplit: headerRow, TopLeftCell: cell(1, headerRow+1), ActivePane: "bottomLeft"}); err != nil {
			return err
		}
	}
	row := 1
	if source.Meta.Title != "" {
		if err := charge([]any{source.Meta.Title}); err != nil {
			return err
		}
		if err := stream.SetRow(cell(1, row), []any{source.Meta.Title}); err != nil {
			return err
		}
		row++
	}
	headers := make([]any, len(source.Fields))
	for i, field := range source.Fields {
		headers[i] = fieldLabel(field)
	}
	if err := charge(headers); err != nil {
		return err
	}
	if err := stream.SetRow(cell(1, row), headers); err != nil {
		return err
	}
	row++
	var yieldErr error
	err = source.Rows(ctx, func(values []any) error {
		if yieldErr != nil {
			return yieldErr
		}
		write := func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if len(values) != len(source.Fields) {
				return fmt.Errorf("evidence row has %d cells, expected %d", len(values), len(source.Fields))
			}
			if row-headerRow > w.options.MaxRows || row > 1048576 {
				return resource.NewLimitError("export rows", int64(row-headerRow), int64(w.options.MaxRows))
			}
			if err := charge(values); err != nil {
				return err
			}
			cells := make([]any, len(values))
			for i, value := range values {
				exact := exactValue(value)
				value, err = normalizeExactValue(value)
				if err != nil {
					return err
				}
				item := excelize.Cell{Value: excelValue(value)}
				switch v := value.(type) {
				case frame.Formula:
					item.Formula = strings.TrimPrefix(strings.TrimSpace(v.Expression), "=")
					item.Value = excelValue(v.Result)
				case *frame.Formula:
					if v == nil {
						item.Value = nil
					} else {
						item.Formula = strings.TrimPrefix(strings.TrimSpace(v.Expression), "=")
						item.Value = excelValue(v.Result)
					}
				case frame.Hyperlink:
					item.Formula = fmt.Sprintf(`HYPERLINK("%s","%s")`, excelFormulaString(v.URL), excelFormulaString(v.Label))
					item.Value = v.Label
				case *frame.Hyperlink:
					if v == nil {
						item.Value = nil
					} else {
						item.Formula = fmt.Sprintf(`HYPERLINK("%s","%s")`, excelFormulaString(v.URL), excelFormulaString(v.Label))
						item.Value = v.Label
					}
				}
				format := numberFormat(hintFormats(source.Fields[i]), value)
				if exact && hintFormats(source.Fields[i]) == nil {
					format = "General"
				}
				if format != "" {
					id, err := book.style(format)
					if err != nil {
						return err
					}
					item.StyleID = id
				}
				cells[i] = item
			}
			if err := stream.SetRow(cell(1, row), cells); err != nil {
				return err
			}
			row++
			return nil
		}
		yieldErr = write()
		return yieldErr
	})
	if yieldErr != nil {
		return yieldErr
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(spec.TableName) != "" {
		if row == headerRow+1 {
			if err := stream.SetRow(cell(1, row), make([]any, len(source.Fields))); err != nil {
				return err
			}
			row++
		}
		if err := stream.AddTable(&excelize.Table{Range: cell(1, headerRow) + ":" + cell(len(source.Fields), row-1), Name: spec.TableName, StyleName: "TableStyleMedium2", ShowRowStripes: pointer(true)}); err != nil {
			return err
		}
	}
	if err := stream.Flush(); err != nil {
		return err
	}
	return setSheetDimension(book.file, sheet, row-1, len(source.Fields))
}

type boundedExportWriter struct {
	ctx            context.Context
	writer         io.Writer
	written, limit int64
}

func (w *boundedExportWriter) Write(data []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(data)) > w.limit-w.written {
		return 0, resource.NewLimitError("export spool bytes", w.written+int64(len(data)), w.limit)
	}
	n, err := w.writer.Write(data)
	w.written += int64(n)
	return n, err
}

func (w *exportWork) zipWriter(ctx context.Context, output io.Writer) excelize.ZipWriter {
	return zip.NewWriter(&boundedExportWriter{ctx: ctx, writer: output, limit: w.options.MaxBytes - w.xmlBytes})
}
