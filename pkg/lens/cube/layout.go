package cube

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
)

// LayoutItem represents a declarative row element passed to Builder.Layout.
type LayoutItem interface {
	layoutRow() LayoutRowSpec
}

type LayoutRowKind string

const (
	LayoutRowKPI    LayoutRowKind = "kpi"
	LayoutRowCharts LayoutRowKind = "charts"
)

// LayoutRowSpec is the compiled representation of one declarative layout row.
type LayoutRowSpec struct {
	Kind     LayoutRowKind
	Measures []string
	Charts   []ChartSpec
}

// ChartSpec describes one chart panel inside a declarative cube Layout.
type ChartSpec struct {
	ID             string
	Title          string
	Description    string
	Kind           panel.Kind
	Dimension      string
	Series         string
	Measure        string
	TopN           int
	WithRemainder  bool
	RemainderLabel string
	Span           int
	Height         string
	Colors         []string
	Presentation   panel.PresentationHints
}

type layoutRowItem struct {
	spec LayoutRowSpec
}

func (l layoutRowItem) layoutRow() LayoutRowSpec {
	return l.spec
}

// KPIRow declares a horizontal KPI stat strip for the named measures.
// When called with no arguments, all cube measures are included.
func KPIRow(measures ...string) LayoutItem {
	cleaned := make([]string, 0, len(measures))
	for _, m := range measures {
		if trimmed := strings.TrimSpace(m); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return layoutRowItem{
		spec: LayoutRowSpec{
			Kind:     LayoutRowKPI,
			Measures: cleaned,
		},
	}
}

// SplitRow arranges multiple chart items side-by-side in one 12-column row.
func SplitRow(items ...LayoutItem) LayoutItem {
	charts := make([]ChartSpec, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		row := item.layoutRow()
		if row.Kind == LayoutRowCharts {
			charts = append(charts, row.Charts...)
		}
	}
	return layoutRowItem{
		spec: LayoutRowSpec{
			Kind:   LayoutRowCharts,
			Charts: charts,
		},
	}
}

// ChartBuilder constructs one declarative chart panel for Builder.Layout or SplitRow.
type ChartBuilder struct {
	spec ChartSpec
}

// Chart starts a declarative chart panel with the given panel id and kind.
func Chart(id string, kind panel.Kind) *ChartBuilder {
	return &ChartBuilder{
		spec: ChartSpec{
			ID:   strings.TrimSpace(id),
			Kind: kind,
		},
	}
}

func (c *ChartBuilder) Title(title string) *ChartBuilder {
	c.spec.Title = title
	return c
}

func (c *ChartBuilder) Description(description string) *ChartBuilder {
	c.spec.Description = description
	return c
}

func (c *ChartBuilder) Dimension(name string) *ChartBuilder {
	c.spec.Dimension = strings.TrimSpace(name)
	return c
}

func (c *ChartBuilder) Series(name string) *ChartBuilder {
	c.spec.Series = strings.TrimSpace(name)
	return c
}

func (c *ChartBuilder) Measure(name string) *ChartBuilder {
	c.spec.Measure = strings.TrimSpace(name)
	return c
}

func (c *ChartBuilder) TopN(n int) *ChartBuilder {
	c.spec.TopN = n
	return c
}

// WithRemainder collapses tail categories beyond TopN into an expandable
// remainder bucket. Optional label overrides the default "Other" bucket title.
func (c *ChartBuilder) WithRemainder(label ...string) *ChartBuilder {
	c.spec.WithRemainder = true
	if len(label) > 0 && strings.TrimSpace(label[0]) != "" {
		c.spec.RemainderLabel = strings.TrimSpace(label[0])
	} else if strings.TrimSpace(c.spec.RemainderLabel) == "" {
		c.spec.RemainderLabel = "Other"
	}
	return c
}

func (c *ChartBuilder) Span(span int) *ChartBuilder {
	c.spec.Span = span
	return c
}

func (c *ChartBuilder) Height(height string) *ChartBuilder {
	c.spec.Height = height
	return c
}

func (c *ChartBuilder) Colors(colors ...string) *ChartBuilder {
	c.spec.Colors = append([]string(nil), colors...)
	return c
}

func (c *ChartBuilder) Presentation(hints panel.PresentationHints) *ChartBuilder {
	c.spec.Presentation = hints
	return c
}

func (c *ChartBuilder) Build() ChartSpec {
	return c.spec
}

func (c *ChartBuilder) layoutRow() LayoutRowSpec {
	return LayoutRowSpec{
		Kind:   LayoutRowCharts,
		Charts: []ChartSpec{c.spec},
	}
}

// LeafLinkResolver generates a destination URL (for example a portfolio or
// registry route) from the accumulated drill context and resolved cube params.
type LeafLinkResolver func(ctx context.Context, drill DrillContext, params map[string]any) string

// LeafTableSpec configures an embedded paginated Lens table rendered at the
// leaf level of a cube drill-down.
type LeafTableSpec struct {
	ID          string              `json:"id"`
	Title       string              `json:"title"`
	Description string              `json:"description,omitempty"`
	IDField     string              `json:"idField,omitempty"`
	Columns     []panel.TableColumn `json:"columns,omitempty"`
	SQL         string              `json:"sql,omitempty"`
	OrderBy     string              `json:"orderBy,omitempty"`
	PageSize    int                 `json:"pageSize,omitempty"`
	Searchable  bool                `json:"searchable,omitempty"`
	RowAction   *action.Spec        `json:"rowAction,omitempty"`
	Height      string              `json:"height,omitempty"`
}

// LeafTable declares an embedded paginated Lens table for a cube's leaf level.
func LeafTable(id, title string) *LeafTableSpec {
	return &LeafTableSpec{
		ID:         strings.TrimSpace(id),
		Title:      strings.TrimSpace(title),
		PageSize:   50,
		Searchable: true,
	}
}

func (t *LeafTableSpec) WithDescription(desc string) *LeafTableSpec {
	t.Description = desc
	return t
}

func (t *LeafTableSpec) WithIDField(field string) *LeafTableSpec {
	t.IDField = strings.TrimSpace(field)
	return t
}

func (t *LeafTableSpec) WithColumns(cols ...panel.TableColumn) *LeafTableSpec {
	t.Columns = append([]panel.TableColumn(nil), cols...)
	return t
}

func (t *LeafTableSpec) WithQuery(sql string) *LeafTableSpec {
	t.SQL = sql
	return t
}

func (t *LeafTableSpec) WithOrderBy(orderBy string) *LeafTableSpec {
	t.OrderBy = orderBy
	return t
}

func (t *LeafTableSpec) WithPageSize(size int) *LeafTableSpec {
	t.PageSize = size
	return t
}

func (t *LeafTableSpec) WithSearchable(searchable bool) *LeafTableSpec {
	t.Searchable = searchable
	return t
}

func (t *LeafTableSpec) WithRowAction(spec action.Spec) *LeafTableSpec {
	cloned := cloneActionSpec(spec)
	t.RowAction = &cloned
	return t
}

func (t *LeafTableSpec) WithHeight(height string) *LeafTableSpec {
	t.Height = height
	return t
}

func cloneActionSpec(spec action.Spec) action.Spec {
	cloned := spec
	if spec.Payload != nil {
		cloned.Payload = make(map[string]action.ValueSource, len(spec.Payload))
		for key, value := range spec.Payload {
			cloned.Payload[key] = value
		}
	}
	if spec.Params != nil {
		cloned.Params = append([]action.Param(nil), spec.Params...)
	}
	if spec.Drill != nil {
		drill := *spec.Drill
		cloned.Drill = &drill
	}
	return cloned
}

func normalizeTimeGranularity(granularity string) string {
	switch strings.ToLower(strings.TrimSpace(granularity)) {
	case "day", "week", "month", "quarter", "year":
		return strings.ToLower(strings.TrimSpace(granularity))
	case "":
		return "month"
	default:
		return strings.ToLower(strings.TrimSpace(granularity))
	}
}

func validTimeGranularity(granularity string) bool {
	switch strings.ToLower(strings.TrimSpace(granularity)) {
	case "day", "week", "month", "quarter", "year":
		return true
	default:
		return false
	}
}

// timeBucketSQLExpr returns the PostgreSQL expression producing canonical
// bucket keys ("2026", "2026-Q1", "2026-03", "2026-03-02", "2026-03-15").
func timeBucketSQLExpr(column, granularity string) string {
	g := normalizeTimeGranularity(granularity)
	switch g {
	case "year":
		return fmt.Sprintf("to_char(date_trunc('year', %s), 'YYYY')", column)
	case "quarter":
		return fmt.Sprintf(`to_char(date_trunc('quarter', %s), 'YYYY-"Q"Q')`, column)
	case "month":
		return fmt.Sprintf("to_char(date_trunc('month', %s), 'YYYY-MM')", column)
	case "week":
		return fmt.Sprintf("to_char(date_trunc('week', %s), 'YYYY-MM-DD')", column)
	case "day":
		return fmt.Sprintf("to_char(date_trunc('day', %s), 'YYYY-MM-DD')", column)
	default:
		return fmt.Sprintf("to_char(date_trunc('month', %s), 'YYYY-MM')", column)
	}
}

// FormatTimeBucket formats a timestamp into the canonical bucket key for the
// given granularity ("day", "week", "month", "quarter", "year").
func FormatTimeBucket(value time.Time, granularity string) string {
	if value.IsZero() {
		return ""
	}
	g := normalizeTimeGranularity(granularity)
	loc := value.Location()
	switch g {
	case "year":
		return fmt.Sprintf("%04d", value.Year())
	case "quarter":
		q := (int(value.Month())-1)/3 + 1
		return fmt.Sprintf("%04d-Q%d", value.Year(), q)
	case "month":
		return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, loc).Format("2006-01")
	case "week":
		weekday := int(value.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		monday := time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -(weekday - 1))
		return monday.Format("2006-01-02")
	case "day":
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc).Format("2006-01-02")
	default:
		return value.Format("2006-01")
	}
}

// TimeBucketBounds parses a canonical bucket key into its inclusive start and
// exclusive end timestamps in UTC.
func TimeBucketBounds(granularity, value string) (time.Time, time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, time.Time{}, false
	}
	switch normalizeTimeGranularity(granularity) {
	case "year":
		start, err := time.Parse("2006", value)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return start, start.AddDate(1, 0, 0), true
	case "quarter":
		var year, quarter int
		if _, err := fmt.Sscanf(value, "%4d-Q%1d", &year, &quarter); err != nil || quarter < 1 || quarter > 4 {
			return time.Time{}, time.Time{}, false
		}
		start := time.Date(year, time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 3, 0), true
	case "month":
		start, err := time.Parse("2006-01", value)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return start, start.AddDate(0, 1, 0), true
	case "week":
		start, err := time.Parse("2006-01-02", value)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return start, start.AddDate(0, 0, 7), true
	case "day":
		start, err := time.Parse("2006-01-02", value)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		return start, start.AddDate(0, 0, 1), true
	default:
		return time.Time{}, time.Time{}, false
	}
}

func coerceRowTimeBucket(raw any, granularity string) string {
	switch typed := raw.(type) {
	case time.Time:
		return FormatTimeBucket(typed, granularity)
	case *time.Time:
		if typed != nil {
			return FormatTimeBucket(*typed, granularity)
		}
		return ""
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return ""
		}
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(layout, trimmed); err == nil {
				return FormatTimeBucket(parsed, granularity)
			}
		}
		return trimmed
	default:
		return strings.TrimSpace(fmt.Sprint(raw))
	}
}
