package document

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDashboardDocumentValidate_DistributionPanelsRejectInvalidContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		kind     PanelKind
		encoding Encoding
		frame    Frame
		want     string
	}{
		{
			name: "histogram without value", kind: PanelKindHistogram,
			encoding: Encoding{Category: "bucket"},
			frame:    Frame{Columns: []Column{{Name: "bucket", Type: ColumnString}}, Rows: [][]any{{"0-10"}}},
			want:     "histogram requires category or label and value encoding",
		},
		{
			name: "boxplot with string median", kind: PanelKindBoxPlot,
			encoding: Encoding{Category: "bucket", Lower: "lower", Q1: "q1", Median: "median", Q3: "q3", Upper: "upper"},
			frame: Frame{Columns: []Column{
				{Name: "bucket", Type: ColumnString}, {Name: "lower", Type: ColumnNumber}, {Name: "q1", Type: ColumnNumber},
				{Name: "median", Type: ColumnString}, {Name: "q3", Type: ColumnNumber}, {Name: "upper", Type: ColumnNumber},
			}, Rows: [][]any{{"A", 1.0, 2.0, "3", 4.0, 5.0}}},
			want: "median field \"median\" must be number",
		},
		{
			name: "heatmap without series", kind: PanelKindHeatmap,
			encoding: Encoding{Category: "region", Value: "value"},
			frame:    Frame{Columns: []Column{{Name: "region", Type: ColumnString}, {Name: "value", Type: ColumnNumber}}, Rows: [][]any{{"North", 1.0}}},
			want:     "heatmap requires category, series, and value encoding",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := testDocument()
			doc.Panels[0].Kind = test.kind
			doc.Panels[0].Encoding = test.encoding
			doc.Frames["panel:total"] = test.frame
			require.ErrorContains(t, doc.Validate(), test.want)
		})
	}
}

func TestDashboardDocumentValidate_LogarithmicAxisDefersFramePolicyToRenderer(t *testing.T) {
	t.Parallel()
	doc := testDocument()
	doc.Panels[0].Kind = PanelKindHBar
	doc.Panels[0].ValueAxis = &ValueAxis{Scale: AxisScaleLogarithmic, LogBase: 10}
	doc.Panels[0].Presentation = &Presentation{ValueSpreadThreshold: 25}
	doc.Frames["panel:total"] = Frame{
		Columns: []Column{{Name: "label", Type: ColumnString}, {Name: "value", Type: ColumnNumber}},
		Rows:    [][]any{{"Large", 20.0}, {"Small", 2.0}},
	}
	// Too few categories and a spread below the producer's threshold are valid
	// document data: the renderer presents this frame linearly instead.
	require.NoError(t, doc.Validate())

	doc.Frames["panel:total"] = Frame{
		Columns: []Column{{Name: "label", Type: ColumnString}, {Name: "value", Type: ColumnNumber}},
		Rows:    [][]any{{"Large", 1000.0}, {"Medium", 0.0}, {"Small", -1.0}},
	}
	// The same fallback applies when a logarithmic axis cannot represent the
	// values. Go validates the contract shape; it does not contradict the
	// runtime by rejecting a frame the runtime can still render honestly.
	require.NoError(t, doc.Validate())

	doc = testDocument()
	doc.Panels[0].Kind = PanelKindHistogram
	doc.Panels[0].ValueAxis = &ValueAxis{Scale: AxisScaleLogarithmic, LogBase: 10}
	require.ErrorContains(t, doc.Validate(), "logarithmic value axis is unsupported for kind")
}
