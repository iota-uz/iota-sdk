package document

import (
	"encoding/json"
	"time"
)

type FrameRef string

type DashboardDocument struct {
	Version      string             `json:"version"`
	SnapshotID   string             `json:"snapshotId"`
	Meta         Meta               `json:"meta"`
	Layout       Layout             `json:"layout"`
	Panels       []Panel            `json:"panels"`
	Frames       map[FrameRef]Frame `json:"frames"`
	Drill        Drill              `json:"drill"`
	Perspectives []Perspective      `json:"perspectives"`
	// Filters declares the dashboard's controls. Optional so documents from
	// older producers keep parsing under the same contract major.
	Filters   []Filter          `json:"filters,omitempty"`
	Endpoints Endpoints         `json:"endpoints"`
	I18n      map[string]string `json:"i18n"`
	Theme     Theme             `json:"theme"`
	// Header, when set, renders a document-level identity block (title + a
	// muted subtitle) on the left of the dashboard's action bar. Optional so
	// documents from older producers keep parsing under the same contract major.
	Header *DocumentHeader `json:"header,omitempty"`
	// Drawer, when set, is the single identity block a host drawer renders in
	// its own sticky top bar (eyebrow = what kind of thing, title = which one,
	// caption = period / note). A document that carries it expects its own
	// dashboard heading to be suppressed so the drawer chrome owns the header —
	// producers empty Meta.Title accordingly. Optional; absent keeps the
	// generic drawer eyebrow.
	Drawer *DrawerHeader `json:"drawer,omitempty"`
}

// DrawerHeader is the identity block a drawer renders once, in its sticky top
// bar, instead of repeating a page heading and per-panel titles. The producer
// localizes every string.
type DrawerHeader struct {
	// Eyebrow names the kind of value the drawer explains (a metric name),
	// rendered small and uppercase above the title.
	Eyebrow string `json:"eyebrow,omitempty"`
	// Title names the specific scope the drawer is about (a product, group, or
	// account), rendered as the strong heading.
	Title string `json:"title,omitempty"`
	// Caption is the muted supporting line (period, optional note). Newlines are
	// preserved for a secondary note line.
	Caption string `json:"caption,omitempty"`
}

// DocumentHeader is the dashboard's own identity block: a strong title and a
// muted subtitle line. The producer localizes both strings; the runtime may
// append its own live freshness read to the subtitle.
type DocumentHeader struct {
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

func (d DashboardDocument) MarshalJSON() ([]byte, error) {
	type wireDocument DashboardDocument
	d.Version = ContractVersion
	return json.Marshal(wireDocument(d))
}

type Meta struct {
	DashboardID string    `json:"dashboardId"`
	Title       string    `json:"title"`
	GeneratedAt time.Time `json:"generatedAt"`
	Locale      string    `json:"locale"`
}

type Layout struct {
	Rows []LayoutRow `json:"rows"`
}

type LayoutRow struct {
	Heading string       `json:"heading,omitempty"`
	Class   string       `json:"class,omitempty"`
	Panels  []LayoutItem `json:"panels"`
}

type LayoutItem struct {
	PanelID string `json:"panelId"`
	Span    int    `json:"span"`
	// Group, when set, folds this item and its consecutive siblings that
	// carry the same group ID into one shared container card. Renderers that
	// do not understand groups keep laying the items out individually.
	Group *LayoutGroup `json:"group,omitempty"`
}

// LayoutGroupKind selects how a shared container arranges its members.
type LayoutGroupKind string

const (
	// LayoutGroupMetrics renders the members as one compact metric strip
	// inside a single card (the wire form of a stat group).
	LayoutGroupMetrics LayoutGroupKind = "metrics"
	// LayoutGroupTabs renders one segmented tab per distinct member Tab
	// value, showing only the selected tab's members.
	LayoutGroupTabs LayoutGroupKind = "tabs"
)

// LayoutGroupLayout selects a metrics group's member arrangement.
type LayoutGroupLayout string

const (
	LayoutGroupColumns LayoutGroupLayout = "columns"
	LayoutGroupRows    LayoutGroupLayout = "rows"
)

// LayoutGroup describes the container card that owns a run of layout items.
// Every item of the same group repeats the identical descriptor except Tab.
type LayoutGroup struct {
	ID     string            `json:"id"`
	Kind   LayoutGroupKind   `json:"kind"`
	Label  string            `json:"label,omitempty"`
	Layout LayoutGroupLayout `json:"layout,omitempty"`
	Span   int               `json:"span"`
	// Tab names the tab this item belongs to inside a tabs group.
	Tab string `json:"tab,omitempty"`
	// Status, when set, is a single group-level chip rendered once in the
	// group's heading row. The producer hoists it here when every member of the
	// group shares one status, so the chip shows once instead of repeating on
	// each metric. Per-metric chips remain when the members' statuses differ.
	Status *PanelStatus `json:"status,omitempty"`
}

// FilterKind selects a declared dashboard control's type. The shape is
// deliberately a discriminated union: each kind owns a dedicated payload
// field on Filter (Period today; an enumeration payload slots in beside it
// later) so new kinds never overload another kind's fields.
type FilterKind string

const (
	// FilterKindPeriod is a date-range control: a calendar with presets.
	FilterKindPeriod FilterKind = "period"
)

// Filter declares one dashboard-level control. The document owns the
// declaration and the normalized current value; the runtime renders the
// control, writes the chosen value onto the document URL under the declared
// parameter names, and refetches the document. Filter values never reach the
// query endpoint: a query is addressed by SnapshotID alone, so the runtime
// cannot ask the snapshot store for a filter combination the server has not
// normalized and keyed itself.
type Filter struct {
	ID   string     `json:"id"`
	Kind FilterKind `json:"kind"`
	// Label is the already-localized control label.
	Label string `json:"label,omitempty"`
	// Period carries the payload of a period filter. Exactly the kinds' own
	// payload field must be set.
	Period *PeriodFilter `json:"period,omitempty"`
}

// PeriodFilter is the payload of a period (date-range) filter.
//
// Wire format: every date is the server's own "2006-01-02" string. The
// runtime treats boundaries as opaque calendar dates — it never converts them
// through a client timezone — and the server re-anchors whatever it receives
// (Tashkent-anchored for EAI), then echoes the normalized selection back in
// Value. That echo is the display truth after a refetch.
type PeriodFilter struct {
	// StartParam and EndParam name the document-URL query parameters the
	// runtime writes the selection to.
	StartParam string `json:"startParam"`
	EndParam   string `json:"endParam"`
	// Value is the server-normalized selection this document was built from.
	Value PeriodValue `json:"value"`
	// Min and Max, when set, bound the calendar ("2006-01-02").
	Min string `json:"min,omitempty"`
	Max string `json:"max,omitempty"`
	// AllowEmpty permits an unbounded selection: the runtime submits the
	// declared parameters present but empty, the server's "all time" form.
	AllowEmpty bool `json:"allowEmpty,omitempty"`
	// Presets are one-click ranges (years, quarters, "all time"). Labels are
	// already localized by the producer.
	Presets []PeriodPreset `json:"presets,omitempty"`
}

// PeriodValue is a closed or half-open date range. Empty strings mean the
// boundary is unbounded (only meaningful when the filter allows it).
type PeriodValue struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// PeriodPreset is a declared one-click range.
type PeriodPreset struct {
	ID    string      `json:"id"`
	Label string      `json:"label"`
	Value PeriodValue `json:"value"`
}

type PanelKind string

const (
	PanelKindStat    PanelKind = "stat"
	PanelKindPie     PanelKind = "pie"
	PanelKindDonut   PanelKind = "donut"
	PanelKindBar     PanelKind = "bar"
	PanelKindHBar    PanelKind = "hbar"
	PanelKindLine    PanelKind = "line"
	PanelKindArea    PanelKind = "area"
	PanelKindCascade PanelKind = "cascade"
	PanelKindTable   PanelKind = "table"
	// PanelKindCoverage renders a headline value, an optional caption, a
	// segmented progress bar and one legend row per segment. It is the wire
	// form of a segment-bar panel: a part-of-whole statement about a single
	// amount, not a chart.
	PanelKindCoverage PanelKind = "coverage"
)

type Semantics string

const (
	SemanticsPartition      Semantics = "partition"
	SemanticsReconciliation Semantics = "reconciliation"
	SemanticsSeries         Semantics = "series"
	SemanticsEvidence       Semantics = "evidence"
)

type Panel struct {
	ID        string                 `json:"id"`
	Kind      PanelKind              `json:"kind"`
	Title     string                 `json:"title"`
	Semantics Semantics              `json:"semantics"`
	Frame     FrameRef               `json:"frame"`
	Encoding  Encoding               `json:"encoding"`
	Format    map[string]FieldFormat `json:"format"`
	Total     *float64               `json:"total,omitempty"`
	Columns   []TableColumn          `json:"columns,omitempty"`
	DrillRoot *NodeKey               `json:"drillRoot,omitempty"`
	Actions   []Action               `json:"actions"`
	// Accent is the panel's own color (CSS color or theme palette key). Stat
	// metrics render it as a bullet; coverage panels use it for the first
	// segment when no series color matches.
	Accent string `json:"accent,omitempty"`
	// Status is a small chip rendered next to the panel's label, e.g. an
	// estimate marker on a KPI.
	Status *PanelStatus `json:"status,omitempty"`
	// Caption is already-localized supporting text rendered below panel chrome.
	// Newlines are preserved so callers can carry multiple notes or caveats.
	Caption string `json:"caption,omitempty"`
	// Headline overrides a coverage panel's computed headline value.
	Headline *float64 `json:"headline,omitempty"`
	// Trend is a signed change chip rendered in the panel footer, e.g.
	// "▲ +12.4% vs previous period".
	Trend *PanelTrend `json:"trend,omitempty"`
	// Presentation carries opt-in rendering hints. Every field is optional
	// and absent hints keep the renderer's default treatment.
	Presentation *Presentation `json:"presentation,omitempty"`
}

// StatusTone selects a status chip's color treatment.
type StatusTone string

const (
	StatusToneNeutral  StatusTone = "neutral"
	StatusTonePositive StatusTone = "positive"
	StatusToneWarning  StatusTone = "warning"
)

type PanelStatus struct {
	Label string     `json:"label"`
	Tone  StatusTone `json:"tone,omitempty"`
}

// PanelTrend is a period-over-period change chip. Percent is already in
// percent units (12.4 renders as "+12.4%"). Invert flips the good/bad color
// mapping for down-is-good metrics; the arrow always follows the sign.
type PanelTrend struct {
	Percent float64 `json:"percent"`
	Label   string  `json:"label,omitempty"`
	Invert  bool    `json:"invert,omitempty"`
}

// LegendPlacement selects where a chart panel renders its own legend.
type LegendPlacement string

const (
	// LegendBelow renders a centered, wrapping legend under the plot with one
	// `label · value` entry per slice.
	LegendBelow LegendPlacement = "below"
)

// SliceLabels selects the in-plot label treatment of a partition chart.
type SliceLabels string

const (
	// SliceLabelsPercent writes each slice's share inside the slice.
	SliceLabelsPercent SliceLabels = "percent"
)

// TotalBadgePlacement selects where Panel.Total is rendered.
type TotalBadgePlacement string

const (
	// TotalBadgeHeader is the default: a badge in the panel header.
	TotalBadgeHeader TotalBadgePlacement = "header"
	// TotalBadgePlot floats a compact badge inside the plot area.
	TotalBadgePlot TotalBadgePlacement = "plot"
	// TotalBadgeNone suppresses the badge entirely, e.g. when a trend chip
	// carries the panel's summary instead.
	TotalBadgeNone TotalBadgePlacement = "none"
)

// ColorBy selects how a chart assigns series colors.
type ColorBy string

const (
	// ColorByCategory gives every category its own palette color instead of
	// one color for the whole series.
	ColorByCategory ColorBy = "category"
)

type Presentation struct {
	Legend      LegendPlacement     `json:"legend,omitempty"`
	SliceLabels SliceLabels         `json:"sliceLabels,omitempty"`
	TotalBadge  TotalBadgePlacement `json:"totalBadge,omitempty"`
	ColorBy     ColorBy             `json:"colorBy,omitempty"`
	// Fill lets the plot occupy the whole card instead of the default inset.
	Fill bool `json:"fill,omitempty"`
	// BarWidthPx pins the rendered bar thickness in CSS pixels.
	BarWidthPx int `json:"barWidthPx,omitempty"`
	// Sortable, when explicitly false, removes a table panel's sort affordances
	// (column sort buttons + the "sort applies to this page" footer). A static
	// identity table (e.g. a fixed decomposition) sets it false; nil keeps the
	// default sortable table.
	Sortable *bool `json:"sortable,omitempty"`
	// Expandable, when explicitly false, removes the panel's expand-to-overlay
	// control. Panels rendered inside a drawer set it false — an overlay over a
	// modal is meaningless. nil keeps the default expand control.
	Expandable *bool `json:"expandable,omitempty"`
	// Exportable, when explicitly false, removes the panel's export control. A
	// small derived table sets it false; long record tables keep export. nil
	// keeps the default export control.
	Exportable *bool `json:"exportable,omitempty"`
	// RowGroupField names a frame column carrying a per-row group tag on a table
	// panel. An empty tag is a normal row; a tag ending in ":toggle" marks a
	// synthetic expander row for the group named by its prefix; any other tag
	// marks a collapsed member, hidden until its toggle is expanded.
	RowGroupField string `json:"rowGroupField,omitempty"`
}

type TableColumn struct {
	Field  string     `json:"field"`
	Label  string     `json:"label"`
	Align  TableAlign `json:"align,omitempty"`
	Cell   TableCell  `json:"cell"`
	Action *Action    `json:"action,omitempty"`
	// Text is the literal cell content of an action-only column (a column
	// with no field), e.g. the label of a row-level "open" link.
	Text string `json:"text,omitempty"`
	// WidthPx, when > 0, is the column's minimum width in CSS pixels.
	WidthPx int `json:"widthPx,omitempty"`
	// Clamp, when > 0, limits the cell text to that many lines.
	Clamp int `json:"clamp,omitempty"`
	// Affordance selects how an actionable cell advertises its action.
	Affordance TableAffordance `json:"affordance,omitempty"`
	// BadgeField names a frame column carrying a per-row badge tooltip. A row
	// with a non-empty value renders a muted "?" badge (with that text as its
	// title) after the cell's value — e.g. flagging an unmatched source row.
	BadgeField string `json:"badgeField,omitempty"`
}

// TableAffordance selects the visual treatment of an actionable table cell.
type TableAffordance string

const (
	// TableAffordancePill renders the cell as a compact pill with a drill
	// arrow, marking every value in the column as a drill entry point.
	TableAffordancePill TableAffordance = "pill"
	// TableAffordanceQuiet makes the whole cell the drill target with no
	// standing chrome: plain value, and on hover/focus a subtle accent
	// underline plus an arrow that fades in at the cell's trailing edge.
	TableAffordanceQuiet TableAffordance = "quiet"
)

type TableAlign string

const (
	TableAlignLeft  TableAlign = "left"
	TableAlignRight TableAlign = "right"
)

type TableCellKind string

const (
	TableCellPlain TableCellKind = "plain"
	TableCellBar   TableCellKind = "bar"
	TableCellDelta TableCellKind = "delta"
	// TableCellUnderline renders the value over a thin proportional rule
	// colored by sign — a low-ink alternative to a mini-bar.
	TableCellUnderline TableCellKind = "underline"
)

// TableCellLayout selects a rich cell's internal arrangement.
type TableCellLayout string

const (
	// TableCellStacked puts the secondary value on its own line under the
	// primary one instead of beside it.
	TableCellStacked TableCellLayout = "stacked"
)

type TableCell struct {
	Kind TableCellKind `json:"kind"`
	// SecondaryField holds a delta cell's percent-change field. Its values are
	// already expressed in percent units: -4 renders as -4.0%, not -400%. A
	// 0..1 share would silently render as 0.1%.
	SecondaryField string          `json:"secondaryField,omitempty"`
	Layout         TableCellLayout `json:"layout,omitempty"`
	// ToneField names a frame column carrying a per-row status tone applied to
	// the cell's value color: "pos", "warn", or "neg" (empty keeps the default
	// text color). The producer sets the tone from its own business thresholds.
	ToneField string `json:"toneField,omitempty"`
}

type Encoding struct {
	Label    string `json:"label,omitempty"`
	Value    string `json:"value,omitempty"`
	ID       string `json:"id,omitempty"`
	Series   string `json:"series,omitempty"`
	Category string `json:"category,omitempty"`
	Cut      string `json:"cut,omitempty"`
	CutLabel string `json:"cutLabel,omitempty"`
	Final    string `json:"final,omitempty"`
}

type FormatKind string

const (
	FormatMoney   FormatKind = "money"
	FormatPercent FormatKind = "percent"
	FormatDate    FormatKind = "date"
	FormatNumber  FormatKind = "number"
	FormatString  FormatKind = "string"
)

// PrecisionOf returns a pointer to n, for FieldFormat.Precision. A deliberate
// 0 ("whole units") must reach the wire, which is why the field is a pointer.
func PrecisionOf(n int) *int { return &n }

type FieldFormat struct {
	Kind       FormatKind `json:"kind"`
	Currency   string     `json:"currency,omitempty"`
	MinorUnits bool       `json:"minorUnits"`
	// Precision is a pointer so "no decimals" and "unspecified" stay
	// distinguishable on the wire. As a plain `int,omitempty` a deliberate 0
	// was dropped, and the runtime fell back to the locale's default fraction
	// digits — rendering "…533,993" where the spec asked for whole units.
	Precision *int   `json:"precision,omitempty"`
	Layout    string `json:"layout,omitempty"`
	// Compact abbreviates magnitudes with the locale's own compact notation
	// (ru: "9,36 млрд", en: "9.36B", uz: "9,36 mlrd").
	Compact bool `json:"compact,omitempty"`
	// DecimalSeparator overrides the locale's decimal separator and puts the
	// runtime in Go-renderer parity mode: ASCII spaces instead of the
	// locale's non-breaking ones, and no space before a percent sign. The Go
	// renderer prints abbreviated numbers with %.*f — a dot in every locale —
	// so a document that must match it byte for byte sets "." here.
	DecimalSeparator string `json:"decimalSeparator,omitempty"`
	// Symbol is the currency's display grapheme (UZS → "so’m"), taken from the
	// same pkg/money definition the Go renderer formats with. When set, money
	// renders as "<amount> <symbol>" instead of the locale's own currency
	// display for the ISO code, which is how the Go renderer prints it.
	Symbol string `json:"symbol,omitempty"`
}

type ActionKind string

const (
	ActionNavigate       ActionKind = "navigate"
	ActionNavigateToLeaf ActionKind = "navigate_to_leaf"
	ActionOpenDrawer     ActionKind = "open_drawer"
	ActionEmitEvent      ActionKind = "emit_event"
)

type ValueSourceKind string

const (
	ValueSourceField    ValueSourceKind = "field"
	ValueSourceVariable ValueSourceKind = "variable"
	ValueSourceLiteral  ValueSourceKind = "literal"
)

type Action struct {
	Kind          ActionKind        `json:"kind"`
	Method        string            `json:"method,omitempty"`
	URLTemplate   string            `json:"urlTemplate,omitempty"`
	URLSource     *Source           `json:"urlSource,omitempty"`
	Event         string            `json:"event,omitempty"`
	Params        []ActionParam     `json:"params"`
	Payload       map[string]Source `json:"payload"`
	PreserveQuery bool              `json:"preserveQuery,omitempty"`
}

type ActionParam struct {
	Name   string `json:"name"`
	Source Source `json:"source"`
}

type Source struct {
	Kind     ValueSourceKind `json:"kind"`
	Name     string          `json:"name,omitempty"`
	Value    any             `json:"value,omitempty"`
	Fallback any             `json:"fallback,omitempty"`
}

type NodeKey string
type NodePath []NodeKey

type Drill struct {
	Edges       map[NodeKey]Level `json:"edges"`
	InlineDepth int               `json:"inlineDepth"`
}

type Level struct {
	Path            NodePath         `json:"path"`
	Label           string           `json:"label"`
	Children        []Node           `json:"children"`
	DynamicChildren *DynamicChildren `json:"dynamicChildren,omitempty"`
	Frame           FrameRef         `json:"frame,omitempty"`
	Encoding        *Encoding        `json:"encoding,omitempty"`
	Perspectives    []PerspectiveRef `json:"perspectives"`
}

type DynamicChildren struct {
	Key    Source  `json:"key"`
	Label  Source  `json:"label"`
	Target *Source `json:"target,omitempty"`
	Action *Action `json:"action,omitempty"`
}

type Node struct {
	Key    NodeKey  `json:"key"`
	Path   NodePath `json:"path"`
	Label  string   `json:"label"`
	Target NodeKey  `json:"target,omitempty"`
	Action *Action  `json:"action,omitempty"`
}

type PerspectiveRef struct {
	ID string `json:"id"`
}

type Perspective struct {
	ID         string    `json:"id"`
	ExplorerID string    `json:"explorerId"`
	BranchKey  NodeKey   `json:"branchKey"`
	Key        string    `json:"key"`
	Label      string    `json:"label"`
	Semantics  Semantics `json:"semantics"`
	Root       NodeKey   `json:"root"`
}

type ColumnType string

const (
	ColumnString ColumnType = "string"
	ColumnNumber ColumnType = "number"
	ColumnBool   ColumnType = "bool"
	ColumnTime   ColumnType = "time"
)

type Column struct {
	Name string     `json:"name"`
	Type ColumnType `json:"type"`
}

type Frame struct {
	Columns  []Column `json:"columns"`
	Rows     [][]any  `json:"rows"`
	Children []Node   `json:"children,omitempty"`
}

type Endpoints struct {
	Query  string `json:"query,omitempty"`
	Export string `json:"export,omitempty"`
}

type Theme struct {
	Palette map[string]string `json:"palette"`
	Series  map[string]string `json:"series"`
}

type QueryRequest struct {
	SnapshotID  string   `json:"snapshotId"`
	Path        NodePath `json:"path"`
	Perspective string   `json:"perspective,omitempty"`
	Page        int      `json:"page,omitempty"`
}

type QueryPage struct {
	Number  int  `json:"number"`
	Size    int  `json:"size"`
	HasNext bool `json:"hasNext,omitempty"`
}

func (p QueryPage) MarshalJSON() ([]byte, error) {
	// Keep older responses parseable by generated clients while always emitting
	// an authoritative false value from current servers.
	type queryPageWire struct {
		Number  int  `json:"number"`
		Size    int  `json:"size"`
		HasNext bool `json:"hasNext"`
	}
	return json.Marshal(queryPageWire(p))
}

type QueryResponse struct {
	Frames map[FrameRef]Frame `json:"frames"`
	Page   *QueryPage         `json:"page,omitempty"`
}

type QueryErrorCode string

const (
	QueryErrorSnapshotGone QueryErrorCode = "snapshot_gone"
	QueryErrorBadRequest   QueryErrorCode = "bad_request"
	QueryErrorInternal     QueryErrorCode = "internal"
)

type QueryErrorResponse struct {
	Error   QueryErrorCode `json:"error"`
	Message string         `json:"message"`
}
