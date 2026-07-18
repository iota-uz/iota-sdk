package export

import (
	"bytes"
	"context"
	"io"
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
	evidence := mustFrames(t, "evidence",
		frame.Field{Name: "policy", Values: []any{frame.Hyperlink{URL: "https://example.test/policies/1", Label: "P-1"}}},
		frame.Field{Name: "amount", Values: []any{42.0}},
		frame.Field{Name: "verified", Values: []any{frame.Formula{Expression: "[@amount]", Result: 42.0}}},
	)
	panelSpec := panel.Spec{ID: "premium", Title: "Premium", Kind: panel.KindPie, Dataset: "chart", Fields: panel.FieldMapping{Label: panel.Ref("label"), Value: panel.Ref("value")}}
	result := &lensruntime.DashboardResult{SnapshotID: "snap", Spec: lens.DashboardSpec{Title: "Dashboard", Datasets: []lens.DatasetSpec{{Name: "chart", Export: exportmeta.Spec{EvidenceDatasets: []string{"evidence"}}}, {Name: "evidence", Title: "Policy evidence", Export: exportmeta.Spec{SheetName: "Policies", TableName: "Policies", FreezeHeader: true}}}}, Variables: map[string]any{"year": 2026}, Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}, "evidence": {Frames: evidence}}, Panels: map[string]*lensruntime.PanelResult{"premium": {Panel: panelSpec, Frames: chart}}}
	var out bytes.Buffer
	require.NoError(t, New().Write(context.Background(), &out, Request{Result: result, PanelID: "premium"}))
	book, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	require.NoError(t, err)
	defer func() { _ = book.Close() }()
	require.NotContains(t, book.GetSheetList(), "Premium")
	require.Contains(t, book.GetSheetList(), "Policies")
	require.Equal(t, []string{"Summary", "Parameters", "Sources", "Policies"}, book.GetSheetList())
	formula, err := book.GetCellFormula("Summary", "B5")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Sources'!A1","Sources")`, formula)
	formula, err = book.GetCellFormula("Sources", "A2")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Summary'!A7","chart")`, formula)
	formula, err = book.GetCellFormula("Sources", "A3")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("#'Policies'!A1","Policy evidence")`, formula)
	value, err := book.GetCellValue("Summary", "A7")
	require.NoError(t, err)
	require.Equal(t, "Premium", value)
	value, err = book.GetCellValue("Summary", "A8")
	require.NoError(t, err)
	require.Equal(t, "label", value)
	value, err = book.GetCellValue("Summary", "B9")
	require.NoError(t, err)
	require.Equal(t, "42", value)
	dimension, err := book.GetSheetDimension("Policies")
	require.NoError(t, err)
	require.Equal(t, "A1:C2", dimension)
	formula, err = book.GetCellFormula("Policies", "C2")
	require.NoError(t, err)
	require.Equal(t, "[@amount]", formula)
	formula, err = book.GetCellFormula("Policies", "A2")
	require.NoError(t, err)
	require.Equal(t, `HYPERLINK("https://example.test/policies/1","P-1")`, formula)
	tables, err := book.GetTables("Policies")
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "Policies", tables[0].Name)
}

func TestLabelsForLocaleSupportsEAILocales(t *testing.T) {
	t.Parallel()
	require.Equal(t, "Сводка", LabelsForLocale("ru").Summary)
	require.Equal(t, "Xulosa", LabelsForLocale("uz").Summary)
	require.Equal(t, "Хулоса", LabelsForLocale("uz-Cyrl").Summary)
	require.Equal(t, "Summary", LabelsForLocale("en").Summary)
}

func TestSelectedPanelsFollowsDashboardLayout(t *testing.T) {
	t.Parallel()
	result := &lensruntime.DashboardResult{
		Spec: lens.DashboardSpec{Rows: []lens.RowSpec{{Panels: []panel.Spec{
			{ID: "second", Kind: panel.KindStat},
			{ID: "group", Kind: panel.KindStatGroup, Children: []panel.Spec{{ID: "first", Kind: panel.KindStat}}},
		}}}},
		Panels: map[string]*lensruntime.PanelResult{
			"first":  {Panel: panel.Spec{ID: "first", Kind: panel.KindStat}},
			"second": {Panel: panel.Spec{ID: "second", Kind: panel.KindStat}},
		},
	}

	panels := selectedPanels(result, "")
	require.Len(t, panels, 2)
	require.Equal(t, "second", panels[0].Panel.ID)
	require.Equal(t, "first", panels[1].Panel.ID)
}

func TestExporterRejectsMissingDeclaredEvidence(t *testing.T) {
	t.Parallel()
	chart := mustFrames(t, "chart", frame.Field{Name: "value", Values: []any{1}})
	panelSpec := panel.Spec{ID: "premium", Title: "Premium", Dataset: "chart", Export: exportmeta.Spec{EvidenceDatasets: []string{"missing"}}}
	result := &lensruntime.DashboardResult{
		Spec:     lens.DashboardSpec{Datasets: []lens.DatasetSpec{{Name: "chart"}}},
		Datasets: map[string]*lensruntime.DatasetResult{"chart": {Frames: chart}},
		Panels:   map[string]*lensruntime.PanelResult{"premium": {Panel: panelSpec, Frames: chart}},
	}
	err := New().Write(context.Background(), io.Discard, Request{Result: result, PanelID: "premium"})
	require.ErrorContains(t, err, `declared export evidence dataset "missing" is unavailable`)
}

func mustFrames(t *testing.T, name string, fields ...frame.Field) *frame.FrameSet {
	t.Helper()
	fr, err := frame.New(name, fields...)
	require.NoError(t, err)
	fs, err := frame.NewFrameSet(fr)
	require.NoError(t, err)
	return fs
}
