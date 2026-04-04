package engine

import (
	"context"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	lenscompile "github.com/iota-uz/iota-sdk/pkg/lens/compile"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	lensspec "github.com/iota-uz/iota-sdk/pkg/lens/spec"
	"github.com/stretchr/testify/require"
)

func TestRunExecutesManualStaticDashboard(t *testing.T) {
	t.Parallel()

	statsBuilder := frame.NewBuilder("stats").Number("value", frame.RoleMetric)
	err := statsBuilder.Append(frame.Row{"value": 42})
	require.NoError(t, err)
	stats, err := statsBuilder.FrameSet()
	require.NoError(t, err)

	doc := lensspec.Document{
		Version:     lensspec.DocumentVersion,
		ID:          "engine-report",
		Title:       lensspec.LiteralText("Engine"),
		Description: lensspec.LiteralText("Run"),
		Datasets: []lensspec.DatasetSpec{
			{Name: "stats", Kind: lens.DatasetKindStatic, StaticRef: "stats_dataset"},
		},
		Rows: []lensspec.RowSpec{
			{Panels: []lensspec.PanelSpec{{ID: "total", Title: lensspec.LiteralText("Total"), Kind: panel.KindStat, Dataset: "stats", Fields: lensspec.FieldMappingSpec{Value: "value"}}}},
		},
	}

	result, err := Run(context.Background(), doc, lenscompile.Options{
		Locale: "en",
		Values: map[string]any{"stats_dataset": stats},
	}, Request{
		Runtime: lensruntime.Request{Locale: "en", Timezone: "UTC"},
	})
	require.NoError(t, err)
	require.NotNil(t, result.Dashboard)
	require.NotNil(t, result.Dashboard.Panel("total"))
	require.NoError(t, result.Dashboard.Panel("total").Error)
}
