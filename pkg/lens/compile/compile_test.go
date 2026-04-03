package compile

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
	"github.com/stretchr/testify/require"
)

func TestDocumentCompilesSemanticDatasetMode(t *testing.T) {
	t.Parallel()

	baseBuilder := frame.NewBuilder("base").
		String("product_code", frame.RoleDimension).
		String("product_label", frame.RoleDimension).
		Number("premium", frame.RoleMetric)
	err := baseBuilder.Append(frame.Row{
		"product_code":  "osago",
		"product_label": "OSAGO",
		"premium":       125000,
	})
	require.NoError(t, err)
	base, err := baseBuilder.FrameSet()
	require.NoError(t, err)

	doc := lensspec.Document{
		Version:     lensspec.DocumentVersion,
		ID:          "semantic-report",
		Title:       lensspec.LiteralText("Semantic"),
		Description: lensspec.LiteralText("Dataset mode"),
		DataMode:    cube.DataModeDataset,
		DataRef:     "base_dataset",
		Dimensions: []lensspec.DimensionSpec{
			{Name: "product", Label: lensspec.LiteralText("Product"), Field: "product_code", LabelField: "product_label", PanelKind: panel.KindBar},
		},
		Measures: []lensspec.MeasureSpec{
			{Name: "total_revenue", Label: lensspec.LiteralText("Revenue"), Field: "premium", Aggregation: cube.AggregationSum},
		},
		DefaultDimension: "product",
	}

	compiled, err := Document(doc, Options{
		Locale: "en",
		Values: map[string]any{"base_dataset": base},
	})
	require.NoError(t, err)
	require.NotNil(t, compiled.Semantic)
	require.Equal(t, "semantic-report", compiled.Spec.ID)
	require.NotEmpty(t, compiled.Spec.Rows)
	require.NotEmpty(t, compiled.Spec.Datasets)
}

func TestDocumentCompilesManualStaticDashboard(t *testing.T) {
	t.Parallel()

	statsBuilder := frame.NewBuilder("stats").Number("value", frame.RoleMetric)
	err := statsBuilder.Append(frame.Row{"value": 7})
	require.NoError(t, err)
	stats, err := statsBuilder.FrameSet()
	require.NoError(t, err)

	doc := lensspec.Document{
		Version:     lensspec.DocumentVersion,
		ID:          "manual-report",
		Title:       lensspec.LiteralText("Manual"),
		Description: lensspec.LiteralText("Static"),
		Datasets: []lensspec.DatasetSpec{
			{Name: "stats", Kind: lens.DatasetKindStatic, StaticRef: "stats_dataset"},
		},
		Rows: []lensspec.RowSpec{
			{
				Panels: []lensspec.PanelSpec{
					{
						ID:      "total",
						Title:   lensspec.LiteralText("Total"),
						Kind:    panel.KindStat,
						Dataset: "stats",
						Span:    12,
						Fields:  lensspec.FieldMappingSpec{Value: "value"},
					},
				},
			},
		},
	}

	compiled, err := Document(doc, Options{
		Locale: "en",
		Values: map[string]any{"stats_dataset": stats},
	})
	require.NoError(t, err)
	require.Nil(t, compiled.Semantic)
	require.Len(t, compiled.Spec.Datasets, 1)
	require.Len(t, compiled.Spec.Rows, 1)
	require.Equal(t, "stats", compiled.Spec.Datasets[0].Name)
	require.Equal(t, "total", compiled.Spec.Rows[0].Panels[0].ID)
}
