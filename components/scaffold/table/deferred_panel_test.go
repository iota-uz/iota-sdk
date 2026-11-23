package table

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshEvent(t *testing.T) {
	assert.Equal(t, "tbl:orders:refresh", (&TableConfig{ID: "orders"}).RefreshEvent())
	// Falls back to a stable name when ID is empty.
	assert.Equal(t, "tbl:default:refresh", (&TableConfig{}).RefreshEvent())
}

func TestWithDeferredPanels(t *testing.T) {
	cfg := NewTableConfig("Orders", "/orders",
		WithID("orders"),
		WithDeferredPanels(DeferredPanel{ID: "orders-summary", URL: "/orders/summary"}),
	)
	require.Len(t, cfg.DeferredPanels, 1)
	assert.Equal(t, "orders-summary", cfg.DeferredPanels[0].ID)
	assert.Equal(t, "/orders/summary", cfg.DeferredPanels[0].URL)
}

func TestTableDefinition_WithDeferredPanels(t *testing.T) {
	builder := NewTableDefinition("Debts", "/debts").
		WithDeferredPanels(DeferredPanel{ID: "debt-balances", URL: "/balances"})
	definition := builder.Build()
	builder.WithDeferredPanels(DeferredPanel{ID: "later", URL: "/later"})

	require.Len(t, definition.DeferredPanels(), 1, "a built definition does not change with its builder")
	assert.Equal(t, "debt-balances", definition.DeferredPanels()[0].ID)
	assert.Equal(t, "/balances", definition.DeferredPanels()[0].URL)
}

func TestFormHxAttrs(t *testing.T) {
	// No panels → no re-broadcast attribute.
	assert.Empty(t, (&TableConfig{ID: "orders"}).FormHxAttrs())

	// With panels → form re-broadcasts RefreshEvent only for its OWN successful
	// request (loop-free guard), so infinite-scroll/sort don't reload panels.
	attrs := (&TableConfig{ID: "orders", DeferredPanels: []DeferredPanel{{ID: "s", URL: "/s"}}}).FormHxAttrs()
	js, ok := attrs["hx-on::after-request"].(string)
	require.True(t, ok, "expected hx-on::after-request handler")
	assert.Contains(t, js, "event.detail.elt===this")
	assert.Contains(t, js, "event.detail.successful")
	assert.Contains(t, js, "tbl:orders:refresh")
}

func TestDeferredPanelRender(t *testing.T) {
	cfg := &TableConfig{ID: "orders"}
	panel := DeferredPanel{ID: "orders-summary", URL: "/orders/summary"}

	var buf strings.Builder
	require.NoError(t, deferredPanel(cfg, panel).Render(context.Background(), &buf))
	html := buf.String()

	assert.Contains(t, html, `id="orders-summary"`)
	assert.Contains(t, html, `hx-get="/orders/summary"`)
	// load = first paint; RefreshEvent from the closest form = reload on filter change.
	assert.Contains(t, html, `hx-trigger="load, tbl:orders:refresh from:closest form"`)
	// Carries the current filter/search form to the panel endpoint.
	assert.Contains(t, html, `hx-include="closest form"`)
	assert.Contains(t, html, `hx-target="this"`)
	assert.Contains(t, html, `hx-swap="innerHTML"`)
	// Cancel stale aggregate requests on rapid filter changes.
	assert.Contains(t, html, `hx-sync="this:replace"`)
	// Default skeleton painted immediately (no blocking query).
	assert.Contains(t, html, "animate-pulse")
}

func TestDeferredPanelPlacement(t *testing.T) {
	cfg := &TableConfig{ID: "orders", DeferredPanels: []DeferredPanel{
		{ID: "above", URL: "/a"},
		{ID: "inside", URL: "/b", Placement: PanelInTable},
	}}
	require.Len(t, cfg.PanelsAt(PanelAbove), 1)
	assert.Equal(t, "above", cfg.PanelsAt(PanelAbove)[0].ID)
	require.Len(t, cfg.PanelsAt(PanelInTable), 1)
	assert.Equal(t, "inside", cfg.PanelsAt(PanelInTable)[0].ID)

	var section strings.Builder
	require.NoError(t, TableSection(cfg).Render(testPageCtx(t, "/orders"), &section))
	html := section.String()
	above := strings.Index(html, `id="above"`)
	frame := strings.Index(html, "bg-surface-600 border border-subtle")
	inside := strings.Index(html, `id="inside"`)
	head := strings.Index(html, "<table")
	require.True(t, above >= 0 && frame >= 0 && inside >= 0 && head >= 0, html)
	assert.Less(t, above, frame, "a default panel stays above the table frame")
	assert.Less(t, frame, inside, "an in-table panel renders inside the frame")
	assert.Less(t, inside, head, "an in-table panel renders above the table head")
	assert.Equal(t, 1, strings.Count(html, `id="inside"`))
}
