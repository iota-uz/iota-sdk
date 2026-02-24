package lens

import "github.com/a-h/templ"

// NewDashboard creates a dashboard from a title and rows.
func NewDashboard(title string, rows ...Row) Dashboard {
	return Dashboard{
		Title: title,
		Rows:  rows,
	}
}

// NewRow creates a row from panels.
func NewRow(panels ...Panel) Row {
	return Row{Panels: panels}
}

// PanelBuilder provides a fluent API for constructing panels.
type PanelBuilder struct {
	panel Panel
}

func newBuilder(id, title string, t PanelType) *PanelBuilder {
	return &PanelBuilder{
		panel: Panel{
			ID:    id,
			Title: title,
			Type:  t,
			Span:  6, // sensible default: half-width
		},
	}
}

// -- Panel type constructors --

// Metric creates a metric card panel builder.
func Metric(id, title string) *PanelBuilder { return newBuilder(id, title, TypeMetric) }

// Line creates a line chart panel builder.
func Line(id, title string) *PanelBuilder { return newBuilder(id, title, TypeLine) }

// Bar creates a bar chart panel builder.
func Bar(id, title string) *PanelBuilder { return newBuilder(id, title, TypeBar) }

// StackedBar creates a stacked bar chart panel builder.
func StackedBar(id, title string) *PanelBuilder { return newBuilder(id, title, TypeStackedBar) }

// Column creates a column chart panel builder.
func Column(id, title string) *PanelBuilder { return newBuilder(id, title, TypeColumn) }

// Pie creates a pie chart panel builder.
func Pie(id, title string) *PanelBuilder { return newBuilder(id, title, TypePie) }

// Donut creates a donut chart panel builder.
func Donut(id, title string) *PanelBuilder { return newBuilder(id, title, TypeDonut) }

// Area creates an area chart panel builder.
func Area(id, title string) *PanelBuilder { return newBuilder(id, title, TypeArea) }

// Gauge creates a gauge chart panel builder.
func Gauge(id, title string) *PanelBuilder { return newBuilder(id, title, TypeGauge) }

// Table creates a data table panel builder.
func Table(id, title string) *PanelBuilder { return newBuilder(id, title, TypeTable) }

// -- Builder methods --

// Span sets the column span (1-12) for the panel.
func (b *PanelBuilder) Span(n int) *PanelBuilder {
	b.panel.Span = n
	return b
}

// Query sets the SQL query for the panel.
func (b *PanelBuilder) Query(q string) *PanelBuilder {
	b.panel.Query = q
	return b
}

// Height sets the chart height (e.g. "320px", "400px").
func (b *PanelBuilder) Height(h string) *PanelBuilder {
	b.panel.Options.Height = h
	return b
}

// Unit sets the metric value unit (e.g. "USD", "%").
func (b *PanelBuilder) Unit(u string) *PanelBuilder {
	b.panel.Options.Unit = u
	return b
}

// Prefix sets a value prefix (e.g. "$").
func (b *PanelBuilder) Prefix(p string) *PanelBuilder {
	b.panel.Options.Prefix = p
	return b
}

// Color sets the primary accent color.
func (b *PanelBuilder) Color(c string) *PanelBuilder {
	b.panel.Options.Color = c
	return b
}

// Colors sets multiple series/category colors.
func (b *PanelBuilder) Colors(c ...string) *PanelBuilder {
	b.panel.Options.Colors = c
	return b
}

// Icon sets the metric card icon component.
func (b *PanelBuilder) Icon(c templ.Component) *PanelBuilder {
	b.panel.Options.Icon = c
	return b
}

// Stacked enables stacked mode for bar/area charts.
func (b *PanelBuilder) Stacked() *PanelBuilder {
	b.panel.Options.Stacked = true
	return b
}

// Legend enables the chart legend.
func (b *PanelBuilder) Legend() *PanelBuilder {
	b.panel.Options.ShowLegend = true
	return b
}

// DrillTo sets a click-through URL with {column} placeholders.
func (b *PanelBuilder) DrillTo(url string) *PanelBuilder {
	b.panel.Options.DrillDown = &DrillDown{URL: url}
	return b
}

// DrillToTarget sets a click-through URL that swaps into an HTMX target.
func (b *PanelBuilder) DrillToTarget(url, target string) *PanelBuilder {
	b.panel.Options.DrillDown = &DrillDown{URL: url, Target: target}
	return b
}

// RefreshEvery configures periodic HTMX panel refresh.
func (b *PanelBuilder) RefreshEvery(interval, url string) *PanelBuilder {
	b.panel.Options.RefreshInterval = interval
	b.panel.Options.RefreshURL = url
	return b
}

// TableColumns sets column definitions for a table panel.
func (b *PanelBuilder) TableColumns(cols ...TableColumn) *PanelBuilder {
	b.panel.Options.Columns = cols
	return b
}

// Class sets extra CSS classes on the panel.
func (b *PanelBuilder) Class(c string) *PanelBuilder {
	b.panel.Options.Class = c
	return b
}

// Build returns the constructed Panel.
func (b *PanelBuilder) Build() Panel {
	return b.panel
}
