package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/document"
	"github.com/iota-uz/iota-sdk/pkg/lens/report"
	lensruntime "github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

var catalogSources = []string{"Sales", "Claims", "Underwriting", "Finance", "Reserves"}

var catalogOwners = []report.CatalogOwner{{ID: "u-ann", Name: "Ann Ivanova"}, {ID: "u-bek", Name: "Bek Karimov"}}

// catalogExamples emits fifty synthetic readable catalog entries for a target
// composition that already places one of them, the destinations a report can
// be added to, and the preview document of the first entry.
func catalogExamples() error {
	ctx := context.Background()
	definitions := make([]report.Definition, 0, 50)
	entries := make([]report.CatalogEntry, 0, 50)
	for i := range 50 {
		source := catalogSources[i%len(catalogSources)]
		id := fmt.Sprintf("report-%02d", i+1)
		title := fmt.Sprintf("%s report %02d", source, i+1)
		entry := report.CatalogEntry{ID: id, Revision: "1", Title: title, Source: source, Description: fmt.Sprintf("Monthly %s figures by channel.", source)}
		switch i % 3 {
		case 0:
			entry.Ownership = report.OwnershipSystem
		case 1:
			entry.Ownership, entry.Owner = report.OwnershipPersonal, &catalogOwners[0]
		default:
			entry.Ownership, entry.Owner = report.OwnershipShared, &catalogOwners[1]
		}
		entries = append(entries, entry)
	}
	entries[0].Title, entries[0].Description = "Revenue", "Revenue by channel for the opened period."
	entries[1].Title, entries[1].Members, entries[1].Description = "Portfolio overview", 3, "Premium, claims and loss ratio read together."
	entries[2].Title, entries[2].Restriction = "Large losses", "Shows the last 12 months only; export is unavailable."
	for _, entry := range entries {
		definitions = append(definitions, report.Definition{ID: entry.ID, Revision: entry.Revision, Title: entry.Title, Source: report.SourceRef{ID: "monthly", Version: 1}, View: "summary", Parameters: report.Parameters{"months": json.RawMessage(`12`)}})
	}
	source := report.SourceFunc{Metadata: report.SourceDescriptor{ID: "monthly", Version: 1, Revision: "1", Capabilities: report.Capabilities{Filters: []string{"months"}, Views: []string{"summary"}, Save: true, Export: true}}, BuildSpec: build}
	catalog := report.Catalog{Definitions: definitions, Sources: []report.Source{source}}
	target := report.Composition{ID: "sales-board", Revision: "3", Title: "Sales board", Rows: [][]report.Placement{{{ID: "placed-revenue", ReportID: entries[0].ID}}}}
	page := report.MarkPlaced(report.CatalogPage{Entries: entries}, target)
	if err := page.Validate(); err != nil {
		return err
	}
	preview := report.Composition{ID: "preview", Revision: "1", Title: entries[0].Title, Rows: [][]report.Placement{{{ID: "preview", ReportID: entries[0].ID}}}}
	spec, _, err := report.Compose(ctx, preview, catalog)
	if err != nil {
		return err
	}
	result, err := lensruntime.New(lensruntime.Options{}).Execute(ctx, spec, lensruntime.Request{Locale: "en", DataScope: "synthetic"}, lensruntime.DashboardScope())
	if err != nil {
		return err
	}
	doc, err := document.Build(spec, result, document.BuildOptions{SnapshotID: "catalog-preview", Locale: "en", GeneratedAt: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), InlineDepth: 10})
	if err != nil {
		return err
	}
	if err = doc.Validate(); err != nil {
		return err
	}
	destinations := []report.Destination{{ID: "sales-board", Title: "Sales board", Placed: 1}, {ID: "claims-board", Title: "Claims board"}, {ID: "weekly", Title: "Weekly review"}}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Target       report.Composition          `json:"target"`
		Page         report.CatalogPage          `json:"page"`
		Destinations []report.Destination        `json:"destinations"`
		Preview      *document.DashboardDocument `json:"preview"`
	}{target, page, destinations, doc})
}
