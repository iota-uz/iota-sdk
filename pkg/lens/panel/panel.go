// Package panel defines Lens dashboard panel specs and builders.
package panel

import (
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/chrome"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type Kind string

const (
	KindStat          Kind = "stat"
	KindTimeSeries    Kind = "time_series"
	KindBar           Kind = "bar"
	KindHorizontalBar Kind = "horizontal_bar"
	KindStackedBar    Kind = "stacked_bar"
	// KindSegmentBar renders a part-to-whole value as a single horizontal
	// segmented bar (a headline total, the segmented track, and a legend)
	// using native HTML/CSS rather than a chart engine. Each dataset row is
	// one segment: LabelField → name, ValueField → amount; Colors are applied
	// positionally. Built for two- or few-part splits (e.g. claims paid =
	// within reserve + over reserve) where a chart's axes and plot area are
	// pure noise.
	KindSegmentBar Kind = "segment_bar"
	// KindCascade renders a bridge/cascade as narrowing running-total rows
	// with deduction connectors between them. It is native HTML/CSS rather
	// than an ApexCharts chart.
	KindCascade Kind = "cascade"
	KindPie     Kind = "pie"
	KindDonut   Kind = "donut"
	KindTable   Kind = "table"
	KindGauge   Kind = "gauge"
	KindTabs    Kind = "tabs"
	KindGrid    Kind = "grid"
	KindSplit   Kind = "split"
	KindRepeat  Kind = "repeat"
	// KindStatGroup renders several Stat children inside ONE card, separated
	// by hairline dividers (columns or rows per GroupLayout), instead of one
	// card per KPI. The group is a layout container: it has no dataset of its
	// own; every child is a regular Stat leaf with its own dataset lookup.
	KindStatGroup Kind = "stat_group"
)

// IsContainer reports whether the kind is a layout container that renders its
// Children rather than its own dataset. Membership: KindTabs, KindGrid,
// KindSplit, KindRepeat, KindStatGroup. These are the kinds the runtime/render
// code recurses into instead of validating a dataset or drawing a chart body.
//
// Keeping this membership in one predicate lets the recursion sites branch on a
// category instead of re-enumerating the container kinds in every switch, so a
// new container kind only has to be added here.
func (k Kind) IsContainer() bool {
	switch k {
	case KindTabs, KindGrid, KindSplit, KindRepeat, KindStatGroup:
		return true
	case KindStat, KindTimeSeries, KindBar, KindHorizontalBar, KindStackedBar,
		KindSegmentBar, KindCascade, KindPie, KindDonut, KindTable, KindGauge:
		return false
	}
	return false
}

// IsChart reports whether the kind is a leaf panel rendered through the Apex
// charts engine. Membership: KindTimeSeries, KindBar, KindHorizontalBar,
// KindStackedBar, KindPie, KindDonut, KindGauge.
//
// This is the complement, among leaf panels, of RendersNatively: every leaf is
// either an apex chart or a native (non-apex) render. KindStat, KindSegmentBar,
// KindCascade and KindTable draw their own HTML/CSS and are therefore NOT charts.
func (k Kind) IsChart() bool {
	switch k {
	case KindTimeSeries, KindBar, KindHorizontalBar, KindStackedBar,
		KindPie, KindDonut, KindGauge:
		return true
	case KindStat, KindSegmentBar, KindCascade, KindTable,
		KindTabs, KindGrid, KindSplit, KindRepeat, KindStatGroup:
		return false
	}
	return false
}

// RendersNatively reports whether the kind is a leaf panel drawn with native
// HTML/CSS rather than the ApexCharts engine. Membership: KindStat,
// KindSegmentBar, KindCascade, KindTable.
//
// Together, IsChart() and RendersNatively() partition the leaf (non-container)
// panel kinds, so "this kind is a renderable leaf" is exactly
// `k.IsChart() || k.RendersNatively()`.
func (k Kind) RendersNatively() bool {
	switch k {
	case KindStat, KindSegmentBar, KindCascade, KindTable:
		return true
	case KindTimeSeries, KindBar, KindHorizontalBar, KindStackedBar,
		KindPie, KindDonut, KindGauge,
		KindTabs, KindGrid, KindSplit, KindRepeat, KindStatGroup:
		return false
	}
	return false
}

// StatusTone selects the color treatment of a stat card's status chip.
type StatusTone string

const (
	StatusNeutral  StatusTone = "neutral"
	StatusPositive StatusTone = "positive"
	StatusWarning  StatusTone = "warning"
)

// StatusSpec renders a small uppercase chip in a stat card's label row (e.g.
// "ON TRACK"), colored by Tone.
type StatusSpec struct {
	Label string     `json:"label"`
	Tone  StatusTone `json:"tone,omitempty"`
}

// SparklineSpec renders a small inline trend polyline in a stat card's footer
// row. Values are plotted left-to-right, normalized to the sparkline viewbox.
// Color overrides the default accent stroke when set (any CSS color).
type SparklineSpec struct {
	Values []float64 `json:"values"`
	Color  string    `json:"color,omitempty"`
}

// PresentationHints are optional, renderer-level density choices carried on a
// panel spec. Every field is opt-in: the zero value keeps today's rendering.
type PresentationHints struct {
	// LegendBelow renders a centered wrapping legend under the plot with one
	// "label · value" entry per slice.
	LegendBelow bool
	// SliceLabelsPercent writes each partition slice's share inside the slice.
	SliceLabelsPercent bool
	// TotalBadgeInPlot floats the total badge inside the plot area instead of
	// placing it in the panel header.
	TotalBadgeInPlot bool
	// FillPlot lets the plot occupy the whole card instead of the default
	// inset.
	FillPlot bool
	// BarWidthPx pins the rendered bar thickness in CSS pixels.
	BarWidthPx int
	// ColorByCategory gives every category its own palette color.
	ColorByCategory bool
	// HideTotalBadge suppresses the total badge, e.g. when a trend chip
	// already carries the panel's summary.
	HideTotalBadge bool
	// NonSortable removes a table panel's sort affordances. A static identity
	// table (a fixed decomposition, not a record list) sets it so the header
	// does not offer to reorder rows that have an inherent order.
	NonSortable bool
	// NonExpandable removes a panel's expand-to-overlay control, e.g. every
	// panel rendered inside a drawer where an overlay over the modal is
	// meaningless.
	NonExpandable bool
	// NonExportable removes a panel's export control, e.g. a small derived table
	// whose figures are already the drawer's whole point.
	NonExportable bool
}

// GroupLayout selects how a StatGroup panel arranges its children inside the
// shared card: side-by-side columns with vertical hairlines, or a vertical
// list with horizontal hairlines.
type GroupLayout string

const (
	GroupColumns GroupLayout = "columns"
	GroupRows    GroupLayout = "rows"
)

// LegendPosition controls where a chart places its legend. Empty keeps the
// renderer default (bottom). A side legend preserves vertical plot area for
// charts with long category labels, especially pies and donuts.
type LegendPosition string

const (
	LegendTop    LegendPosition = "top"
	LegendRight  LegendPosition = "right"
	LegendBottom LegendPosition = "bottom"
	LegendLeft   LegendPosition = "left"
)

type AxisScale string

const (
	AxisScaleLinear      AxisScale = "linear"
	AxisScaleLogarithmic AxisScale = "logarithmic"
)

type ValueAxis struct {
	Scale   AxisScale
	LogBase int
}

type TableColumn struct {
	Field     FieldRef
	Label     string
	Formatter *format.Spec
	Action    *action.Spec
	Text      string
	// Align controls the column's text alignment: "" (left) or "right".
	Align string
	// Cell selects a rich cell renderer (bar / delta) instead of the default
	// plain-text cell. Nil means plain text.
	Cell *TableCellSpec
	// WidthPx, when > 0, sets a min-width (px) on the column's header and
	// body cells (inline style, so it survives consumer CSS purging).
	WidthPx int
	// ClampLines, when > 0, limits the cell text to that many lines so long
	// labels cannot inflate row height.
	ClampLines int
	// Affordance selects how an actionable cell advertises its action;
	// "pill" renders a compact pill with a drill arrow.
	Affordance string
}

// Pill marks an actionable column's cells as compact drill pills.
func (c TableColumn) Pill() TableColumn {
	c.Affordance = "pill"
	return c
}

// Width sets a min-width (px) on the column's cells.
func (c TableColumn) Width(px int) TableColumn {
	c.WidthPx = px
	return c
}

// Clamp limits the column's cell text to lines rendered lines.
func (c TableColumn) Clamp(lines int) TableColumn {
	c.ClampLines = lines
	return c
}

// TableCellKind selects a Table panel's rich cell renderer.
type TableCellKind string

const (
	// TableCellBar renders the numeric value alongside a proportional mini-bar
	// scaled against the column's max absolute value.
	TableCellBar TableCellKind = "bar"
	// TableCellDelta renders a signed delta plus a percent change, colored by
	// sign.
	TableCellDelta TableCellKind = "delta"
	// TableCellUnderline renders the value over a thin proportional rule
	// colored by sign — a low-ink alternative to TableCellBar.
	TableCellUnderline TableCellKind = "underline"
)

// TableCellSpec configures a Table panel column's rich cell renderer.
type TableCellSpec struct {
	Kind TableCellKind
	// PercentField is used by TableCellDelta cells: the field holding the
	// percent number rendered alongside the delta. Its values are already in
	// percent units: -4 renders as -4.0%, and a 0..1 share would render as
	// 0.1% instead of 10%.
	PercentField FieldRef
	// Stacked puts the secondary value on its own line under the primary one.
	Stacked bool
}

type FieldRef string

const (
	DefaultLabelField    FieldRef = "label"
	DefaultValueField    FieldRef = "value"
	DefaultSeriesField   FieldRef = "series"
	DefaultCategoryField FieldRef = "category"
	DefaultIDField       FieldRef = "id"
	DefaultCutField      FieldRef = "cut"
	DefaultCutLabelField FieldRef = "cutLabel"
	DefaultFinalField    FieldRef = "final"
)

func (f FieldRef) Name() string {
	return string(f)
}

func (f FieldRef) Empty() bool {
	return strings.TrimSpace(f.Name()) == ""
}

type Spec struct {
	ID             string
	Title          string
	Description    string
	Info           string
	Kind           Kind
	Dataset        string
	Span           int
	Height         string
	Colors         []string
	ShowLegend     bool
	LegendPosition LegendPosition
	LegendWidthPx  int
	LegendOffsetY  int
	LegendFloating bool
	// CircularScale and CircularOffsetX let dense pie/donut panels reserve a
	// stable plot area while a floating side legend occupies the other half.
	// CircularScale is zero when unset and must be positive when configured.
	// Both settings are ignored by non-circular panels.
	CircularScale   float64
	CircularOffsetX int
	ShowTotalBadge  bool
	// TotalBadgeValue, when set, renders the total badge with this
	// server-computed value instead of summing the plotted data points
	// client-side. Required for panels whose plotted series are not the raw
	// amounts (e.g. log-transformed values with an epsilon floor). The badge
	// then stays constant across legend toggles — it is the period total.
	TotalBadgeValue *float64
	// HeadlineValue overrides the computed headline without changing the
	// values used for chart geometry. SegmentBar uses it for a focal result
	// whose allocation segments still sum to a different denominator.
	HeadlineValue  *float64
	DrillHierarchy *DrillHierarchy
	// DrillTree enables stable, key-based in-place navigation for Pie and
	// Donut panels. Its branch keys match the panel's ID field in the initial
	// dataset; nested node keys remain stable when labels or ordering change.
	DrillTree *DrillTree
	Trend     *TrendSpec
	// Status renders a small tone-colored chip in a stat card's label row.
	// Only Stat panels (including StatGroup children) render it.
	Status *StatusSpec
	// Sparkline renders a small inline trend polyline in a stat card's footer
	// row. Only Stat panels (including StatGroup children) render it.
	Sparkline *SparklineSpec
	// GroupLayout arranges a StatGroup's children ("columns" default, or
	// "rows"). Ignored on every other kind.
	GroupLayout GroupLayout
	// Presentation carries opt-in rendering hints for wire (document)
	// renderers. The templ/Apex renderer ignores them; they exist so a
	// dashboard can ask the React runtime for a denser treatment without a
	// bespoke panel kind.
	Presentation PresentationHints
	Fields       FieldMapping
	Formatter    *format.Spec
	Columns      []TableColumn
	Transforms   []transform.Spec
	Action       *action.Spec
	Children     []Spec
	ClassName    string
	Chrome       chrome.Spec
	ValueAxis    ValueAxis
	Distributed  bool
	ColorField   FieldRef
	ColorScale   string
	Export       exportmeta.Spec
}

// DrillHierarchy carries a pre-computed multi-level dataset that lets a Bar
// panel "zoom" client-side (year -> quarter, and expand a trailing-years
// "Others" bucket) with zero further server round-trips. See EAI's
// analytics dashboards.buildPremiumBySourceYearChart for the producer and
// render/apex's buildDrillHierarchyJS for the consumer.
type DrillHierarchy struct {
	// Sources lists the raw (untranslated) source keys in the same order as
	// the chart's series/Colors (index i is series i). Lets the click
	// handler resolve ApexCharts' numeric seriesIndex to a stable key
	// without string-matching the locale-dependent series display name.
	Sources []string
	// OthersLabel is the already-localized category label for the top-level
	// bucket bar (e.g. "Остальные"). Empty means the dataset fit within the
	// recent-years window and there is no bucket.
	OthersLabel string
	// OthersYears lists, ascending, every year folded into the bucket. This
	// is the "expand Others" view's category axis.
	OthersYears []int
	// Years maps "<year>|<sourceKey>" to that cell's raw (unfloored,
	// unscaled) amount, for EVERY year in the dataset — both the recently
	// shown years and every bucketed year.
	Years map[string]float64
	// Quarters maps "<year>|<sourceKey>" to that pair's Q1..Q4 breakdown,
	// for the same full set of years as Years.
	Quarters map[string]QuarterBreakdown
}

// QuarterBreakdown is one (year, source) pair's quarterly detail.
type QuarterBreakdown struct {
	Amounts      [4]float64 // Q1..Q4, index 0 = Q1; raw, unfloored
	NavigateURLs [4]string  // Q1..Q4 navigate target; "" = not navigable
}

// DrillTree carries pre-computed, key-based branches for in-place Pie and
// Donut navigation. Every DrillBranch.TriggerKey must match exactly one value
// in the panel's ID field. Keys are identity and must stay stable across
// translation and reordering; labels are presentation only.
//
// Branches must contain at least one child. Node keys must be nonblank and
// unique among siblings, values must be finite and nonnegative, and a node may
// have either Children or Action, but not both. Leaf nodes may be informational
// and omit Action.
type DrillTree struct {
	Branches []DrillBranch `json:"branches"`
	// ExpandedSpan optionally widens the panel's dashboard grid slot while
	// the user is inside any drill level. Zero preserves the panel's root
	// span. Returning to the root restores the original span automatically.
	ExpandedSpan int `json:"expandedSpan,omitempty"`
}

// DrillBranch binds one initial chart point to its first detail level.
type DrillBranch struct {
	TriggerKey string          `json:"triggerKey"`
	Label      string          `json:"label"`
	View       *DrillLevelView `json:"view,omitempty"`
	Children   []DrillNode     `json:"children"`
}

// DrillLevelView controls the presentation of a branch's or node's child
// level. The closest configured ancestor is inherited by deeper levels, so a
// dense detail journey can keep a side legend without repeating the layout on
// every node. Returning to the root restores the panel's original layout.
type DrillLevelView struct {
	LegendPosition  LegendPosition `json:"legendPosition,omitempty"`
	LegendWidthPx   int            `json:"legendWidthPx,omitempty"`
	LegendOffsetY   int            `json:"legendOffsetY,omitempty"`
	LegendFloating  bool           `json:"legendFloating,omitempty"`
	CircularScale   float64        `json:"circularScale,omitempty"`
	CircularOffsetX int            `json:"circularOffsetX,omitempty"`
}

// DrillNode is one stable item in a DrillTree detail level. Navigate,
// HtmxSwap, and EmitEvent actions are supported on leaves; actions that depend
// on an unresolved dataset row are not supported.
type DrillNode struct {
	Key      string          `json:"key"`
	Label    string          `json:"label"`
	Value    float64         `json:"value"`
	Color    string          `json:"color,omitempty"`
	Action   *action.Spec    `json:"action,omitempty"`
	View     *DrillLevelView `json:"view,omitempty"`
	Children []DrillNode     `json:"children,omitempty"`
}

// TrendSpec renders a small colored chip in a panel's header showing a signed
// percent change alongside a comparison label (e.g. "vs last month").
type TrendSpec struct {
	Percent float64 `json:"percent"`
	Label   string  `json:"label,omitempty"`
	// Invert flips the good/bad color mapping for down-is-good metrics
	// (e.g. loss ratio): a negative percent renders with the positive
	// (green) treatment and vice versa. The arrow always follows the sign.
	Invert bool `json:"invert,omitempty"`
}

type FieldMapping struct {
	Label     FieldRef
	Value     FieldRef
	Series    FieldRef
	Category  FieldRef
	ID        FieldRef
	StartTime FieldRef
	EndTime   FieldRef
	Cut       FieldRef
	CutLabel  FieldRef
	Final     FieldRef
}

type Builder struct {
	spec Spec
}

func Stat(id, title, dataset string) *Builder { return newBuilder(KindStat, id, title, dataset) }
func TimeSeries(id, title, dataset string) *Builder {
	return newBuilder(KindTimeSeries, id, title, dataset)
}
func Bar(id, title, dataset string) *Builder { return newBuilder(KindBar, id, title, dataset) }
func HorizontalBar(id, title, dataset string) *Builder {
	return newBuilder(KindHorizontalBar, id, title, dataset)
}
func StackedBar(id, title, dataset string) *Builder {
	return newBuilder(KindStackedBar, id, title, dataset)
}
func SegmentBar(id, title, dataset string) *Builder {
	return newBuilder(KindSegmentBar, id, title, dataset)
}
func Cascade(id, title, dataset string) *Builder { return newBuilder(KindCascade, id, title, dataset) }
func Pie(id, title, dataset string) *Builder     { return newBuilder(KindPie, id, title, dataset) }
func Donut(id, title, dataset string) *Builder   { return newBuilder(KindDonut, id, title, dataset) }
func Table(id, title, dataset string) *Builder   { return newBuilder(KindTable, id, title, dataset) }
func Gauge(id, title, dataset string) *Builder   { return newBuilder(KindGauge, id, title, dataset) }

func Tabs(id, title string, children ...Spec) *Builder {
	return &Builder{
		spec: Spec{
			ID:       id,
			Title:    title,
			Kind:     KindTabs,
			Span:     6,
			Children: children,
		},
	}
}

func Grid(id, title string, children ...Spec) *Builder {
	return &Builder{
		spec: Spec{
			ID:       id,
			Title:    title,
			Kind:     KindGrid,
			Span:     12,
			Children: children,
		},
	}
}

// StatGroup builds a container that renders its Stat children inside one
// shared card, separated by hairlines (columns by default; see Layout).
func StatGroup(id, title string, children ...Spec) *Builder {
	return &Builder{
		spec: Spec{
			ID:          id,
			Title:       title,
			Kind:        KindStatGroup,
			Span:        12,
			GroupLayout: GroupColumns,
			Children:    children,
		},
	}
}

func newBuilder(kind Kind, id, title, dataset string) *Builder {
	return &Builder{
		spec: Spec{
			ID:      id,
			Title:   title,
			Kind:    kind,
			Dataset: dataset,
			Span:    6,
			Fields: FieldMapping{
				Label:    DefaultLabelField,
				Value:    DefaultValueField,
				Series:   DefaultSeriesField,
				Category: DefaultCategoryField,
				ID:       DefaultIDField,
				Cut:      DefaultCutField,
				CutLabel: DefaultCutLabelField,
				Final:    DefaultFinalField,
			},
		},
	}
}

func (b *Builder) Span(span int) *Builder           { b.spec.Span = span; return b }
func (b *Builder) Height(height string) *Builder    { b.spec.Height = height; return b }
func (b *Builder) Colors(colors ...string) *Builder { b.spec.Colors = colors; return b }
func (b *Builder) Legend() *Builder                 { b.spec.ShowLegend = true; return b }
func (b *Builder) LegendAt(position LegendPosition) *Builder {
	b.spec.ShowLegend = true
	b.spec.LegendPosition = position
	return b
}
func (b *Builder) LegendWidth(px int) *Builder {
	b.spec.ShowLegend = true
	b.spec.LegendWidthPx = px
	return b
}
func (b *Builder) LegendOffsetY(px int) *Builder {
	b.spec.ShowLegend = true
	b.spec.LegendOffsetY = px
	return b
}
func (b *Builder) FloatingLegend() *Builder {
	b.spec.ShowLegend = true
	b.spec.LegendFloating = true
	return b
}
func (b *Builder) CircularScale(scale float64) *Builder {
	b.spec.CircularScale = scale
	return b
}
func (b *Builder) CircularOffsetX(px int) *Builder {
	b.spec.CircularOffsetX = px
	return b
}
func (b *Builder) TotalBadge() *Builder { b.spec.ShowTotalBadge = true; return b }
func (b *Builder) TotalBadgeValue(v float64) *Builder {
	b.spec.ShowTotalBadge = true
	b.spec.TotalBadgeValue = &v
	return b
}
func (b *Builder) HeadlineValue(v float64) *Builder {
	b.spec.HeadlineValue = &v
	return b
}
func (b *Builder) DrillHierarchy(h DrillHierarchy) *Builder {
	b.spec.DrillHierarchy = &h
	return b
}

// DrillTree enables stable, key-based in-place navigation. Configure IDField
// with the initial dataset field whose values match branch trigger keys.
func (b *Builder) DrillTree(tree DrillTree) *Builder {
	b.spec.DrillTree = &tree
	return b
}

func (b *Builder) Trend(percent float64, label string) *Builder {
	b.spec.Trend = &TrendSpec{Percent: percent, Label: label}
	return b
}

// TrendWithInvert is Trend for down-is-good metrics: invert flips the
// good/bad color mapping while the arrow still follows the sign.
func (b *Builder) TrendWithInvert(percent float64, label string, invert bool) *Builder {
	b.spec.Trend = &TrendSpec{Percent: percent, Label: label, Invert: invert}
	return b
}

// Status renders a small tone-colored chip in the stat card's label row.
func (b *Builder) Status(label string, tone StatusTone) *Builder {
	b.spec.Status = &StatusSpec{Label: label, Tone: tone}
	return b
}

// Sparkline renders an inline trend polyline in the stat card's footer row
// using the default accent stroke.
func (b *Builder) Sparkline(values []float64) *Builder {
	b.spec.Sparkline = &SparklineSpec{Values: values}
	return b
}

// SparklineColored is Sparkline with an explicit stroke color.
func (b *Builder) SparklineColored(values []float64, color string) *Builder {
	b.spec.Sparkline = &SparklineSpec{Values: values, Color: color}
	return b
}

// Layout selects a StatGroup's child arrangement (columns or rows).
func (b *Builder) Layout(l GroupLayout) *Builder {
	b.spec.GroupLayout = l
	return b
}

// Presentation sets the panel's wire renderer density hints.
func (b *Builder) Presentation(hints PresentationHints) *Builder {
	b.spec.Presentation = hints
	return b
}
func (b *Builder) Format(spec format.Spec) *Builder { b.spec.Formatter = &spec; return b }
func (b *Builder) Action(spec action.Spec) *Builder { b.spec.Action = &spec; return b }
func (b *Builder) Description(text string) *Builder { b.spec.Description = text; return b }
func (b *Builder) Info(text string) *Builder        { b.spec.Info = text; return b }
func (b *Builder) Export(url string, evidenceDatasets ...string) *Builder {
	b.spec.Export = exportmeta.Spec{Enabled: true, URL: url, EvidenceDatasets: append([]string(nil), evidenceDatasets...)}
	return b
}
func (b *Builder) ClassName(name string) *Builder { b.spec.ClassName = name; return b }
func (b *Builder) ValueAxisScale(scale AxisScale, base int) *Builder {
	b.spec.ValueAxis.Scale = scale
	if base > 1 {
		b.spec.ValueAxis.LogBase = base
	}
	return b
}
func (b *Builder) LogarithmicValueAxis(base int) *Builder {
	return b.ValueAxisScale(AxisScaleLogarithmic, base)
}
func (b *Builder) Icon(icon chrome.Icon) *Builder {
	b.spec.Chrome.Icon = icon
	return b
}
func (b *Builder) AccentColor(color string) *Builder {
	b.spec.Chrome.AccentColor = color
	return b
}
func (b *Builder) DistributedColors() *Builder {
	b.spec.Distributed = true
	return b
}
func (b *Builder) SemanticColors(scale string, field FieldRef) *Builder {
	b.spec.ColorScale = strings.TrimSpace(scale)
	b.spec.ColorField = field
	return b
}
func (b *Builder) Fields(mapping FieldMapping) *Builder {
	b.spec.Fields = mapping
	return b
}
func (b *Builder) LabelField(name FieldRef) *Builder    { b.spec.Fields.Label = name; return b }
func (b *Builder) ValueField(name FieldRef) *Builder    { b.spec.Fields.Value = name; return b }
func (b *Builder) SeriesField(name FieldRef) *Builder   { b.spec.Fields.Series = name; return b }
func (b *Builder) CategoryField(name FieldRef) *Builder { b.spec.Fields.Category = name; return b }
func (b *Builder) IDField(name FieldRef) *Builder       { b.spec.Fields.ID = name; return b }
func (b *Builder) StartField(name FieldRef) *Builder    { b.spec.Fields.StartTime = name; return b }
func (b *Builder) EndField(name FieldRef) *Builder      { b.spec.Fields.EndTime = name; return b }
func (b *Builder) CutField(name FieldRef) *Builder      { b.spec.Fields.Cut = name; return b }
func (b *Builder) CutLabelField(name FieldRef) *Builder { b.spec.Fields.CutLabel = name; return b }
func (b *Builder) FinalField(name FieldRef) *Builder    { b.spec.Fields.Final = name; return b }
func (b *Builder) Columns(columns ...TableColumn) *Builder {
	b.spec.Columns = columns
	return b
}
func (b *Builder) Transforms(specs ...transform.Spec) *Builder {
	b.spec.Transforms = append(b.spec.Transforms, specs...)
	return b
}
func (b *Builder) Children(children ...Spec) *Builder {
	b.spec.Children = append(b.spec.Children, children...)
	return b
}
func (b *Builder) Build() Spec { return b.spec }

func Ref(name string) FieldRef {
	return FieldRef(name)
}
