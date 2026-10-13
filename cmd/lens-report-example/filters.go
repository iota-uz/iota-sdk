package main

import (
	"context"
	"encoding/json"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/filtercontext"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"os"
	"time"
)

// Synthetic adapters run through the same compiler/runtime as consumer reports.
func filterExamples() error {
	ctx := context.Background()
	calendar := filtercontext.ContextCalendar{Timezone: "Asia/Tashkent", AsOf: "2026-07-22T12:00:00Z", DefaultPeriod: filtercontext.ContextPeriod{Mode: "relative", Days: 30}}
	annual := filtercontext.ContextPeriod{Mode: "fixed", Start: "2026-01-01", End: "2026-12-31"}
	june := filtercontext.ContextPeriod{Mode: "fixed", Start: "2026-06-01", End: "2026-06-30"}
	definitions := []report.Definition{
		{ID: "revenue", Revision: "1", Title: "Revenue by channel", Source: report.SourceRef{ID: "monthly", Version: 1}, View: "summary", Filters: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "relative", Days: 30}, Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"A"}}}}},
		{ID: "orders", Revision: "1", Title: "All orders", Source: report.SourceRef{ID: "limited", Version: 1}, View: "summary", Filters: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "relative", Days: 90}}},
	}
	builder := func(ctx context.Context, definition report.Definition, _ report.Parameters, effective filtercontext.EffectiveContext) (lens.DashboardSpec, error) {
		months := 12
		if effective.Period.Start != "" {
			start, parseErr := time.Parse("2006-01-02", effective.Period.Start)
			if parseErr != nil {
				return lens.DashboardSpec{}, parseErr
			}
			end, parseErr := time.Parse("2006-01-02", effective.Period.End)
			if parseErr != nil {
				return lens.DashboardSpec{}, parseErr
			}
			months = int(end.Sub(start).Hours()/24+1) / 30
			if months < 1 {
				months = 1
			}
			if months > 12 {
				months = 12
			}
		}
		raw, err := json.Marshal(months)
		if err != nil {
			return lens.DashboardSpec{}, err
		}
		return build(ctx, definition, report.Parameters{"months": raw})
	}
	sources := []report.Source{
		report.SourceFunc{Metadata: report.SourceDescriptor{ID: "monthly", Version: 1, Revision: "1", Capabilities: report.Capabilities{Filters: []string{"period"}, Views: []string{"summary"}, Dimensions: []string{"product"}, Save: true, Export: true}}, BuildContextSpec: builder},
		report.SourceFunc{Metadata: report.SourceDescriptor{ID: "limited", Version: 1, Revision: "1", Capabilities: report.Capabilities{Filters: []string{"period"}, Views: []string{"summary"}, Save: true, Export: true}}, BuildContextSpec: builder},
	}
	boards := []struct {
		Name  string
		Board filtercontext.ContextBoard
	}{
		{"Report periods", filtercontext.ContextBoard{Calendar: calendar, Dashboard: filtercontext.ContextFilters{Period: &filtercontext.ContextPeriod{Mode: "reports"}}}},
		{"Annual dashboard", filtercontext.ContextBoard{Calendar: calendar, Dashboard: filtercontext.ContextFilters{Period: &annual}}},
		{"June card", filtercontext.ContextBoard{Calendar: calendar, Dashboard: filtercontext.ContextFilters{Period: &annual}, Cards: map[string]filtercontext.ContextFilters{"revenue": {Period: &june}}}},
		{"Conflict and unsupported", filtercontext.ContextBoard{Calendar: calendar, Dashboard: filtercontext.ContextFilters{Period: &annual, Constraints: []filtercontext.ContextConstraint{{Dimension: "product", Values: []string{"B"}}}}}},
	}
	examples := []struct {
		Name     string                      `json:"name"`
		Document *document.DashboardDocument `json:"document"`
	}{}
	for _, example := range boards {
		composition := report.Composition{ID: "scoped-filters", Revision: "1", Title: "Scoped report filters", Context: &example.Board, Rows: [][]report.Placement{{{ID: "revenue", ReportID: "revenue", SectionID: "sales", Span: 6}, {ID: "orders", ReportID: "orders", SectionID: "orders", Span: 6}}}}
		spec, _, err := report.Compose(ctx, composition, report.Catalog{Definitions: definitions, Sources: sources})
		if err != nil {
			return err
		}
		result, err := lensruntime.New(lensruntime.Options{}).Execute(ctx, spec, lensruntime.Request{Locale: "en", DataScope: "synthetic"}, lensruntime.DashboardScope())
		if err != nil {
			return err
		}
		doc, err := document.Build(spec, result, document.BuildOptions{SnapshotID: example.Name, Locale: "en", GeneratedAt: time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC), InlineDepth: 10})
		if err != nil {
			return err
		}
		doc.ContextEditor = &filtercontext.ContextEditor{URL: "/", Value: example.Board, Saved: boards[0].Board, DimensionLabels: map[string]string{"product": "Product"}}
		if err := doc.Validate(); err != nil {
			return err
		}
		examples = append(examples, struct {
			Name     string                      `json:"name"`
			Document *document.DashboardDocument `json:"document"`
		}{example.Name, doc})
	}
	return json.NewEncoder(os.Stdout).Encode(examples)
}
