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

// ---------------------------------------------------------------------------
// MetricBuilder — builds metric card panels
// ---------------------------------------------------------------------------

// MetricBuilder provides a fluent API for constructing metric panels.
type MetricBuilder struct {
	panel Panel
}

// Metric creates a metric card panel builder.
func Metric(id, title string) *MetricBuilder {
	return &MetricBuilder{
		panel: Panel{
			ID:     id,
			Title:  title,
			Type:   TypeMetric,
			Span:   3,
			Metric: &MetricOptions{},
		},
	}
}

func (b *MetricBuilder) Query(q string) *MetricBuilder         { b.panel.Query = q; return b }
func (b *MetricBuilder) Span(n int) *MetricBuilder             { b.panel.Span = n; return b }
func (b *MetricBuilder) Class(c string) *MetricBuilder         { b.panel.Class = c; return b }
func (b *MetricBuilder) Unit(u string) *MetricBuilder          { b.panel.Metric.Unit = u; return b }
func (b *MetricBuilder) Prefix(p string) *MetricBuilder        { b.panel.Metric.Prefix = p; return b }
func (b *MetricBuilder) Color(c string) *MetricBuilder         { b.panel.Metric.Color = c; return b }
func (b *MetricBuilder) Icon(c templ.Component) *MetricBuilder { b.panel.Metric.Icon = c; return b }

// ValueColumn sets the column name used for the metric value.
func (b *MetricBuilder) ValueColumn(name string) *MetricBuilder {
	b.panel.ColumnMap.Value = name
	return b
}

// DrillTo sets a click-through URL.
func (b *MetricBuilder) DrillTo(url string) *MetricBuilder {
	b.panel.DrillDown = &DrillDown{URL: url}
	return b
}

// Build returns the constructed Panel.
func (b *MetricBuilder) Build() Panel { return b.panel }

// ---------------------------------------------------------------------------
// ChartBuilder — builds chart panels (line, bar, pie, area, gauge, etc.)
// ---------------------------------------------------------------------------

// ChartBuilder provides a fluent API for constructing chart panels.
type ChartBuilder struct {
	panel Panel
}

func newChartBuilder(id, title string, t PanelType) *ChartBuilder {
	return &ChartBuilder{
		panel: Panel{
			ID:    id,
			Title: title,
			Type:  t,
			Span:  6,
			Chart: &ChartOptions{},
		},
	}
}

// Line creates a line chart panel builder.
func Line(id, title string) *ChartBuilder { return newChartBuilder(id, title, TypeLine) }

// Bar creates a bar chart panel builder.
func Bar(id, title string) *ChartBuilder { return newChartBuilder(id, title, TypeBar) }

// StackedBar creates a stacked bar chart panel builder.
func StackedBar(id, title string) *ChartBuilder {
	b := newChartBuilder(id, title, TypeStackedBar)
	b.panel.Chart.Stacked = true
	return b
}

// Column creates a column chart panel builder.
func Column(id, title string) *ChartBuilder { return newChartBuilder(id, title, TypeColumn) }

// Pie creates a pie chart panel builder.
func Pie(id, title string) *ChartBuilder { return newChartBuilder(id, title, TypePie) }

// Donut creates a donut chart panel builder.
func Donut(id, title string) *ChartBuilder { return newChartBuilder(id, title, TypeDonut) }

// Area creates an area chart panel builder.
func Area(id, title string) *ChartBuilder { return newChartBuilder(id, title, TypeArea) }

// Gauge creates a gauge chart panel builder.
func Gauge(id, title string) *ChartBuilder { return newChartBuilder(id, title, TypeGauge) }

func (b *ChartBuilder) Query(q string) *ChartBuilder  { b.panel.Query = q; return b }
func (b *ChartBuilder) Span(n int) *ChartBuilder      { b.panel.Span = n; return b }
func (b *ChartBuilder) Class(c string) *ChartBuilder  { b.panel.Class = c; return b }
func (b *ChartBuilder) Height(h string) *ChartBuilder { b.panel.Chart.Height = h; return b }
func (b *ChartBuilder) Stacked() *ChartBuilder        { b.panel.Chart.Stacked = true; return b }
func (b *ChartBuilder) Legend() *ChartBuilder         { b.panel.Chart.ShowLegend = true; return b }

// Colors sets multiple series/category colors.
func (b *ChartBuilder) Colors(c ...string) *ChartBuilder {
	b.panel.Chart.Colors = c
	return b
}

// LabelColumn sets the column name for x-axis labels.
func (b *ChartBuilder) LabelColumn(name string) *ChartBuilder {
	b.panel.ColumnMap.Label = name
	return b
}

// ValueColumn sets the column name for y-axis values.
func (b *ChartBuilder) ValueColumn(name string) *ChartBuilder {
	b.panel.ColumnMap.Value = name
	return b
}

// SeriesColumn sets the column name for series grouping (multi-series/stacked).
func (b *ChartBuilder) SeriesColumn(name string) *ChartBuilder {
	b.panel.ColumnMap.Series = name
	return b
}

// CategoryColumn sets the column name for category grouping (stacked charts).
func (b *ChartBuilder) CategoryColumn(name string) *ChartBuilder {
	b.panel.ColumnMap.Category = name
	return b
}

// DrillTo sets a click-through URL with {column} placeholders.
func (b *ChartBuilder) DrillTo(url string) *ChartBuilder {
	b.panel.DrillDown = &DrillDown{URL: url}
	return b
}

// DrillToTarget sets a click-through URL that swaps into an HTMX target.
func (b *ChartBuilder) DrillToTarget(url, target string) *ChartBuilder {
	b.panel.DrillDown = &DrillDown{URL: url, Target: target}
	return b
}

// Build returns the constructed Panel.
func (b *ChartBuilder) Build() Panel { return b.panel }

// ---------------------------------------------------------------------------
// TableBuilder — builds data table panels
// ---------------------------------------------------------------------------

// TableBuilder provides a fluent API for constructing table panels.
type TableBuilder struct {
	panel Panel
}

// Table creates a data table panel builder.
func Table(id, title string) *TableBuilder {
	return &TableBuilder{
		panel: Panel{
			ID:    id,
			Title: title,
			Type:  TypeTable,
			Span:  12,
			Table: &TableOptions{},
		},
	}
}

func (b *TableBuilder) Query(q string) *TableBuilder { b.panel.Query = q; return b }
func (b *TableBuilder) Span(n int) *TableBuilder     { b.panel.Span = n; return b }
func (b *TableBuilder) Class(c string) *TableBuilder { b.panel.Class = c; return b }

// Columns sets explicit column definitions for the table.
func (b *TableBuilder) Columns(cols ...TableColumn) *TableBuilder {
	b.panel.Table.Columns = cols
	return b
}

// DrillTo sets a click-through URL with {column} placeholders for row clicks.
func (b *TableBuilder) DrillTo(url string) *TableBuilder {
	b.panel.DrillDown = &DrillDown{URL: url}
	return b
}

// DrillToTarget sets a click-through URL that swaps into an HTMX target.
func (b *TableBuilder) DrillToTarget(url, target string) *TableBuilder {
	b.panel.DrillDown = &DrillDown{URL: url, Target: target}
	return b
}

// Build returns the constructed Panel.
func (b *TableBuilder) Build() Panel { return b.panel }
