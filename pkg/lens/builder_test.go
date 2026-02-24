package lens

import (
	"testing"
)

// ---------------------------------------------------------------------------
// MetricBuilder tests
// ---------------------------------------------------------------------------

func TestMetricBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	p := Metric("rev", "Revenue").Build()

	if p.ID != "rev" {
		t.Errorf("expected ID=rev, got %q", p.ID)
	}
	if p.Title != "Revenue" {
		t.Errorf("expected Title=Revenue, got %q", p.Title)
	}
	if p.Type != TypeMetric {
		t.Errorf("expected Type=TypeMetric, got %q", p.Type)
	}
	if p.Span != 3 {
		t.Errorf("expected default Span=3, got %d", p.Span)
	}
	if p.Metric == nil {
		t.Fatal("expected Metric options to be non-nil")
	}
	if p.Chart != nil {
		t.Error("expected Chart options to be nil for metric panel")
	}
	if p.Table != nil {
		t.Error("expected Table options to be nil for metric panel")
	}
}

func TestMetricBuilder_Query(t *testing.T) {
	t.Parallel()

	q := "SELECT SUM(amount) AS value FROM orders"
	p := Metric("m1", "Total").Query(q).Build()

	if p.Query != q {
		t.Errorf("expected Query=%q, got %q", q, p.Query)
	}
}

func TestMetricBuilder_Span(t *testing.T) {
	t.Parallel()

	p := Metric("m1", "Total").Span(6).Build()

	if p.Span != 6 {
		t.Errorf("expected Span=6, got %d", p.Span)
	}
}

func TestMetricBuilder_Unit(t *testing.T) {
	t.Parallel()

	p := Metric("m1", "Revenue").Unit("USD").Build()

	if p.Metric.Unit != "USD" {
		t.Errorf("expected Unit=USD, got %q", p.Metric.Unit)
	}
}

func TestMetricBuilder_Prefix(t *testing.T) {
	t.Parallel()

	p := Metric("m1", "Revenue").Prefix("$").Build()

	if p.Metric.Prefix != "$" {
		t.Errorf("expected Prefix=$, got %q", p.Metric.Prefix)
	}
}

func TestMetricBuilder_Color(t *testing.T) {
	t.Parallel()

	p := Metric("m1", "Revenue").Color("#10b981").Build()

	if p.Metric.Color != "#10b981" {
		t.Errorf("expected Color=#10b981, got %q", p.Metric.Color)
	}
}

func TestMetricBuilder_ValueColumn(t *testing.T) {
	t.Parallel()

	p := Metric("m1", "Revenue").ValueColumn("amount").Build()

	if p.ColumnMap.Value != "amount" {
		t.Errorf("expected ColumnMap.Value=amount, got %q", p.ColumnMap.Value)
	}
}

func TestMetricBuilder_DrillTo(t *testing.T) {
	t.Parallel()

	p := Metric("m1", "Revenue").DrillTo("/orders").Build()

	if p.DrillDown == nil {
		t.Fatal("expected DrillDown to be non-nil")
	}
	if p.DrillDown.URL != "/orders" {
		t.Errorf("expected DrillDown.URL=/orders, got %q", p.DrillDown.URL)
	}
	if p.DrillDown.Target != "" {
		t.Errorf("expected DrillDown.Target to be empty, got %q", p.DrillDown.Target)
	}
}

func TestMetricBuilder_Chaining(t *testing.T) {
	t.Parallel()

	p := Metric("m1", "Revenue").
		Query("SELECT 1 AS value").
		Span(4).
		Unit("USD").
		Prefix("$").
		Color("#3b82f6").
		ValueColumn("amount").
		DrillTo("/detail").
		Build()

	if p.ID != "m1" {
		t.Errorf("expected ID=m1, got %q", p.ID)
	}
	if p.Type != TypeMetric {
		t.Errorf("expected TypeMetric, got %q", p.Type)
	}
	if p.Query != "SELECT 1 AS value" {
		t.Errorf("unexpected Query: %q", p.Query)
	}
	if p.Span != 4 {
		t.Errorf("expected Span=4, got %d", p.Span)
	}
	if p.Metric.Unit != "USD" {
		t.Errorf("expected Unit=USD, got %q", p.Metric.Unit)
	}
	if p.Metric.Prefix != "$" {
		t.Errorf("expected Prefix=$, got %q", p.Metric.Prefix)
	}
	if p.Metric.Color != "#3b82f6" {
		t.Errorf("expected Color=#3b82f6, got %q", p.Metric.Color)
	}
	if p.ColumnMap.Value != "amount" {
		t.Errorf("expected ColumnMap.Value=amount, got %q", p.ColumnMap.Value)
	}
	if p.DrillDown == nil || p.DrillDown.URL != "/detail" {
		t.Errorf("unexpected DrillDown: %+v", p.DrillDown)
	}
}

// ---------------------------------------------------------------------------
// ChartBuilder tests
// ---------------------------------------------------------------------------

func TestChartBuilder_Line(t *testing.T) {
	t.Parallel()

	p := Line("l1", "Trend").Build()

	if p.Type != TypeLine {
		t.Errorf("expected TypeLine, got %q", p.Type)
	}
	if p.Span != 6 {
		t.Errorf("expected default Span=6, got %d", p.Span)
	}
	if p.Chart == nil {
		t.Fatal("expected Chart options to be non-nil")
	}
	if p.Metric != nil {
		t.Error("expected Metric options to be nil for chart panel")
	}
	if p.Table != nil {
		t.Error("expected Table options to be nil for chart panel")
	}
}

func TestChartBuilder_Bar(t *testing.T) {
	t.Parallel()

	p := Bar("b1", "Sales").Build()

	if p.Type != TypeBar {
		t.Errorf("expected TypeBar, got %q", p.Type)
	}
}

func TestChartBuilder_StackedBar(t *testing.T) {
	t.Parallel()

	p := StackedBar("sb1", "Stacked").Build()

	if p.Type != TypeStackedBar {
		t.Errorf("expected TypeStackedBar, got %q", p.Type)
	}
	// StackedBar should default Stacked=true.
	if p.Chart == nil || !p.Chart.Stacked {
		t.Error("expected Chart.Stacked=true for StackedBar")
	}
}

func TestChartBuilder_Pie(t *testing.T) {
	t.Parallel()

	p := Pie("p1", "Breakdown").Build()

	if p.Type != TypePie {
		t.Errorf("expected TypePie, got %q", p.Type)
	}
}

func TestChartBuilder_Donut(t *testing.T) {
	t.Parallel()

	p := Donut("d1", "Share").Build()

	if p.Type != TypeDonut {
		t.Errorf("expected TypeDonut, got %q", p.Type)
	}
}

func TestChartBuilder_Area(t *testing.T) {
	t.Parallel()

	p := Area("a1", "Area").Build()

	if p.Type != TypeArea {
		t.Errorf("expected TypeArea, got %q", p.Type)
	}
}

func TestChartBuilder_Gauge(t *testing.T) {
	t.Parallel()

	p := Gauge("g1", "Progress").Build()

	if p.Type != TypeGauge {
		t.Errorf("expected TypeGauge, got %q", p.Type)
	}
}

func TestChartBuilder_Column(t *testing.T) {
	t.Parallel()

	p := Column("c1", "Columns").Build()

	if p.Type != TypeColumn {
		t.Errorf("expected TypeColumn, got %q", p.Type)
	}
}

func TestChartBuilder_Colors(t *testing.T) {
	t.Parallel()

	colors := []string{"#ff0000", "#00ff00", "#0000ff"}
	p := Line("l1", "Trend").Colors(colors...).Build()

	if len(p.Chart.Colors) != 3 {
		t.Fatalf("expected 3 colors, got %d", len(p.Chart.Colors))
	}
	for i, c := range colors {
		if p.Chart.Colors[i] != c {
			t.Errorf("color[%d]: expected %q, got %q", i, c, p.Chart.Colors[i])
		}
	}
}

func TestChartBuilder_Height(t *testing.T) {
	t.Parallel()

	p := Line("l1", "Trend").Height("400px").Build()

	if p.Chart.Height != "400px" {
		t.Errorf("expected Height=400px, got %q", p.Chart.Height)
	}
}

func TestChartBuilder_Stacked(t *testing.T) {
	t.Parallel()

	p := Bar("b1", "Sales").Stacked().Build()

	if !p.Chart.Stacked {
		t.Error("expected Chart.Stacked=true after calling Stacked()")
	}
}

func TestChartBuilder_Legend(t *testing.T) {
	t.Parallel()

	p := Pie("p1", "Breakdown").Legend().Build()

	if !p.Chart.ShowLegend {
		t.Error("expected Chart.ShowLegend=true after calling Legend()")
	}
}

func TestChartBuilder_LabelColumn(t *testing.T) {
	t.Parallel()

	p := Line("l1", "Trend").LabelColumn("date").Build()

	if p.ColumnMap.Label != "date" {
		t.Errorf("expected ColumnMap.Label=date, got %q", p.ColumnMap.Label)
	}
}

func TestChartBuilder_ValueColumn(t *testing.T) {
	t.Parallel()

	p := Line("l1", "Trend").ValueColumn("revenue").Build()

	if p.ColumnMap.Value != "revenue" {
		t.Errorf("expected ColumnMap.Value=revenue, got %q", p.ColumnMap.Value)
	}
}

func TestChartBuilder_SeriesColumn(t *testing.T) {
	t.Parallel()

	p := Line("l1", "Trend").SeriesColumn("category").Build()

	if p.ColumnMap.Series != "category" {
		t.Errorf("expected ColumnMap.Series=category, got %q", p.ColumnMap.Series)
	}
}

func TestChartBuilder_CategoryColumn(t *testing.T) {
	t.Parallel()

	p := StackedBar("sb1", "Stacked").CategoryColumn("region").Build()

	if p.ColumnMap.Category != "region" {
		t.Errorf("expected ColumnMap.Category=region, got %q", p.ColumnMap.Category)
	}
}

func TestChartBuilder_DrillTo(t *testing.T) {
	t.Parallel()

	p := Bar("b1", "Sales").DrillTo("/analytics?cat={label}").Build()

	if p.DrillDown == nil {
		t.Fatal("expected DrillDown to be non-nil")
	}
	if p.DrillDown.URL != "/analytics?cat={label}" {
		t.Errorf("expected DrillDown.URL, got %q", p.DrillDown.URL)
	}
	if p.DrillDown.Target != "" {
		t.Errorf("expected Target to be empty, got %q", p.DrillDown.Target)
	}
}

func TestChartBuilder_DrillToTarget(t *testing.T) {
	t.Parallel()

	p := Bar("b1", "Sales").DrillToTarget("/analytics?cat={label}", "#detail").Build()

	if p.DrillDown == nil {
		t.Fatal("expected DrillDown to be non-nil")
	}
	if p.DrillDown.URL != "/analytics?cat={label}" {
		t.Errorf("unexpected DrillDown.URL: %q", p.DrillDown.URL)
	}
	if p.DrillDown.Target != "#detail" {
		t.Errorf("expected DrillDown.Target=#detail, got %q", p.DrillDown.Target)
	}
}

func TestChartBuilder_Chaining(t *testing.T) {
	t.Parallel()

	p := Line("l1", "Revenue Trend").
		Query("SELECT date AS label, SUM(amount) AS value FROM orders GROUP BY 1").
		Span(8).
		Colors("#10b981", "#3b82f6").
		Height("400px").
		Legend().
		LabelColumn("date").
		ValueColumn("amount").
		SeriesColumn("region").
		CategoryColumn("cat").
		DrillToTarget("/detail?label={label}", "#panel").
		Build()

	if p.ID != "l1" {
		t.Errorf("expected ID=l1, got %q", p.ID)
	}
	if p.Type != TypeLine {
		t.Errorf("expected TypeLine, got %q", p.Type)
	}
	if p.Span != 8 {
		t.Errorf("expected Span=8, got %d", p.Span)
	}
	if len(p.Chart.Colors) != 2 {
		t.Errorf("expected 2 colors, got %d", len(p.Chart.Colors))
	}
	if p.Chart.Height != "400px" {
		t.Errorf("expected Height=400px, got %q", p.Chart.Height)
	}
	if !p.Chart.ShowLegend {
		t.Error("expected ShowLegend=true")
	}
	if p.ColumnMap.Label != "date" {
		t.Errorf("expected ColumnMap.Label=date, got %q", p.ColumnMap.Label)
	}
	if p.ColumnMap.Value != "amount" {
		t.Errorf("expected ColumnMap.Value=amount, got %q", p.ColumnMap.Value)
	}
	if p.ColumnMap.Series != "region" {
		t.Errorf("expected ColumnMap.Series=region, got %q", p.ColumnMap.Series)
	}
	if p.ColumnMap.Category != "cat" {
		t.Errorf("expected ColumnMap.Category=cat, got %q", p.ColumnMap.Category)
	}
	if p.DrillDown == nil || p.DrillDown.Target != "#panel" {
		t.Errorf("unexpected DrillDown: %+v", p.DrillDown)
	}
}

// ---------------------------------------------------------------------------
// TableBuilder tests
// ---------------------------------------------------------------------------

func TestTableBuilder_DefaultValues(t *testing.T) {
	t.Parallel()

	p := Table("t1", "Orders").Build()

	if p.ID != "t1" {
		t.Errorf("expected ID=t1, got %q", p.ID)
	}
	if p.Title != "Orders" {
		t.Errorf("expected Title=Orders, got %q", p.Title)
	}
	if p.Type != TypeTable {
		t.Errorf("expected TypeTable, got %q", p.Type)
	}
	if p.Span != 12 {
		t.Errorf("expected default Span=12, got %d", p.Span)
	}
	if p.Table == nil {
		t.Fatal("expected Table options to be non-nil")
	}
	if p.Metric != nil {
		t.Error("expected Metric options to be nil for table panel")
	}
	if p.Chart != nil {
		t.Error("expected Chart options to be nil for table panel")
	}
}

func TestTableBuilder_Columns(t *testing.T) {
	t.Parallel()

	cols := []TableColumn{
		{Key: "id", Label: "ID", Format: "number"},
		{Key: "name", Label: "Name", Format: ""},
		{Key: "amount", Label: "Amount", Format: "currency"},
	}
	p := Table("t1", "Orders").Columns(cols...).Build()

	if len(p.Table.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(p.Table.Columns))
	}
	for i, col := range cols {
		if p.Table.Columns[i].Key != col.Key {
			t.Errorf("col[%d].Key: expected %q, got %q", i, col.Key, p.Table.Columns[i].Key)
		}
		if p.Table.Columns[i].Label != col.Label {
			t.Errorf("col[%d].Label: expected %q, got %q", i, col.Label, p.Table.Columns[i].Label)
		}
		if p.Table.Columns[i].Format != col.Format {
			t.Errorf("col[%d].Format: expected %q, got %q", i, col.Format, p.Table.Columns[i].Format)
		}
	}
}

func TestTableBuilder_DrillTo(t *testing.T) {
	t.Parallel()

	p := Table("t1", "Orders").DrillTo("/orders/{id}").Build()

	if p.DrillDown == nil {
		t.Fatal("expected DrillDown to be non-nil")
	}
	if p.DrillDown.URL != "/orders/{id}" {
		t.Errorf("expected DrillDown.URL=/orders/{id}, got %q", p.DrillDown.URL)
	}
	if p.DrillDown.Target != "" {
		t.Errorf("expected empty Target, got %q", p.DrillDown.Target)
	}
}

func TestTableBuilder_DrillToTarget(t *testing.T) {
	t.Parallel()

	p := Table("t1", "Orders").DrillToTarget("/orders/{id}", "#detail-panel").Build()

	if p.DrillDown == nil {
		t.Fatal("expected DrillDown to be non-nil")
	}
	if p.DrillDown.URL != "/orders/{id}" {
		t.Errorf("unexpected DrillDown.URL: %q", p.DrillDown.URL)
	}
	if p.DrillDown.Target != "#detail-panel" {
		t.Errorf("expected Target=#detail-panel, got %q", p.DrillDown.Target)
	}
}

func TestTableBuilder_Chaining(t *testing.T) {
	t.Parallel()

	p := Table("t1", "Orders").
		Query("SELECT id, name, amount FROM orders").
		Span(8).
		Columns(
			TableColumn{Key: "id", Label: "ID"},
			TableColumn{Key: "amount", Label: "Amount", Format: "currency"},
		).
		DrillToTarget("/orders/{id}", "#order-detail").
		Build()

	if p.ID != "t1" {
		t.Errorf("expected ID=t1, got %q", p.ID)
	}
	if p.Type != TypeTable {
		t.Errorf("expected TypeTable, got %q", p.Type)
	}
	if p.Span != 8 {
		t.Errorf("expected Span=8, got %d", p.Span)
	}
	if p.Query != "SELECT id, name, amount FROM orders" {
		t.Errorf("unexpected Query: %q", p.Query)
	}
	if len(p.Table.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(p.Table.Columns))
	}
	if p.DrillDown == nil || p.DrillDown.Target != "#order-detail" {
		t.Errorf("unexpected DrillDown: %+v", p.DrillDown)
	}
}

// ---------------------------------------------------------------------------
// NewDashboard / NewRow tests
// ---------------------------------------------------------------------------

func TestNewDashboard(t *testing.T) {
	t.Parallel()

	dash := NewDashboard("Finance Overview",
		NewRow(
			Metric("rev", "Revenue").Build(),
			Metric("orders", "Orders").Build(),
		),
		NewRow(
			Line("trend", "Trend").Build(),
		),
	)

	if dash.Title != "Finance Overview" {
		t.Errorf("expected Title=Finance Overview, got %q", dash.Title)
	}
	if len(dash.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(dash.Rows))
	}
	if len(dash.Rows[0].Panels) != 2 {
		t.Errorf("expected 2 panels in row 0, got %d", len(dash.Rows[0].Panels))
	}
	if len(dash.Rows[1].Panels) != 1 {
		t.Errorf("expected 1 panel in row 1, got %d", len(dash.Rows[1].Panels))
	}
}

func TestNewDashboard_NoRows(t *testing.T) {
	t.Parallel()

	dash := NewDashboard("Empty Dashboard")

	if dash.Title != "Empty Dashboard" {
		t.Errorf("expected Title=Empty Dashboard, got %q", dash.Title)
	}
	if len(dash.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(dash.Rows))
	}
}

func TestNewRow_Empty(t *testing.T) {
	t.Parallel()

	row := NewRow()

	if len(row.Panels) != 0 {
		t.Errorf("expected 0 panels, got %d", len(row.Panels))
	}
}

func TestNewRow_WithPanels(t *testing.T) {
	t.Parallel()

	row := NewRow(
		Metric("m1", "Metric 1").Build(),
		Line("c1", "Chart 1").Build(),
		Table("t1", "Table 1").Build(),
	)

	if len(row.Panels) != 3 {
		t.Fatalf("expected 3 panels, got %d", len(row.Panels))
	}
	if row.Panels[0].ID != "m1" {
		t.Errorf("panel[0].ID: expected m1, got %q", row.Panels[0].ID)
	}
	if row.Panels[1].ID != "c1" {
		t.Errorf("panel[1].ID: expected c1, got %q", row.Panels[1].ID)
	}
	if row.Panels[2].ID != "t1" {
		t.Errorf("panel[2].ID: expected t1, got %q", row.Panels[2].ID)
	}
}

// ---------------------------------------------------------------------------
// Default Span verification across panel types
// ---------------------------------------------------------------------------

func TestDefaultSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		panel    Panel
		wantSpan int
	}{
		{"Metric", Metric("m", "M").Build(), 3},
		{"Line", Line("l", "L").Build(), 6},
		{"Bar", Bar("b", "B").Build(), 6},
		{"StackedBar", StackedBar("sb", "SB").Build(), 6},
		{"Column", Column("c", "C").Build(), 6},
		{"Pie", Pie("p", "P").Build(), 6},
		{"Donut", Donut("d", "D").Build(), 6},
		{"Area", Area("a", "A").Build(), 6},
		{"Gauge", Gauge("g", "G").Build(), 6},
		{"Table", Table("t", "T").Build(), 12},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.panel.Span != tt.wantSpan {
				t.Errorf("%s: expected default Span=%d, got %d", tt.name, tt.wantSpan, tt.panel.Span)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// No DrillDown by default
// ---------------------------------------------------------------------------

func TestNoDrillDownByDefault(t *testing.T) {
	t.Parallel()

	panels := []Panel{
		Metric("m", "M").Build(),
		Line("l", "L").Build(),
		Table("t", "T").Build(),
	}

	for _, p := range panels {
		if p.DrillDown != nil {
			t.Errorf("panel %q: expected DrillDown=nil by default, got %+v", p.ID, p.DrillDown)
		}
	}
}
