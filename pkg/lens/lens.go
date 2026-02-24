// Package lens provides a framework for building interactive BI dashboards.
//
// Dashboards are composed of rows containing panels. Each panel occupies
// a span of columns in a 12-column grid and renders a specific visualization
// (metric card, chart, or data table).
//
// Usage:
//
//	dash := lens.NewDashboard("Finance Overview",
//	    lens.NewRow(
//	        lens.Metric("revenue", "Revenue").Query("SELECT ...").Unit("USD").Span(3).Build(),
//	        lens.Metric("orders", "Orders").Query("SELECT ...").Span(3).Build(),
//	    ),
//	    lens.NewRow(
//	        lens.Line("trend", "Revenue Trend").Query("SELECT ...").Span(8).Build(),
//	        lens.Pie("breakdown", "By Category").Query("SELECT ...").Span(4).Build(),
//	    ),
//	)
//
//	results := lens.Execute(r.Context(), ds, dash)
//	// in template: @lensui.Dashboard(dash, results)
package lens

import "github.com/a-h/templ"

// Dashboard is the top-level configuration for a BI dashboard.
type Dashboard struct {
	ID          string
	Title       string
	Description string
	Rows        []Row
}

// Row is a horizontal group of panels using a 12-column grid.
type Row struct {
	Panels []Panel
	Class  string // optional extra Tailwind classes for the row
}

// Panel represents a single dashboard widget (metric, chart, or table).
type Panel struct {
	ID      string
	Title   string
	Type    PanelType
	Span    int    // 1–12 column span
	Query   string // SQL or data source query
	Options PanelOptions
}

// PanelType identifies the visualization type of a panel.
type PanelType string

const (
	TypeMetric     PanelType = "metric"
	TypeLine       PanelType = "line"
	TypeBar        PanelType = "bar"
	TypeStackedBar PanelType = "stacked_bar"
	TypeColumn     PanelType = "column"
	TypePie        PanelType = "pie"
	TypeDonut      PanelType = "donut"
	TypeArea       PanelType = "area"
	TypeGauge      PanelType = "gauge"
	TypeTable      PanelType = "table"
)

// PanelOptions holds type-specific rendering and interaction options.
type PanelOptions struct {
	// Common
	Height string   // chart height (default "320px")
	Class  string   // extra CSS classes
	Colors []string // series/category colors

	// Metric
	Unit   string          // "USD", "%", etc.
	Color  string          // accent color
	Icon   templ.Component // icon component (e.g. phosphor icon)
	Prefix string          // value prefix like "$"

	// Chart
	Stacked    bool
	ShowLegend bool

	// Table
	Columns []TableColumn

	// Interaction — drill-through
	DrillDown *DrillDown

	// HTMX — per-panel refresh
	RefreshInterval string // e.g. "30s" → hx-trigger="every 30s"
	RefreshURL      string // hx-get endpoint for panel refresh
}

// DrillDown configures click-through navigation from a panel.
// URL supports {column_name} placeholders substituted from the clicked data.
type DrillDown struct {
	URL    string // URL template, e.g. "/orders?category={label}"
	Target string // HTMX target selector, or "" for full navigation
}

// TableColumn defines a column in a table panel.
type TableColumn struct {
	Key    string // column name from query result
	Label  string // display header text
	Format string // "currency", "date", "number", "percent", or "" for auto
}
