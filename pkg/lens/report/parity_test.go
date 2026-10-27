package report_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
)

// False green: comparing the two compositions' JSON definitions, or only the
// panel ids, would pass while one rendering carried other values or formats.
func TestPlacementRendersTheSameResultStandaloneAndEmbedded(t *testing.T) {
	t.Parallel()
	host, _ := fixture()
	open := func(rows [][]report.Placement) *document.DashboardDocument {
		spec, _, err := report.Compose(context.Background(), report.Composition{ID: "board", Revision: "1", Title: "Board", Rows: rows}, host)
		require.NoError(t, err)
		result, err := lensruntime.New(lensruntime.Options{}).Execute(context.Background(), spec, lensruntime.Request{Locale: "en", DataScope: "test"}, lensruntime.DashboardScope())
		require.NoError(t, err)
		doc, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "parity", GeneratedAt: time.Unix(1, 0), Locale: "en", InlineDepth: 10})
		require.NoError(t, err)
		require.NoError(t, doc.Validate())
		return doc
	}
	standalone := open([][]report.Placement{{{ID: "a", ReportID: "revenue"}}})
	embedded := open([][]report.Placement{{{ID: "a", ReportID: "revenue", Span: 6}, {ID: "b", ReportID: "revenue", Span: 6}}})
	own := func(doc *document.DashboardDocument) map[string]document.Panel {
		panels := map[string]document.Panel{}
		for _, item := range doc.Panels {
			if strings.HasPrefix(item.ID, "a::") {
				panels[item.ID] = item
			}
		}
		return panels
	}
	alone, placed := own(standalone), own(embedded)
	require.NotEmpty(t, alone)
	require.Equal(t, alone, placed, "kind, title, encoding, format and actions match")
	for id, item := range alone {
		require.Equal(t, standalone.Frames[item.Frame], embedded.Frames[placed[id].Frame], "panel %s values match", id)
	}
}
