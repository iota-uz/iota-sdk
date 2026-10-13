// lens-report-example emits a synthetic composition through the production Go runtime.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "--filters" {
		return filterExamples()
	}
	definition := report.Definition{ID: "revenue", Revision: "1", Title: "Revenue", Source: report.SourceRef{ID: "monthly", Version: 1}, View: "summary", Parameters: report.Parameters{"months": json.RawMessage(`12`)}}
	source := report.SourceFunc{Metadata: report.SourceDescriptor{ID: "monthly", Version: 1, Revision: "1", Capabilities: report.Capabilities{Filters: []string{"months"}, Views: []string{"summary"}, Save: true, Export: true}}, BuildSpec: build}
	title := "First six months"
	composition := report.Composition{ID: "reusable-reports", Revision: "1", Title: "Reusable reports", Rows: [][]report.Placement{{
		{ID: "first", ReportID: definition.ID, Title: &title, Span: 6, Parameters: report.Parameters{"months": json.RawMessage(`6`)}},
		{ID: "second", ReportID: definition.ID, Span: 6},
	}}}
	spec, _, err := report.Compose(context.Background(), composition, report.Catalog{Definitions: []report.Definition{definition}, Sources: []report.Source{source}})
	if err != nil {
		return err
	}
	result, err := lensruntime.New(lensruntime.Options{}).Execute(context.Background(), spec, lensruntime.Request{Locale: "en", DataScope: "synthetic"}, lensruntime.DashboardScope())
	if err != nil {
		return err
	}
	doc, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "report-example", Locale: "en", GeneratedAt: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), InlineDepth: 10})
	if err != nil {
		return err
	}
	if err = doc.Validate(); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Definition  report.Definition           `json:"definition"`
		Composition report.Composition          `json:"composition"`
		Source      report.SourceDescriptor     `json:"source"`
		Document    *document.DashboardDocument `json:"document"`
	}{definition, composition, source.Metadata, doc})
}

func build(_ context.Context, _ report.Definition, params report.Parameters) (lens.DashboardSpec, error) {
	var months int
	if err := json.Unmarshal(params["months"], &months); err != nil {
		return lens.DashboardSpec{}, err
	}
	if months < 1 || months > 12 {
		return lens.DashboardSpec{}, fmt.Errorf("months outside supported range")
	}
	builder := frame.NewBuilder("data").String("label", frame.RoleDimension).Number("revenue", frame.RoleMetric)
	if err := builder.Append(frame.Row{"label": "Online", "revenue": float64(months * 1200)}); err != nil {
		return lens.DashboardSpec{}, err
	}
	if err := builder.Append(frame.Row{"label": "Agency", "revenue": float64(months * 800)}); err != nil {
		return lens.DashboardSpec{}, err
	}
	data, err := builder.FrameSet()
	if err != nil {
		return lens.DashboardSpec{}, err
	}
	return lens.DashboardSpec{ID: "summary", Title: "Summary", Datasets: []lens.DatasetSpec{{Name: "data", Kind: lens.DatasetKindStatic, Static: data}}, Rows: []lens.RowSpec{
		{Panels: []panel.Spec{panel.StatGroup("kpis", "Revenue by channel",
			panel.Stat("online", "Online", "data").ValueField("revenue").Terminal().Build(),
		).Build()}},
		{Panels: []panel.Spec{panel.Table("table", "Source rows", "data").IDField("label").Columns(panel.TableColumn{Field: "label", Label: "Channel"}, panel.TableColumn{Field: "revenue", Label: "Revenue"}).Terminal().Build()}},
	}}, nil
}
