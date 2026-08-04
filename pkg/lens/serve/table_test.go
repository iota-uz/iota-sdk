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

	view, summary := tableFrameView(spec, source, "MOTOR", nil)
	require.Len(t, view.Rows, 2)
	require.InDelta(t, 7.5, view.Rows[0][3], 1e-9)
	require.InDelta(t, 2.5, view.Rows[1][3], 1e-9)
	require.InDelta(t, 0.0, source.Rows[0][3], 1e-9, "server view must not mutate the cached source frame")
	require.NotNil(t, summary)
	require.Equal(t, 2, summary.FilteredRows)
	require.InDelta(t, 5.0, summary.Values["claims"], 1e-9)
	require.InDelta(t, 100.0, summary.Values["paid"], 1e-9)
	require.InDelta(t, 10.0, summary.Values["share"], 1e-9)
	require.InDelta(t, 1000.0, summary.FullValues["paid"], 1e-9)
	require.InDelta(t, 100.0, summary.FullValues["share"], 1e-9)
	require.Equal(t, 3, summary.TotalRows)
}

func TestTableFrameViewKeepsNonTableFramesUntouched(t *testing.T) {
	t.Parallel()

	source := document.Frame{Columns: []document.Column{{Name: "value", Type: document.ColumnNumber}}, Rows: [][]any{{1.0}}}
	view, summary := tableFrameView(panel.Stat("metric", "Metric", "metric").Terminal().Build(), source, "1", nil)
	require.Equal(t, source, view)
	require.Nil(t, summary)
}

func TestTableFrameViewSortsFilteredRowsOnServer(t *testing.T) {
	t.Parallel()
	spec := panel.Table("claims", "Claims", "claims").Searchable().Columns(
		panel.TableColumn{Field: "product", Label: "Product"},
		panel.TableColumn{Field: "paid", Label: "Paid"},
	).Terminal().Build()
	source := document.Frame{
		Columns: []document.Column{{Name: "product", Type: document.ColumnString}, {Name: "paid", Type: document.ColumnNumber}},
		Rows:    [][]any{{"Motor A", 10.0}, {"Travel", 999.0}, {"Motor B", 30.0}},
	}
	view, _ := tableFrameView(spec, source, "motor", &document.TableSort{Field: "paid", Direction: document.SortDescending})
	require.Equal(t, [][]any{{"Motor B", 30.0}, {"Motor A", 10.0}}, view.Rows)
	require.Equal(t, "Motor A", source.Rows[0][0], "sorting must not mutate the cached frame")
}

func TestPaginatePanelFramePreservesServerSortAcrossPages(t *testing.T) {
	t.Parallel()
	spec := panel.Table("claims", "Claims", "claims").Columns(panel.TableColumn{Field: "paid", Label: "Paid"}).Terminal().Build()
	source := document.Frame{Columns: []document.Column{{Name: "paid", Type: document.ColumnNumber}}, Rows: [][]any{{40.0}, {30.0}, {20.0}, {10.0}}}
	page, metadata := paginatePanelFrame(spec, source, 2, 2)
	require.Equal(t, [][]any{{20.0}, {10.0}}, page.Rows)
	require.Equal(t, &document.QueryPage{Number: 2, Size: 2, HasNext: false}, metadata)
}
