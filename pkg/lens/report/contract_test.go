package report_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/require"
)

type memoryHost struct {
	reports map[string]report.Definition
	source  *testSource
}

// False green: checking parameter strings while both executed queries still receive the dashboard's first period.
func TestBoundPlacementsExecuteWithTheirOwnTimeRange(t *testing.T) {
	t.Parallel()
	_, definition := fixture()
	source := report.SourceFunc{Metadata: report.SourceDescriptor{ID: "sales", Version: 1, Revision: "1", Capabilities: report.Capabilities{Filters: []string{"period"}, Views: []string{"summary"}}}, BuildSpec: func(_ context.Context, _ report.Definition, params report.Parameters) (lens.DashboardSpec, error) {
		var month int
		if err := json.Unmarshal(params["period"], &month); err != nil {
			return lens.DashboardSpec{}, err
		}
		spec := lens.DashboardSpec{ID: "monthly", Title: "Monthly", Variables: []lens.VariableSpec{{Name: "period", Kind: lens.VariableDateRange, RequestKeys: []string{"period", "range-start", "range-end"}, AllowAllTime: true}}, Datasets: []lens.DatasetSpec{{Name: "amount", Kind: lens.DatasetKindQuery, Source: "named", Query: &lens.QuerySpec{Kind: datasource.QueryKindNamed, Text: "amount"}}}, Rows: []lens.RowSpec{{Panels: []panel.Spec{panel.Stat("amount", "Amount", "amount").ValueField("amount").Terminal().Build()}}}}
		return report.BindRequest(spec, lensruntime.Request{Request: url.Values{"period": {"bounded"}, "range-start": {fmt.Sprintf("2026-%02d-01", month)}, "range-end": {fmt.Sprintf("2026-%02d-20", month)}}})
	}}
	composition := report.Composition{ID: "monthly", Revision: "1", Title: "Monthly", Rows: [][]report.Placement{{{ID: "a", ReportID: definition.ID, Span: 6, Parameters: report.Parameters{"period": json.RawMessage(`6`)}}, {ID: "b", ReportID: definition.ID, Span: 6}}}}
	spec, _, err := report.Compose(context.Background(), composition, report.Catalog{Definitions: []report.Definition{definition}, Sources: []report.Source{source}})
	require.NoError(t, err)
	ds := datasource.NewNamedDataSource()
	months := []int{}
	var lock sync.Mutex
	require.NoError(t, ds.Register("amount", func(_ context.Context, request datasource.QueryRequest) (*frame.FrameSet, error) {
		if request.TimeRange.Start == nil {
			return nil, fmt.Errorf("query lost its placement time range")
		}
		lock.Lock()
		months = append(months, int(request.TimeRange.Start.Month()))
		lock.Unlock()
		builder := frame.NewBuilder("amount").Number("amount", frame.RoleMetric)
		if err := builder.Append(frame.Row{"amount": float64(request.TimeRange.Start.Month())}); err != nil {
			return nil, err
		}
		return builder.FrameSet()
	}))
	_, err = lensruntime.New(lensruntime.Options{}).Execute(context.Background(), spec, lensruntime.Request{Locale: "en", DataScope: "test", DataSources: map[string]datasource.DataSource{"named": ds}, DataSourceIdentities: map[string]string{"named": "synthetic-v1"}}, lensruntime.DashboardScope())
	require.NoError(t, err)
	require.ElementsMatch(t, []int{6, 10}, months)
}

func (h *memoryHost) Report(_ context.Context, id string) (report.Definition, error) {
	d, ok := h.reports[id]
	if !ok {
		return report.Definition{}, serrors.New(serrors.NotFound, "report missing")
	}
	return d, nil
}
func (h *memoryHost) Source(_ context.Context, id string) (report.Source, error) {
	if id != h.source.descriptor.ID {
		return nil, serrors.New(serrors.NotFound, "source missing")
	}
	return h.source, nil
}

type fileHost struct {
	root   string
	source *testSource
}

func (h *fileHost) Report(_ context.Context, id string) (report.Definition, error) {
	data, err := os.ReadFile(filepath.Join(h.root, id+".json"))
	if err != nil {
		return report.Definition{}, err
	}
	return report.DecodeDefinition(data)
}
func (h *fileHost) Source(_ context.Context, _ string) (report.Source, error) { return h.source, nil }

type testSource struct{ descriptor report.SourceDescriptor }

func (s *testSource) Descriptor() report.SourceDescriptor { return s.descriptor }
func (s *testSource) Build(_ context.Context, _ report.Definition, params report.Parameters) (lens.DashboardSpec, error) {
	var amount float64
	if err := json.Unmarshal(params["period"], &amount); err != nil {
		return lens.DashboardSpec{}, err
	}
	builder := frame.NewBuilder("values").Number("amount", frame.RoleMetric)
	if err := builder.Append(frame.Row{"amount": amount}); err != nil {
		return lens.DashboardSpec{}, err
	}
	data, err := builder.FrameSet()
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	return lens.DashboardSpec{
		ID: "block", Title: "Block", Datasets: []lens.DatasetSpec{{Name: "values", Kind: lens.DatasetKindStatic, Static: data}},
		Rows: []lens.RowSpec{{Panels: []panel.Spec{panel.StatGroup("summary", "Summary",
			panel.Stat("amount", "Amount", "values").ValueField("amount").Terminal().Build(),
		).Build()}}},
	}, nil
}

func fixture() (*memoryHost, report.Definition) {
	d := report.Definition{ID: "revenue", Revision: "r1", Title: "Revenue", Source: report.SourceRef{ID: "sales", Version: 1}, View: "summary", Parameters: report.Parameters{"period": json.RawMessage(`10`)}}
	s := &testSource{descriptor: report.SourceDescriptor{ID: "sales", Version: 1, Revision: "v1", Capabilities: report.Capabilities{Filters: []string{"period"}, Views: []string{"summary"}, Save: true, Export: true}}}
	return &memoryHost{reports: map[string]report.Definition{d.ID: d}, source: s}, d
}

// False green: only comparing serialized definitions without executing both placements.
func TestCompositionRoundTripsAcrossIndependentHostsAndExecutesIsolatedPlacements(t *testing.T) {
	t.Parallel()
	host, definition := fixture()
	data, err := json.Marshal(definition)
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "revenue.json"), data, 0600))
	title := "June revenue"
	composition := report.Composition{ID: "sales", Revision: "1", Title: "Sales", Rows: [][]report.Placement{{
		{ID: "a", ReportID: "revenue", Title: &title, Span: 6, Parameters: report.Parameters{"period": json.RawMessage(`6`)}},
		{ID: "b", ReportID: "revenue", Span: 6},
	}}}
	for name, adapter := range map[string]report.Host{"memory": host, "file": &fileHost{root: dir, source: host.source}} {
		t.Run(name, func(t *testing.T) {
			spec, opened, err := report.Compose(context.Background(), composition, adapter)
			require.NoError(t, err)
			runtime := lensruntime.New(lensruntime.Options{})
			result, err := runtime.Execute(context.Background(), spec, lensruntime.Request{Locale: "en", DataScope: "test"}, lensruntime.DashboardScope())
			require.NoError(t, err)
			wire, err := document.Build(spec, result, document.BuildOptions{SnapshotID: name, GeneratedAt: time.Unix(1, 0), Locale: "en", InlineDepth: 10})
			require.NoError(t, err)
			require.NoError(t, wire.Validate())
			require.Len(t, wire.Panels, 2)
			require.Equal(t, "June revenue", wire.Layout.Rows[0].Panels[0].Groups[0].Label)
			require.Equal(t, "Revenue", wire.Layout.Rows[0].Panels[1].Groups[0].Label)
			require.NotEqual(t, opened[0].Parameters["period"], opened[1].Parameters["period"])
			require.JSONEq(t, `10`, string(host.reports["revenue"].Parameters["period"]))
			values := []float64{}
			for _, p := range result.Panels {
				if p.Frames != nil {
					values = append(values, p.Frames.Primary().Fields[0].Values[0].(float64))
				}
			}
			require.ElementsMatch(t, []float64{6, 10}, values)
		})
	}
}

// False green: a copy shares mutable parameters or the opened report is changed in-place.
func TestCopiesAndSourceUpdatesKeepOpenedResultImmutable(t *testing.T) {
	t.Parallel()
	host, definition := fixture()
	placement := report.Placement{ID: "first", ReportID: definition.ID}
	opened, err := report.Resolve(context.Background(), host, placement, nil)
	require.NoError(t, err)
	independent, err := report.CopyDefinition(definition, "independent", "r1")
	require.NoError(t, err)
	independent.Parameters["period"][0] = '9'
	require.JSONEq(t, `10`, string(definition.Parameters["period"]))
	duplicate, err := report.DuplicatePlacement(placement, "second")
	require.NoError(t, err)
	require.Equal(t, placement.ReportID, duplicate.ReportID)
	definition.Title = "Renamed"
	definition.Revision = "r2"
	host.reports[definition.ID] = definition
	host.source.descriptor.Revision = "v2"
	changed, err := opened.Changed(context.Background(), host)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "Revenue", opened.Definition.Title)
	require.Equal(t, "v1", opened.Source.Revision)
	latest, err := report.Resolve(context.Background(), host, placement, nil)
	require.NoError(t, err)
	require.Equal(t, "Renamed", latest.Definition.Title)
	host.source.descriptor.Version = 2
	_, err = report.Resolve(context.Background(), host, placement, nil)
	require.True(t, serrors.HasCode(err, serrors.FailedPrecondition))
}

// False green: unsupported inputs reach the source and are silently ignored.
func TestRejectsUnsupportedCapabilitiesAndSnapshotDocuments(t *testing.T) {
	t.Parallel()
	host, _ := fixture()
	_, err := report.Resolve(context.Background(), host, report.Placement{ID: "a", ReportID: "revenue", Parameters: report.Parameters{"unknown": json.RawMessage(`1`)}}, nil)
	require.True(t, serrors.HasCode(err, serrors.FailedPrecondition))
	_, err = report.DecodeDefinition([]byte(`{"id":"r","revision":"1","title":"R","source":{"id":"sales","version":1},"view":"summary","snapshotId":"expired"}`))
	require.True(t, serrors.HasCode(err, serrors.Invalid))
}
