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
	Filters         []Filter          `json:"filters,omitempty"`
	ActiveFilters   []ActiveFilter    `json:"activeFilters,omitempty"`
	ResetFiltersURL string            `json:"resetFiltersUrl,omitempty"`
	Endpoints       Endpoints         `json:"endpoints"`
	I18n            map[string]string `json:"i18n"`
	Theme           Theme             `json:"theme"`
	URLState        *URLStateContract `json:"urlState,omitempty"`
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

const (
	URLStateVersion  = 1
	URLStateParam    = "lens-state"
	URLStateMaxBytes = 4096
)

type URLStateContract struct {
	Version  int    `json:"version"`
	Param    string `json:"param"`
	MaxBytes int    `json:"maxBytes"`
}

// DrawerSize selects a host drawer's width treatment. An empty size keeps the
// drawer's default width.
type DrawerSize string

const (
	// DrawerSizeWide widens the drawer for cross-metric documents (a focus
	// canvas hosted inside a drawer).
	DrawerSizeWide DrawerSize = "wide"
)

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
	// Size, when set, selects the host drawer's width treatment. Absent keeps
	// the default drawer width.
	Size DrawerSize `json:"size,omitempty"`
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
	Heading string `json:"heading,omitempty"`
	Class   string `json:"class,omitempty"`
	// Anchor is rendered as the section's element id, making the row a link
	// target within the page.
	Anchor string       `json:"anchor,omitempty"`
	Panels []LayoutItem `json:"panels"`
}

type LayoutItem struct {
	PanelID string `json:"panelId"`
	Span    int    `json:"span"`
	// Groups is the nested-container chain this item belongs to, ordered
	// outermost → innermost (the last element is the item's immediate container).
	Groups []LayoutGroup `json:"groups,omitempty"`
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
	ID   string          `json:"id"`
	Kind LayoutGroupKind `json:"kind"`
	// Caption explains the group as a whole — why this strip of metrics sits
	// here, what basis it reads on — the same role Panel.Caption plays for a
	// single panel. Without it a container's Description would be silently
	// dropped on the wire, since a group owns no panel of its own.
	Caption string            `json:"caption,omitempty"`
	Label   string            `json:"label,omitempty"`
	Layout  LayoutGroupLayout `json:"layout,omitempty"`
	Span    int               `json:"span"`
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
	// FilterKindFacet is a searchable multi-select control backed by a
	// producer-owned options endpoint.
	FilterKindFacet   FilterKind = "facet"
	FilterKindCompare FilterKind = "compare"
	// FilterKindSegmented is a small closed set of mutually exclusive values
	// rendered inline — the shape a dashboard reaches for when a choice is
	// short, permanent and cheap enough that hiding it behind a popover costs
	// more than showing it. It exists so that such a choice can be a *filter*:
	// URL state that survives reload and sharing, re-entering through
	// syncFiltersFromURL like every other control. Before it, the only way to
	// express "one of two/three modes" was a Tabs panel group, which is a
	// layout container — its selection is renderer state, it is invisible to
	// the URL, and it forces every branch of the choice to be materialized as
	// a nested panel inside a panel.
	FilterKindSegmented FilterKind = "segmented"
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
	// Facet carries the payload of a searchable multi-select filter.
	Facet     *FacetFilter     `json:"facet,omitempty"`
	Compare   *CompareFilter   `json:"compare,omitempty"`
	Segmented *SegmentedFilter `json:"segmented,omitempty"`
}

// SegmentedFilter declares a single-choice control over a closed option set.
// Param is the one query parameter the choice is written to; Value is the
// server-normalized current selection and must be one of Options.
type SegmentedFilter struct {
	Param   string            `json:"param"`
	Value   string            `json:"value"`
	Options []SegmentedOption `json:"options"`
}

// SegmentedOption is one segment. Label is already localized by the producer:
// a segmented filter's vocabulary is business vocabulary («По кварталам»), not
// runtime chrome, so it is not part of the SDK i18n catalogue.
type SegmentedOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type CompareMode string

const (
	CompareModeOff            CompareMode = "off"
	CompareModePreviousPeriod CompareMode = "previous_period"
	CompareModeYearAgo        CompareMode = "year_ago"
	CompareModeCustom         CompareMode = "custom"
)

type CompareFilter struct {
	ModeParam  string       `json:"modeParam"`
	StartParam string       `json:"startParam"`
	EndParam   string       `json:"endParam"`
	CompareTo  string       `json:"compareTo"`
	Value      CompareValue `json:"value"`
}

type CompareValue struct {
	Mode  CompareMode `json:"mode"`
	Start string      `json:"start,omitempty"`
	End   string      `json:"end,omitempty"`
}

// ActiveFilter is one condition in the shared cross-filter/cube-drill stack.
type ActiveFilter struct {
	Dimension string `json:"dimension"`
	Label     string `json:"label"`
	Value     string `json:"value"`
	RemoveURL string `json:"removeUrl"`
}

// FacetFilter declares a producer-owned searchable option list and the
// server-resolved active selections. URLs are opaque same-origin navigation
// targets so the runtime never has to reconstruct producer query semantics.
type FacetFilter struct {
	Dimension       string           `json:"dimension"`
	OptionsEndpoint string           `json:"optionsEndpoint"`
	SearchParam     string           `json:"searchParam,omitempty"`
	Selections      []FacetSelection `json:"selections,omitempty"`
	ClearURL        string           `json:"clearUrl,omitempty"`
}

// FacetSelection is one active, human-readable filter. RemoveURL removes only
// this selection while preserving every unrelated query parameter.
type FacetSelection struct {
	Label     string `json:"label"`
	RemoveURL string `json:"removeUrl"`
}

// FacetOptionsResponse is the wire response returned by FacetFilter's options
// endpoint.
type FacetOptionsResponse struct {
	Options  []FacetOption `json:"options"`
	ApplyURL string        `json:"applyUrl"`
}

// FacetOption is one searchable option. ToggleURL adds or removes it while
// preserving the producer's complete filter state.
type FacetOption struct {
	Label     string `json:"label"`
	Value     string `json:"value"`
	Count     int    `json:"count,omitempty"`
	Selected  bool   `json:"selected,omitempty"`
	ToggleURL string `json:"toggleUrl"`
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
	PanelKindStat      PanelKind = "stat"
	PanelKindPie       PanelKind = "pie"
	PanelKindDonut     PanelKind = "donut"
	PanelKindRadial    PanelKind = "radial"
	PanelKindBar       PanelKind = "bar"
	PanelKindHBar      PanelKind = "hbar"
	PanelKindLine      PanelKind = "line"
	PanelKindArea      PanelKind = "area"
	PanelKindGauge     PanelKind = "gauge"
	PanelKindHistogram PanelKind = "histogram"
	PanelKindBoxPlot   PanelKind = "boxplot"
	PanelKindHeatmap   PanelKind = "heatmap"
	PanelKindMap       PanelKind = "map"
	PanelKindCascade   PanelKind = "cascade"
	PanelKindTable     PanelKind = "table"
	// PanelKindCoverage renders a headline value, an optional caption, a
	// segmented progress bar and one legend row per segment. It is the wire
	// form of a segment-bar panel: a part-of-whole statement about a single
	// amount, not a chart.
	PanelKindCoverage PanelKind = "coverage"
	// PanelKindMetricFlow renders a left-to-right result formula: a sequence of
	// signed operand stages that read to a single supplied result. It covers the
	// "формула" link type — how a headline metric is composed arithmetically —
	// and is not a chart. See MetricFlowConfig.
	PanelKindMetricFlow PanelKind = "metric_flow"
	// PanelKindMetricHierarchy renders a compact, indented decomposition of one
	// parent metric into child rows with integrity checks (roots, cycles,
	// optional parent↔children reconciliation). See MetricHierarchyConfig.
	PanelKindMetricHierarchy PanelKind = "metric_hierarchy"
	// PanelKindMetricRelationship renders an explicit, non-arithmetic link
	// between two metrics (the "связь" link type) — a neutral association, a
	// derivation, or a reconciliation, with an explicit direction. See
	// MetricRelationshipConfig.
	PanelKindMetricRelationship PanelKind = "metric_relationship"
)

// Confidence states how a displayed number was produced. It is orthogonal to
// Availability: Confidence qualifies a number that exists, Availability says
// whether a number can exist at all. Both are settable at the panel level and
// per element; a frame column value overrides the structural default. When the
// chip precedence resolves to a visible quality chip, Confidence is shown only
// when Availability is available (see Availability's chip-precedence note). An
// empty Confidence is unset (no confidence chip).
type Confidence string

const (
	// ConfidenceVerified is a directly measured / booked number (ФАКТ).
	ConfidenceVerified Confidence = "verified"
	// ConfidenceCalculated is produced by a documented formula from verified
	// inputs (РАСЧЁТ).
	ConfidenceCalculated Confidence = "calculated"
	// ConfidenceProxy is a heuristic reconstruction / stand-in (ПРОКСИ).
	ConfidenceProxy Confidence = "proxy"
	// ConfidenceRequiresReconciliation marks a number not yet reconciled against
	// its authoritative source.
	ConfidenceRequiresReconciliation Confidence = "requires_reconciliation"
)

// Availability states whether a number can exist for an element, independent of
// how trustworthy it would be (that is Confidence). Chip precedence is
// availability-first: when Availability is anything other than available, the
// renderer shows the Availability chip and never the Confidence chip, so an
// element never carries two competing chips. A genuine numeric 0 is a distinct
// state and is never a substitute for a non-available status. An empty
// Availability is unset and treated as available.
type Availability string

const (
	// AvailabilityAvailable means the number exists and can be shown.
	AvailabilityAvailable Availability = "available"
	// AvailabilityConfigRequired means a plan/threshold/assumption is missing, so
	// the number cannot be produced until configured (НАСТРОЙКА).
	AvailabilityConfigRequired Availability = "config_required"
	// AvailabilityEmptySource means the structure exists but there are no
	// operational rows behind it yet (КОНТУР ПУСТ).
	AvailabilityEmptySource Availability = "empty_source"
	// AvailabilityUnavailable means the business event is simply not stored, so
	// no number exists (НЕТ ДАННЫХ).
	AvailabilityUnavailable Availability = "unavailable"
)

// MetricFlowStageRole is the formula grammar of a flow stage. It governs the
// single normalized operator glyph the renderer prints for the stage and
// nothing else: frame values stay signed business amounts, and the renderer is
// never the authoritative calculator. An add/subtract stage renders exactly one
// operator plus the absolute formatted magnitude, so a negative movement under
// subtract reads as "+ |v|", never "− −v". The formula reads left-to-right to
// exactly one result stage, which carries an independently supplied server
// value.
type MetricFlowStageRole string

const (
	// MetricFlowRoleInput is the leading operand the formula starts from.
	MetricFlowRoleInput MetricFlowStageRole = "input"
	// MetricFlowRoleAdd contributes its magnitude with a "+" operator.
	MetricFlowRoleAdd MetricFlowStageRole = "add"
	// MetricFlowRoleSubtract contributes its magnitude with a "−" operator.
	MetricFlowRoleSubtract MetricFlowStageRole = "subtract"
	// MetricFlowRoleIntermediate is a non-terminal running subtotal shown inline.
	MetricFlowRoleIntermediate MetricFlowStageRole = "intermediate"
	// MetricFlowRoleResult is the single terminal stage; its value is supplied by
	// the server, not computed by the renderer.
	MetricFlowRoleResult MetricFlowStageRole = "result"
)

// MetricRelationshipType selects the kind of non-arithmetic link a relationship
// panel states between its two ends.
type MetricRelationshipType string

const (
	// MetricRelationshipAssociation is a neutral economic link between two
	// metrics that must not imply part-of, subtraction, or derivation. It is the
	// primary case and renders with a symmetric connector.
	MetricRelationshipAssociation MetricRelationshipType = "association"
	// MetricRelationshipDerivation states that the target is derived from the
	// source. Direction carries the emphasis.
	MetricRelationshipDerivation MetricRelationshipType = "derivation"
	// MetricRelationshipReconciliation states that the two ends reconcile against
	// each other; symmetric by nature.
	MetricRelationshipReconciliation MetricRelationshipType = "reconciliation"
)

// MetricRelationshipDirection is the explicit direction of a relationship. It is
// never inferred from the order of the ends. An empty direction defaults to
// bidirectional.
type MetricRelationshipDirection string

const (
	// MetricRelationshipBidirectional is a neutral, symmetric connector (⇄).
	MetricRelationshipBidirectional MetricRelationshipDirection = "bidirectional"
	// MetricRelationshipSourceToTarget points from source to target (→).
	MetricRelationshipSourceToTarget MetricRelationshipDirection = "source_to_target"
	// MetricRelationshipTargetToSource points from target to source (←).
	MetricRelationshipTargetToSource MetricRelationshipDirection = "target_to_source"
)

// MetricFlowConfig is the structure of a metric_flow panel: an ordered list of
// operand stages plus an optional reconciliation tolerance. Numeric values are
// joined from the panel frame by stage Key via the panel encoding; the config
// carries only structure (keys, labels, roles, captions, quality, actions).
type MetricFlowConfig struct {
	Stages    []MetricFlowStage   `json:"stages"`
	Reconcile *FlowReconciliation `json:"reconcile,omitempty"`
}

// MetricFlowStage is one operand of a result formula. Role is formula grammar
// only (see MetricFlowStageRole); the stage's numeric value is joined from the
// frame by Key and stays a signed business amount.
type MetricFlowStage struct {
	Key   string              `json:"key"`
	Label string              `json:"label"`
	Role  MetricFlowStageRole `json:"role"`
	// Caption is one muted secondary line rendered under the stage value.
	Caption      string       `json:"caption,omitempty"`
	Confidence   Confidence   `json:"confidence,omitempty"`
	Availability Availability `json:"availability,omitempty"`
	// Action, when set, makes the stage an actionable link/button.
	Action *Action `json:"action,omitempty"`
}

// FlowReconciliation opts a flow into a compact mismatch note: the renderer
// compares the displayed operands against the supplied result and, when the
// difference exceeds Tolerance, shows a note. It never replaces the result.
type FlowReconciliation struct {
	Tolerance float64 `json:"tolerance,omitempty"`
}

// MetricHierarchyConfig is the structure of a metric_hierarchy panel: a flat
// list of rows linked by Parent, plus an optional reconciliation tolerance.
// Numeric values are joined from the panel frame by row Key via the encoding.
type MetricHierarchyConfig struct {
	Rows      []MetricHierarchyRow     `json:"rows"`
	Reconcile *HierarchyReconciliation `json:"reconcile,omitempty"`
}

// MetricHierarchyRow is one node of a metric decomposition. Parent is the
// authoritative single source of the hierarchy shape; Depth is derived from the
// Parent chains at build time and carried only as a renderer convenience — it is
// never producer-supplied truth. When Depth is non-zero it must equal the
// derived depth.
type MetricHierarchyRow struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	// Parent names the Key of this row's parent; empty marks a root row.
	Parent string `json:"parent,omitempty"`
	// Depth is build-derived from the Parent chain (0 for a root). Producers
	// leave it zero; the builder fills it for renderer convenience.
	Depth int `json:"depth,omitempty"`
	// Unallocated marks an explicit producer-provided «Не распределено» child.
	// The SDK never manufactures one; at most one per parent.
	Unallocated bool `json:"unallocated,omitempty"`
	// Selected marks the single row rendered as aria-current; at most one row.
	Selected     bool         `json:"selected,omitempty"`
	Confidence   Confidence   `json:"confidence,omitempty"`
	Availability Availability `json:"availability,omitempty"`
	Action       *Action      `json:"action,omitempty"`
}

// HierarchyReconciliation opts a hierarchy into per-parent integrity checks: for
// each parent the renderer compares SUM(direct children incl. unallocated)
// against the parent within Tolerance and shows a compact summary.
type HierarchyReconciliation struct {
	Tolerance float64 `json:"tolerance,omitempty"`
}

// MetricRelationshipConfig is the structure of a metric_relationship panel: two
// ends, a link type, an explicit direction, and an optional note.
type MetricRelationshipConfig struct {
	Source    MetricRelationshipEnd       `json:"source"`
	Target    MetricRelationshipEnd       `json:"target"`
	Type      MetricRelationshipType      `json:"type"`
	Direction MetricRelationshipDirection `json:"direction,omitempty"`
	Note      string                      `json:"note,omitempty"`
}

// MetricRelationshipEnd is one side of a relationship. Its numeric value is
// joined from the panel frame by Key via the encoding.
type MetricRelationshipEnd struct {
	Key          string       `json:"key"`
	Label        string       `json:"label"`
	Confidence   Confidence   `json:"confidence,omitempty"`
	Availability Availability `json:"availability,omitempty"`
	Action       *Action      `json:"action,omitempty"`
}

type Semantics string

const (
	SemanticsPartition      Semantics = "partition"
	SemanticsReconciliation Semantics = "reconciliation"
	SemanticsSeries         Semantics = "series"
	SemanticsEvidence       Semantics = "evidence"
)

type AxisScale string

const (
	AxisScaleLinear      AxisScale = "linear"
	AxisScaleLogarithmic AxisScale = "logarithmic"
)

type ValueAxis struct {
	Scale   AxisScale `json:"scale"`
	LogBase int       `json:"logBase,omitempty"`
}

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
	Table     *TableOptions          `json:"table,omitempty"`
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
	// Info is the already-localized long-form explanation a dashboard keeps
	// behind an ⓘ affordance — how a figure is obtained, what it excludes. On
	// screen it costs nothing until asked for; on paper there is nothing to ask,
	// so a printed report carries it as the figure's footnote.
	Info string `json:"info,omitempty"`
	// Headline overrides a coverage panel's computed headline value.
	Headline *float64 `json:"headline,omitempty"`
	// Trend is a signed change chip rendered in the panel footer, e.g.
	// "▲ +12.4% vs previous period".
	Trend *PanelTrend `json:"trend,omitempty"`
	// Sparkline is a small inline trend polyline rendered in a stat panel's
	// footer row.
	Sparkline *Sparkline `json:"sparkline,omitempty"`
	// Target is a target/threshold marker rendered on a coverage or hbar panel
	// (bullet-style), or the first horizontal reference line on a time series.
	Target *PanelTarget `json:"target,omitempty"`
	// Temporal carries opt-in overlays for line and area time series.
	Temporal *PanelTemporal `json:"temporal,omitempty"`
	// Presentation carries opt-in rendering hints. Every field is optional
	// and absent hints keep the renderer's default treatment.
	Presentation *Presentation `json:"presentation,omitempty"`
	// ValueAxis carries the numeric-axis scale requested by the producer.
	// Absent keeps the runtime's linear default.
	ValueAxis *ValueAxis `json:"valueAxis,omitempty"`
	// MetricFlow carries the structure of a metric_flow panel. Exactly the
	// panel kind's own config field must be set.
	MetricFlow *MetricFlowConfig `json:"metricFlow,omitempty"`
	// MetricHierarchy carries the structure of a metric_hierarchy panel.
	MetricHierarchy *MetricHierarchyConfig `json:"metricHierarchy,omitempty"`
	// MetricRelationship carries the structure of a metric_relationship panel.
	MetricRelationship *MetricRelationshipConfig `json:"metricRelationship,omitempty"`
	// Radial carries the geometry contract for a radial panel.
	Radial *RadialConfig `json:"radial,omitempty"`
	// Map carries a bounded GeoJSON source and an exact frame-to-feature join.
	Map *MapConfig `json:"map,omitempty"`
	// Confidence is the panel-level default confidence for its elements; a
	// frame column value or an element's own confidence overrides it.
	Confidence Confidence `json:"confidence,omitempty"`
	// Availability is the panel-level default availability for its elements; a
	// frame column value or an element's own availability overrides it.
	Availability Availability `json:"availability,omitempty"`
	// Deferred marks a panel whose primary frame is intentionally absent from
	// the initial document. Consumers load it from Endpoints.Panel using the
	// document snapshot and this panel's ID.
	Deferred bool `json:"deferred,omitempty"`
	// Terminal explicitly states that the panel has no deeper data action.
	Terminal bool `json:"terminal,omitempty"`
	// ComparisonUnsupported marks an active comparison that this panel kind
	// cannot visualize. The client renders its localized explanatory notice.
	ComparisonUnsupported bool `json:"comparisonUnsupported,omitempty"`
}

// RadialMode selects a radial panel's geometry.
type RadialMode string

const (
	RadialModeProgress  RadialMode = "progress"
	RadialModePartition RadialMode = "partition"
)

// RadialConfig configures concentric progress arcs or concentric partitions.
// Max is required only by progress mode.
type RadialConfig struct {
	Mode      RadialMode   `json:"mode"`
	Max       float64      `json:"max,omitempty"`
	Rings     []RadialRing `json:"rings,omitempty"`
	Tolerance float64      `json:"tolerance,omitempty"`
}

// RadialRing is an explicitly ordered, reconciled partition ring.
type RadialRing struct {
	Key   string  `json:"key"`
	Label string  `json:"label"`
	Order int     `json:"order,omitempty"`
	Total float64 `json:"total"`
}

// MaxMapGeoJSONBytes is the hard ceiling for a fetched map source. A panel may
// request a smaller bound but cannot move this client safety limit upward.
const MaxMapGeoJSONBytes = 5 * 1024 * 1024

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Geometry   map[string]any `json:"geometry"`
}

// GeoJSONSource selects exactly one source. A fetched URL is a same-origin
// absolute path and must provide a positive MaxBytes bound.
type GeoJSONSource struct {
	Inline   *GeoJSONFeatureCollection `json:"inline,omitempty"`
	URL      string                    `json:"url,omitempty"`
	MaxBytes int                       `json:"maxBytes,omitempty"`
}

// MapConfig declares an exact, case-sensitive one-to-one region join:
// Encoding.ID values match FeatureProperty values. LabelProperty optionally
// names the human-readable feature property used on the map.
type MapConfig struct {
	Source          GeoJSONSource     `json:"source"`
	FeatureProperty string            `json:"featureProperty"`
	LabelProperty   string            `json:"labelProperty,omitempty"`
	LabelProperties map[string]string `json:"labelProperties,omitempty"`
	Attribution     string            `json:"attribution,omitempty"`
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

// TrendPolarity says whether a movement in a metric is good news, bad news or
// neither; see panel.TrendPolarity. Unset reads as neutral, which prints the
// direction without a verdict.
type TrendPolarity string

const (
	TrendPolarityNeutral      TrendPolarity = "neutral"
	TrendPolarityHigherBetter TrendPolarity = "higher_better"
	TrendPolarityLowerBetter  TrendPolarity = "lower_better"
)

// PanelTrend is a period-over-period change chip. Percent is already in
// percent units (12.4 renders as "+12.4%"). Polarity decides the chip's colour;
// Invert is its legacy two-state form (true = lower is better). The arrow always
// follows the sign.
type PanelTrend struct {
	Percent           float64        `json:"percent"`
	Label             string         `json:"label,omitempty"`
	Polarity          TrendPolarity  `json:"polarity,omitempty"`
	Invert            bool           `json:"invert,omitempty"`
	AbsoluteField     string         `json:"absoluteField,omitempty"`
	PercentField      string         `json:"percentField,omitempty"`
	AbsoluteDeltaUnit TrendDeltaUnit `json:"absoluteDeltaUnit,omitempty"`
}

type TrendDeltaUnit string

const (
	TrendDeltaValue            TrendDeltaUnit = "value"
	TrendDeltaPercentagePoints TrendDeltaUnit = "percentage_points"
)

// Sparkline is a small inline trend polyline rendered in a stat panel's footer
// row. Values are plotted left-to-right, normalized to the sparkline viewbox;
// Color overrides the default accent stroke when set (any CSS color).
type Sparkline struct {
	Values []float64 `json:"values"`
	Color  string    `json:"color,omitempty"`
}

// PanelTarget is a target/threshold marker rendered on a coverage or hbar
// panel: a bullet-style tick at Value with an optional already-localized
// Label beside it. Value is in the same units as the panel's plotted amounts;
// the renderer never rescales it.
type PanelTarget struct {
	Value float64 `json:"value"`
	Label string  `json:"label,omitempty"`
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
	// SliceLabelsLabel writes each slice's category label inside the slice,
	// leaving the share to the legend. For dimensions whose labels are short
	// and self-explaining — a year, a quarter — where naming the slice beats
	// a number the legend can carry instead. A slice too narrow to hold its
	// label is left unlabelled rather than clipped.
	SliceLabelsLabel SliceLabels = "label"
)

// LegendValue selects what a legend entry prints after its label.
type LegendValue string

const (
	// LegendValueAmount is the default: the row's own formatted value.
	LegendValueAmount LegendValue = "value"
	// LegendValuePercent prints the row's share of the frame total instead.
	// Pairs with SliceLabelsLabel: the slice names itself, the legend
	// quantifies it.
	LegendValuePercent LegendValue = "percent"
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
	// ColorBySequence shades an *ordered* dimension along one hue instead of
	// giving each step an unrelated one. Age bands, tenure buckets and rating
	// grades are ranked, and a categorical palette says the opposite: a rainbow
	// across 0-24 … 65+ claims the buckets are unrelated and hides the shape of
	// the distribution behind ten competing hues. Only the producer knows a
	// dimension is ordered, so only the producer can ask for this.
	ColorBySequence ColorBy = "sequence"
)

// BridgeLayout selects the visual projection of a cascade/bridge panel.
type BridgeLayout string

const (
	// BridgeLayoutWaterfall renders opening/closing totals as columns from zero
	// and the intervening running-balance changes as floating columns.
	BridgeLayoutWaterfall BridgeLayout = "waterfall"
)

// CascadeTone is the explicit per-stage semantic color of a cascade/waterfall
// panel, carried per row by the Encoding.Tone column. It overrides the
// flow-direction default so a stage reads by meaning (a cost is negative/red)
// rather than by direction (a decrease is green). Values are validated
// leniently: an unrecognized tone falls back to the direction default.
type CascadeTone string

const (
	// CascadeToneNeutral is a total / running balance (navy accent).
	CascadeToneNeutral CascadeTone = "neutral"
	// CascadeTonePositive is a genuine favorable movement, e.g. a reserve
	// release that increases profit (green).
	CascadeTonePositive CascadeTone = "positive"
	// CascadeToneNegative is a cost / deduction that reduces the result (red).
	CascadeToneNegative CascadeTone = "negative"
	// CascadeToneInflow is an incoming amount added to the running balance
	// (orange).
	CascadeToneInflow CascadeTone = "inflow"
)

// FocusMode selects the drill chrome of a drill-hosting panel. An empty mode
// keeps today's in-card drill rendering.
type FocusMode string

const (
	// FocusModeCanvas renders the panel's drill flow as a focus canvas: a
	// persistent breadcrumb, a parent-context header, and a standalone lens
	// selector around the drilled visualization.
	FocusModeCanvas FocusMode = "canvas"
)

type Presentation struct {
	// DataLabels writes formatted values directly on bar and line marks.
	DataLabels  bool                `json:"dataLabels,omitempty"`
	Legend      LegendPlacement     `json:"legend,omitempty"`
	LegendValue LegendValue         `json:"legendValue,omitempty"`
	SliceLabels SliceLabels         `json:"sliceLabels,omitempty"`
	TotalBadge  TotalBadgePlacement `json:"totalBadge,omitempty"`
	ColorBy     ColorBy             `json:"colorBy,omitempty"`
	// Fill lets the plot occupy the whole card instead of the default inset.
	Fill bool `json:"fill,omitempty"`
	// BarWidthPx pins the rendered bar thickness in CSS pixels.
	BarWidthPx int `json:"barWidthPx,omitempty"`
	// BridgeLayout selects an alternate visual projection for a cascade.
	BridgeLayout BridgeLayout `json:"bridgeLayout,omitempty"`
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
	// Focus, when set on a drill-hosting panel, opts its drill flow into focus
	// chrome. Absent keeps today's in-card drill rendering.
	Focus FocusMode `json:"focus,omitempty"`
	// Stack draws a bar panel's series as one column per category instead of
	// one column per series. Only set it where the series really are parts of
	// a whole: a stack states that its segments add up, and a reader takes the
	// column height as that sum.
	Stack bool `json:"stack,omitempty"`
	// LineSeries names series drawn as a line over the bars. A measure on a
	// different basis — earned against written, actual against plan — belongs
	// beside the stack rather than inside it, where it would claim to be one of
	// its parts.
	LineSeries []string `json:"lineSeries,omitempty"`
}

// isZero reports whether no presentation hint is set. LineSeries makes
// Presentation uncomparable, so the check is spelled out rather than compared
// against a zero value.
func (p Presentation) isZero() bool {
	return len(p.LineSeries) == 0 && !p.DataLabels &&
		p.Legend == "" && p.LegendValue == "" && p.SliceLabels == "" &&
		p.TotalBadge == "" && p.ColorBy == "" && !p.Fill && p.BarWidthPx == 0 &&
		p.BridgeLayout == "" && p.Sortable == nil && p.Expandable == nil &&
		p.Exportable == nil && p.RowGroupField == "" && p.Focus == "" && !p.Stack
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
	// Heat shades numeric cells by their value relative to the returned frame.
	Heat bool `json:"heat,omitempty"`
	// SampleSizeField names the numeric observation-count column used to mark
	// low-confidence derived values without suppressing the number.
	SampleSizeField string `json:"sampleSizeField,omitempty"`
	// MinSampleSize is the exclusive threshold; zero is normalized to five.
	MinSampleSize int `json:"minSampleSize,omitempty"`
	// Total includes this numeric column in the server-computed footer.
	Total bool `json:"total,omitempty"`
	// ShareOf names the numeric source field whose filtered total is this
	// percentage column's denominator.
	ShareOf string `json:"shareOf,omitempty"`
}

type TableOptions struct {
	Searchable bool `json:"searchable,omitempty"`
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
	Previous string `json:"previous,omitempty"`
	Lower    string `json:"lower,omitempty"`
	Q1       string `json:"q1,omitempty"`
	Median   string `json:"median,omitempty"`
	Q3       string `json:"q3,omitempty"`
	Upper    string `json:"upper,omitempty"`
	ID       string `json:"id,omitempty"`
	Series   string `json:"series,omitempty"`
	Category string `json:"category,omitempty"`
	Cut      string `json:"cut,omitempty"`
	CutLabel string `json:"cutLabel,omitempty"`
	Final    string `json:"final,omitempty"`
	// Annotation names a frame column carrying an optional short, preformatted
	// stage note. Cascade renderers present non-empty values as compact badges.
	Annotation string `json:"annotation,omitempty"`
	// Tone names a frame column carrying a per-row semantic tone for a
	// cascade/waterfall stage: one of CascadeTone's values ("neutral",
	// "positive", "negative", "inflow" → navy / green / red / orange). It
	// overrides the flow-direction default color, so a deduction reads as a cost
	// (negative/red) instead of merely a decrease (green). An unknown or empty
	// per-row value falls back to the direction default. Optional; absent keeps
	// the panel's flow-direction coloring byte- and behavior-identical.
	Tone string `json:"tone,omitempty"`
	// Split names a frame column carrying the part of a cascade stage's own
	// movement that differs in kind from the rest of it — claims met out of a
	// product's reserve versus the overflow beyond it, say. Waterfall renderers
	// band it separately at the leading end of the same bar, so the composition
	// is read off the bar rather than from a caption beside it. It is a portion
	// OF the movement, never an addition to it: a value outside (0, |movement|)
	// is ignored and the bar renders whole. Optional; absent keeps solid bars.
	Split string `json:"split,omitempty"`
	// SplitLabel names a frame column naming that band. Renderers place it next
	// to the band; an empty value leaves the band drawn but unlabeled.
	SplitLabel string `json:"splitLabel,omitempty"`
	// Share names a frame column carrying a per-element share (already in the
	// units its format declares); the renderer never recomputes it.
	Share string `json:"share,omitempty"`
	// Confidence names a frame column carrying a per-element Confidence value
	// that overrides the structural default.
	Confidence string `json:"confidence,omitempty"`
	// Availability names a frame column carrying a per-element Availability
	// value that overrides the structural default.
	Availability string `json:"availability,omitempty"`
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
	// runtime in server-format parity mode: ASCII spaces instead of the
	// locale's non-breaking ones, and no space before a percent sign. The
	// server formatter uses a dot in every locale, so a document that must
	// match it byte for byte sets "." here.
	DecimalSeparator string `json:"decimalSeparator,omitempty"`
	// Symbol is the currency's display grapheme (UZS → "so’m"), taken from the
	// same pkg/money definition the server formatter uses. When set, money
	// renders as "<amount> <symbol>" instead of the locale's own currency
	// display for the ISO code.
	Symbol string `json:"symbol,omitempty"`
}

type ActionKind string

const (
	ActionNavigate       ActionKind = "navigate"
	ActionNavigateToLeaf ActionKind = "navigate_to_leaf"
	ActionOpenDrawer     ActionKind = "open_drawer"
	ActionEmitEvent      ActionKind = "emit_event"
	ActionCubeDrill      ActionKind = "cube_drill"
	ActionCrossFilter    ActionKind = "cross_filter"
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
	DrawerKey     *Source           `json:"drawerKey,omitempty"`
	Event         string            `json:"event,omitempty"`
	Params        []ActionParam     `json:"params"`
	Payload       map[string]Source `json:"payload"`
	PreserveQuery bool              `json:"preserveQuery,omitempty"`
	Filter        *ActionFilter     `json:"filter,omitempty"`
}

type ActionFilter struct {
	Dimension string `json:"dimension"`
	Value     Source `json:"value"`
	GroupBy   string `json:"groupBy,omitempty"`
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
	// DefaultPerspective identifies the producer-selected lens for a
	// perspective fork. Focus canvases use it to enter a segment directly
	// into a useful view while still exposing the sibling lenses as switches.
	DefaultPerspective string `json:"defaultPerspective,omitempty"`
	// View, when set, names the chart kind the runtime renders this level
	// with, instead of the drill host panel's own kind. Only chart kinds are
	// valid (no tables or metric panels); absent keeps host-kind rendering.
	View PanelKind `json:"view,omitempty"`
	// Presentation carries opt-in rendering hints for this level. Absent keeps
	// the renderer's default treatment.
	Presentation *Presentation `json:"presentation,omitempty"`
	// Status, when set, is a data-quality chip rendered next to this level's
	// heading (already localized by the producer).
	Status *PanelStatus `json:"status,omitempty"`
	// Source, when set, declares this level's audit table: the source rows
	// behind the level's aggregate, rendered behind a collapsed disclosure.
	Source *LevelSource `json:"source,omitempty"`
}

// LevelSource declares a drill level's audit table. Frame references the
// document's Frames; Columns and Format mirror a table panel's shapes so the
// runtime renders the disclosure exactly like a table panel body.
type LevelSource struct {
	// Label is the already-localized disclosure heading (e.g. "Source data").
	Label   string                 `json:"label,omitempty"`
	Frame   FrameRef               `json:"frame"`
	Columns []TableColumn          `json:"columns,omitempty"`
	Format  map[string]FieldFormat `json:"format,omitempty"`
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
	// Total is the authoritative whole this frame's rows are shares of, as
	// the producer computed it — not the sum of the rows shipped here.
	//
	// The two differ whenever the producer collapses a tail into an "other"
	// bucket, drops non-positive rows, or the viewer hides a series from the
	// legend. Renderers that derive a percentage must use this when present,
	// so a share means the same thing at every drill depth and stops moving
	// when the viewer toggles the legend.
	//
	// It rides on the frame rather than the level because a lazily loaded
	// level arrives as nothing but a frame: a total on Level would be absent
	// exactly where the drill goes deepest.
	Total *float64 `json:"total,omitempty"`
	// Presentation and Colors are the rendering decisions of the panel that
	// PRODUCED this frame, which is not always the panel that will draw it: in
	// document mode a drill level is served as a bare frame into a placeholder
	// panel frozen before anyone knew which dimension that level would render.
	// A level that draws years wants the year on the slice and the share in
	// the legend; the level below it draws quarters, or — after a remainder
	// step — years again. Only the producer knows, and only the frame reaches
	// the client, so the decisions travel with the data.
	//
	// Absent means "keep the panel's own"; they never merge field by field.
	Presentation *Presentation `json:"presentation,omitempty"`
	Colors       []string      `json:"colors,omitempty"`
}

type Endpoints struct {
	Query  string `json:"query,omitempty"`
	Panel  string `json:"panel,omitempty"`
	Drawer string `json:"drawer,omitempty"`
	Export string `json:"export,omitempty"`
}

type Theme struct {
	Palette    map[string]string `json:"palette"`
	Series     map[string]string `json:"series"`
	DebounceMS int               `json:"debounceMs,omitempty"`
}

type QueryRequest struct {
	SnapshotID  string     `json:"snapshotId"`
	Path        NodePath   `json:"path"`
	Perspective string     `json:"perspective,omitempty"`
	Prefetch    bool       `json:"prefetch,omitempty"`
	Page        int        `json:"page,omitempty"`
	Sort        *TableSort `json:"sort,omitempty"`
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

type PanelRequest struct {
	PanelID   string     `json:"panelId"`
	Recompute bool       `json:"recompute,omitempty"`
	Search    string     `json:"search,omitempty"`
	Sort      *TableSort `json:"sort,omitempty"`
	Page      int        `json:"page,omitempty"`
}

type SortDirection string

const (
	SortAscending  SortDirection = "asc"
	SortDescending SortDirection = "desc"
)

type TableSort struct {
	Field     string        `json:"field"`
	Direction SortDirection `json:"direction"`
}

type PanelBatchRequest struct {
	SnapshotID string         `json:"snapshotId"`
	Panels     []PanelRequest `json:"panels"`
}

// PanelCalculation describes the work that produced a panel frame. Duration
// is the complete scoped runtime execution, including its required datasets.
type PanelCalculation struct {
	DurationMS   int64     `json:"durationMs"`
	CacheHit     bool      `json:"cacheHit"`
	CalculatedAt time.Time `json:"calculatedAt"`
}

type PanelResponse struct {
	Frames      map[FrameRef]Frame `json:"frames"`
	Calculation PanelCalculation   `json:"calculation"`
	Summary     *TableSummary      `json:"summary,omitempty"`
	Page        *QueryPage         `json:"page,omitempty"`
}

type PanelBatchResult struct {
	Frames      map[FrameRef]Frame  `json:"frames,omitempty"`
	Calculation *PanelCalculation   `json:"calculation,omitempty"`
	Summary     *TableSummary       `json:"summary,omitempty"`
	Page        *QueryPage          `json:"page,omitempty"`
	Error       *QueryErrorResponse `json:"error,omitempty"`
}

type PanelBatchResponse struct {
	Panels map[string]PanelBatchResult `json:"panels"`
}

// PanelBatchStreamEvent is one newline-delimited event from the progressive
// panel endpoint. A panel event carries exactly one independently completed
// result. The final event has Complete set, which lets clients distinguish a
// clean stream from a truncated response.
type PanelBatchStreamEvent struct {
	PanelID  string            `json:"panelId,omitempty"`
	Result   *PanelBatchResult `json:"result,omitempty"`
	Complete bool              `json:"complete,omitempty"`
}

type DrawerResolveRequest struct {
	SnapshotID string         `json:"snapshotId"`
	MetricKey  string         `json:"metricKey"`
	Params     map[string]any `json:"params,omitempty"`
}

type DrawerResolveResponse struct {
	URL string `json:"url"`
}

type TableSummary struct {
	Values       map[string]any `json:"values"`
	FullValues   map[string]any `json:"fullValues,omitempty"`
	FilteredRows int            `json:"filteredRows"`
	TotalRows    int            `json:"totalRows"`
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
