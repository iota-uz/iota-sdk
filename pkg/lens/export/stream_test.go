package export

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/resource"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func streamingRequest(t *testing.T) Request {
	t.Helper()
	chart := mustFrames(t, "chart", frame.Field{Name: "value", Values: []any{42}})
	p := panel.Spec{ID: "summary", Kind: panel.KindStat, Dataset: "chart", Fields: panel.FieldMapping{Value: panel.Ref("value")}}
	result := &lensruntime.DashboardResult{Spec: lens.DashboardSpec{Title: "Audit", Export: exportmeta.Spec{EvidenceDatasets: []string{"evidence"}},
		Datasets: []lens.DatasetSpec{{Name: "chart"}, {Name: "evidence", Export: exportmeta.Spec{SheetName: "Evidence", TableName: "EvidenceRows", FreezeHeader: true}}, {Name: "unused"}},
		Rows:     []lens.RowSpec{{Panels: []panel.Spec{p}}}}, Panels: map[string]*lensruntime.PanelResult{"summary": {Panel: p, Frames: chart}}, Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}}}
	return Request{Result: result, Spool: &SpoolOptions{Directory: t.TempDir(), Budget: resource.NewBudget(64 << 20)}}
}

// False green if the source is materialized first or unselected providers are
// opened: reuse one row, assert provider closure, and inspect the saved workbook.
func TestStreamingEvidenceSelectionTypesAndMetadata(t *testing.T) {
	t.Parallel()
	req := streamingRequest(t)
	called, closed := 0, false
	req.EvidenceSources = map[string]EvidenceSource{
		"unused": {Rows: func(context.Context, func([]any) error) error { t.Fatal("unselected source opened"); return nil }},
		"evidence": {Fields: []frame.Field{{Name: "identifier"}, {Name: "amount", Type: frame.FieldTypeNumber}, {Name: "issued", Type: frame.FieldTypeTime}, {Name: "verified"}}, Meta: frame.FrameMeta{Title: "Audit evidence"},
			Rows: func(ctx context.Context, yield func([]any) error) error {
				called++
				defer func() { closed = true }()
				return yield([]any{"000123", decimal.RequireFromString("12.34"), time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), frame.Formula{Expression: "B3*2", Result: decimal.RequireFromString("24.68")}})
			}},
	}
	names, err := RequiredEvidence(req.Result.Spec, "")
	require.NoError(t, err)
	require.Equal(t, []string{"evidence"}, names)
	require.NoError(t, New().WriteHTTP(context.Background(), httptest.NewRecorder(), "audit.xlsx", Request{Result: req.Result, PanelID: "summary", Spool: req.Spool, EvidenceSources: req.EvidenceSources}))
	require.Zero(t, called, "panel export must not open dashboard-only evidence")
	prepared, err := New().Prepare(context.Background(), req)
	require.NoError(t, err)
	defer func() { _ = prepared.Close() }()
	require.Equal(t, 1, called)
	require.True(t, closed)
	book, err := excelize.OpenFile(prepared.file.Name())
	require.NoError(t, err)
	defer func() { _ = book.Close() }()
	for cell, want := range map[string]string{"A1": "Audit evidence", "A3": "000123", "B3": "12.34"} {
		got, err := book.GetCellValue("Evidence", cell)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
	formula, err := book.GetCellFormula("Evidence", "D3")
	require.NoError(t, err)
	require.Equal(t, "B3*2", formula)
	raw, err := book.GetCellValue("Evidence", "C3", excelize.Options{RawCellValue: true})
	require.NoError(t, err)
	require.NotContains(t, raw, "2026")
	display, err := book.GetCellValue("Evidence", "C3")
	require.NoError(t, err)
	require.Equal(t, "01.06.2026", display)
	tables, err := book.GetTables("Evidence")
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "A2:D3", tables[0].Range)
	panes, err := book.GetPanes("Evidence")
	require.NoError(t, err)
	require.True(t, panes.Freeze)
	require.Equal(t, 2, panes.YSplit)
	require.Zero(t, req.Spool.Budget.Used(), "working reservation must release after Prepare")
}

// False green if repetitive data only checks ZIP length: count every inline
// cell in uncompressed worksheet XML, whose logical strings exceed 64 MiB.
func TestStreamingEvidenceExceedsCachedPayloadWithoutRetainingRows(t *testing.T) {
	req := streamingRequest(t)
	const count = 70000
	closed := false
	spilled := false
	req.EvidenceSources = map[string]EvidenceSource{"evidence": {Fields: []frame.Field{{Name: "payload"}}, Rows: func(ctx context.Context, yield func([]any) error) error {
		defer func() { closed = true }()
		row := make([]any, 1)
		for i := 0; i < count; i++ {
			row[0] = fmt.Sprintf("%06d", i) + strings.Repeat("x", 1018)
			if err := yield(row); err != nil {
				return err
			}
		}
		if err := filepath.WalkDir(req.Spool.Directory, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), "excelize-") {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if info.Size() > 32<<20 {
					spilled = true
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}}}
	prepared, err := New().Prepare(context.Background(), req)
	require.NoError(t, err)
	defer func() { _ = prepared.Close() }()
	require.True(t, closed)
	require.True(t, spilled, "large worksheet must spill to the private disk workspace")
	archive, err := zip.OpenReader(prepared.file.Name())
	require.NoError(t, err)
	defer func() { _ = archive.Close() }()
	var cells int
	var last string
	for _, part := range archive.File {
		if !strings.HasPrefix(part.Name, "xl/worksheets/sheet") {
			continue
		}
		reader, err := part.Open()
		require.NoError(t, err)
		decoder := xml.NewDecoder(reader)
		localCells := 0
		first, localLast := "", ""
		for {
			token, err := decoder.Token()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			if start, ok := token.(xml.StartElement); ok && start.Name.Local == "t" {
				var value string
				require.NoError(t, decoder.DecodeElement(&value, &start))
				if localCells == 0 {
					first = value
				}
				localCells++
				localLast = value
			}
		}
		require.NoError(t, reader.Close())
		if first == "payload" {
			cells = localCells
			last = localLast
		}
	}
	require.Equal(t, count+1, cells)
	require.True(t, strings.HasPrefix(last, "069999"))
	require.Zero(t, req.Spool.Budget.Used())
}

// False green if failure occurs after headers or an archive fragment: every
// provider error/limit/cancellation must leave the response and spool empty.
func TestStreamingEvidenceFailureIsAtomicAndReleasesResources(t *testing.T) {
	for _, kind := range []string{"rows", "disk", "row_bytes", "cancel", "provider", "shape", "precision"} {
		t.Run(kind, func(t *testing.T) {
			req := streamingRequest(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			switch kind {
			case "rows":
				req.Spool.MaxRows = 1
			case "disk":
				req.Spool.MaxBytes = 2 << 20
			case "row_bytes":
				req.Spool.MaxRowBytes = 512
			}
			closed := false
			req.EvidenceSources = map[string]EvidenceSource{"evidence": {Fields: []frame.Field{{Name: "value"}}, Rows: func(ctx context.Context, yield func([]any) error) error {
				defer func() { closed = true }()
				if kind == "cancel" {
					cancel()
				}
				if kind == "provider" {
					return errors.New("provider failed")
				}
				if kind == "shape" {
					return yield([]any{})
				}
				if kind == "precision" {
					return yield([]any{decimal.RequireFromString("1234567890123456.78")})
				}
				for i := 0; i < 1000; i++ {
					if err := yield([]any{strings.Repeat("x", 2048)}); err != nil {
						return err
					}
				}
				return nil
			}}}
			response := httptest.NewRecorder()
			err := New().WriteHTTP(ctx, response, "audit.xlsx", req)
			require.Error(t, err)
			require.True(t, closed)
			require.Empty(t, response.Body.String())
			require.Empty(t, response.Header().Get("Content-Disposition"))
			entries, err := os.ReadDir(req.Spool.Directory)
			require.NoError(t, err)
			require.Empty(t, entries)
			require.Zero(t, req.Spool.Budget.Used())
		})
	}
}

func TestPreparedWorkbookCleanup(t *testing.T) {
	t.Parallel()
	req := streamingRequest(t)
	req.Result.Spec.Export.EvidenceDatasets = nil
	prepared, err := New().Prepare(context.Background(), req)
	require.NoError(t, err)
	path := prepared.file.Name()
	require.Positive(t, prepared.Size())
	require.NoError(t, prepared.Close())
	_, err = os.Stat(filepath.Dir(path))
	require.True(t, os.IsNotExist(err))
}

// False green if a fixed chunk is charged for empty sheets: all nine tiny audit
// sources must fit the default 64 MiB working budget, with capacity returned.
func TestStreamingNineTinySheetsFitDefaultWorkingBudget(t *testing.T) {
	req := streamingRequest(t)
	req.Spool.Budget = nil
	req.EvidenceSources = map[string]EvidenceSource{}
	req.Result.Spec.Export.EvidenceDatasets = nil
	for i := 0; i < 9; i++ {
		name := fmt.Sprintf("tiny_%d", i)
		req.Result.Spec.Datasets = append(req.Result.Spec.Datasets, lens.DatasetSpec{Name: name})
		req.Result.Spec.Export.EvidenceDatasets = append(req.Result.Spec.Export.EvidenceDatasets, name)
		req.EvidenceSources[name] = EvidenceSource{Fields: []frame.Field{{Name: "value"}}, Rows: func(context.Context, func([]any) error) error { return nil }}
	}
	prepared, err := New().Prepare(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, prepared.Close())
	require.Zero(t, exportWorkingBudget.Used())
}

// False green if working capacity is released after each sheet despite retained
// buffers: six >16 MiB sheets share 256 MiB until the archive is complete.
func TestStreamingSixLargeSheetsShareWorkingBudget(t *testing.T) {
	req := streamingRequest(t)
	req.Spool.Budget = resource.NewBudget(256 << 20)
	req.Result.Spec.Export.EvidenceDatasets = nil
	req.EvidenceSources = map[string]EvidenceSource{}
	completed := 0
	var peak int64
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("large_%d", i)
		req.Result.Spec.Datasets = append(req.Result.Spec.Datasets, lens.DatasetSpec{Name: name})
		req.Result.Spec.Export.EvidenceDatasets = append(req.Result.Spec.Export.EvidenceDatasets, name)
		req.EvidenceSources[name] = EvidenceSource{Fields: []frame.Field{{Name: "payload"}}, Rows: func(ctx context.Context, yield func([]any) error) error {
			row := []any{strings.Repeat("x", 1024)}
			for range 24000 {
				if err := yield(row); err != nil {
					return err
				}
			}
			peak = max(peak, req.Spool.Budget.Used())
			completed++
			return nil
		}}
	}
	prepared, err := New().Prepare(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 6, completed)
	require.Greater(t, peak, int64(96<<20))
	require.LessOrEqual(t, peak, int64(256<<20))
	require.Zero(t, req.Spool.Budget.Used())
	require.NoError(t, prepared.Close())
}
