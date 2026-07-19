package document

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDashboardDocumentValidate_FrameReferences(t *testing.T) {
	t.Parallel()

	t.Run("panel", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Frame = "missing"
		require.ErrorContains(t, doc.Validate(), "references missing frame")
	})
	t.Run("drill level", func(t *testing.T) {
		doc := testDocument()
		doc.Drill.Edges["root"] = Level{Path: NodePath{"root"}, Frame: "missing", Children: []Node{}, Perspectives: []PerspectiveRef{}}
		require.ErrorContains(t, doc.Validate(), "references missing frame")
	})
}

func TestDashboardDocumentValidate_DrillIdentity(t *testing.T) {
	t.Parallel()

	t.Run("duplicate siblings", func(t *testing.T) {
		doc := testDocument()
		doc.Drill.Edges["root"] = Level{
			Path: NodePath{"root"}, Perspectives: []PerspectiveRef{},
			Children: []Node{
				{Key: "same", Path: NodePath{"root", "same"}, Label: "First"},
				{Key: "same", Path: NodePath{"root", "same"}, Label: "Second"},
			},
		}
		require.ErrorContains(t, doc.Validate(), "duplicate child key")
	})
	t.Run("duplicate full paths cannot bypass parent consistency", func(t *testing.T) {
		doc := testDocument()
		doc.Drill.Edges["first"] = Level{Path: NodePath{"first"}, Children: []Node{{Key: "leaf", Path: NodePath{"root", "leaf"}, Label: "One"}}, Perspectives: []PerspectiveRef{}}
		doc.Drill.Edges["second"] = Level{Path: NodePath{"second"}, Children: []Node{{Key: "leaf", Path: NodePath{"root", "leaf"}, Label: "Two"}}, Perspectives: []PerspectiveRef{}}
		require.ErrorContains(t, doc.Validate(), "must extend parent level")
	})
	t.Run("level path must end with registered edge key", func(t *testing.T) {
		doc := testDocument()
		doc.Drill.Edges["root"] = Level{Path: NodePath{"other"}, Children: []Node{}, Perspectives: []PerspectiveRef{}}
		require.ErrorContains(t, doc.Validate(), "invalid full path")
	})
	t.Run("child path must extend parent path", func(t *testing.T) {
		doc := testDocument()
		doc.Drill.Edges["root"] = Level{
			Path: NodePath{"root"}, Perspectives: []PerspectiveRef{},
			Children: []Node{{Key: "leaf", Path: NodePath{"unrelated", "leaf"}}},
		}
		require.ErrorContains(t, doc.Validate(), "must extend parent level")
	})
	t.Run("child path cannot skip a parent segment", func(t *testing.T) {
		doc := testDocument()
		doc.Drill.Edges["root"] = Level{
			Path: NodePath{"root"}, Perspectives: []PerspectiveRef{},
			Children: []Node{{Key: "leaf", Path: NodePath{"root", "extra", "leaf"}}},
		}
		require.ErrorContains(t, doc.Validate(), "must extend parent level")
	})
}

func TestDashboardDocumentValidate_PartitionDrillFrame(t *testing.T) {
	t.Parallel()
	doc := testDocument()
	doc.Frames["detail"] = Frame{Columns: []Column{{Name: "value", Type: ColumnNumber}}, Rows: [][]any{{-1.0}}}
	doc.Perspectives = []Perspective{{
		ID: "metric/branch/composition", ExplorerID: "metric", BranchKey: "metric/branch", Key: "composition",
		Label: "Composition", Semantics: SemanticsPartition, Root: "detail",
	}}
	encoding := Encoding{Value: "value"}
	doc.Drill.Edges["detail"] = Level{
		Path: NodePath{"detail"}, Children: []Node{}, Frame: "detail", Encoding: &encoding,
		Perspectives: []PerspectiveRef{{ID: "metric/branch/composition"}},
	}
	require.ErrorContains(t, doc.Validate(), "partition value row 0")
}

func TestDashboardDocumentValidate_Semantics(t *testing.T) {
	t.Parallel()

	t.Run("reconciliation circular", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Kind = PanelKindPie
		doc.Panels[0].Semantics = SemanticsReconciliation
		require.ErrorContains(t, doc.Validate(), "reconciliation semantics")
	})
	t.Run("evidence leaf action", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Semantics = SemanticsEvidence
		doc.Panels[0].Actions = nil
		require.ErrorContains(t, doc.Validate(), "requires a leaf action")
		doc.Panels[0].Actions = []Action{{Kind: ActionNavigateToLeaf, URLTemplate: "/evidence/{id}", Params: []ActionParam{}, Payload: map[string]Source{}}}
		require.NoError(t, doc.Validate())
	})
	t.Run("emit event action", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Actions = []Action{{
			Kind: ActionEmitEvent, Event: "lens.selected", Params: []ActionParam{},
			Payload: map[string]Source{"id": {Kind: ValueSourceField, Name: "label"}},
		}}
		require.NoError(t, doc.Validate())
	})
	for _, value := range []float64{-1, math.Inf(1), math.NaN()} {
		t.Run(fmt.Sprintf("invalid partition value %v", value), func(t *testing.T) {
			doc := testDocument()
			doc.Panels[0].Semantics = SemanticsPartition
			frame := doc.Frames[doc.Panels[0].Frame]
			frame.Rows[0][1] = value
			doc.Frames[doc.Panels[0].Frame] = frame
			require.ErrorContains(t, doc.Validate(), "finite")
		})
	}
}

func TestDashboardDocumentValidate_MoneyMetadata(t *testing.T) {
	t.Parallel()
	doc := testDocument()
	doc.Panels[0].Format["value"] = FieldFormat{Kind: FormatMoney}
	require.ErrorContains(t, doc.Validate(), "requires currency")

	doc.Panels[0].Format["value"] = FieldFormat{Kind: FormatMoney, Currency: "UZS", MinorUnits: false}
	require.NoError(t, doc.Validate())
	payload, err := json.Marshal(doc)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"minorUnits":false`)
}

func TestQueryPageJSON_EmitsFalseHasNext(t *testing.T) {
	t.Parallel()
	payload, err := json.Marshal(QueryPage{Number: 1, Size: 50})
	require.NoError(t, err)
	require.JSONEq(t, `{"number":1,"size":50,"hasNext":false}`, string(payload))
}

func TestDashboardDocumentJSON_IsDeterministicAndPinsVersion(t *testing.T) {
	t.Parallel()
	doc := testDocument()
	doc.Version = "9.0.0"
	first, err := json.MarshalIndent(doc, "", "  ")
	require.NoError(t, err)
	for range 20 {
		next, marshalErr := json.MarshalIndent(doc, "", "  ")
		require.NoError(t, marshalErr)
		require.Equal(t, first, next)
	}
	require.Contains(t, string(first), `"version": "1.0.0"`)
	require.Equal(t, golden(t, "small.json"), string(first)+"\n")
}

func TestDashboardDocumentValidate_DetectsVersionMismatch(t *testing.T) {
	t.Parallel()
	doc := testDocument()
	doc.Version = "2.0.0"
	require.ErrorContains(t, doc.Validate(), "unsupported contract version")
}

func testDocument() *DashboardDocument {
	return &DashboardDocument{
		Version:    ContractVersion,
		SnapshotID: "snapshot-test",
		Meta:       Meta{DashboardID: "overview", Title: "Overview", GeneratedAt: time.Date(2026, time.July, 19, 9, 30, 0, 0, time.UTC), Locale: "en"},
		Layout:     Layout{Rows: []LayoutRow{{Panels: []LayoutItem{{PanelID: "total", Span: 6}}}}},
		Panels: []Panel{{
			ID: "total", Kind: PanelKindStat, Title: "Total", Semantics: SemanticsSeries, Frame: "panel:total",
			Encoding: Encoding{Label: "label", Value: "value"}, Format: map[string]FieldFormat{}, Actions: []Action{},
		}},
		Frames: map[FrameRef]Frame{
			"panel:total": {Columns: []Column{{Name: "label", Type: ColumnString}, {Name: "value", Type: ColumnNumber}}, Rows: [][]any{{"Total", 42.0}}},
		},
		Drill:        Drill{Edges: map[NodeKey]Level{}, InlineDepth: 0},
		Perspectives: []Perspective{},
		Endpoints:    Endpoints{Query: "/lens/query", Export: "/lens/export"},
		I18n:         map[string]string{"dashboard.total": "Total", "dashboard.title": "Overview"},
		Theme:        Theme{Palette: map[string]string{"accent": "#2563eb", "danger": "#dc2626"}, Series: map[string]string{"total": "accent"}},
	}
}

func golden(t *testing.T, name string) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return strings.ReplaceAll(string(payload), "\r\n", "\n")
}
