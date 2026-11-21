// Package lens defines dashboard specs, datasets, and variable models for Lens.
package lens

import (
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
	"github.com/iota-uz/iota-sdk/pkg/lens/explore"
	"github.com/iota-uz/iota-sdk/pkg/lens/exportmeta"
	"github.com/iota-uz/iota-sdk/pkg/lens/frame"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/transform"
)

type DatasetKind string

const (
	DatasetKindQuery     DatasetKind = "query"
	DatasetKindTransform DatasetKind = "transform"
	DatasetKindJoin      DatasetKind = "join"
	DatasetKindUnion     DatasetKind = "union"
	DatasetKindFormula   DatasetKind = "formula"
	DatasetKindStatic    DatasetKind = "static"
)

type VariableKind string

const (
	VariableDateRange    VariableKind = "date_range"
	VariableSingleSelect VariableKind = "single_select"
	VariableMultiSelect  VariableKind = "multi_select"
	VariableText         VariableKind = "text"
	VariableNumber       VariableKind = "number"
	VariableToggle       VariableKind = "toggle"
	VariableCompare      VariableKind = "compare"
)

type VariableComponent string

const (
	VariableComponentDateRangePicker VariableComponent = "date_range_picker"
	VariableComponentSelect          VariableComponent = "select"
	VariableComponentMultiSelect     VariableComponent = "multi_select"
	VariableComponentTextInput       VariableComponent = "text_input"
	VariableComponentNumberInput     VariableComponent = "number_input"
	VariableComponentToggle          VariableComponent = "toggle"
	VariableComponentComparePicker   VariableComponent = "compare_picker"
	// VariableComponentFilterChips renders a date-range variable as a chip of
	// the shared filter builder (solid-ui FilterBuilder) instead of the
	// calendar trigger, so a host whose lists filter by chips reads the same
	// on its dashboard.
	VariableComponentFilterChips VariableComponent = "filter_chips"
)

type DashboardSpec struct {
	ID          string
	Revision    string
	Title       string
	Description string
	Rows        []RowSpec
	Variables   []VariableSpec
	Datasets    []DatasetSpec
	Drill       *DrillMeta
	Cache       CachePolicy
	Export      exportmeta.Spec
	Explorers   []explore.Spec
}

type CacheMode string

const (
	CacheDefault  CacheMode = "default"
	CacheDisabled CacheMode = "disabled"
)

// CachePolicy controls reuse of immutable execution snapshots. Zero values use
// the Runtime defaults.
type CachePolicy struct {
	Mode CacheMode
	TTL  time.Duration
}

// DrillMeta describes the active drill path and available dimensions.
type DrillMeta struct {
	BaseURL             string
	Dimensions          []DrillDimensionMeta
	Filters             []DrillFilterMeta
	RemainingDimensions []DrillDimensionMeta
	ActiveDimension     string
	GroupBy             string
}

type DrillDimensionMeta struct {
	Name  string
	Label string
}

type DrillFilterMeta struct {
	Dimension string
	Value     string
	Display   string
}

type DrillFacetOptionMeta struct {
	Dimension string
	Value     string
	Label     string
	Count     int
	Selected  bool
}

type RowSpec struct {
	Panels []panel.Spec
	Class  string
	// Anchor, when non-empty, becomes the rendered section's element id so the
	// row can be linked to with a fragment from elsewhere on the page.
	Anchor string
	// Heading, when non-empty, renders the row as a section header band
	// instead of a panel grid.
	Heading string
}

type VariableOption struct {
	Label string
	Value string
}

type VariableSpec struct {
	Name string
	// Bound keeps a host-resolved value immutable inside a composed execution.
	Bound           bool
	Label           string
	Kind            VariableKind
	Component       VariableComponent
	RequestKeys     []string
	Default         any
	Required        bool
	Description     string
	Options         []VariableOption
	AllowAllTime    bool
	DefaultDuration time.Duration
	// CompareTo names the date-range variable whose interval this comparison
	// derives from. Required for VariableCompare.
	CompareTo string
}

func DefaultVariableComponent(kind VariableKind) VariableComponent {
	switch kind {
	case VariableDateRange:
		return VariableComponentDateRangePicker
	case VariableSingleSelect:
		return VariableComponentSelect
	case VariableMultiSelect:
		return VariableComponentMultiSelect
	case VariableNumber:
		return VariableComponentNumberInput
	case VariableToggle:
		return VariableComponentToggle
	case VariableCompare:
		return VariableComponentComparePicker
	case VariableText:
		fallthrough
	default:
		return VariableComponentTextInput
	}
}

type DateRangeValue struct {
	Mode  string
	Start *time.Time
	End   *time.Time
}

type CompareMode string

const (
	CompareOff            CompareMode = "off"
	ComparePreviousPeriod CompareMode = "previous_period"
	CompareYearAgo        CompareMode = "year_ago"
	CompareCustom         CompareMode = "custom"
)

// CompareValue is the normalized comparison selection. Range is empty when
// Mode is off; every active mode resolves to an explicit bounded interval.
type CompareValue struct {
	Mode  CompareMode
	Range DateRangeValue
}

// CompareRequestKeys returns the canonical request keys for a comparison
// variable. Keeping this mapping on VariableSpec's owning package prevents the
// filter model, cube planner, and runtime resolver from drifting apart.
// The three results are the mode, start and end request keys, in that order.
func CompareRequestKeys(spec VariableSpec) (string, string, string) {
	if len(spec.RequestKeys) >= 3 {
		return spec.RequestKeys[0], spec.RequestKeys[1], spec.RequestKeys[2]
	}
	mode := CanonicalRequestKey(spec.Name)
	return mode, mode + "-start", mode + "-end"
}

// DateRangeRequestKeys returns the start and end request keys, in that order.
func DateRangeRequestKeys(spec VariableSpec) (string, string) {
	if len(spec.RequestKeys) >= 3 {
		return spec.RequestKeys[1], spec.RequestKeys[2]
	}
	if len(spec.RequestKeys) >= 2 {
		return spec.RequestKeys[0], spec.RequestKeys[1]
	}
	base := CanonicalRequestKey(spec.Name)
	return base + "-start", base + "-end"
}

func CanonicalRequestKey(name string) string {
	var result strings.Builder
	lastHyphen := true
	var previous rune
	for _, current := range strings.TrimSpace(name) {
		if unicode.IsLetter(current) || unicode.IsDigit(current) {
			if unicode.IsUpper(current) && !lastHyphen && (unicode.IsLower(previous) || unicode.IsDigit(previous)) {
				result.WriteByte('-')
			}
			result.WriteRune(unicode.ToLower(current))
			lastHyphen = false
			previous = current
			continue
		}
		if !lastHyphen && result.Len() > 0 {
			result.WriteByte('-')
			lastHyphen = true
		}
		previous = current
	}
	return strings.Trim(result.String(), "-")
}

// ResolveCompareMode normalizes the requested comparison mode. A custom mode
// is active only when both boundaries are valid and ordered; callers must not
// build or execute an unbounded comparison graph for malformed input.
func ResolveCompareMode(spec VariableSpec, values url.Values) CompareMode {
	modeKey, startKey, endKey := CompareRequestKeys(spec)
	raw := strings.TrimSpace(values.Get(modeKey))
	mode := CompareMode(raw)
	if raw == "" {
		if value, ok := spec.Default.(CompareValue); ok {
			mode = value.Mode
			if mode == CompareCustom && value.Range.Start != nil && value.Range.End != nil && !value.Range.End.Before(*value.Range.Start) {
				return mode
			}
		}
	}
	switch mode {
	case ComparePreviousPeriod, CompareYearAgo:
		return mode
	case CompareCustom:
		start, startErr := time.Parse("2006-01-02", values.Get(startKey))
		end, endErr := time.Parse("2006-01-02", values.Get(endKey))
		if startErr == nil && endErr == nil && !end.Before(start) {
			return mode
		}
	case CompareOff:
	}
	return CompareOff
}

// RangePart names the piece of a date-range variable a query parameter reads.
type RangePart string

const (
	// RangeStart is the first instant of the range.
	RangeStart RangePart = "start"
	// RangeEnd is the last instant of the range, inclusive. Databases that
	// store microseconds round the end of a day up to the next midnight, so a
	// SQL upper bound should use RangeUntil.
	RangeEnd RangePart = "end"
	// RangeUntil is the first instant after the range: the exclusive upper
	// bound for `column >= @since AND column < @until`.
	RangeUntil RangePart = "until"
)

// ParamValue is one named query parameter: a literal, or the value of a
// dashboard variable. A date-range variable is a structure the database driver
// cannot take, so name the part the query needs with Part (RangeParam); an
// all-time range, which has no bounds, resolves to NULL.
type ParamValue struct {
	Literal  any
	Variable string
	Part     RangePart `json:"Part,omitempty"`
}

// VariableParam binds a query parameter to a scalar variable.
func VariableParam(variable string) ParamValue { return ParamValue{Variable: variable} }

// RangeParam binds a query parameter to one end of a date-range variable:
//
//	Params: map[string]lens.ParamValue{
//		"since": lens.RangeParam("period", lens.RangeStart),
//		"until": lens.RangeParam("period", lens.RangeUntil),
//	}
func RangeParam(variable string, part RangePart) ParamValue {
	return ParamValue{Variable: variable, Part: part}
}

// ResolveRangePart reads one end of a date-range value (or of a comparison's
// range). It returns nil for an unbounded end, so the parameter reaches SQL as
// NULL and `@since IS NULL OR ...` expresses "all time". The second result is
// false when value is not a date range or part is unknown.
func ResolveRangePart(value any, part RangePart) (any, bool) {
	if comparison, ok := value.(CompareValue); ok {
		value = comparison.Range
	}
	var dateRange DateRangeValue
	switch typed := value.(type) {
	case DateRangeValue:
		dateRange = typed
	case *DateRangeValue:
		if typed == nil {
			return nil, false
		}
		dateRange = *typed
	default:
		return nil, false
	}
	switch part {
	case RangeStart:
		if dateRange.Start == nil {
			return nil, true
		}
		return *dateRange.Start, true
	case RangeEnd:
		if dateRange.End == nil {
			return nil, true
		}
		return *dateRange.End, true
	case RangeUntil:
		if dateRange.End == nil {
			return nil, true
		}
		return dateRange.End.Add(time.Nanosecond), true
	default:
		return nil, false
	}
}

type QuerySpec struct {
	Text    string
	Params  map[string]ParamValue
	Kind    datasource.QueryKind
	MaxRows int
}

// ComparisonAlignment controls how rows from a comparison period are paired
// with the current dataset. The zero value preserves the inferred identity
// join; ordinal alignment is an explicit opt-in for ordered datasets whose
// category values necessarily differ between periods.
type ComparisonAlignment string

const (
	ComparisonAlignmentInferred ComparisonAlignment = ""
	ComparisonAlignmentOrdinal  ComparisonAlignment = "ordinal"
)

type DatasetSpec struct {
	Name        string
	Title       string
	Kind        DatasetKind
	Source      string
	DependsOn   []string
	Query       *QuerySpec
	Transforms  []transform.Spec
	Static      *frame.FrameSet
	Description string
	Cache       CachePolicy
	Export      exportmeta.Spec
	// TimeRangeVariable overrides the dashboard's primary date range for this
	// dataset. Comparison branches use it to execute the same query against a
	// second normalized interval.
	TimeRangeVariable string
	// ComparisonAlignment may explicitly opt an ordered dataset into ordinal
	// comparison. The default remains a stable identity join and never falls
	// back to row position implicitly.
	ComparisonAlignment ComparisonAlignment
}

func ResolveTimeRange(value any) datasource.TimeRange {
	if comparison, ok := value.(CompareValue); ok {
		value = comparison.Range
	}
	dateRange, ok := value.(DateRangeValue)
	if !ok {
		return datasource.TimeRange{}
	}
	return datasource.TimeRange{
		Start: dateRange.Start,
		End:   dateRange.End,
		Mode:  datasource.TimeRangeMode(dateRange.Mode),
	}
}

func TopLevelPanels(spec DashboardSpec) []panel.Spec {
	panels := make([]panel.Spec, 0)
	for _, row := range spec.Rows {
		panels = append(panels, row.Panels...)
	}
	return panels
}

func FlattenPanels(spec DashboardSpec) []panel.Spec {
	panels := make([]panel.Spec, 0)
	for _, row := range spec.Rows {
		for _, panelSpec := range row.Panels {
			panels = append(panels, flattenPanel(panelSpec)...)
		}
	}
	return panels
}

func FindPanel(spec DashboardSpec, panelID string) (panel.Spec, bool) {
	for _, row := range spec.Rows {
		for _, panelSpec := range row.Panels {
			if found, ok := findPanel(panelSpec, panelID); ok {
				return found, true
			}
		}
	}
	return panel.Spec{}, false
}

// ApplyExportDefaults enables zero-config panel exports from the dashboard
// endpoint while preserving panel/dataset evidence overrides.
func ApplyExportDefaults(spec *DashboardSpec) {
	if spec == nil || !spec.Export.Enabled {
		return
	}
	var apply func([]panel.Spec)
	apply = func(items []panel.Spec) {
		for i := range items {
			if !items[i].Kind.IsContainer() {
				if !items[i].Export.Enabled {
					items[i].Export.Enabled = true
				}
				if items[i].Export.URL == "" {
					items[i].Export.URL = spec.Export.URL
				}
				if items[i].Export.Filename == "" {
					items[i].Export.Filename = spec.Export.Filename
				}
			}
			apply(items[i].Children)
		}
	}
	for i := range spec.Rows {
		apply(spec.Rows[i].Panels)
	}
}

func flattenPanel(spec panel.Spec) []panel.Spec {
	panels := []panel.Spec{spec}
	for _, child := range spec.Children {
		panels = append(panels, flattenPanel(child)...)
	}
	return panels
}

func findPanel(spec panel.Spec, panelID string) (panel.Spec, bool) {
	if spec.ID == panelID {
		return spec, true
	}
	for _, child := range spec.Children {
		if found, ok := findPanel(child, panelID); ok {
			return found, true
		}
	}
	return panel.Spec{}, false
}
