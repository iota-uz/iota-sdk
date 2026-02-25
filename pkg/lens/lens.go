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
// Exactly one of Metric, Chart, or Table is non-nil, determined by Type.
type Panel struct {
	ID        string
	Title     string
	Type      PanelType
	Span      int    // 1–12 column span
	Query     string // SQL query
	Class     string // extra CSS classes
	ColumnMap ColumnMap
	DrillDown *DrillDown

	// Type-specific options — exactly one is non-nil.
	Metric *MetricOptions
	Chart  *ChartOptions
	Table  *TableOptions
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

// ColumnMap allows explicit mapping of query result columns to chart axes.
// Empty strings fall back to convention-based detection (label, value, series, etc.).
type ColumnMap struct {
	Label    string // column for x-axis labels / chart categories
	Value    string // column for y-axis values
	Series   string // column for series grouping (stacked/multi-series)
	Category string // column for category grouping (stacked charts)
}

// MetricOptions holds options specific to metric card panels.
type MetricOptions struct {
	Unit   string          // display unit, e.g. "USD", "%"
	Prefix string          // value prefix, e.g. "$"
	Color  string          // accent color (left border + icon background)
	Icon   templ.Component // icon component (e.g. phosphor icon)
}

// ChartOptions holds options specific to chart panels (line, bar, pie, etc.).
type ChartOptions struct {
	Height     string   // chart height (default "320px")
	Colors     []string // series/category colors
	Stacked    bool     // enable stacked mode
	ShowLegend bool     // show chart legend
}

// TableOptions holds options specific to data table panels.
type TableOptions struct {
	Columns []TableColumn // explicit column definitions; auto-detected if empty
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
