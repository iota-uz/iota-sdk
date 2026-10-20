package main

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

// cardHost serves the synthetic card catalog and answers the two report
// lookups a stored host would fail with: a deleted and a hidden report.
type cardHost struct{ report.Catalog }

func (h cardHost) Report(ctx context.Context, id string) (report.Definition, error) {
	switch id {
	case "deleted":
		return report.Definition{}, report.ErrReportDeleted
	case "hidden":
		return report.Definition{}, report.ErrReportForbidden
	}
	return h.Catalog.Report(ctx, id)
}

// cardExamples emits one report opened on its own and the same report placed
// on a dashboard beside a host-rendered, a deleted and a forbidden placement.
// Both documents come from one definition and source through Compose, so a
// parity check compares two renderings of one result, not two hand-written ones.
func cardExamples() error {
	ctx := context.Background()
	calendar := filtercontext.ContextCalendar{Timezone: "Asia/Tashkent", AsOf: "2026-10-12T00:00:00Z", DefaultPeriod: filtercontext.ContextPeriod{Mode: "relative", Days: 30}}
	revenue := report.Definition{ID: "revenue", Revision: "1", Title: "Revenue", Source: report.SourceRef{ID: "monthly", Version: 1}, View: "summary"}
	legacy := report.Definition{ID: "legacy", Revision: "1", Title: "Legacy chart", Source: report.SourceRef{ID: "legacy", Version: 1}, View: "chart"}
	capabilities := report.Capabilities{Filters: []string{"period"}, Views: []string{"summary"}, Drill: true, Save: true, Export: true}
	monthly := report.SourceFunc{Metadata: report.SourceDescriptor{ID: "monthly", Version: 1, Revision: "1", Capabilities: capabilities},
		BuildContextSpec: func(context.Context, report.Definition, report.Parameters, filtercontext.EffectiveContext) (lens.DashboardSpec, error) {
			return cardSpec()
		}}
	legacySource := report.SourceFunc{Metadata: report.SourceDescriptor{ID: "legacy", Version: 1, Revision: "1", Capabilities: report.Capabilities{Filters: []string{"period"}, Views: []string{"chart"}, Export: true}},
		BuildContextSpec: func(context.Context, report.Definition, report.Parameters, filtercontext.EffectiveContext) (lens.DashboardSpec, error) {
			return legacySpec()
		}}
	host := cardHost{report.Catalog{Definitions: []report.Definition{revenue, legacy}, Sources: []report.Source{monthly, legacySource}}}
	board := func(id, title string, rows [][]report.Placement) report.Composition {
		return report.Composition{ID: id, Revision: "1", Title: title, Rows: rows, Context: &filtercontext.ContextBoard{Calendar: calendar}}
	}
	removed := "Archived forecast"
	compositions := map[string]report.Composition{
		"standalone": board("revenue-report", "Revenue", [][]report.Placement{{{ID: "revenue", ReportID: revenue.ID}}}),
		"embedded": board("card-dashboard", "Card dashboard", [][]report.Placement{
			{{ID: "revenue", ReportID: revenue.ID}},
			{{ID: "legacy", ReportID: legacy.ID, Span: 4}, {ID: "deleted", ReportID: "deleted", Title: &removed, Span: 4}, {ID: "forbidden", ReportID: "hidden", Span: 4}},
		}),
	}
	documents := map[string]*document.DashboardDocument{}
	for name, composition := range compositions {
		spec, _, err := report.Compose(ctx, composition, host)
		if err != nil {
			return err
		}
		result, err := lensruntime.New(lensruntime.Options{}).Execute(ctx, spec, lensruntime.Request{Locale: "en", DataScope: "synthetic"}, lensruntime.DashboardScope())
		if err != nil {
			return err
		}
		doc, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "card-" + name, Locale: "en", GeneratedAt: time.Date(2026, 10, 12, 6, 30, 0, 0, time.UTC), InlineDepth: 10})
		if err != nil {
			return err
		}
		if err = doc.Validate(); err != nil {
			return err
		}
		documents[name] = doc
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Source     report.SourceDescriptor     `json:"source"`
		Standalone *document.DashboardDocument `json:"standalone"`
		Embedded   *document.DashboardDocument `json:"embedded"`
	}{monthly.Metadata, documents["standalone"], documents["embedded"]})
}

func cardSpec() (lens.DashboardSpec, error) {
	builder := frame.NewBuilder("data").String("label", frame.RoleDimension).Number("revenue", frame.RoleMetric).Number("refunds", frame.RoleMetric)
	for _, row := range []frame.Row{{"label": "Online", "revenue": 7200.0, "refunds": 0.0}, {"label": "Agency", "revenue": 4800.0, "refunds": 0.0}} {
		if err := builder.Append(row); err != nil {
			return lens.DashboardSpec{}, err
		}
	}
	data, err := builder.FrameSet()
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	return lens.DashboardSpec{ID: "summary", Title: "Summary", Datasets: []lens.DatasetSpec{{Name: "data", Kind: lens.DatasetKindStatic, Static: data}}, Rows: []lens.RowSpec{
		{Panels: []panel.Spec{panel.StatGroup("kpis", "Revenue by channel",
			panel.Stat("online", "Online revenue", "data").ValueField("revenue").Terminal().Build(),
			panel.Stat("refunds", "Refunds", "data").ValueField("refunds").Terminal().Build(),
		).Build()}},
		{Panels: []panel.Spec{
			panel.Bar("channels", "Revenue by channel", "data").CategoryField("label").ValueField("revenue").Terminal().Span(6).Build(),
			panel.Table("table", "Source rows", "data").IDField("label").Columns(panel.TableColumn{Field: "label", Label: "Channel"}, panel.TableColumn{Field: "revenue", Label: "Revenue"}).Terminal().Span(6).Build(),
		}},
	}}, nil
}

func legacySpec() (lens.DashboardSpec, error) {
	builder := frame.NewBuilder("legacy").String("month", frame.RoleDimension).Number("value", frame.RoleMetric)
	for _, row := range []frame.Row{{"month": "Aug", "value": 3.0}, {"month": "Sep", "value": 5.0}} {
		if err := builder.Append(row); err != nil {
			return lens.DashboardSpec{}, err
		}
	}
	data, err := builder.FrameSet()
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	chart := panel.Table("legacy-chart", "Legacy chart", "legacy").IDField("month").
		Columns(panel.TableColumn{Field: "month", Label: "Month"}, panel.TableColumn{Field: "value", Label: "Value"}).
		Renderer("legacy-bars", json.RawMessage(`{"field":"value","label":"month"}`)).Terminal().Build()
	return lens.DashboardSpec{ID: "chart", Title: "Chart", Datasets: []lens.DatasetSpec{{Name: "legacy", Kind: lens.DatasetKindStatic, Static: data}}, Rows: []lens.RowSpec{{Panels: []panel.Spec{chart}}}}, nil
}
