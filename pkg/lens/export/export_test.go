package export

import (
	"bytes"
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestExporterWritesPanelAndEvidenceWithoutDatasourceQueries(t *testing.T) {
	t.Parallel()
	chart := mustFrames(t, "chart", frame.Field{Name: "label", Values: []any{"A"}}, frame.Field{Name: "value", Values: []any{42.0}})
	evidence := mustFrames(t, "evidence", frame.Field{Name: "policy", Values: []any{"P-1"}}, frame.Field{Name: "amount", Values: []any{42.0}})
	panelSpec := panel.Spec{ID: "premium", Title: "Premium", Kind: panel.KindPie, Dataset: "chart", Fields: panel.FieldMapping{Label: panel.Ref("label"), Value: panel.Ref("value")}}
	result := &lensruntime.DashboardResult{SnapshotID: "snap", Spec: lens.DashboardSpec{Title: "Dashboard", Datasets: []lens.DatasetSpec{{Name: "chart", Export: exportmeta.Spec{EvidenceDataset: "evidence"}}, {Name: "evidence"}}}, Variables: map[string]any{"year": 2026}, Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}, "evidence": {Frames: evidence}}, Panels: map[string]*lensruntime.PanelResult{"premium": {Panel: panelSpec, Frames: chart}}}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result, PanelID: "premium"}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()
	require.Contains(t, book.GetSheetList(), "Premium")
	require.Contains(t, book.GetSheetList(), "Breakdown evidence")
}

func mustFrames(t *testing.T, name string, fields ...frame.Field) *frame.FrameSet {
	t.Helper()
	fr, err := frame.New(name, fields...)
	require.NoError(t, err)
	fs, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return fs
}
