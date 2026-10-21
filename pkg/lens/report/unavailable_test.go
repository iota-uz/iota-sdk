package report_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
)

type unreadableHost struct {
	*memoryHost
	errs map[string]error
}

func (h unreadableHost) Report(ctx context.Context, id string) (report.Definition, error) {
	if err, ok := h.errs[id]; ok {
		return report.Definition{}, err
	}
	return h.memoryHost.Report(ctx, id)
}

// False green: asserting only that Compose succeeds would pass if the hidden
// placements were dropped, or if a forbidden slot still carried its caption.
func TestComposeRendersDeletedAndForbiddenPlacementsInPlace(t *testing.T) {
	t.Parallel()
	base, _ := fixture()
	host := unreadableHost{memoryHost: base, errs: map[string]error{
		"gone":   serrors.New(serrors.NotFound, "gone").WithCause(report.ErrReportDeleted),
		"secret": report.ErrReportForbidden,
		"broken": errors.New("storage offline"),
	}}
	caption, hidden := "Archived revenue", "Board-only margin"
	composition := report.Composition{ID: "sales", Revision: "1", Title: "Sales", Rows: [][]report.Placement{{
		{ID: "a", ReportID: "revenue", Span: 4},
		{ID: "d", ReportID: "gone", Title: &caption, Span: 4},
		{ID: "f", ReportID: "secret", Title: &hidden, Span: 4},
	}}}
	spec, opened, err := report.Compose(context.Background(), composition, host)
	require.NoError(t, err)
	require.Len(t, opened, 3)
	require.Empty(t, opened[0].Unavailable)
	require.Equal(t, document.GroupUnavailableReportDeleted, opened[1].Unavailable)
	require.Equal(t, document.GroupUnavailableReportForbidden, opened[2].Unavailable)

	result, err := lensruntime.New(lensruntime.Options{}).Execute(context.Background(), spec, lensruntime.Request{Locale: "en", DataScope: "test"}, lensruntime.DashboardScope())
	require.NoError(t, err)
	wire, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "unavailable", GeneratedAt: time.Unix(1, 0), Locale: "en", InlineDepth: 10})
	require.NoError(t, err)
	require.NoError(t, wire.Validate())
	groups := map[string]document.LayoutGroup{}
	for _, item := range wire.Layout.Rows[0].Panels {
		groups[item.Groups[0].ID] = item.Groups[0]
	}
	require.Empty(t, groups["a"].Unavailable)
	require.Equal(t, document.GroupUnavailableReportDeleted, groups["d"].Unavailable)
	require.Equal(t, caption, groups["d"].Label, "a deleted report keeps the card's own caption")
	require.Equal(t, document.GroupUnavailableReportForbidden, groups["f"].Unavailable)
	require.Empty(t, groups["f"].Label, "a forbidden slot must not reveal its caption")
	require.Nil(t, groups["f"].FilterContext)
	require.Equal(t, 4, groups["f"].Span)
	require.NotContains(t, string(mustJSON(t, wire)), hidden)

	composition.Rows[0][1].ReportID = "broken"
	_, _, err = report.Compose(context.Background(), composition, host)
	require.Error(t, err, "other host failures still fail the composition")

	_, err = report.Resolve(context.Background(), host, report.Placement{ID: "x", ReportID: "secret"}, nil)
	require.ErrorIs(t, err, report.ErrReportForbidden)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
