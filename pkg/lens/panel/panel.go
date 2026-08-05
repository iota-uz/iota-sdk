// Package panel defines Lens dashboard panel specs and builders.
package panel

import (
	"bytes"
	"encoding/json"
	"reflect"
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
	// with deduction connectors between them rather than a chart canvas.
	KindCascade Kind = "cascade"
	KindPie     Kind = "pie"
	KindDonut   Kind = "donut"
	// KindRadial renders concentric circular series. RadialProgress compares
	// each row against an explicit maximum; RadialPartition renders one
	// part-to-whole decomposition per SeriesField ring.
	KindRadial Kind = "radial"
	KindTable  Kind = "table"
	KindGauge  Kind = "gauge"
	// KindHistogram renders already-bucketed distribution rows. Category is
	// the bucket label/bound and Value is its observation count.
	KindHistogram Kind = "histogram"
	// KindBoxPlot renders producer-computed five-number summaries. The SDK does
	// not infer quantiles from raw observations.
	KindBoxPlot Kind = "boxplot"
	// KindHeatmap renders Category × Series cells colored by Value.
	KindHeatmap Kind = "heatmap"
	// KindMap renders a choropleth by joining the panel ID field to one exact
	// string property on a GeoJSON Polygon/MultiPolygon feature collection.
	KindMap    Kind = "map"
	KindTabs   Kind = "tabs"
	KindGrid   Kind = "grid"
	KindSplit  Kind = "split"
	KindRepeat Kind = "repeat"
	// KindStatGroup renders several Stat children inside ONE card, separated
	// by hairline dividers (columns or rows per GroupLayout), instead of one
	// card per KPI. The group is a layout container: it has no dataset of its
	// own; every child is a regular Stat leaf with its own dataset lookup.
	KindStatGroup Kind = "stat_group"
	// KindMetricFlow renders a left-to-right result formula: a sequence of
	// signed operand stages that read to a single supplied result. It covers
	// the "формула" link type and is native HTML/CSS, not a chart. See
	// FlowStage / RelationshipSpec doc comments for the locked contracts:
	//
	//   - Role is formula grammar only. Frame values stay signed business
	//     amounts; add/subtract stages render exactly one normalized operator
	//     (+/−) plus the absolute formatted magnitude, so a negative movement
	//     under subtract never renders as "− −v".
	//   - The result stage's value is server-supplied; the renderer is never
	//     the authoritative calculator.
	KindMetricFlow Kind = "metric_flow"
	// KindMetricHierarchy renders a compact, indented decomposition of one
	// parent metric into child rows with integrity checks (roots, cycles,
	// optional parent<->children reconciliation).
	//
	//   - Parent is authoritative; Depth is always derived during document
	//     build from the Parent chain, never accepted from a builder caller.
	//   - The SDK never manufactures an Unallocated row: it must be an
	//     explicit producer-provided row.
	KindMetricHierarchy Kind = "metric_hierarchy"
	// KindMetricRelationship renders an explicit, non-arithmetic link between
	// two metrics (the "связь" link type) — a neutral association, a
	// derivation, or a reconciliation, with an explicit direction that is
	// never inferred from end order.
	//
	//   - Per-element actions (v1) support literal/variable params only; field
	//     params resolve against the whole panel frame, not a per-element row.
	KindMetricRelationship Kind = "metric_relationship"
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
		KindSegmentBar, KindCascade, KindPie, KindDonut, KindRadial, KindTable, KindGauge,
		KindHistogram, KindBoxPlot, KindHeatmap, KindMap,
		KindMetricFlow, KindMetricHierarchy, KindMetricRelationship:
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

// TargetSpec renders a target/threshold marker on a coverage (segment-bar),
// horizontal-bar, or time-series panel. Value is in the same units as the
// panel's plotted amounts; the renderer never rescales it.
type TargetSpec struct {
	Value float64 `json:"value"`
	Label string  `json:"label,omitempty"`
}

// MovingAverageSpec names a server-computed SMA field exposed as an optional
// time-series overlay. Window is constrained to the reader-facing choices
// supported by Lens (3, 7, or 12 points).
type MovingAverageSpec struct {
	Window int
	Field  FieldRef
	Label  string
}

type TemporalPeriodState string

const (
	TemporalPeriodYTD        TemporalPeriodState = "ytd"
	TemporalPeriodAnnualized TemporalPeriodState = "annualized"
)

// TemporalPeriodSpec marks one category as incomplete. AnnualizedField may
// supply a separately computed estimate; it never silently replaces the fact.
type TemporalPeriodSpec struct {
	Category        string
	State           TemporalPeriodState
	Label           string
	AnnualizedField FieldRef
}

// TimeAnnotationSpec is a vertical time-axis event marker.
type TimeAnnotationSpec struct {
	At    string
	Label string
}

// ForecastSpec names forecast and confidence-bound fields already present on
// the frame. Start is the first forecast category and visually separates fact
// from projection.
type ForecastSpec struct {
	Start      string
	ValueField FieldRef
	LowerField FieldRef
	UpperField FieldRef
	Label      string
}

// TemporalSpec is the generic time-series overlay contract. Computed fields
// stay in the frame so CSV/report exports contain the same values the chart
// draws.
type TemporalSpec struct {
	RegressionField FieldRef
	RegressionLabel string
	MovingAverages  []MovingAverageSpec
	ReferenceLines  []TargetSpec
	Period          *TemporalPeriodSpec
	Annotations     []TimeAnnotationSpec
	Forecast        *ForecastSpec
}

// Confidence states how a displayed number was produced (mirrors
// document.Confidence). It is orthogonal to Availability: Confidence
// qualifies a number that exists, Availability says whether a number can
// exist at all. An empty Confidence is unset (no confidence chip).
type Confidence string

const (
	ConfidenceVerified               Confidence = "verified"
	ConfidenceCalculated             Confidence = "calculated"
	ConfidenceProxy                  Confidence = "proxy"
	ConfidenceRequiresReconciliation Confidence = "requires_reconciliation"
)

// Availability states whether a number can exist for an element, independent
// of how trustworthy it would be (mirrors document.Availability). An empty
// Availability is unset and treated as available.
type Availability string

const (
	AvailabilityAvailable      Availability = "available"
	AvailabilityConfigRequired Availability = "config_required"
	AvailabilityEmptySource    Availability = "empty_source"
	AvailabilityUnavailable    Availability = "unavailable"
)

// FlowStageRole is the formula grammar of a MetricFlow stage (mirrors
// document.MetricFlowStageRole). It governs the single normalized operator
// glyph the renderer prints for the stage and nothing else: frame values stay
// signed business amounts, and the renderer is never the authoritative
// calculator. An add/subtract stage renders exactly one operator plus the
// absolute formatted magnitude, so a negative movement under subtract reads
// as "+ |v|", never "− −v". The formula reads left-to-right to exactly one
// result stage, which carries an independently supplied server value.
type FlowStageRole string

const (
	FlowRoleInput        FlowStageRole = "input"
	FlowRoleAdd          FlowStageRole = "add"
	FlowRoleSubtract     FlowStageRole = "subtract"
	FlowRoleIntermediate FlowStageRole = "intermediate"
	FlowRoleResult       FlowStageRole = "result"
)

// FlowStage is one operand of a MetricFlow result formula (mirrors
// document.MetricFlowStage). Role is formula grammar only; the stage's
// numeric value is joined from the panel frame by Key and stays a signed
// business amount.
type FlowStage struct {
	Key   string
	Label string
	Role  FlowStageRole
	// Caption is one muted secondary line rendered under the stage value.
	Caption      string
	Confidence   Confidence
	Availability Availability
	// Action, when set, makes the stage an actionable link/button. v1 supports
	// literal/variable params only; field params resolve against the whole
	// panel frame, not the stage's own row.
	Action *action.Spec
}

// FlowReconciliation opts a MetricFlow into a compact mismatch note (mirrors
// document.FlowReconciliation): the renderer compares the displayed operands
// against the supplied result and, when the difference exceeds Tolerance,
// shows a note. It never replaces the result.
type FlowReconciliation struct {
	Tolerance float64
}

// HierarchyRow is one node of a MetricHierarchy decomposition (mirrors
// document.MetricHierarchyRow). Parent is authoritative: depth is always
// derived from the Parent chain during document build, never accepted here.
type HierarchyRow struct {
	Key         string
	Label       string
	Description string
	// Parent names the Key of this row's parent; empty marks a root row.
	Parent string
	// Unallocated marks an explicit producer-provided «Не распределено» child.
	// The SDK never manufactures one; at most one per parent.
	Unallocated bool
	// Selected marks the single row rendered as aria-current; at most one row.
	Selected     bool
	Confidence   Confidence
	Availability Availability
	Action       *action.Spec
}

// HierarchyReconciliation opts a MetricHierarchy into per-parent integrity
// checks (mirrors document.HierarchyReconciliation): for each parent the
// renderer compares SUM(direct children incl. unallocated) against the
// parent within Tolerance and shows a compact summary.
type HierarchyReconciliation struct {
	Tolerance float64
}

// RelationshipType selects the kind of non-arithmetic link a
// MetricRelationship panel states between its two ends (mirrors
// document.MetricRelationshipType).
type RelationshipType string

const (
	RelationshipAssociation    RelationshipType = "association"
	RelationshipDerivation     RelationshipType = "derivation"
	RelationshipReconciliation RelationshipType = "reconciliation"
)

// RelationshipDirection is the explicit direction of a MetricRelationship
// (mirrors document.MetricRelationshipDirection). It is never inferred from
// the order of the ends. An empty direction defaults to bidirectional.
type RelationshipDirection string

const (
	RelationshipBidirectional  RelationshipDirection = "bidirectional"
	RelationshipSourceToTarget RelationshipDirection = "source_to_target"
	RelationshipTargetToSource RelationshipDirection = "target_to_source"
)

// RelationshipEnd is one side of a MetricRelationship (mirrors
// document.MetricRelationshipEnd). Its numeric value is joined from the panel
// frame by Key via the encoding.
type RelationshipEnd struct {
	Key          string
	Label        string
	Confidence   Confidence
	Availability Availability
	Action       *action.Spec
}

// RelationshipSpec is the structure of a metric_relationship panel (mirrors
// document.MetricRelationshipConfig): two ends, a link type, an explicit
// direction, and an optional note.
type RelationshipSpec struct {
	Source    RelationshipEnd
	Target    RelationshipEnd
	Type      RelationshipType
	Direction RelationshipDirection
	Note      string
}

// PresentationHints are optional, renderer-level density choices carried on a
// panel spec. Every field is opt-in: the zero value leaves policy to the
// renderer's neutral defaults.
type PresentationHints struct {
	// DataLabels writes formatted values directly on bar and line marks. It is
	// intentionally opt-in: dense charts keep tooltips as their reading surface.
	DataLabels bool
	// LegendBelow renders a centered wrapping legend under the plot with one
	// "label · value" entry per slice.
	LegendBelow bool
	// SliceLabelsPercent writes each partition slice's share inside the slice.
	SliceLabelsPercent bool
	// SliceLabelsCategory writes each partition slice's category label inside
	// the slice instead of its share. Only for dimensions whose labels are
	// inherently short — a year, a quarter; a product name would be clipped
	// away. Pair with LegendPercent so the share still has somewhere to live.
	SliceLabelsCategory bool
	// LegendPercent prints each legend entry's share of the frame total
	// instead of its formatted value.
	LegendPercent bool
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
	// ColorBySequence shades an ordered dimension along one hue. Use it where
	// the categories are ranked — age bands, tenure buckets, rating grades —
	// and a categorical palette would claim they are unrelated.
	ColorBySequence bool
	// ColorByRank shades numeric map values by their order rather than their
	// distance. This is useful for highly skewed choropleths, but changes the
	// meaning of the colour ramp and therefore requires producer opt-in.
	ColorByRank bool
	// ValueSpreadThreshold is the minimum max/min ratio at which the renderer
	// treats a requested logarithmic scale as material and identifies a linear
	// scale as obscuring small values. Zero leaves this policy unset; when set,
	// the value must be greater than one.
	ValueSpreadThreshold float64
	// HideTotalBadge suppresses the total badge, e.g. when a trend chip
	// already carries the panel's summary.
	HideTotalBadge bool
	// Waterfall renders a Cascade as a conventional vertical waterfall:
	// opening total, floating signed movements, and closing total connected
	// at their running-balance levels. The Cascade data contract remains
	// unchanged; this is only a visual projection.
	Waterfall bool
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
	// RowGroupField names a frame column carrying a per-row group tag on a table
	// panel, enabling collapsible member rows behind a synthetic toggle row.
	RowGroupField string
	// LineSeries names series a stacked bar draws as a line over the columns.
	// A measure on another basis — earned against written, actual against plan
	// — reads beside the stack, never as one of its parts.
	LineSeries []string
	// FocusCanvas opts a drill-hosting panel into focus chrome in the wire
	// runtime: a persistent breadcrumb, a parent-context header, and a
	// standalone lens selector around the drilled visualization. Non-focus
	// panels render exactly as before.
	FocusCanvas bool
}

// IsZero reports whether no hint is set. LineSeries makes PresentationHints
// uncomparable, so callers ask this instead of comparing against a zero value.
func (h PresentationHints) IsZero() bool {
	lineSeries := h.LineSeries
	h.LineSeries = nil
	return len(lineSeries) == 0 && reflect.DeepEqual(h, PresentationHints{})
}

// RadialMode selects the geometry of a radial panel.
type RadialMode string

const (
	// RadialProgress renders one concentric progress arc per frame row.
	RadialProgress RadialMode = "progress"
	// RadialPartition renders one concentric part-to-whole ring per distinct
	// SeriesField value. Stable IDField values align category identity and
	// color across rings.
	RadialPartition RadialMode = "partition"
)

// RadialSpec configures a radial panel. Max is required only for progress
// mode and is expressed in the same units as ValueField.
type RadialSpec struct {
	Mode      RadialMode   `json:"mode"`
	Max       float64      `json:"max,omitempty"`
	Rings     []RadialRing `json:"rings,omitempty"`
	Tolerance float64      `json:"tolerance,omitempty"`
}

// RadialRing declares one partition ring. Key is stored in SeriesField,
// Label is presentation text, Order controls outside-to-inside placement, and
// Total is the authoritative whole against which row values reconcile.
type RadialRing struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Order int     `json:"order,omitempty"`
	Total float64 `json:"total"`
}

// MaxMapGeoJSONBytes is the absolute client/server contract limit for a
// fetched map source. Producers may request a lower MaxBytes but never a
// higher one. Inline sources are already bounded by the dashboard response.
const MaxMapGeoJSONBytes = 5 * 1024 * 1024

// GeoJSONFeatureCollection is the deliberately narrow geometry accepted by a
// choropleth panel. Validation restricts every feature to Polygon or
// MultiPolygon; the generic map preserves coordinates as JSON-shaped data.
type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   map[string]any `json:"geometry"`
}

// GeoJSONSource selects exactly one source. URL must be a same-origin absolute
// path and carries a positive MaxBytes no larger than MaxMapGeoJSONBytes.
type GeoJSONSource struct {
	Inline   *GeoJSONFeatureCollection `json:"inline,omitempty"`
	URL      string                    `json:"url,omitempty"`
	MaxBytes int                       `json:"maxBytes,omitempty"`
}

// MapSpec configures an exact, case-sensitive one-to-one region join.
// FeatureProperty is compared to the panel's ID field; LabelProperty, when
// set, supplies human-readable region labels from feature properties.
type MapSpec struct {
	Source          GeoJSONSource     `json:"source"`
	FeatureProperty string            `json:"featureProperty"`
	LabelProperty   string            `json:"labelProperty,omitempty"`
	LabelProperties map[string]string `json:"labelProperties,omitempty"`
	Attribution     string            `json:"attribution,omitempty"`
}

// RadialNodeKey returns the stable mark key emitted for a partition segment.
// It includes both ring and category identity so the same category can carry a
// different drill action in each decomposition.
// The runtime builds the same key with JSON.stringify, which leaves `<`, `>`
// and `&` alone; encoding/json escapes them to < and friends unless told
// not to, and a category named "A&B" would then carry two different identities
// on the two sides of the wire.
func RadialNodeKey(ringKey, categoryKey string) string {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode([2]string{ringKey, categoryKey}); err != nil {
		panic(err)
	}
	return "radial:" + strings.TrimSuffix(buffer.String(), "\n")
}

// GroupLayout selects how a StatGroup panel arranges its children inside the
// shared card: side-by-side columns with vertical hairlines, or a vertical
// list with horizontal hairlines.
type GroupLayout string

const (
	GroupColumns GroupLayout = "columns"
	GroupRows    GroupLayout = "rows"
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
	// "pill" renders a compact pill with a drill arrow, "quiet" makes the whole
	// cell the drill target with hover-only chrome.
	Affordance string
	// ToneField names a frame column carrying a per-row status tone ("pos",
	// "warn", "neg") applied to the cell value's color.
	ToneField FieldRef
	// BadgeField names a frame column carrying a per-row badge tooltip; a
	// non-empty value renders a muted "?" badge after the cell value.
	BadgeField FieldRef
	// Heat shades numeric cells by their value relative to the visible column
	// range. It is opt-in so ordinary numeric columns remain visually neutral.
	Heat bool
	// SampleSizeField names the numeric frame column carrying the observation
	// count behind this value. Values below MinSampleSize remain visible but are
	// visually qualified as a small sample.
	SampleSizeField FieldRef
	MinSampleSize   int
	// Total includes this numeric column in the server-computed footer.
	Total bool
	// ShareOf names the numeric denominator field used to recompute this
	// percentage column after server-side search/filtering.
	ShareOf FieldRef
}

// Pill marks an actionable column's cells as compact drill pills.
func (c TableColumn) Pill() TableColumn {
	c.Affordance = "pill"
	return c
}

// Quiet makes an actionable column's whole cell the drill target with no
// standing chrome; the drill arrow and value underline appear only on hover.
func (c TableColumn) Quiet() TableColumn {
	c.Affordance = "quiet"
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

// Heatmap opts a numeric column into a value-intensity background.
func (c TableColumn) Heatmap() TableColumn {
	c.Heat = true
	return c
}

// LowSample qualifies values backed by fewer than minimum observations.
func (c TableColumn) LowSample(sampleSizeField FieldRef, minimum int) TableColumn {
	c.SampleSizeField = sampleSizeField
	c.MinSampleSize = minimum
	return c
}

// WithTotal includes this numeric column in the server-computed total row.
func (c TableColumn) WithTotal() TableColumn {
	c.Total = true
	return c
}

// AsShareOf computes this percentage column from sourceField after filtering.
func (c TableColumn) AsShareOf(sourceField FieldRef) TableColumn {
	c.ShareOf = sourceField
	return c
}

// TableOptions configures server-backed table interactions.
type TableOptions struct {
	Searchable bool
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
	DefaultLabelField        FieldRef = "label"
	DefaultValueField        FieldRef = "value"
	DefaultSeriesField       FieldRef = "series"
	DefaultCategoryField     FieldRef = "category"
	DefaultIDField           FieldRef = "id"
	DefaultCutField          FieldRef = "cut"
	DefaultCutLabelField     FieldRef = "cutLabel"
	DefaultFinalField        FieldRef = "final"
	DefaultShareField        FieldRef = "share"
	DefaultConfidenceField   FieldRef = "confidence"
	DefaultAvailabilityField FieldRef = "availability"
)

func (f FieldRef) Name() string {
	return string(f)
}

func (f FieldRef) Empty() bool {
	return strings.TrimSpace(f.Name()) == ""
}

type Spec struct {
	ID          string
	Title       string
	Description string
	Info        string
	Kind        Kind
	Dataset     string
	Span        int
	Colors      []string
	ShowLegend  bool
	// TotalBadgeValue, when set, renders the total badge with this
	// server-computed value instead of summing the plotted data points
	// client-side. Required for panels whose plotted series are not the raw
	// amounts (e.g. log-transformed values with an epsilon floor). The badge
	// then stays constant across legend toggles — it is the period total.
	TotalBadgeValue *float64
	// HeadlineValue overrides the computed headline without changing the
	// values used for chart geometry. SegmentBar uses it for a focal result
	// whose allocation segments still sum to a different denominator.
	HeadlineValue *float64
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
	// Target renders a target/threshold marker on a SegmentBar, HorizontalBar,
	// or TimeSeries panel. TimeSeries renders it as a horizontal reference line.
	Target *TargetSpec
	// Temporal carries optional overlays for a time-series panel.
	Temporal *TemporalSpec
	// GroupLayout arranges a StatGroup's children ("columns" default, or
	// "rows"). Ignored on every other kind.
	GroupLayout GroupLayout
	// Presentation carries opt-in rendering hints for the document runtime so
	// a dashboard can ask for a denser treatment without a bespoke panel kind.
	Presentation PresentationHints
	Fields       FieldMapping
	Formatter    *format.Spec
	Columns      []TableColumn
	Table        *TableOptions
	Transforms   []transform.Spec
	Action       *action.Spec
	// Terminal explicitly marks a panel whose displayed value is the end of
	// the interaction path. It is mutually exclusive with Action and drill.
	Terminal bool
	// ComparisonUnsupported marks a panel whose declared comparison cannot be
	// represented by its chart kind. Renderers localize the explanatory notice.
	ComparisonUnsupported bool
	Children              []Spec
	Chrome                chrome.Spec
	ValueAxis             ValueAxis
	Distributed           bool
	Export                exportmeta.Spec
	// FlowStages, when set (KindMetricFlow), declares the panel's ordered
	// operand stages.
	FlowStages []FlowStage
	// FlowReconcile opts a MetricFlow panel into a tolerance-based mismatch
	// note between the displayed operands and the supplied result.
	FlowReconcile *FlowReconciliation
	// HierarchyRows, when set (KindMetricHierarchy), declares the panel's
	// flat row list, linked by HierarchyRow.Parent.
	HierarchyRows []HierarchyRow
	// HierarchyReconcile opts a MetricHierarchy panel into per-parent
	// integrity checks against its children.
	HierarchyReconcile *HierarchyReconciliation
	// Relationship, when set (KindMetricRelationship), declares the panel's
	// two ends, link type, and direction.
	Relationship *RelationshipSpec
	// Radial, when set (KindRadial), selects progress or partition geometry.
	Radial *RadialSpec
	// Map, when set (KindMap), declares the bounded geometry source and exact
	// region-key join.
	Map *MapSpec
	// Confidence is the panel-level default confidence for its elements; a
	// frame column value or an element's own confidence overrides it.
	Confidence Confidence
	// Availability is the panel-level default availability for its elements;
	// a frame column value or an element's own availability overrides it.
	Availability Availability
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
	TriggerKey string      `json:"triggerKey"`
	Label      string      `json:"label"`
	Children   []DrillNode `json:"children"`
}

// DrillNode is one stable item in a DrillTree detail level. Navigate,
// HtmxSwap, and EmitEvent actions are supported on leaves; actions that depend
// on an unresolved dataset row are not supported.
type DrillNode struct {
	Key      string       `json:"key"`
	Label    string       `json:"label"`
	Value    float64      `json:"value"`
	Color    string       `json:"color,omitempty"`
	Action   *action.Spec `json:"action,omitempty"`
	Children []DrillNode  `json:"children,omitempty"`
}

// TrendPolarity says whether a movement in this metric is good news, bad news,
// or neither. It is the producer's judgement, not the renderer's: growth in
// premium earned is good, growth in loss ratio is bad, and growth in sum
// insured is exposure — more of a thing, with no verdict attached.
//
// Unset means TrendNeutral. A tri-state exists because Invert could only ever
// say "up is bad instead of good"; it had no way to say "no opinion", so every
// metric nobody had thought about was coloured as though higher were better.
type TrendPolarity string

const (
	// TrendNeutral prints the movement without a good/bad colour. The arrow and
	// the sign still say which way it went.
	TrendNeutral TrendPolarity = "neutral"
	// TrendHigherBetter colours a rise positive and a fall negative.
	TrendHigherBetter TrendPolarity = "higher_better"
	// TrendLowerBetter colours a fall positive and a rise negative.
	TrendLowerBetter TrendPolarity = "lower_better"
)

// TrendSpec renders a small colored chip in a panel's header showing a signed
// percent change alongside a comparison label (e.g. "vs last month").
type TrendSpec struct {
	Percent float64 `json:"percent"`
	Label   string  `json:"label,omitempty"`
	// Polarity decides the chip's colour. Unset reads as TrendNeutral, except
	// when Invert is set — see below.
	Polarity TrendPolarity `json:"polarity,omitempty"`
	// Invert is the legacy two-state form of Polarity, kept because producers
	// already declare it: true means TrendLowerBetter. It cannot express
	// "higher is better" distinctly from "no opinion", so new producers should
	// set Polarity instead.
	Invert bool `json:"invert,omitempty"`
	// AbsoluteField and PercentField opt into frame-backed automatic deltas.
	// When set they take precedence over the manual Percent value.
	AbsoluteField     FieldRef       `json:"-"`
	PercentField      FieldRef       `json:"-"`
	AbsoluteDeltaUnit TrendDeltaUnit `json:"-"`
}

type TrendDeltaUnit string

const (
	TrendDeltaValue            TrendDeltaUnit = "value"
	TrendDeltaPercentagePoints TrendDeltaUnit = "percentage_points"
)

type FieldMapping struct {
	Label     FieldRef
	Value     FieldRef
	Previous  FieldRef
	Lower     FieldRef
	Q1        FieldRef
	Median    FieldRef
	Q3        FieldRef
	Upper     FieldRef
	Series    FieldRef
	Category  FieldRef
	ID        FieldRef
	StartTime FieldRef
	EndTime   FieldRef
	Cut       FieldRef
	CutLabel  FieldRef
	Final     FieldRef
	// Annotation names a frame column carrying an optional short, already
	// formatted note for an individual cascade stage. Renderers show it as a
	// compact badge beside/below the stage label; empty values render nothing.
	Annotation FieldRef
	// Tone names a frame column carrying a per-row semantic tone for a cascade
	// panel's stages ("neutral", "positive", "negative", "inflow"). It overrides
	// the flow-direction default color. Empty (the default) keeps direction-based
	// coloring; it is opt-in, so it is never defaulted to a column name.
	Tone FieldRef
	// Split names a frame column carrying the part of this stage's own movement
	// that is materially different in kind from the rest of it — claims paid out
	// of a product's own reserve versus the overflow beyond it, say. The renderer
	// draws it as a distinct band at the leading end of the same bar, so the
	// composition is read off the bar instead of a caption beside it. It is a
	// portion OF the movement, never an addition to it: a value outside
	// (0, |movement|) is ignored and the bar stays whole.
	Split FieldRef
	// SplitLabel names a frame column naming that band. Renderers show it next
	// to the band; empty values leave the band unlabeled but still drawn.
	SplitLabel FieldRef
	// Share names a frame column carrying a per-element share; the renderer
	// never recomputes it.
	Share FieldRef
	// Confidence names a frame column carrying a per-element Confidence value
	// that overrides the structural default.
	Confidence FieldRef
	// Availability names a frame column carrying a per-element Availability
	// value that overrides the structural default.
	Availability FieldRef
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
func RadialBars(id, title, dataset string, maximum float64) *Builder {
	b := newBuilder(KindRadial, id, title, dataset)
	b.spec.Radial = &RadialSpec{Mode: RadialProgress, Max: maximum}
	b.spec.Presentation.LegendBelow = true
	return b
}
func MultiRingDonut(id, title, dataset string, rings ...RadialRing) *Builder {
	b := newBuilder(KindRadial, id, title, dataset)
	b.spec.Radial = &RadialSpec{Mode: RadialPartition, Rings: append([]RadialRing(nil), rings...)}
	b.spec.Presentation.LegendBelow = true
	b.spec.Presentation.SliceLabelsPercent = true
	return b
}
func Table(id, title, dataset string) *Builder { return newBuilder(KindTable, id, title, dataset) }
func Gauge(id, title, dataset string) *Builder { return newBuilder(KindGauge, id, title, dataset) }
func Histogram(id, title, dataset string) *Builder {
	return newBuilder(KindHistogram, id, title, dataset)
}
func BoxPlot(id, title, dataset string) *Builder {
	builder := newBuilder(KindBoxPlot, id, title, dataset)
	builder.spec.Fields.Value = ""
	return builder
}
func Heatmap(id, title, dataset string) *Builder { return newBuilder(KindHeatmap, id, title, dataset) }
func Choropleth(id, title, dataset string, source GeoJSONSource, featureProperty string) *Builder {
	builder := newBuilder(KindMap, id, title, dataset)
	builder.spec.Map = &MapSpec{Source: source, FeatureProperty: featureProperty}
	return builder
}

// MetricFlow builds a result-formula panel: an ordered sequence of signed
// operand stages reading left-to-right to a single supplied result. See
// KindMetricFlow for the locked sign contract.
func MetricFlow(id, title, dataset string, stages ...FlowStage) *Builder {
	b := newBuilder(KindMetricFlow, id, title, dataset)
	b.spec.Span = 12
	b.spec.FlowStages = stages
	return b
}

// MetricHierarchy builds a compact decomposition of one parent metric into
// child rows linked by HierarchyRow.Parent. Depth is always derived at
// document build time, never accepted from the caller.
func MetricHierarchy(id, title, dataset string, rows ...HierarchyRow) *Builder {
	b := newBuilder(KindMetricHierarchy, id, title, dataset)
	b.spec.Span = 6
	b.spec.HierarchyRows = rows
	return b
}

// MetricRelationship builds an explicit, non-arithmetic link between two
// metrics: a neutral association, a derivation, or a reconciliation.
func MetricRelationship(id, title, dataset string, rel RelationshipSpec) *Builder {
	b := newBuilder(KindMetricRelationship, id, title, dataset)
	b.spec.Span = 6
	b.spec.Relationship = &rel
	return b
}

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
				Label:        DefaultLabelField,
				Value:        DefaultValueField,
				Series:       DefaultSeriesField,
				Category:     DefaultCategoryField,
				ID:           DefaultIDField,
				Cut:          DefaultCutField,
				CutLabel:     DefaultCutLabelField,
				Final:        DefaultFinalField,
				Share:        DefaultShareField,
				Confidence:   DefaultConfidenceField,
				Availability: DefaultAvailabilityField,
			},
		},
	}
}

func (b *Builder) Span(span int) *Builder           { b.spec.Span = span; return b }
func (b *Builder) Colors(colors ...string) *Builder { b.spec.Colors = colors; return b }
func (b *Builder) Legend() *Builder                 { b.spec.ShowLegend = true; return b }
func (b *Builder) TotalBadgeValue(v float64) *Builder {
	b.spec.TotalBadgeValue = &v
	return b
}
func (b *Builder) HeadlineValue(v float64) *Builder {
	b.spec.HeadlineValue = &v
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

// AutoTrend renders absolute and percent deltas from the panel frame.
func (b *Builder) AutoTrend(absoluteField, percentField FieldRef, label string, invert bool) *Builder {
	b.spec.Trend = &TrendSpec{
		Label: label, Invert: invert, AbsoluteField: absoluteField, PercentField: percentField,
	}
	return b
}

// TrendPolarity declares whether a rise in this metric is good, bad or neither.
// Without it a chip is neutral: it shows the direction and withholds the verdict.
func (b *Builder) TrendPolarity(polarity TrendPolarity) *Builder {
	if b.spec.Trend != nil {
		b.spec.Trend.Polarity = polarity
	}
	return b
}

func (b *Builder) TrendAbsoluteDeltaUnit(unit TrendDeltaUnit) *Builder {
	if b.spec.Trend != nil {
		b.spec.Trend.AbsoluteDeltaUnit = unit
	}
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

// Target renders a target/threshold marker: a bullet tick on SegmentBar and
// HorizontalBar, or a labelled horizontal line on TimeSeries.
func (b *Builder) Target(value float64, label string) *Builder {
	b.spec.Target = &TargetSpec{Value: value, Label: label}
	return b
}

// Regression exposes a server-computed least-squares field behind a reader
// toggle in the panel header.
func (b *Builder) Regression(field FieldRef, label string) *Builder {
	if b.spec.Temporal == nil {
		b.spec.Temporal = &TemporalSpec{}
	}
	b.spec.Temporal.RegressionField = field
	b.spec.Temporal.RegressionLabel = label
	return b
}

// MovingAverage exposes a server-computed SMA field for window 3, 7, or 12.
func (b *Builder) MovingAverage(window int, field FieldRef, label string) *Builder {
	if b.spec.Temporal == nil {
		b.spec.Temporal = &TemporalSpec{}
	}
	b.spec.Temporal.MovingAverages = append(b.spec.Temporal.MovingAverages, MovingAverageSpec{Window: window, Field: field, Label: label})
	return b
}

// ReferenceLine adds a labelled horizontal threshold to a time series.
func (b *Builder) ReferenceLine(value float64, label string) *Builder {
	if b.spec.Temporal == nil {
		b.spec.Temporal = &TemporalSpec{}
	}
	b.spec.Temporal.ReferenceLines = append(b.spec.Temporal.ReferenceLines, TargetSpec{Value: value, Label: label})
	return b
}

// IncompletePeriod marks a YTD or explicitly annualized category.
func (b *Builder) IncompletePeriod(category string, state TemporalPeriodState, label string, annualizedField FieldRef) *Builder {
	if b.spec.Temporal == nil {
		b.spec.Temporal = &TemporalSpec{}
	}
	b.spec.Temporal.Period = &TemporalPeriodSpec{Category: category, State: state, Label: label, AnnualizedField: annualizedField}
	return b
}

// AnnotateTime adds a labelled event marker to the category axis.
func (b *Builder) AnnotateTime(at, label string) *Builder {
	if b.spec.Temporal == nil {
		b.spec.Temporal = &TemporalSpec{}
	}
	b.spec.Temporal.Annotations = append(b.spec.Temporal.Annotations, TimeAnnotationSpec{At: at, Label: label})
	return b
}

// Forecast exposes projected values and their confidence interval.
func (b *Builder) Forecast(start string, valueField, lowerField, upperField FieldRef, label string) *Builder {
	if b.spec.Temporal == nil {
		b.spec.Temporal = &TemporalSpec{}
	}
	b.spec.Temporal.Forecast = &ForecastSpec{
		Start: start, ValueField: valueField, LowerField: lowerField, UpperField: upperField, Label: label,
	}
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

// Waterfall renders this Cascade as vertical total/delta columns instead of
// narrowing horizontal running-total rows.
func (b *Builder) Waterfall() *Builder {
	b.spec.Presentation.Waterfall = true
	return b
}

// FocusCanvas opts this drill-hosting panel into focus chrome in the wire
// runtime (breadcrumb, parent-context header, standalone lens selector).
func (b *Builder) FocusCanvas() *Builder {
	b.spec.Presentation.FocusCanvas = true
	return b
}
func (b *Builder) Format(spec format.Spec) *Builder { b.spec.Formatter = &spec; return b }
func (b *Builder) Action(spec action.Spec) *Builder { b.spec.Action = &spec; return b }
func (b *Builder) Terminal() *Builder               { b.spec.Terminal = true; return b }
func (b *Builder) ComparisonUnsupported() *Builder {
	b.spec.ComparisonUnsupported = true
	return b
}
func (b *Builder) Description(text string) *Builder { b.spec.Description = text; return b }
func (b *Builder) Info(text string) *Builder        { b.spec.Info = text; return b }
func (b *Builder) Export(url string, evidenceDatasets ...string) *Builder {
	b.spec.Export = exportmeta.Spec{Enabled: true, URL: url, EvidenceDatasets: append([]string(nil), evidenceDatasets...)}
	return b
}
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
func (b *Builder) Fields(mapping FieldMapping) *Builder {
	b.spec.Fields = mapping
	return b
}
func (b *Builder) LabelField(name FieldRef) *Builder { b.spec.Fields.Label = name; return b }
func (b *Builder) ValueField(name FieldRef) *Builder { b.spec.Fields.Value = name; return b }
func (b *Builder) BoxFields(lower, q1, median, q3, upper FieldRef) *Builder {
	b.spec.Fields.Lower = lower
	b.spec.Fields.Q1 = q1
	b.spec.Fields.Median = median
	b.spec.Fields.Q3 = q3
	b.spec.Fields.Upper = upper
	return b
}
func (b *Builder) SeriesField(name FieldRef) *Builder   { b.spec.Fields.Series = name; return b }
func (b *Builder) CategoryField(name FieldRef) *Builder { b.spec.Fields.Category = name; return b }
func (b *Builder) IDField(name FieldRef) *Builder       { b.spec.Fields.ID = name; return b }
func (b *Builder) StartField(name FieldRef) *Builder    { b.spec.Fields.StartTime = name; return b }
func (b *Builder) EndField(name FieldRef) *Builder      { b.spec.Fields.EndTime = name; return b }
func (b *Builder) CutField(name FieldRef) *Builder      { b.spec.Fields.Cut = name; return b }
func (b *Builder) CutLabelField(name FieldRef) *Builder { b.spec.Fields.CutLabel = name; return b }
func (b *Builder) FinalField(name FieldRef) *Builder    { b.spec.Fields.Final = name; return b }
func (b *Builder) AnnotationField(name FieldRef) *Builder {
	b.spec.Fields.Annotation = name
	return b
}

// ToneField declares the frame column carrying a per-row semantic tone for a
// cascade panel's stages ("neutral", "positive", "negative", "inflow"). It
// overrides the flow-direction default color; absent keeps direction coloring.
func (b *Builder) ToneField(name FieldRef) *Builder { b.spec.Fields.Tone = name; return b }

// SplitField declares the frame column carrying the part of a cascade stage's
// own movement that differs in kind from the rest of it; the renderer bands it
// separately inside the same bar.
func (b *Builder) SplitField(name FieldRef) *Builder { b.spec.Fields.Split = name; return b }

// SplitLabelField declares the frame column naming that band.
func (b *Builder) SplitLabelField(name FieldRef) *Builder {
	b.spec.Fields.SplitLabel = name
	return b
}

func (b *Builder) ShareField(name FieldRef) *Builder { b.spec.Fields.Share = name; return b }
func (b *Builder) ConfidenceField(name FieldRef) *Builder {
	b.spec.Fields.Confidence = name
	return b
}
func (b *Builder) AvailabilityField(name FieldRef) *Builder {
	b.spec.Fields.Availability = name
	return b
}

// Confidence sets the panel-level default confidence for its elements.
func (b *Builder) Confidence(confidence Confidence) *Builder {
	b.spec.Confidence = confidence
	return b
}

// Availability sets the panel-level default availability for its elements.
func (b *Builder) Availability(availability Availability) *Builder {
	b.spec.Availability = availability
	return b
}

// RadialTolerance permits small source-rounding differences when reconciling
// partition rows to each ring's declared total.
func (b *Builder) RadialTolerance(tolerance float64) *Builder {
	if b.spec.Radial != nil {
		b.spec.Radial.Tolerance = tolerance
	}
	return b
}

// MapLabelProperty chooses the human-readable GeoJSON feature property while
// the exact identity join remains FeatureProperty ↔ IDField.
func (b *Builder) MapLabelProperty(name string) *Builder {
	if b.spec.Map != nil {
		b.spec.Map.LabelProperty = strings.TrimSpace(name)
	}
	return b
}

// MapLabelProperties maps document locales to localized GeoJSON feature
// properties. The runtime falls back to MapLabelProperty for an unknown locale.
func (b *Builder) MapLabelProperties(properties map[string]string) *Builder {
	if b.spec.Map != nil {
		b.spec.Map.LabelProperties = cloneStringMap(properties)
	}
	return b
}

// MapAttribution declares the human-readable source or licence credit shown
// with the map. The renderer must display it whenever it is non-empty.
func (b *Builder) MapAttribution(attribution string) *Builder {
	if b.spec.Map != nil {
		b.spec.Map.Attribution = strings.TrimSpace(attribution)
	}
	return b
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

// FlowReconcile opts a MetricFlow panel into a tolerance-based mismatch note
// between the displayed operands and the supplied result.
func (b *Builder) FlowReconcile(tolerance float64) *Builder {
	b.spec.FlowReconcile = &FlowReconciliation{Tolerance: tolerance}
	return b
}

// HierarchyReconcile opts a MetricHierarchy panel into per-parent integrity
// checks: SUM(direct children incl. unallocated) compared against the parent
// within tolerance.
func (b *Builder) HierarchyReconcile(tolerance float64) *Builder {
	b.spec.HierarchyReconcile = &HierarchyReconciliation{Tolerance: tolerance}
	return b
}

func (b *Builder) Columns(columns ...TableColumn) *Builder {
	b.spec.Columns = columns
	return b
}

// Searchable enables server-side substring search for a table panel.
func (b *Builder) Searchable() *Builder {
	b.spec.Table = &TableOptions{Searchable: true}
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
