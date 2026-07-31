package serve

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/stretchr/testify/require"
)

func TestTableFrameViewSearchesFullFrameAndRecomputesTotalsAndShares(t *testing.T) {
	t.Parallel()

	spec := panel.Table("claims", "Claims", "claims").Searchable().Columns(
		panel.TableColumn{Field: "product", Label: "Product"},
		panel.TableColumn{Field: "claims", Label: "Claims"}.WithTotal(),
		panel.TableColumn{Field: "paid", Label: "Paid"}.WithTotal(),
		panel.TableColumn{Field: "share", Label: "Share"}.AsShareOf("paid").WithTotal(),
	).Terminal().Build()
	source := document.Frame{
		Columns: []document.Column{
			{Name: "product", Type: document.ColumnString},
			{Name: "claims", Type: document.ColumnNumber},
			{Name: "paid", Type: document.ColumnNumber},
			{Name: "share", Type: document.ColumnNumber},
		},
		Rows: [][]any{
			{"Motor liability", 2.0, 75.0, 0.0},
			{"Motor hull", 3.0, 25.0, 0.0},
			{"Travel", 7.0, 900.0, 0.0},
		},
	}

	view, summary := tableFrameView(spec, source, "MOTOR")
	require.Len(t, view.Rows, 2)
	require.InDelta(t, 75.0, view.Rows[0][3], 1e-9)
	require.InDelta(t, 25.0, view.Rows[1][3], 1e-9)
	require.InDelta(t, 0.0, source.Rows[0][3], 1e-9, "server view must not mutate the cached source frame")
	require.NotNil(t, summary)
	require.Equal(t, 2, summary.FilteredRows)
	require.InDelta(t, 5.0, summary.Values["claims"], 1e-9)
	require.InDelta(t, 100.0, summary.Values["paid"], 1e-9)
	require.InDelta(t, 100.0, summary.Values["share"], 1e-9)
}

func TestTableFrameViewKeepsNonTableFramesUntouched(t *testing.T) {
	t.Parallel()

	source := document.Frame{Columns: []document.Column{{Name: "value", Type: document.ColumnNumber}}, Rows: [][]any{{1.0}}}
	view, summary := tableFrameView(panel.Stat("metric", "Metric", "metric").Terminal().Build(), source, "1")
	require.Equal(t, source, view)
	require.Nil(t, summary)
}
