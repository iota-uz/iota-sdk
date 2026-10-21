package report_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens/report"
)

func catalogOf(count int) report.Catalog {
	definitions := make([]report.Definition, 0, count)
	for i := range count {
		definitions = append(definitions, report.Definition{ID: fmt.Sprintf("r%02d", i), Revision: "1", Title: fmt.Sprintf("Report %02d", i), Source: report.SourceRef{ID: "s", Version: 1}, View: "v"})
	}
	definitions[3].Title = "Loss ratio"
	return report.Catalog{Definitions: definitions}
}

// False green: a search that ignored the cursor would return the first page twice and still "page".
func TestCatalogSearchPagesSearchesAndScopes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	catalog := catalogOf(45)

	first, err := catalog.Search(ctx, report.CatalogQuery{})
	require.NoError(t, err)
	require.Len(t, first.Entries, report.DefaultCatalogLimit)
	require.Equal(t, "20", first.Next)
	require.NoError(t, first.Validate())
	require.Equal(t, report.OwnershipSystem, first.Entries[0].Ownership)

	second, err := catalog.Search(ctx, report.CatalogQuery{Cursor: first.Next, Limit: 30})
	require.NoError(t, err)
	require.Len(t, second.Entries, 25)
	require.Empty(t, second.Next)
	require.Equal(t, "r20", second.Entries[0].ID)

	found, err := catalog.Search(ctx, report.CatalogQuery{Search: "  LOSS "})
	require.NoError(t, err)
	require.Equal(t, []string{"r03"}, entryIDs(found))

	personal, err := catalog.Search(ctx, report.CatalogQuery{Scope: report.CatalogScopePersonal})
	require.NoError(t, err)
	require.Empty(t, personal.Entries)
	shared, err := catalog.Search(ctx, report.CatalogQuery{Scope: report.CatalogScopeShared})
	require.NoError(t, err)
	require.Empty(t, shared.Entries, "built-in reports were not shared by anyone")
	require.True(t, report.CatalogScopeShared.Includes(report.OwnershipShared))
	require.False(t, report.CatalogScopeShared.Includes(report.OwnershipSystem))
	require.True(t, report.CatalogScopeAll.Includes(report.OwnershipSystem))

	beyond, err := catalog.Search(ctx, report.CatalogQuery{Cursor: "999"})
	require.NoError(t, err)
	require.NotNil(t, beyond.Entries)
	require.Empty(t, beyond.Entries)

	encoded, err := json.Marshal(beyond)
	require.NoError(t, err)
	require.JSONEq(t, `{"entries":[]}`, string(encoded))
}

func TestCatalogQueryRejectsUnservableValues(t *testing.T) {
	t.Parallel()
	for name, query := range map[string]report.CatalogQuery{
		"scope":    {Scope: "everyone"},
		"negative": {Limit: -1},
		"too many": {Limit: report.MaxCatalogLimit + 1},
		"target":   {Target: "../other"},
	} {
		_, err := query.Normalize()
		require.Error(t, err, name)
	}
	_, err := catalogOf(5).Search(context.Background(), report.CatalogQuery{Cursor: "next"})
	require.Error(t, err)
	normalized, err := report.CatalogQuery{Search: " a ", Target: "board"}.Normalize()
	require.NoError(t, err)
	require.Equal(t, report.CatalogQuery{Search: "a", Scope: report.CatalogScopeAll, Limit: report.DefaultCatalogLimit, Target: "board"}, normalized)
}

func TestCatalogPageValidationAndPlacementCounts(t *testing.T) {
	t.Parallel()
	entry := report.CatalogEntry{ID: "revenue", Revision: "1", Title: "Revenue", Ownership: report.OwnershipShared, Owner: &report.CatalogOwner{ID: "u1", Name: "Ann"}}
	require.NoError(t, report.CatalogPage{Entries: []report.CatalogEntry{entry}}.Validate())
	require.Error(t, report.CatalogPage{Entries: []report.CatalogEntry{entry, entry}}.Validate())
	for name, broken := range map[string]func(*report.CatalogEntry){
		"title":     func(e *report.CatalogEntry) { e.Title = " " },
		"ownership": func(e *report.CatalogEntry) { e.Ownership = "" },
		"members":   func(e *report.CatalogEntry) { e.Members = -1 },
		"owner":     func(e *report.CatalogEntry) { e.Owner = &report.CatalogOwner{ID: "u1"} },
	} {
		candidate := entry
		broken(&candidate)
		require.Error(t, candidate.Validate(), name)
	}

	composition := report.Composition{ID: "board", Revision: "1", Title: "Board", Rows: [][]report.Placement{
		{{ID: "a", ReportID: "revenue", Span: 6}, {ID: "b", ReportID: "revenue", Span: 6}},
		{{ID: "c", ReportID: "claims"}},
	}}
	require.Equal(t, map[string]int{"revenue": 2, "claims": 1}, report.PlacementCounts(composition))
	page := report.CatalogPage{Entries: []report.CatalogEntry{entry, {ID: "fresh", Revision: "1", Title: "Fresh", Ownership: report.OwnershipPersonal, Placed: 4}}}
	marked := report.MarkPlaced(page, composition)
	require.Equal(t, 2, marked.Entries[0].Placed)
	require.Equal(t, 0, marked.Entries[1].Placed)
	require.Equal(t, 4, page.Entries[1].Placed, "MarkPlaced must not mutate its input")
}

func entryIDs(page report.CatalogPage) []string {
	ids := make([]string, 0, len(page.Entries))
	for _, entry := range page.Entries {
		ids = append(ids, entry.ID)
	}
	return ids
}
