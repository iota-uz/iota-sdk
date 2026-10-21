package report_test

import (
	"context"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/stretchr/testify/require"
	"net/url"
	"sync"
	"testing"
	"time"
)

// False green: metadata-only assertions cannot detect a query executing with another card's period.
func TestContextPlacementsExecuteIndependentlyAndBlockConflictingQueries(t *testing.T) {
	ctx := context.Background()
	source := report.SourceFunc{Metadata: report.SourceDescriptor{ID: "sales", Version: 1, Revision: "1", Capabilities: report.Capabilities{Views: []string{"summary"}, Filters: []string{"period"}, Dimensions: []string{"product"}}}}
	built := 0
	source.BuildContextSpec = func(_ context.Context, _ report.Definition, _ report.Parameters, effective filtercontext.EffectiveContext) (lens.DashboardSpec, error) {
		built++
		spec := lens.DashboardSpec{ID: "values", Variables: []lens.VariableSpec{{Name: "period", Kind: lens.VariableDateRange, RequestKeys: []string{"period", "range-start", "range-end"}, AllowAllTime: true}}, Datasets: []lens.DatasetSpec{{Name: "amount", Kind: lens.DatasetKindQuery, Source: "named", Query: &lens.QuerySpec{Kind: datasource.QueryKindNamed, Text: "amount"}}}, Rows: []lens.RowSpec{{Panels: []panel.Spec{panel.Stat("amount", "Amount", "amount").ValueField("amount").Terminal().Build()}}}}
		return report.BindRequest(spec, lensruntime.Request{Request: url.Values{"period": {"bounded"}, "range-start": {effective.Period.Start}, "range-end": {effective.Period.End}}})
	}
	definition := report.Definition{ID: "revenue", Revision: "1", Title: "Revenue", Source: report.SourceRef{ID: "sales", Version: 1}, View: "summary", Filters: filtercontext.ContextFilters{Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"A"}}}}}
	june := filtercontext.ContextPeriod{Mode: "fixed", Start: "2026-06-01", End: "2026-06-30"}
	board := filtercontext.ContextBoard{Calendar: filtercontext.ContextCalendar{Timezone: "Asia/Tashkent", AsOf: "2026-10-13T12:00:00Z", DefaultPeriod: filtercontext.ContextPeriod{Mode: "relative", Days: 30}}, Dashboard: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "fixed", Start: "2026-01-01", End: "2026-12-31"}}}
	composition := report.Composition{ID: "dashboard", Revision: "1", Title: "Sales", Context: &board, Rows: [][]report.Placement{{{ID: "june", ReportID: "revenue", Span: 4, Filters: filtercontext.ContextFilters{Period: &june}}, {ID: "annual", ReportID: "revenue", Span: 4}, {ID: "conflict", ReportID: "revenue", Span: 4, Filters: filtercontext.ContextFilters{Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"B"}}}}}}}}
	spec, resolved, err := report.Compose(ctx, composition, report.Catalog{Definitions: []report.Definition{definition}, Sources: []report.Source{source}})
	require.NoError(t, err)
	require.Equal(t, 2, built)
	require.Equal(t, []string{"product"}, resolved[2].Context.Conflicts)
	require.Len(t, resolved[2].Context.Restrictions, 2)
	opened, err := report.PlaceAnalysis("opened", definition, *resolved[0].Context)
	require.NoError(t, err)
	library := report.PlaceLibraryReport("library", definition.ID)
	composition.Rows = [][]report.Placement{{opened}, {library}}
	_, placed, err := report.Compose(ctx, composition, report.Catalog{Definitions: []report.Definition{definition}, Sources: []report.Source{source}})
	require.NoError(t, err)
	require.Equal(t, "2026-06-01", placed[0].Context.Period.Start)
	require.Equal(t, "2026-01-01", placed[1].Context.Period.Start)

	ds := datasource.NewNamedDataSource()
	var lock sync.Mutex
	starts := []string{}
	require.NoError(t, ds.Register("amount", func(_ context.Context, request datasource.QueryRequest) (*frame.FrameSet, error) {
		lock.Lock()
		starts = append(starts, request.TimeRange.Start.Format("2006-01-02"))
		lock.Unlock()
		b := frame.NewBuilder("amount").Number("amount", frame.RoleMetric)
		if err := b.Append(frame.Row{"amount": float64(request.TimeRange.Start.Month())}); err != nil {
			return nil, err
		}
		return b.FrameSet()
	}))
	result, err := lensruntime.New(lensruntime.Options{}).Execute(ctx, spec, lensruntime.Request{Locale: "en", DataScope: "test", DataSources: map[string]datasource.DataSource{"named": ds}, DataSourceIdentities: map[string]string{"named": "context-test"}}, lensruntime.DashboardScope())
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"2026-06-01", "2026-01-01"}, starts)
	wire, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "context", GeneratedAt: time.Unix(1, 0), Locale: "en", InlineDepth: 10})
	require.NoError(t, err)
	require.NoError(t, wire.Validate())
}
