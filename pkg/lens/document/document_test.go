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

func TestDashboardDocumentValidateAndSerialize_DynamicChildren(t *testing.T) {
	t.Parallel()
	doc := testDocument()
	doc.Frames["dynamic"] = Frame{
		Columns: []Column{{Name: "child_key", Type: ColumnString}, {Name: "child_label", Type: ColumnString}, {Name: "url", Type: ColumnString}},
		Rows:    [][]any{{"2025", "2025 year", "/years/2025"}},
	}
	doc.Drill.Edges["root"] = Level{
		Path: NodePath{"root"}, Frame: "dynamic", Children: []Node{}, Perspectives: []PerspectiveRef{},
		DynamicChildren: &DynamicChildren{
			Key: Source{Kind: ValueSourceField, Name: "child_key"}, Label: Source{Kind: ValueSourceField, Name: "child_label"},
			Action: &Action{Kind: ActionNavigateToLeaf, URLSource: &Source{Kind: ValueSourceField, Name: "url"}, Params: []ActionParam{}, Payload: map[string]Source{}},
		},
	}
	require.NoError(t, doc.Validate())
	dynamicFrame := doc.Frames["dynamic"]
	require.NoError(t, ResolveDynamicChildren(&dynamicFrame, doc.Drill.Edges["root"]))
	doc.Frames["dynamic"] = dynamicFrame
	payload, err := json.Marshal(doc)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"dynamicChildren":{"key":{"kind":"field","name":"child_key"}`)
	require.Contains(t, string(payload), `"children":[{"key":"2025","path":["root","2025"],"label":"2025 year"`)

	malformed := testDocument()
	malformed.Drill.Edges["root"] = Level{
		Path: NodePath{"root"}, Children: []Node{}, Perspectives: []PerspectiveRef{},
		DynamicChildren: &DynamicChildren{Key: Source{Kind: ValueSourceLiteral, Value: "fixed"}, Label: Source{Kind: ValueSourceField, Name: "label"}},
	}
	require.ErrorContains(t, malformed.Validate(), "key requires a field source")
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

func TestDashboardDocumentValidate_TableColumns(t *testing.T) {
	t.Parallel()
	doc := testDocument()
	doc.Panels[0].Kind = PanelKindTable
	doc.Panels[0].Columns = []TableColumn{{
		Field: "value", Label: "Value", Align: TableAlignRight, Cell: TableCell{Kind: TableCellDelta, SecondaryField: "label"},
		Action: &Action{
			Kind: ActionNavigateToLeaf, URLSource: &Source{Kind: ValueSourceField, Name: "label"},
			Params: []ActionParam{}, Payload: map[string]Source{},
		},
	}}
	doc.Panels[0].Semantics = SemanticsEvidence
	require.NoError(t, doc.Validate())

	doc.Panels[0].Columns[0].Cell.SecondaryField = "missing"
	require.ErrorContains(t, doc.Validate(), "missing secondary field")
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

func metricBaseDocument(id string, kind PanelKind) *DashboardDocument {
	frame := FrameRef("panel:" + id)
	return &DashboardDocument{
		Version:    ContractVersion,
		SnapshotID: "snapshot-metric",
		Meta:       Meta{DashboardID: "metrics", Title: "Metrics", GeneratedAt: time.Date(2026, time.July, 19, 9, 30, 0, 0, time.UTC), Locale: "en"},
		Layout:     Layout{Rows: []LayoutRow{{Panels: []LayoutItem{{PanelID: id, Span: 6}}}}},
		Panels: []Panel{{
			ID: id, Kind: kind, Title: "Metric", Semantics: SemanticsSeries, Frame: frame,
			Encoding: Encoding{ID: "id", Value: "value"}, Format: map[string]FieldFormat{}, Actions: []Action{},
		}},
		Frames: map[FrameRef]Frame{
			frame: {Columns: []Column{{Name: "id", Type: ColumnString}, {Name: "value", Type: ColumnNumber}}, Rows: [][]any{{"opening", 10.0}}},
		},
		Drill:        Drill{Edges: map[NodeKey]Level{}, InlineDepth: 0},
		Perspectives: []Perspective{},
		Endpoints:    Endpoints{Query: "/lens/query", Export: "/lens/export"},
		I18n:         map[string]string{"dashboard.title": "Metrics"},
		Theme:        Theme{Palette: map[string]string{"accent": "#2563eb"}, Series: map[string]string{}},
	}
}

func metricFlowDocument() *DashboardDocument {
	doc := metricBaseDocument("flow", PanelKindMetricFlow)
	doc.Panels[0].MetricFlow = &MetricFlowConfig{Stages: []MetricFlowStage{
		{Key: "opening", Label: "Opening", Role: MetricFlowRoleInput},
		{Key: "added", Label: "Added", Role: MetricFlowRoleAdd, Confidence: ConfidenceCalculated},
		{Key: "released", Label: "Released", Role: MetricFlowRoleSubtract, Availability: AvailabilityConfigRequired},
		{Key: "closing", Label: "Closing", Role: MetricFlowRoleResult},
	}}
	return doc
}

func metricHierarchyDocument() *DashboardDocument {
	doc := metricBaseDocument("hierarchy", PanelKindMetricHierarchy)
	doc.Panels[0].MetricHierarchy = &MetricHierarchyConfig{
		Rows: []MetricHierarchyRow{
			{Key: "root", Label: "Root", Selected: true},
			{Key: "child", Label: "Child", Parent: "root", Confidence: ConfidenceVerified},
			{Key: "rest", Label: "Unallocated", Parent: "root", Unallocated: true},
		},
		Reconcile: &HierarchyReconciliation{Tolerance: 0.5},
	}
	return doc
}

func metricRelationshipDocument() *DashboardDocument {
	doc := metricBaseDocument("relationship", PanelKindMetricRelationship)
	doc.Panels[0].MetricRelationship = &MetricRelationshipConfig{
		Source: MetricRelationshipEnd{Key: "left", Label: "Left"},
		Target: MetricRelationshipEnd{Key: "right", Label: "Right"},
		Type:   MetricRelationshipAssociation,
	}
	return doc
}

func TestDashboardDocumentValidate_MetricKindsValid(t *testing.T) {
	t.Parallel()

	t.Run("metric flow", func(t *testing.T) {
		require.NoError(t, metricFlowDocument().Validate())
	})
	t.Run("metric hierarchy", func(t *testing.T) {
		require.NoError(t, metricHierarchyDocument().Validate())
	})
	t.Run("metric relationship", func(t *testing.T) {
		require.NoError(t, metricRelationshipDocument().Validate())
	})
	t.Run("nested groups chain", func(t *testing.T) {
		doc := metricFlowDocument()
		doc.Layout.Rows[0].Panels[0].Groups = []LayoutGroup{
			{ID: "stock-movement", Kind: LayoutGroupTabs, Span: 12, Tab: "stock", Label: "Stock / Movement"},
			{ID: "strip", Kind: LayoutGroupMetrics, Span: 6, Layout: LayoutGroupColumns},
		}
		require.NoError(t, doc.Validate())
	})
}

func TestDashboardDocumentValidate_MetricFlow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*DashboardDocument)
		wantErr string
	}{
		{"bad role", func(d *DashboardDocument) {
			d.Panels[0].MetricFlow.Stages[1].Role = "bogus"
		}, "unsupported role"},
		{"fewer than two stages", func(d *DashboardDocument) {
			d.Panels[0].MetricFlow.Stages = d.Panels[0].MetricFlow.Stages[:1]
		}, "at least 2 stages"},
		{"duplicate stage keys", func(d *DashboardDocument) {
			d.Panels[0].MetricFlow.Stages[1].Key = "opening"
		}, "duplicate stage"},
		{"two result stages", func(d *DashboardDocument) {
			d.Panels[0].MetricFlow.Stages[1].Role = MetricFlowRoleResult
		}, "exactly one result stage"},
		{"result not last", func(d *DashboardDocument) {
			s := d.Panels[0].MetricFlow.Stages
			s[2].Role, s[3].Role = MetricFlowRoleResult, MetricFlowRoleAdd
		}, "result stage must be last"},
		{"blank stage label", func(d *DashboardDocument) {
			d.Panels[0].MetricFlow.Stages[0].Label = ""
		}, "requires a label"},
		{"bad stage confidence", func(d *DashboardDocument) {
			d.Panels[0].MetricFlow.Stages[0].Confidence = "bogus"
		}, "unsupported confidence"},
		{"bad stage availability", func(d *DashboardDocument) {
			d.Panels[0].MetricFlow.Stages[0].Availability = "bogus"
		}, "unsupported availability"},
		{"missing required encoding column", func(d *DashboardDocument) {
			d.Panels[0].Encoding.ID = "ghost"
		}, "id encoding references missing field"},
		{"missing declared encoding role", func(d *DashboardDocument) {
			d.Panels[0].Encoding.Value = ""
		}, "requires a value encoding"},
		{"config on wrong kind", func(d *DashboardDocument) {
			d.Panels[0].Kind = PanelKindStat
		}, "metric config for kind"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := metricFlowDocument()
			tc.mutate(doc)
			require.ErrorContains(t, doc.Validate(), tc.wantErr)
		})
	}
}

func TestDashboardDocumentValidate_MetricHierarchy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*DashboardDocument)
		wantErr string
	}{
		{"cycle", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows = []MetricHierarchyRow{
				{Key: "root", Label: "Root"},
				{Key: "a", Label: "A", Parent: "b"},
				{Key: "b", Label: "B", Parent: "a"},
			}
		}, "cycle"},
		{"orphan parent", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows[1].Parent = "ghost"
		}, "references missing parent"},
		{"self parent", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows[1].Parent = "child"
		}, "is its own parent"},
		{"multiple selected", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows[1].Selected = true
		}, "multiple selected"},
		{"multiple unallocated per parent", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows[1].Unallocated = true
		}, "multiple unallocated"},
		{"depth mismatch", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows[1].Depth = 5
		}, "does not match derived depth"},
		{"no root", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows[0].Parent = "child"
		}, "at least one root"},
		{"empty rows", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Rows = nil
		}, "at least 1 row"},
		{"negative tolerance", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy.Reconcile.Tolerance = -1
		}, "tolerance cannot be negative"},
		{"missing config", func(d *DashboardDocument) {
			d.Panels[0].MetricHierarchy = nil
		}, "requires a metric hierarchy config"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := metricHierarchyDocument()
			tc.mutate(doc)
			require.ErrorContains(t, doc.Validate(), tc.wantErr)
		})
	}
}

func TestDashboardDocumentValidate_MetricRelationship(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*DashboardDocument)
		wantErr string
	}{
		{"same end", func(d *DashboardDocument) {
			d.Panels[0].MetricRelationship.Target.Key = "left"
		}, "source and target must differ"},
		{"bad type", func(d *DashboardDocument) {
			d.Panels[0].MetricRelationship.Type = "bogus"
		}, "unsupported type"},
		{"bad direction", func(d *DashboardDocument) {
			d.Panels[0].MetricRelationship.Direction = "bogus"
		}, "unsupported direction"},
		{"blank end label", func(d *DashboardDocument) {
			d.Panels[0].MetricRelationship.Source.Label = ""
		}, "source requires a label"},
		{"bad end confidence", func(d *DashboardDocument) {
			d.Panels[0].MetricRelationship.Target.Confidence = "bogus"
		}, "unsupported confidence"},
		{"directed derivation is valid", func(d *DashboardDocument) {
			d.Panels[0].MetricRelationship.Type = MetricRelationshipDerivation
			d.Panels[0].MetricRelationship.Direction = MetricRelationshipSourceToTarget
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := metricRelationshipDocument()
			tc.mutate(doc)
			if tc.wantErr == "" {
				require.NoError(t, doc.Validate())
				return
			}
			require.ErrorContains(t, doc.Validate(), tc.wantErr)
		})
	}
}

// TestDashboardDocumentValidate_MetricDuplicateFrameKeys pins the missing-data
// contract's duplicate-key rule for all three metric kinds: once the id
// encoding column resolves, two rows sharing a key are a deterministic error
// naming the key, while out-of-order unique keys and a genuine zero value stay
// valid (key order is irrelevant; a real 0 is not a duplicate-with-itself).
func TestDashboardDocumentValidate_MetricDuplicateFrameKeys(t *testing.T) {
	t.Parallel()

	documents := map[string]func() *DashboardDocument{
		"metric flow":         metricFlowDocument,
		"metric hierarchy":    metricHierarchyDocument,
		"metric relationship": metricRelationshipDocument,
	}
	for name, build := range documents {
		t.Run(name, func(t *testing.T) {
			t.Run("duplicate key rejected", func(t *testing.T) {
				doc := build()
				frameRef := doc.Panels[0].Frame
				columns := doc.Frames[frameRef].Columns
				doc.Frames[frameRef] = Frame{Columns: columns, Rows: [][]any{{"opening", 10.0}, {"opening", 5.0}}}
				require.ErrorContains(t, doc.Validate(), `duplicate key "opening"`)
			})
			t.Run("out of order unique keys valid", func(t *testing.T) {
				doc := build()
				frameRef := doc.Panels[0].Frame
				columns := doc.Frames[frameRef].Columns
				doc.Frames[frameRef] = Frame{Columns: columns, Rows: [][]any{{"closing", 40.0}, {"opening", 10.0}}}
				require.NoError(t, doc.Validate())
			})
			t.Run("real zero value valid", func(t *testing.T) {
				doc := build()
				frameRef := doc.Panels[0].Frame
				columns := doc.Frames[frameRef].Columns
				doc.Frames[frameRef] = Frame{Columns: columns, Rows: [][]any{{"opening", 0.0}}}
				require.NoError(t, doc.Validate())
			})
		})
	}
}

func TestDashboardDocumentValidate_PanelQuality(t *testing.T) {
	t.Parallel()

	t.Run("bad panel confidence", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Confidence = "bogus"
		require.ErrorContains(t, doc.Validate(), "unsupported confidence")
	})
	t.Run("bad panel availability", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Availability = "bogus"
		require.ErrorContains(t, doc.Validate(), "unsupported availability")
	})
	t.Run("panel quality accepted", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Confidence = ConfidenceProxy
		doc.Panels[0].Availability = AvailabilityEmptySource
		require.NoError(t, doc.Validate())
	})
	t.Run("config on non-metric kind rejected", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].MetricFlow = &MetricFlowConfig{}
		require.ErrorContains(t, doc.Validate(), "metric config for kind")
	})
}

func TestDashboardDocumentValidate_FocusCanvasAdditions(t *testing.T) {
	t.Parallel()

	bareLevel := func() Level {
		return Level{Path: NodePath{"root"}, Children: []Node{}, Perspectives: []PerspectiveRef{}}
	}

	t.Run("level view must be a chart kind", func(t *testing.T) {
		doc := testDocument()
		level := bareLevel()
		level.View = PanelKindTable
		doc.Drill.Edges["root"] = level
		require.ErrorContains(t, doc.Validate(), "view must be a chart kind")
	})
	t.Run("level chart view accepted", func(t *testing.T) {
		doc := testDocument()
		level := bareLevel()
		level.View = PanelKindCoverage
		doc.Drill.Edges["root"] = level
		require.NoError(t, doc.Validate())
	})
	t.Run("target only on coverage and hbar", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Target = &PanelTarget{Value: 10, Label: "Plan"}
		require.ErrorContains(t, doc.Validate(), "target marker is only supported on coverage and hbar")
		doc.Panels[0].Kind = PanelKindHBar
		require.NoError(t, doc.Validate())
	})
	t.Run("target value must be finite", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Kind = PanelKindCoverage
		doc.Panels[0].Target = &PanelTarget{Value: math.NaN()}
		require.ErrorContains(t, doc.Validate(), "target value must be finite")
	})
	t.Run("focus mode is validated", func(t *testing.T) {
		doc := testDocument()
		doc.Panels[0].Presentation = &Presentation{Focus: "bogus"}
		require.ErrorContains(t, doc.Validate(), "unsupported focus mode")
		doc.Panels[0].Presentation = &Presentation{Focus: FocusModeCanvas}
		require.NoError(t, doc.Validate())
	})
	t.Run("level status is validated", func(t *testing.T) {
		doc := testDocument()
		level := bareLevel()
		level.Status = &PanelStatus{}
		doc.Drill.Edges["root"] = level
		require.ErrorContains(t, doc.Validate(), "status requires a label")
		level.Status = &PanelStatus{Label: "ПРОКСИ", Tone: "bogus"}
		doc.Drill.Edges["root"] = level
		require.ErrorContains(t, doc.Validate(), "unsupported status tone")
	})
	t.Run("level presentation focus is validated", func(t *testing.T) {
		doc := testDocument()
		level := bareLevel()
		level.Presentation = &Presentation{Focus: "bogus"}
		doc.Drill.Edges["root"] = level
		require.ErrorContains(t, doc.Validate(), "unsupported focus mode")
	})
	t.Run("level source frame must exist", func(t *testing.T) {
		doc := testDocument()
		level := bareLevel()
		level.Source = &LevelSource{Frame: "missing"}
		doc.Drill.Edges["root"] = level
		require.ErrorContains(t, doc.Validate(), "source references missing frame")
	})
	t.Run("level source columns and formats must resolve", func(t *testing.T) {
		doc := testDocument()
		level := bareLevel()
		level.Source = &LevelSource{Frame: "panel:total", Columns: []TableColumn{{Field: "ghost", Label: "Ghost"}}}
		doc.Drill.Edges["root"] = level
		require.ErrorContains(t, doc.Validate(), "source column references missing field")
		level.Source = &LevelSource{Frame: "panel:total", Format: map[string]FieldFormat{"ghost": {Kind: FormatNumber}}}
		doc.Drill.Edges["root"] = level
		require.ErrorContains(t, doc.Validate(), "source format references missing field")
		level.Source = &LevelSource{
			Label: "Source data", Frame: "panel:total",
			Columns: []TableColumn{{Field: "value", Label: "Value", Cell: TableCell{Kind: TableCellPlain}}},
			Format:  map[string]FieldFormat{"value": {Kind: FormatNumber}},
		}
		doc.Drill.Edges["root"] = level
		require.NoError(t, doc.Validate())
	})
	t.Run("drawer size is validated", func(t *testing.T) {
		doc := testDocument()
		doc.Drawer = &DrawerHeader{Title: "Metric", Size: "huge"}
		require.ErrorContains(t, doc.Validate(), "unsupported size")
		doc.Drawer.Size = DrawerSizeWide
		require.NoError(t, doc.Validate())
	})
}

// TestWireJSON_FocusCanvasFieldsOmittedWhenUnset pins backward compatibility
// for the focus-canvas additions: a document that uses none of them must
// serialize byte-identical to a pre-focus producer, so every new field is
// absent from the zero-value wire shapes. The golden-file tests in this
// package (small.json, generated_explore.json, metric_kinds.json) assert the
// same at whole-document scope.
func TestWireJSON_FocusCanvasFieldsOmittedWhenUnset(t *testing.T) {
	t.Parallel()

	level, err := json.Marshal(Level{Path: NodePath{"root"}, Label: "L", Children: []Node{}, Perspectives: []PerspectiveRef{}})
	require.NoError(t, err)
	require.JSONEq(t, `{"path":["root"],"label":"L","children":[],"perspectives":[]}`, string(level))

	wirePanel, err := json.Marshal(Panel{
		ID: "p", Kind: PanelKindStat, Title: "T", Semantics: SemanticsSeries, Frame: "panel:p",
		Format: map[string]FieldFormat{}, Actions: []Action{},
	})
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"p","kind":"stat","title":"T","semantics":"series","frame":"panel:p","encoding":{},"format":{},"actions":[]}`, string(wirePanel))

	drawer, err := json.Marshal(DrawerHeader{Title: "T"})
	require.NoError(t, err)
	require.JSONEq(t, `{"title":"T"}`, string(drawer))

	presentation, err := json.Marshal(Presentation{Fill: true})
	require.NoError(t, err)
	require.Equal(t, `{"fill":true}`, string(presentation))
}

func TestDashboardDocumentValidate_NestedGroups(t *testing.T) {
	t.Parallel()

	t.Run("duplicate group id in chain", func(t *testing.T) {
		doc := testDocument()
		doc.Layout.Rows[0].Panels[0].Groups = []LayoutGroup{
			{ID: "g", Kind: LayoutGroupTabs, Span: 12, Tab: "a"},
			{ID: "g", Kind: LayoutGroupMetrics, Span: 6},
		}
		require.ErrorContains(t, doc.Validate(), "repeats group")
	})
	t.Run("empty tab label", func(t *testing.T) {
		doc := testDocument()
		doc.Layout.Rows[0].Panels[0].Groups = []LayoutGroup{{ID: "g", Kind: LayoutGroupTabs, Span: 12}}
		require.ErrorContains(t, doc.Validate(), "requires a tab")
	})
	t.Run("inconsistent descriptors", func(t *testing.T) {
		doc := testDocument()
		doc.Layout.Rows[0].Panels = []LayoutItem{
			{PanelID: "total", Span: 6, Groups: []LayoutGroup{{ID: "g", Kind: LayoutGroupMetrics, Span: 6, Label: "One"}}},
			{PanelID: "total", Span: 6, Groups: []LayoutGroup{{ID: "g", Kind: LayoutGroupMetrics, Span: 6, Label: "Two"}}},
		}
		require.ErrorContains(t, doc.Validate(), "inconsistent descriptors")
	})
	t.Run("consistent chain across items accepted", func(t *testing.T) {
		doc := testDocument()
		doc.Layout.Rows[0].Panels = []LayoutItem{
			{PanelID: "total", Span: 6, Groups: []LayoutGroup{
				{ID: "outer", Kind: LayoutGroupTabs, Span: 12, Tab: "stock"},
				{ID: "inner", Kind: LayoutGroupMetrics, Span: 6},
			}},
			{PanelID: "total", Span: 6, Groups: []LayoutGroup{
				{ID: "outer", Kind: LayoutGroupTabs, Span: 12, Tab: "movement"},
				{ID: "inner", Kind: LayoutGroupMetrics, Span: 6},
			}},
		}
		require.NoError(t, doc.Validate())
	})
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

func TestDashboardDocumentValidate_PanelActionFieldsResolveAgainstLevelFrames(t *testing.T) {
	t.Parallel()

	drillable := func() *DashboardDocument {
		doc := testDocument()
		root := NodeKey("root")
		doc.Panels[0].DrillRoot = &root
		doc.Frames["level:root"] = Frame{
			Columns: []Column{{Name: "policy_id", Type: ColumnString}},
			Rows:    [][]any{{"PL-1"}},
		}
		doc.Drill.Edges["root"] = Level{
			Path: NodePath{"root"}, Frame: "level:root", Children: []Node{}, Perspectives: []PerspectiveRef{},
		}
		doc.Panels[0].Actions = []Action{{
			Kind: ActionNavigateToLeaf, URLTemplate: "/policies/{id}",
			Params:  []ActionParam{{Name: "id", Source: Source{Kind: ValueSourceField, Name: "policy_id"}}},
			Payload: map[string]Source{},
		}}
		return doc
	}

	t.Run("field supplied only by a drill level frame is accepted", func(t *testing.T) {
		require.NoError(t, drillable().Validate())
	})

	t.Run("field on no reachable frame is still rejected", func(t *testing.T) {
		doc := drillable()
		doc.Panels[0].Actions[0].Params[0].Source.Name = "nowhere"
		require.ErrorContains(t, doc.Validate(), "references missing field")
	})
}

func TestDashboardDocumentValidate_LayoutGroups(t *testing.T) {
	t.Parallel()

	t.Run("tabs group requires a tab", func(t *testing.T) {
		doc := testDocument()
		doc.Layout.Rows[0].Panels[0].Group = &LayoutGroup{ID: "g", Kind: LayoutGroupTabs, Span: 12}
		require.ErrorContains(t, doc.Validate(), "requires a tab")
	})
	t.Run("group span is bounded", func(t *testing.T) {
		doc := testDocument()
		doc.Layout.Rows[0].Panels[0].Group = &LayoutGroup{ID: "g", Kind: LayoutGroupMetrics, Span: 13}
		require.ErrorContains(t, doc.Validate(), "span must be between 1 and 12")
	})
	t.Run("metrics group is accepted", func(t *testing.T) {
		doc := testDocument()
		doc.Layout.Rows[0].Panels[0].Group = &LayoutGroup{ID: "g", Kind: LayoutGroupMetrics, Span: 12, Layout: LayoutGroupColumns}
		require.NoError(t, doc.Validate())
	})
}
