package export

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// PreparedWorkbook is a complete request-owned XLSX. Close removes its private
// directory. It is never placed in a report cache or published before success.
type PreparedWorkbook struct {
	file *os.File
	dir  string
	size int64
	ctx  context.Context
}

func (e *Exporter) Prepare(ctx context.Context, req Request) (*PreparedWorkbook, error) {
	opts := SpoolOptions{}
	if req.Spool != nil {
		opts = *req.Spool
	}
	dir, err := os.MkdirTemp(opts.Directory, "lens-workbook-")
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, "workbook.xlsx"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	result := &PreparedWorkbook{file: file, dir: dir, ctx: ctx}
	opts.Directory = dir
	req.Spool = &opts
	if err := e.Write(ctx, file, req); err != nil {
		_ = result.Close()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	result.size = info.Size()
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = result.Close()
		return nil, err
	}
	return result, nil
}

func (p *PreparedWorkbook) Size() int64 { return p.size }
func (p *PreparedWorkbook) Close() error {
	err := p.file.Close()
	removeErr := os.RemoveAll(p.dir)
	if err != nil {
		return err
	}
	return removeErr
}
func (p *PreparedWorkbook) WriteTo(writer io.Writer) (int64, error) {
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	return io.Copy(&boundedExportWriter{ctx: p.ctx, writer: writer, limit: p.size}, io.NewSectionReader(p.file, 0, p.size))
}

// WriteHTTP sets download headers only after Prepare has completed successfully.
func (p *PreparedWorkbook) WriteHTTP(w http.ResponseWriter, filename string) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", ContentDisposition(filename))
	w.Header().Set("Content-Length", strconv.FormatInt(p.size, 10))
	_, err := p.WriteTo(w)
	return err
}

func (e *Exporter) WriteHTTP(ctx context.Context, w http.ResponseWriter, filename string, req Request) error {
	prepared, err := e.Prepare(ctx, req)
	if err != nil {
		return err
	}
	defer func() { _ = prepared.Close() }()
	return prepared.WriteHTTP(w, filename)
}
