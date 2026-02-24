package ui

import (
	"testing"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

// ---------------------------------------------------------------------------
// resolveColumn / resolveValue tests
// ---------------------------------------------------------------------------

func TestResolveColumn_ExplicitMapping(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"amount": float64(9999),
		"value":  float64(1),
	}

	// When ColumnMap.Value is set explicitly, it should override the fallback.
	cm := lens.ColumnMap{Value: "amount"}
	got, ok := resolveValue(row, cm)
	if !ok {
		t.Fatal("expected resolveValue to return ok=true")
	}
	f, isFloat := got.(float64)
	if !isFloat {
		t.Fatalf("expected float64, got %T", got)
	}
	if f != 9999 {
		t.Errorf("expected 9999, got %v", f)
	}
}

func TestResolveColumn_Fallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		row      map[string]any
		wantVal  any
		wantKey  string
	}{
		{
			name:    "value fallback",
			row:     map[string]any{"value": float64(10)},
			wantVal: float64(10),
			wantKey: "value",
		},
		{
			name:    "amount fallback when value absent",
			row:     map[string]any{"amount": float64(20)},
			wantVal: float64(20),
			wantKey: "amount",
		},
		{
			name:    "count fallback",
			row:     map[string]any{"count": float64(30)},
			wantVal: float64(30),
			wantKey: "count",
		},
		{
			name:    "total fallback",
			row:     map[string]any{"total": float64(40)},
			wantVal: float64(40),
			wantKey: "total",
		},
		{
			name:    "value preferred over amount when both present",
			row:     map[string]any{"value": float64(5), "amount": float64(50)},
			wantVal: float64(5),
			wantKey: "value",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Empty ColumnMap → use fallbacks.
			cm := lens.ColumnMap{}
			got, ok := resolveValue(tt.row, cm)
			if !ok {
				t.Fatalf("expected ok=true, got false")
			}
			if got != tt.wantVal {
				t.Errorf("expected %v, got %v", tt.wantVal, got)
			}
		})
	}
}

func TestResolveColumn_NotFound(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"name": "Alice",
		"city": "NYC",
	}

	// Empty ColumnMap, no recognized fallback column present.
	cm := lens.ColumnMap{}
	_, ok := resolveValue(row, cm)
	if ok {
		t.Error("expected resolveValue to return ok=false when no matching column")
	}
}

func TestResolveColumn_ExplicitMappingMissing(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"value": float64(99),
	}

	// Explicit column name that doesn't exist in the row.
	cm := lens.ColumnMap{Value: "nonexistent"}
	_, ok := resolveValue(row, cm)
	if ok {
		t.Error("expected ok=false when explicitly mapped column is not in row")
	}
}

// ---------------------------------------------------------------------------
// resolveLabel tests
// ---------------------------------------------------------------------------

func TestResolveLabel_ExplicitMapping(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"month": "January",
		"label": "fallback",
	}
	cm := lens.ColumnMap{Label: "month"}

	got := resolveLabel(row, cm)
	if got != "January" {
		t.Errorf("expected January, got %q", got)
	}
}

func TestResolveLabel_Fallbacks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		row     map[string]any
		want    string
	}{
		{"label", map[string]any{"label": "Jan"}, "Jan"},
		{"category", map[string]any{"category": "Cat A"}, "Cat A"},
		{"name", map[string]any{"name": "Bob"}, "Bob"},
		{"date", map[string]any{"date": "2024-01-01"}, "2024-01-01"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cm := lens.ColumnMap{}
			got := resolveLabel(tt.row, cm)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestResolveLabel_NotFound(t *testing.T) {
	t.Parallel()

	row := map[string]any{"value": float64(1)}
	cm := lens.ColumnMap{}

	got := resolveLabel(row, cm)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// resolveDrillURL tests
// ---------------------------------------------------------------------------

func TestResolveDrillURL_SinglePlaceholder(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"label": "Electronics",
	}
	tmpl := "/products?category={label}"

	got := resolveDrillURL(tmpl, row)
	want := "/products?category=Electronics"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveDrillURL_MultiplePlaceholders(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"label":  "Q1",
		"series": "North",
	}
	tmpl := "/report?period={label}&region={series}"

	got := resolveDrillURL(tmpl, row)

	// Both placeholders must be replaced.
	if got == tmpl {
		t.Error("expected placeholders to be replaced")
	}
	// Check both substitutions are present in the result.
	for _, sub := range []string{"Q1", "North"} {
		found := false
		for i := range got {
			if i+len(sub) <= len(got) && got[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected substitution %q to appear in URL %q", sub, got)
		}
	}
}

func TestResolveDrillURL_URLEncoding(t *testing.T) {
	t.Parallel()

	row := map[string]any{
		"label": "Hello World & Co",
	}
	tmpl := "/search?q={label}"

	got := resolveDrillURL(tmpl, row)

	// Spaces and ampersands must be URL-encoded.
	if got == "/search?q=Hello World & Co" {
		t.Error("expected URL encoding to be applied")
	}
	// url.QueryEscape replaces spaces with '+'.
	want := "/search?q=Hello+World+%26+Co"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestResolveDrillURL_NoPlaceholders(t *testing.T) {
	t.Parallel()

	row := map[string]any{"label": "X"}
	tmpl := "/static/page"

	got := resolveDrillURL(tmpl, row)
	if got != tmpl {
		t.Errorf("expected unchanged template %q, got %q", tmpl, got)
	}
}

func TestResolveDrillURL_UnmatchedPlaceholder(t *testing.T) {
	t.Parallel()

	row := map[string]any{"label": "Y"}
	tmpl := "/detail?id={id}"

	got := resolveDrillURL(tmpl, row)
	// {id} is not in the row, so it should remain as-is.
	if got != tmpl {
		t.Errorf("expected unmatched placeholder to remain: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// formatNumber tests
// ---------------------------------------------------------------------------

func TestFormatNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input float64
		want  string
	}{
		{1_500_000, "1.5M"},
		{1_000_000, "1.0M"},
		{2_345_678, "2.3M"},
		{1_500, "1.5K"},
		{1_000, "1.0K"},
		{999_999, "1000.0K"},
		{50, "50"},
		{1, "1"},
		{0.5, "0.50"},
		{0.123, "0.12"},
		{0, "0.00"},
		{-1_500_000, "-1.5M"},
		{-1_500, "-1.5K"},
		{-50, "-50"},
		{1_000_000_000, "1.0B"},
		{2_500_000_000, "2.5B"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()
			got := formatNumber(tt.input)
			if got != tt.want {
				t.Errorf("formatNumber(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatCellValue tests
// ---------------------------------------------------------------------------

func TestFormatCellValue(t *testing.T) {
	t.Parallel()

	fixedTime := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"empty string", "", ""},
		{"time.Time", fixedTime, "2024-03-15"},
		{"float64 integer", float64(42), "42"},
		{"float64 decimal", float64(3.14), "3.14"},
		{"float64 large int", float64(1000), "1000"},
		{"float32 integer", float32(10), "10"},
		{"float32 decimal", float32(1.5), "1.50"},
		{"int", int(7), "7"},
		{"bool true", true, "Yes"},
		{"bool false", false, "No"},
		{"unknown type", []int{1, 2, 3}, "[1 2 3]"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := formatCellValue(tt.input)
			if got != tt.want {
				t.Errorf("formatCellValue(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// spanClass tests
// ---------------------------------------------------------------------------

func TestSpanClass_KnownSpans(t *testing.T) {
	t.Parallel()

	tests := []struct {
		span int
		want string
	}{
		{1, "col-span-12 md:col-span-1"},
		{3, "col-span-12 md:col-span-3"},
		{6, "col-span-12 md:col-span-6"},
		{12, "col-span-12 md:col-span-12"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("span"+string(rune('0'+tt.span)), func(t *testing.T) {
			t.Parallel()
			got := spanClass(tt.span)
			if got != tt.want {
				t.Errorf("spanClass(%d) = %q, want %q", tt.span, got, tt.want)
			}
		})
	}
}

func TestSpanClass_AllValidSpans(t *testing.T) {
	t.Parallel()

	for span := 1; span <= 12; span++ {
		span := span
		t.Run("span", func(t *testing.T) {
			t.Parallel()
			got := spanClass(span)
			if got == "" {
				t.Errorf("spanClass(%d) returned empty string", span)
			}
		})
	}
}

func TestSpanClass_InvalidSpanFallsBackToSix(t *testing.T) {
	t.Parallel()

	fallback := spanClass(6)

	for _, bad := range []int{0, -1, 13, 100} {
		bad := bad
		t.Run("invalid", func(t *testing.T) {
			t.Parallel()
			got := spanClass(bad)
			if got != fallback {
				t.Errorf("spanClass(%d) = %q, want fallback %q", bad, got, fallback)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildSingleSeries tests
// ---------------------------------------------------------------------------

func TestBuildSingleSeries_WithValueColumn(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "label", Type: "string"},
			{Name: "value", Type: "number"},
		},
		Rows: []map[string]any{
			{"label": "Jan", "value": float64(100)},
			{"label": "Feb", "value": float64(200)},
			{"label": "Mar", "value": float64(300)},
		},
	}

	cm := lens.ColumnMap{}
	series := buildSingleSeries(result, cm)

	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	if series[0].Name != "Data" {
		t.Errorf("expected series name 'Data', got %q", series[0].Name)
	}
	if len(series[0].Data) != 3 {
		t.Fatalf("expected 3 data points, got %d", len(series[0].Data))
	}
	for i, want := range []float64{100, 200, 300} {
		if series[0].Data[i] != want {
			t.Errorf("data[%d]: expected %v, got %v", i, want, series[0].Data[i])
		}
	}
}

func TestBuildSingleSeries_ExplicitValueColumn(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "revenue", Type: "number"},
		},
		Rows: []map[string]any{
			{"revenue": float64(500), "value": float64(1)},
		},
	}

	// Explicitly map "revenue" as the value column.
	cm := lens.ColumnMap{Value: "revenue"}
	series := buildSingleSeries(result, cm)

	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	if len(series[0].Data) != 1 {
		t.Fatalf("expected 1 data point, got %d", len(series[0].Data))
	}
	if series[0].Data[0] != float64(500) {
		t.Errorf("expected 500, got %v", series[0].Data[0])
	}
}

func TestBuildSingleSeries_MissingValue(t *testing.T) {
	t.Parallel()

	// Row has no recognized value column — should fall back to 0.
	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{{Name: "name", Type: "string"}},
		Rows:    []map[string]any{{"name": "Alice"}, {"name": "Bob"}},
	}

	cm := lens.ColumnMap{}
	series := buildSingleSeries(result, cm)

	if len(series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(series))
	}
	for i, d := range series[0].Data {
		if d != 0 {
			t.Errorf("data[%d]: expected 0 for missing value, got %v", i, d)
		}
	}
}

// ---------------------------------------------------------------------------
// buildGroupedSeries tests
// ---------------------------------------------------------------------------

func TestBuildGroupedSeries_Basic(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "category", Type: "string"},
			{Name: "series", Type: "string"},
			{Name: "value", Type: "number"},
		},
		Rows: []map[string]any{
			{"category": "Q1", "series": "North", "value": float64(100)},
			{"category": "Q1", "series": "South", "value": float64(200)},
			{"category": "Q2", "series": "North", "value": float64(150)},
			{"category": "Q2", "series": "South", "value": float64(250)},
		},
	}

	cm := lens.ColumnMap{}
	series := buildGroupedSeries(result, cm)

	if len(series) != 2 {
		t.Fatalf("expected 2 series (North, South), got %d", len(series))
	}

	// Series order should match first-encounter order.
	if series[0].Name != "North" {
		t.Errorf("expected first series name 'North', got %q", series[0].Name)
	}
	if series[1].Name != "South" {
		t.Errorf("expected second series name 'South', got %q", series[1].Name)
	}

	// Each series should have 2 data points.
	for _, s := range series {
		if len(s.Data) != 2 {
			t.Errorf("series %q: expected 2 data points, got %d", s.Name, len(s.Data))
		}
	}
}

func TestBuildGroupedSeries_ExplicitSeriesColumn(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "region", Type: "string"},
			{Name: "value", Type: "number"},
		},
		Rows: []map[string]any{
			{"region": "EU", "value": float64(10)},
			{"region": "US", "value": float64(20)},
			{"region": "EU", "value": float64(30)},
		},
	}

	cm := lens.ColumnMap{Series: "region"}
	series := buildGroupedSeries(result, cm)

	if len(series) != 2 {
		t.Fatalf("expected 2 series (EU, US), got %d", len(series))
	}
	if series[0].Name != "EU" {
		t.Errorf("expected first series 'EU', got %q", series[0].Name)
	}
	if series[1].Name != "US" {
		t.Errorf("expected second series 'US', got %q", series[1].Name)
	}
	// EU appears twice.
	if len(series[0].Data) != 2 {
		t.Errorf("EU series: expected 2 data points, got %d", len(series[0].Data))
	}
}

func TestBuildGroupedSeries_NoSeriesColumn(t *testing.T) {
	t.Parallel()

	// No series column — all rows fall into the default "Series" group.
	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "value", Type: "number"},
		},
		Rows: []map[string]any{
			{"value": float64(1)},
			{"value": float64(2)},
		},
	}

	cm := lens.ColumnMap{}
	series := buildGroupedSeries(result, cm)

	// All rows should be grouped under the fallback name "Series".
	if len(series) != 1 {
		t.Fatalf("expected 1 series fallback group, got %d", len(series))
	}
	if series[0].Name != "Series" {
		t.Errorf("expected fallback name 'Series', got %q", series[0].Name)
	}
}

// ---------------------------------------------------------------------------
// buildLabels tests
// ---------------------------------------------------------------------------

func TestBuildLabels_ExplicitLabelColumn(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "month", Type: "string"},
			{Name: "value", Type: "number"},
		},
		Rows: []map[string]any{
			{"month": "Jan", "value": float64(1)},
			{"month": "Feb", "value": float64(2)},
			{"month": "Mar", "value": float64(3)},
		},
	}

	cm := lens.ColumnMap{Label: "month"}
	labels := buildLabels(result, cm)

	if len(labels) != 3 {
		t.Fatalf("expected 3 labels, got %d", len(labels))
	}
	for i, want := range []string{"Jan", "Feb", "Mar"} {
		if labels[i] != want {
			t.Errorf("label[%d]: expected %q, got %q", i, want, labels[i])
		}
	}
}

func TestBuildLabels_Fallback(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "label", Type: "string"},
		},
		Rows: []map[string]any{
			{"label": "A"},
			{"label": "B"},
		},
	}

	// Empty ColumnMap — falls back to "label" column.
	cm := lens.ColumnMap{}
	labels := buildLabels(result, cm)

	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}
	if labels[0] != "A" || labels[1] != "B" {
		t.Errorf("unexpected labels: %v", labels)
	}
}

func TestBuildLabels_EmptyResult(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{}
	cm := lens.ColumnMap{}

	labels := buildLabels(result, cm)
	if len(labels) != 0 {
		t.Errorf("expected 0 labels for empty result, got %d", len(labels))
	}
}

// ---------------------------------------------------------------------------
// panelData / resultData accessor tests
// ---------------------------------------------------------------------------

func TestPanelData_NilResults(t *testing.T) {
	t.Parallel()

	p := lens.Metric("m1", "M").Build()
	pr := panelData(p, nil)
	if pr != nil {
		t.Error("expected nil PanelResult for nil Results")
	}
}

func TestPanelData_NilPanelsMap(t *testing.T) {
	t.Parallel()

	results := &lens.Results{Panels: nil}
	p := lens.Metric("m1", "M").Build()
	pr := panelData(p, results)
	if pr != nil {
		t.Error("expected nil PanelResult when Panels map is nil")
	}
}

func TestPanelData_Found(t *testing.T) {
	t.Parallel()

	expected := &lens.PanelResult{Data: &lens.QueryResult{}}
	results := &lens.Results{
		Panels: map[string]*lens.PanelResult{
			"m1": expected,
		},
	}
	p := lens.Metric("m1", "M").Build()
	pr := panelData(p, results)
	if pr != expected {
		t.Errorf("expected matching PanelResult, got %v", pr)
	}
}

func TestResultData_NilPanelResult(t *testing.T) {
	t.Parallel()

	qr := resultData(nil)
	if qr != nil {
		t.Error("expected nil QueryResult for nil PanelResult")
	}
}

func TestResultData_WithData(t *testing.T) {
	t.Parallel()

	qr := &lens.QueryResult{Rows: []map[string]any{{"value": 1}}}
	pr := &lens.PanelResult{Data: qr}
	got := resultData(pr)
	if got != qr {
		t.Error("expected matching QueryResult")
	}
}

// ---------------------------------------------------------------------------
// toFloat64 tests
// ---------------------------------------------------------------------------

func TestToFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", int(7), 7, true},
		{"int32", int32(8), 8, true},
		{"int64", int64(9), 9, true},
		{"string", "not a number", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := toFloat64(tt.input)
			if ok != tt.ok {
				t.Errorf("ok: expected %v, got %v", tt.ok, ok)
			}
			if ok && got != tt.want {
				t.Errorf("value: expected %v, got %v", tt.want, got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// hasSeriesColumn tests
// ---------------------------------------------------------------------------

func TestHasSeriesColumn_WithSeriesColumn(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Rows: []map[string]any{
			{"series": "North", "value": float64(1)},
		},
	}
	cm := lens.ColumnMap{}

	if !hasSeriesColumn(result, cm) {
		t.Error("expected hasSeriesColumn=true when 'series' column is present")
	}
}

func TestHasSeriesColumn_WithExplicitSeriesMapping(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Rows: []map[string]any{
			{"region": "EU", "value": float64(1)},
		},
	}
	cm := lens.ColumnMap{Series: "region"}

	if !hasSeriesColumn(result, cm) {
		t.Error("expected hasSeriesColumn=true when explicitly mapped series column is present")
	}
}

func TestHasSeriesColumn_WithoutSeriesColumn(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{
		Rows: []map[string]any{
			{"label": "Jan", "value": float64(1)},
		},
	}
	cm := lens.ColumnMap{}

	if hasSeriesColumn(result, cm) {
		t.Error("expected hasSeriesColumn=false when no series column is present")
	}
}

func TestHasSeriesColumn_EmptyResult(t *testing.T) {
	t.Parallel()

	result := &lens.QueryResult{}
	cm := lens.ColumnMap{}

	if hasSeriesColumn(result, cm) {
		t.Error("expected hasSeriesColumn=false for empty result")
	}
}

// ---------------------------------------------------------------------------
// tableColumns tests
// ---------------------------------------------------------------------------

func TestTableColumns_ExplicitColumns(t *testing.T) {
	t.Parallel()

	cols := []lens.TableColumn{
		{Key: "id", Label: "ID", Format: "number"},
		{Key: "name", Label: "Name"},
	}
	p := lens.Table("t1", "T").Columns(cols...).Build()
	result := &lens.QueryResult{}

	got := tableColumns(p, result)
	if len(got) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(got))
	}
	for i, c := range cols {
		if got[i].Key != c.Key || got[i].Label != c.Label {
			t.Errorf("col[%d]: expected {%q, %q}, got {%q, %q}", i, c.Key, c.Label, got[i].Key, got[i].Label)
		}
	}
}

func TestTableColumns_AutoDetectFromResult(t *testing.T) {
	t.Parallel()

	p := lens.Table("t1", "T").Build() // No explicit columns.
	result := &lens.QueryResult{
		Columns: []lens.QueryColumn{
			{Name: "id", Type: "number"},
			{Name: "email", Type: "string"},
		},
	}

	got := tableColumns(p, result)
	if len(got) != 2 {
		t.Fatalf("expected 2 auto-detected columns, got %d", len(got))
	}
	if got[0].Key != "id" || got[0].Label != "id" {
		t.Errorf("col[0]: expected key=id, label=id, got key=%q, label=%q", got[0].Key, got[0].Label)
	}
	if got[1].Key != "email" || got[1].Label != "email" {
		t.Errorf("col[1]: expected key=email, label=email, got key=%q, label=%q", got[1].Key, got[1].Label)
	}
}

func TestTableColumns_NilResult(t *testing.T) {
	t.Parallel()

	p := lens.Table("t1", "T").Build()

	got := tableColumns(p, nil)
	if got != nil {
		t.Errorf("expected nil when result is nil and no explicit columns, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// panelHeight tests
// ---------------------------------------------------------------------------

func TestPanelHeight_Default(t *testing.T) {
	t.Parallel()

	p := lens.Line("l1", "L").Build()
	got := panelHeight(p)
	if got != "320px" {
		t.Errorf("expected default height 320px, got %q", got)
	}
}

func TestPanelHeight_Custom(t *testing.T) {
	t.Parallel()

	p := lens.Line("l1", "L").Height("500px").Build()
	got := panelHeight(p)
	if got != "500px" {
		t.Errorf("expected 500px, got %q", got)
	}
}

func TestPanelHeight_MetricPanel(t *testing.T) {
	t.Parallel()

	// Metric panels have no ChartOptions — should return default.
	p := lens.Metric("m1", "M").Build()
	got := panelHeight(p)
	if got != "320px" {
		t.Errorf("expected 320px for metric panel, got %q", got)
	}
}
