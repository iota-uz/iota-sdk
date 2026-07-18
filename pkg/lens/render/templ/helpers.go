package templ

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	templpkg "github.com/a-h/templ"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/js"
	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/cube"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/lens/theme"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var actionPlaceholderPattern = regexp.MustCompile(`\{[a-zA-Z0-9_]+\}`)

func normalizedSpan(span int) int {
	if span < 1 {
		return 12
	}
	if span > 12 {
		return 12
	}
	return span
}

func panelSpanStyle(span int) templpkg.SafeCSS {
	return templpkg.SafeCSS("--lens-col-span:" + strconv.Itoa(normalizedSpan(span)))
}

func panelExportURL(spec panel.Spec) string {
	base := strings.TrimSpace(spec.Export.URL)
	if base == "" {
		return ""
	}
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	query := u.Query()
	query.Set("panel", spec.ID)
	u.RawQuery = query.Encode()
	return u.String()
}

func panelResult(result *runtime.Result, panelID string) *runtime.PanelResult {
	if result == nil {
		return nil
	}
	return result.Panel(panelID)
}

type drillNavModel struct {
	HasNav         bool
	Include        string
	CurrentTitle   string
	CurrentValue   string
	CurrentLabel   string
	CurrentDisplay string
	UpURL          string
	UpLabel        string
	Trail          []drillNavCrumb
	Summary        []drillSummaryItem
	Remaining      []drillDimensionTab
}

type drillNavCrumb struct {
	URL   string
	Label string
}

type drillDimensionTab struct {
	Name        string
	Label       string
	URL         string
	FacetURL    string
	Active      bool
	ActiveCount int
}

type drillSummaryItem struct {
	Label string
	Value string
	URL   string
}

type panelErrorModel struct {
	PanelID string
	Reason  string
	Action  *PanelErrorAction
}

func drillNavigationModel(ctx context.Context, result *runtime.Result) drillNavModel {
	return drillNavigationModelWithInclude(ctx, result, "")
}

func drillNavigationModelWithInclude(ctx context.Context, result *runtime.Result, include string) drillNavModel {
	if result == nil || result.Drill == nil || result.Spec.Drill == nil {
		return drillNavModel{}
	}
	state := result.Drill
	meta := result.Spec.Drill
	baseQuery := drillBaseQueryValues(result.Request)
	labels := map[string]string{}
	for _, dim := range meta.Dimensions {
		labels[dim.Name] = dim.Label
	}
	model := drillNavModel{
		HasNav:  true,
		Include: strings.TrimSpace(include),
		UpURL:   drillURL(meta.BaseURL, baseQuery, nil, meta.GroupBy),
	}
	for idx, filter := range state.Filters {
		for _, value := range normalizedFilterValues(filter) {
			itemFilter := cube.DimensionFilter{Dimension: filter.Dimension, Value: value, Values: []string{value}}
			model.Summary = appendDrillSummary(
				model.Summary,
				firstNonEmptyString(labels[filter.Dimension], filter.Dimension),
				drillFilterDisplay(meta, idx, itemFilter),
				drillURL(meta.BaseURL, baseQuery, state.ToggleFilter(filter.Dimension, value).Filters, meta.GroupBy),
			)
		}
	}
	activeDim := meta.GroupBy
	if activeDim == "" {
		activeDim = meta.ActiveDimension
	}
	if activeDim == "" && len(meta.RemainingDimensions) > 0 {
		activeDim = meta.RemainingDimensions[0].Name
	}
	for _, dim := range meta.RemainingDimensions {
		model.Remaining = append(model.Remaining, drillDimensionTab{
			Name:        dim.Name,
			Label:       dim.Label,
			URL:         dimensionTabURL(meta.BaseURL, baseQuery, state.Filters, dim.Name),
			FacetURL:    facetOptionsURL(meta.BaseURL, baseQuery, state.Filters, activeDim, dim.Name),
			Active:      dim.Name == activeDim,
			ActiveCount: activeFilterCount(state.Filters, dim.Name),
		})
	}
	return model
}

// activeFilterCount returns how many distinct values are currently filtered for
// the given dimension, so a facet trigger can show a "·N" badge.
func activeFilterCount(filters []cube.DimensionFilter, dimension string) int {
	seen := make(map[string]struct{})
	for _, filter := range filters {
		if filter.Dimension == dimension {
			for _, v := range normalizedFilterValues(filter) {
				seen[v] = struct{}{}
			}
		}
	}
	return len(seen)
}

// facetOptionsOrdered returns the options with currently-selected values floated
// to the top (stable otherwise) so a multi-select dropdown never hides a checked
// item below the scroll/search fold.
func facetOptionsOrdered(options []lens.DrillFacetOptionMeta) []lens.DrillFacetOptionMeta {
	ordered := make([]lens.DrillFacetOptionMeta, 0, len(options))
	for _, option := range options {
		if option.Selected {
			ordered = append(ordered, option)
		}
	}
	for _, option := range options {
		if !option.Selected {
			ordered = append(ordered, option)
		}
	}
	return ordered
}

// facetMaxCount is the largest option count, used to scale the magnitude bars.
func facetMaxCount(options []lens.DrillFacetOptionMeta) int {
	maxCount := 0
	for _, option := range options {
		if option.Count > maxCount {
			maxCount = option.Count
		}
	}
	return maxCount
}

// facetBarPercent scales an option count to a 0-100 width for its magnitude bar,
// keeping a visible sliver for any non-zero count.
func facetBarPercent(count, maxCount int) int {
	if maxCount <= 0 || count <= 0 {
		return 0
	}
	percent := count * 100 / maxCount
	if percent < 3 {
		percent = 3
	}
	return percent
}

// facetBarStyle is the inline width style for an option's magnitude bar.
func facetBarStyle(count, maxCount int) string {
	return fmt.Sprintf("width:%d%%", facetBarPercent(count, maxCount))
}

func drillNavigationModelFromSpecWithInclude(ctx context.Context, spec lens.DashboardSpec, include string) drillNavModel {
	if spec.Drill == nil {
		return drillNavModel{}
	}
	state := cube.DrillContext{
		GroupBy:         spec.Drill.GroupBy,
		ActiveDimension: spec.Drill.ActiveDimension,
	}
	for _, filter := range spec.Drill.Filters {
		state = state.ToggleFilter(filter.Dimension, filter.Value)
	}
	return drillNavigationModelWithInclude(ctx, &runtime.Result{
		Spec:    spec,
		Drill:   &state,
		Request: url.Values{},
	}, include)
}

func drillFilterDisplay(meta *lens.DrillMeta, idx int, filter cube.DimensionFilter) string {
	if meta != nil && idx >= 0 && idx < len(meta.Filters) {
		item := meta.Filters[idx]
		if item.Dimension == filter.Dimension && item.Value == filter.Value && strings.TrimSpace(item.Display) != "" {
			return item.Display
		}
	}
	if meta != nil {
		for _, item := range meta.Filters {
			if item.Dimension == filter.Dimension && item.Value == filter.Value && strings.TrimSpace(item.Display) != "" {
				return item.Display
			}
		}
	}
	return filter.Value
}

func drillBaseQueryValues(values url.Values) url.Values {
	base := cloneURLValues(values)
	delete(base, cube.QueryFilter)
	delete(base, cube.QueryDimension)
	delete(base, cube.QueryGroupBy)
	delete(base, cube.QueryFacet)
	delete(base, cube.QueryFacetSearch)
	return base
}

func drillURL(baseURL string, baseQuery url.Values, filters []cube.DimensionFilter, groupBy string) string {
	values := cube.DrillContext{Filters: filters, GroupBy: groupBy}.WithValues(baseQuery)
	return joinURLQuery(baseURL, values)
}

func dimensionTabURL(baseURL string, baseQuery url.Values, filters []cube.DimensionFilter, dimensionName string) string {
	values := cube.DrillContext{Filters: filters, GroupBy: dimensionName}.WithValues(baseQuery)
	return joinURLQuery(baseURL, values)
}

func facetOptionsURL(baseURL string, baseQuery url.Values, filters []cube.DimensionFilter, groupBy, dimensionName string) string {
	values := cube.DrillContext{Filters: filters, GroupBy: groupBy}.WithValues(baseQuery)
	values.Set(cube.QueryFacet, dimensionName)
	return joinURLQuery(baseURL, values)
}

func facetSearchIncludeSelector(include string) string {
	include = strings.TrimSpace(include)
	if include == "" {
		return "closest form"
	}
	return "closest form, " + include
}

func appendDrillSummary(summary []drillSummaryItem, label, value, itemURL string) []drillSummaryItem {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if value == "" {
		return summary
	}
	if len(summary) > 0 {
		last := summary[len(summary)-1]
		if strings.EqualFold(strings.TrimSpace(last.Label), label) && strings.EqualFold(strings.TrimSpace(last.Value), value) {
			return summary
		}
	}
	return append(summary, drillSummaryItem{Label: label, Value: value, URL: itemURL})
}

func normalizedFilterValues(filter cube.DimensionFilter) []string {
	values := filter.Values
	if len(values) == 0 && strings.TrimSpace(filter.Value) != "" {
		values = []string{filter.Value}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func tableColumns(spec panel.Spec, result *runtime.PanelResult) []panel.TableColumn {
	if len(spec.Columns) > 0 {
		return spec.Columns
	}
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil {
		return nil
	}
	columns := make([]panel.TableColumn, 0, len(result.Frames.Primary().Fields))
	for _, field := range result.Frames.Primary().Fields {
		columns = append(columns, panel.TableColumn{Field: panel.Ref(field.Name), Label: field.Name})
	}
	return columns
}

// tableWrapperStyle controls whether a Table panel's scroll container is
// bounded. Panels marked with the "lens-table-scroll" ClassName token get a
// fixed max height with a scrolling body (the header row stays sticky);
// every other table only clips overflow without capping height. The cap is
// an inline style rather than a Tailwind arbitrary-value class because
// consumer apps compile their CSS against the *published* SDK sources — a
// class only this template uses would be purged from their builds.
//
// The cap reads from a CSS custom property (falling back to 26rem) rather
// than a hardcoded value so PanelFullscreenOverlay can relax it: the same
// compact table that should stay bounded next to a short card in the normal
// dashboard grid should instead fill the much taller fullscreen card.
func tableWrapperStyle(spec panel.Spec) templpkg.SafeCSS {
	if panelHasClass(spec, "lens-table-scroll") {
		return "max-height:var(--lens-table-scroll-cap, 26rem)"
	}
	return ""
}

// tableWrapperClass is the scroll container's class list. Only the bounded
// "lens-table-scroll" variant gets h-full: it has a capped max-height (see
// tableWrapperStyle) that PanelFullscreenOverlay can relax, so h-full is
// what lets it actually grow into that relaxed space. The default
// (uncapped) table variant must stay height:auto — giving it h-full would
// clip it to whatever height its card happens to resolve to instead of
// sizing to its content, which is how every other table in the app expects
// to render.
func tableWrapperClass(spec panel.Spec) string {
	if panelHasClass(spec, "lens-table-scroll") {
		return "h-full overflow-auto"
	}
	return "overflow-auto"
}

// tableIsCompact reports whether a table opted into the denser treatment
// that pairs with a bounded scrolling body: tighter cell padding and a
// truncated first column, so a many-column breakdown fits a half-width panel.
func tableIsCompact(spec panel.Spec) bool {
	return panelHasClass(spec, "lens-table-scroll")
}

// tableColumnIsNumeric reports whether a column should be right-aligned: an
// explicit Align=="right", or a numeric formatter kind (money, abbreviated
// money, integer, percent).
func tableColumnIsNumeric(column panel.TableColumn) bool {
	if column.Align == "right" {
		return true
	}
	if column.Formatter == nil {
		return false
	}
	switch column.Formatter.Kind {
	case format.KindMoney, format.KindAbbreviatedMoney, format.KindInteger, format.KindPercent:
		return true
	case format.KindDate, format.KindMonthLabel, format.KindDuration, format.KindLocalizedString:
		return false
	}
	return false
}

func tableHeaderCellClass(spec panel.Spec, column panel.TableColumn) string {
	base := "lens-th whitespace-nowrap"
	if tableColumnIsNumeric(column) {
		return base + " lens-th--num"
	}
	return base
}

func tableCellClass(spec panel.Spec, column panel.TableColumn, first bool) string {
	base := "lens-td whitespace-nowrap"
	if first {
		base += " lens-td--strong"
	}
	if tableColumnIsNumeric(column) {
		base += " lens-td--num lens-num"
	}
	return base
}

// tableColumnWidthStyle is the inline min-width for a column with WidthPx set.
func tableColumnWidthStyle(column panel.TableColumn) templpkg.SafeCSS {
	if column.WidthPx <= 0 {
		return ""
	}
	return templpkg.SafeCSS("min-width:" + strconv.Itoa(column.WidthPx) + "px")
}

// tableElementClass is the <table> class list; the lens-table-sticky-first
// ClassName token passes through so the first column pins during horizontal
// scroll.
func tableElementClass(spec panel.Spec) string {
	base := "min-w-full w-full text-[13px]"
	if panelHasClass(spec, "lens-table-sticky-first") {
		base += " lens-table-sticky-first"
	}
	// Only the bounded "lens-table-scroll" variant stretches to fill its
	// wrapper's resolved height (see tableWrapperClass) — a short row count
	// there shouldn't leave the card looking unfinished, and the browser's
	// table layout distributes the extra height across existing rows rather
	// than adding blank space below them. When rows exceed the wrapper's
	// height this has no visible effect: the table overflows and the
	// wrapper's own scroll behavior still applies. The default (uncapped)
	// variant is left at its natural height, matching every other table.
	if panelHasClass(spec, "lens-table-scroll") {
		base += " h-full"
	}
	return base
}

// tableColumnBarMax computes, once per render, the max absolute numeric
// value of each TableCellBar column across every row, so each cell can scale
// its mini-bar against a shared denominator instead of re-scanning rows.
func tableColumnBarMax(columns []panel.TableColumn, rows []map[string]any) map[panel.FieldRef]float64 {
	out := make(map[panel.FieldRef]float64)
	for _, column := range columns {
		if column.Cell == nil || column.Cell.Kind != panel.TableCellBar {
			continue
		}
		maxAbs := 0.0
		for _, row := range rows {
			abs := math.Abs(segmentNumeric(row[column.Field.Name()]))
			if abs > maxAbs {
				maxAbs = abs
			}
		}
		out[column.Field] = maxAbs
	}
	return out
}

// tableBarCellFloorPct keeps a non-zero bar-cell value from rendering an
// invisible sliver next to the column's max value.
const tableBarCellFloorPct = 3.0

type tableBarCellView struct {
	Text string
	// TextStyle / FillStyle are inline styles built on the lens status vars
	// (var(--lens-pos)/var(--lens-neg)) so the treatment renders regardless of
	// the consumer's Tailwind build.
	TextStyle templpkg.SafeCSS
	FillStyle templpkg.SafeCSS
}

func buildTableBarCell(column panel.TableColumn, row map[string]any, result *runtime.PanelResult, maxAbs float64) tableBarCellView {
	value := segmentNumeric(row[column.Field.Name()])
	fill := "var(--lens-pos)"
	view := tableBarCellView{
		Text:      formatValue(row[column.Field.Name()], column.Formatter, result.Locale, result.Timezone),
		TextStyle: "color:var(--lens-text)",
	}
	if value < 0 {
		view.TextStyle = "color:var(--lens-neg)"
		fill = "var(--lens-neg)"
	}
	pct := 0.0
	if value != 0 && maxAbs > 0 {
		pct = math.Abs(value) / maxAbs * 100
		if pct > 100 {
			pct = 100
		}
		if pct < tableBarCellFloorPct {
			pct = tableBarCellFloorPct
		}
	}
	view.FillStyle = templpkg.SafeCSS(string(cascadeWidthStyle(pct)) + ";background-color:" + fill)
	return view
}

type tableDeltaCellView struct {
	// PctText is the primary line ("+22.2%"); AmountText the secondary
	// absolute change beneath it. Two lines keep the column narrow enough
	// to coexist with four numeric columns in a half-width panel.
	PctText    string
	AmountText string
	Style      templpkg.SafeCSS
}

func buildTableDeltaCell(column panel.TableColumn, row map[string]any, result *runtime.PanelResult) tableDeltaCellView {
	delta := segmentNumeric(row[column.Field.Name()])
	pct := 0.0
	if column.Cell != nil {
		pct = segmentNumeric(row[column.Cell.PercentField.Name()])
	}
	sign := ""
	style := templpkg.SafeCSS("color:var(--lens-text-faint)")
	switch {
	case delta > 0:
		sign = "+"
		style = "color:var(--lens-pos)"
	case delta < 0:
		sign = cascadeMinusSign
		style = "color:var(--lens-neg)"
	}
	amountText := formatValue(math.Abs(delta), column.Formatter, result.Locale, result.Timezone)
	pctText := strconv.FormatFloat(math.Abs(pct), 'f', 1, 64)
	return tableDeltaCellView{
		PctText:    sign + pctText + "%",
		AmountText: sign + amountText,
		Style:      style,
	}
}

func tableRowContainerID(spec panel.Spec) string {
	if panelHasClass(spec, "lens-card-list") {
		return spec.ID + "-cards"
	}
	return spec.ID + "-rows"
}

func tableSentinelID(spec panel.Spec) string {
	return spec.ID + "-pagination"
}

func tableIndicatorID(spec panel.Spec) string {
	return spec.ID + "-pagination-indicator"
}

func tablePagination(result *runtime.PanelResult) *runtime.TablePagination {
	if result == nil || result.TablePagination == nil {
		return nil
	}
	page := result.TablePagination.Page
	if page < runtime.DefaultTablePage {
		page = runtime.DefaultTablePage
	}
	perPage := result.TablePagination.PerPage
	if perPage < 1 {
		perPage = runtime.DefaultTablePerPage
	}
	return &runtime.TablePagination{
		Page:    page,
		PerPage: perPage,
		HasMore: result.TablePagination.HasMore,
	}
}

func tablePaginationURL(result *runtime.PanelResult) string {
	pagination := tablePagination(result)
	if result == nil || pagination == nil || !pagination.HasMore {
		return ""
	}
	path := strings.TrimSpace(result.RequestPath)
	if path == "" {
		return ""
	}
	values := cloneURLValues(result.Request)
	values.Set(runtime.TablePaginationPanelQuery, strings.TrimSpace(result.Panel.ID))
	values.Set(runtime.TablePaginationPageQuery, strconv.Itoa(pagination.Page+1))
	values.Set(runtime.TablePaginationLimitQuery, strconv.Itoa(pagination.PerPage))
	return joinURLQuery(path, values)
}

func statRawValue(spec panel.Spec, result *runtime.PanelResult) any {
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil || result.Frames.Primary().RowCount == 0 {
		return "-"
	}
	rows := result.Frames.Primary().Rows()
	fieldName := spec.Fields.Value
	if fieldName.Empty() {
		fieldName = panel.DefaultValueField
	}
	return rows[0][fieldName.Name()]
}

func statRow(result *runtime.PanelResult) map[string]any {
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil || result.Frames.Primary().RowCount == 0 {
		return nil
	}
	rows := result.Frames.Primary().Rows()
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// ---- Stat card v2 ----

// statView is the resolved data one .lens-stat block renders.
type statView struct {
	Value string
	// Zero demotes the card (lens-stat--zero) and suppresses trend +
	// sparkline when the primary value is 0 or absent.
	Zero bool
	// Swatch is the sanitized accent color for the 8x8 label-row swatch;
	// empty means no swatch.
	Swatch string
	// SparkPoints is the pre-computed polyline points attribute for the
	// native sparkline SVG; empty means no sparkline.
	SparkPoints string
	SparkStyle  templpkg.SafeCSS
}

// sparklineViewBox dimensions must match the viewBox on the .lens-spark SVG.
const (
	sparklineWidth  = 72.0
	sparklineHeight = 22.0
)

func buildStatView(spec panel.Spec, result *runtime.PanelResult) statView {
	raw := statRawValue(spec, result)
	view := statView{
		Value: formatValue(raw, spec.Formatter, result.Locale, result.Timezone),
		Zero:  statValueIsZero(raw),
	}
	if color := strings.TrimSpace(spec.Chrome.AccentColor); color != "" {
		r, g, b := parseHexColor(color)
		view.Swatch = fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
	if !view.Zero && spec.Sparkline != nil {
		view.SparkPoints = sparklinePoints(spec.Sparkline.Values, sparklineWidth, sparklineHeight)
		view.SparkStyle = templpkg.SafeCSS("stroke:" + sparklineStroke(spec.Sparkline.Color))
	}
	return view
}

// sparklineStroke sanitizes the sparkline color: hex colors are normalized,
// anything else falls back to the accent token (the value lands inside an
// inline style, so it must never carry arbitrary CSS).
func sparklineStroke(color string) string {
	color = strings.TrimSpace(color)
	if strings.HasPrefix(color, "#") {
		r, g, b := parseHexColor(color)
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}
	return "var(--lens-accent-500)"
}

// sparklinePoints maps values onto the sparkline viewbox, min..max normalized
// vertically with padding so the stroke never clips.
func sparklinePoints(values []float64, width, height float64) string {
	if len(values) < 2 {
		return ""
	}
	minV, maxV := values[0], values[0]
	for _, v := range values {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	const pad = 1.5
	span := maxV - minV
	usableH := height - 2*pad
	stepX := width / float64(len(values)-1)
	var b strings.Builder
	for i, v := range values {
		x := stepX * float64(i)
		y := height / 2
		if span > 0 {
			y = pad + (1-(v-minV)/span)*usableH
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.FormatFloat(x, 'f', 1, 64))
		b.WriteByte(',')
		b.WriteString(strconv.FormatFloat(y, 'f', 1, 64))
	}
	return b.String()
}

// statValueIsZero reports whether the stat's primary value is zero/absent, in
// which case the card demotes itself (lens-stat--zero) and drops trend +
// sparkline noise.
func statValueIsZero(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case float64:
		return v == 0
	case float32:
		return v == 0
	case int:
		return v == 0
	case int32:
		return v == 0
	case int64:
		return v == 0
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" || trimmed == "-" {
			return true
		}
		if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return parsed == 0
		}
		return false
	}
	return false
}

func statClass(zero, clickable bool) string {
	base := "lens-stat h-full"
	if zero {
		base += " lens-stat--zero"
	}
	if clickable {
		// Mirror the pre-v2 behavior: the body ignores pointer events so the
		// inset overlay link receives the click; fade slightly for affordance.
		base += " relative z-10 pointer-events-none transition-opacity group-hover:opacity-90"
	}
	return base
}

func statusChipClass(tone panel.StatusTone) string {
	switch tone {
	case panel.StatusPositive:
		return "lens-chip lens-chip--positive"
	case panel.StatusWarning:
		return "lens-chip lens-chip--warning"
	case panel.StatusNeutral:
		return "lens-chip"
	}
	return "lens-chip"
}

// statTrendClass colors the trend chip: up=green/down=red by default, flipped
// when TrendSpec.Invert marks the metric down-is-good. The arrow (▲/▼) always
// follows the raw sign via trendArrow.
func statTrendClass(trend panel.TrendSpec) string {
	positive := trend.Percent > 0
	negative := trend.Percent < 0
	if trend.Invert {
		positive, negative = negative, positive
	}
	switch {
	case positive:
		return "lens-trend lens-trend--up"
	case negative:
		return "lens-trend lens-trend--down"
	}
	return "lens-trend"
}

// ---- Stat group ----

func statGroupContainerStyle(spec panel.Spec) templpkg.SafeCSS {
	if spec.GroupLayout == panel.GroupRows {
		return "display:flex;flex-direction:column"
	}
	return "display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr))"
}

func statGroupItemStyle(spec panel.Spec, index int) templpkg.SafeCSS {
	if index == 0 {
		return "min-width:0"
	}
	if spec.GroupLayout == panel.GroupRows {
		return "min-width:0;border-top:1px solid var(--lens-divider)"
	}
	return "min-width:0;border-left:1px solid var(--lens-divider)"
}

// panelResultAllZero reports whether every row's value field is zero — used to
// route pie/donut/gauge panels whose slices are all zero to the empty state
// instead of a blank white chart.
func panelResultAllZero(spec panel.Spec, result *runtime.PanelResult) bool {
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil {
		return false
	}
	rows := result.Frames.Primary().Rows()
	if len(rows) == 0 {
		return false
	}
	valueField := spec.Fields.Value
	if valueField.Empty() {
		valueField = panel.DefaultValueField
	}
	for _, row := range rows {
		if segmentNumeric(row[valueField.Name()]) != 0 {
			return false
		}
	}
	return true
}

// panelCardNeedsFullscreenScope reports whether the panel card carries the
// Alpine `{ fullscreen: false }` scope + rerender-scope marker. Chart and
// tabs cards need it so the header's fullscreen button (rendered by
// panelCard, outside the body) and the overlay (inside the body) share one
// scope that also survives HTMX body swaps.
func panelCardNeedsFullscreenScope(spec panel.Spec) bool {
	return spec.Kind.IsChart() || spec.Kind == panel.KindTabs
}

// segmentBarSegment is one part of a part-to-whole segment bar.
type segmentBarSegment struct {
	Label   string
	Amount  string  // formatted via the panel formatter
	Raw     float64 // unformatted amount (drives bar width + zero styling)
	Pct     float64 // share of the whole, 0..100
	PctTxt  string  // "100%", "0%", "<1%"
	Color   string
	Href    string
	OnClick templpkg.ComponentScript
}

// segmentBarView is the resolved data a SegmentBar panel renders: a headline
// total plus its constituent segments.
type segmentBarView struct {
	HasData  bool
	Total    string // formatted sum of all segments
	Caption  string
	Segments []segmentBarSegment
}

// segmentBarPalette is the default colour ramp when a SegmentBar panel does
// not supply its own Colors. Calm-to-warm so the first segment reads as the
// healthy share and later ones as overflow.
var segmentBarPalette = []string{"#2563eb", "#f59e0b", "#dc2626", "#7c3aed", "#0891b2"}

func buildSegmentBarView(spec panel.Spec, result *runtime.PanelResult) segmentBarView {
	view := segmentBarView{Caption: strings.TrimSpace(spec.Description)}
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil {
		return view
	}
	rows := result.Frames.Primary().Rows()
	if len(rows) == 0 {
		return view
	}
	// Validation accepts a SegmentBar with either Label or Category set, so
	// fall back to Category before the default to honour a category-only spec
	// (otherwise its segments would render "<nil>" labels from the default
	// "label" column that the dataset never produced).
	labelField := spec.Fields.Label
	if labelField.Empty() {
		labelField = spec.Fields.Category
	}
	if labelField.Empty() {
		labelField = panel.DefaultLabelField
	}
	valueField := spec.Fields.Value
	if valueField.Empty() {
		valueField = panel.DefaultValueField
	}

	raws := make([]float64, len(rows))
	labels := make([]string, len(rows))
	var total float64
	for i, row := range rows {
		raws[i] = segmentNumeric(row[valueField.Name()])
		labels[i] = strings.TrimSpace(fmt.Sprint(row[labelField.Name()]))
		total += raws[i]
	}

	view.HasData = true
	headline := total
	if spec.HeadlineValue != nil {
		headline = *spec.HeadlineValue
	}
	view.Total = formatValue(headline, spec.Formatter, result.Locale, result.Timezone)
	view.Segments = make([]segmentBarSegment, len(rows))
	for i := range rows {
		pct := 0.0
		if total > 0 {
			pct = raws[i] / total * 100
		}
		view.Segments[i] = segmentBarSegment{
			Label:   labels[i],
			Amount:  formatValue(raws[i], spec.Formatter, result.Locale, result.Timezone),
			Raw:     raws[i],
			Pct:     pct,
			PctTxt:  formatSharePct(raws[i], pct),
			Color:   segmentColorAt(spec.Colors, i),
			Href:    actionURL(spec.Action, rows[i], result),
			OnClick: actionOnClick(spec.Action, rows[i], result),
		}
	}
	return view
}

func segmentColorAt(colors []string, i int) string {
	raw := segmentBarPalette[i%len(segmentBarPalette)]
	if i < len(colors) && strings.TrimSpace(colors[i]) != "" {
		raw = colors[i]
	}
	r, g, b := parseHexColor(raw)
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// formatSharePct renders a segment's share as a compact integer percent,
// guarding the "rounds to 0% but is non-zero" case so a real overflow never
// reads as nothing.
func formatSharePct(raw, pct float64) string {
	switch {
	case raw > 0 && pct < 1:
		return "<1%"
	case raw <= 0:
		return "0%"
	default:
		return strconv.FormatFloat(math.Round(pct), 'f', 0, 64) + "%"
	}
}

func segmentNumeric(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case int32:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	}
	return 0
}

// segmentSliceStyle is the inline width + fill for a segment's slice of the
// track. color is expected pre-sanitized (segmentColorAt normalizes to
// #rrggbb).
func segmentSliceStyle(pct float64, color string) templpkg.SafeCSS {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return templpkg.SafeCSS(fmt.Sprintf("width:%s%%;background-color:%s", strconv.FormatFloat(pct, 'f', 4, 64), color))
}

func segmentSwatchStyle(color string) templpkg.SafeCSS {
	return templpkg.SafeCSS("background-color:" + color)
}

func segmentBarBodyClass(clickable bool) string {
	base := "relative flex h-full flex-col"
	if clickable {
		// Mirror StatPanel: body ignores pointer events so the inset overlay
		// link receives the click; fade slightly on hover for affordance.
		base += " z-10 pointer-events-none transition-opacity group-hover:opacity-95"
	}
	return base
}

func segmentBarUsesRowActions(spec panel.Spec) bool {
	if spec.Action == nil {
		return false
	}
	usesField := func(source action.ValueSource) bool {
		return source.Kind == action.SourceField
	}
	if spec.Action.URLSource != nil || spec.Action.Kind == action.KindEmitEvent {
		return true
	}
	for _, param := range spec.Action.Params {
		if usesField(param.Source) {
			return true
		}
	}
	for _, source := range spec.Action.Payload {
		if usesField(source) {
			return true
		}
	}
	return spec.Action.Drill != nil && usesField(spec.Action.Drill.Value)
}

type cascadeView struct {
	HasData bool
	Stages  []cascadeStage
}

type cascadeStage struct {
	Label      string
	Value      string
	CutLabel   string
	CutValue   string
	CutClass   string
	Raw        float64
	CutRaw     float64
	WidthPct   float64
	Final      bool
	HasCut     bool
	FillClass  string
	LabelClass string
	ValueClass string
	WidthStyle templpkg.SafeCSS
}

// cascadeWidthFloorPct keeps non-zero stage bars from becoming invisible
// slivers next to a much larger stage.
const cascadeWidthFloorPct = 2.0

func buildCascadeView(spec panel.Spec, result *runtime.PanelResult) cascadeView {
	view := cascadeView{}
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil {
		return view
	}
	rows := result.Frames.Primary().Rows()
	if len(rows) == 0 {
		return view
	}
	labelField := firstField(spec.Fields.Label, spec.Fields.Category, panel.DefaultLabelField)
	valueField := firstField(spec.Fields.Value, panel.DefaultValueField)
	cutField := firstField(spec.Fields.Cut, panel.DefaultCutField)
	cutLabelField := firstField(spec.Fields.CutLabel, panel.DefaultCutLabelField)
	finalField := firstField(spec.Fields.Final, panel.DefaultFinalField)

	maxStageValue := 0.0
	for _, row := range rows {
		value := segmentNumeric(row[valueField.Name()])
		if value > maxStageValue {
			maxStageValue = value
		}
	}
	if maxStageValue <= 0 {
		maxStageValue = 1
	}

	view.HasData = true
	view.Stages = make([]cascadeStage, 0, len(rows))
	for i, row := range rows {
		raw := segmentNumeric(row[valueField.Name()])
		cutRaw := segmentNumeric(row[cutField.Name()])
		final := rowBool(row[finalField.Name()])
		width := 0.0
		if raw > 0 {
			width = raw / maxStageValue * 100
			if width > 100 {
				width = 100
			}
			if width < cascadeWidthFloorPct {
				width = cascadeWidthFloorPct
			}
		}
		cutLabel := strings.TrimSpace(fmt.Sprint(row[cutLabelField.Name()]))
		view.Stages = append(view.Stages, cascadeStage{
			Label:      strings.TrimSpace(fmt.Sprint(row[labelField.Name()])),
			Value:      formatValue(raw, spec.Formatter, result.Locale, result.Timezone),
			CutLabel:   cutLabel,
			CutValue:   cascadeCutValue(cutRaw, spec.Formatter, result.Locale, result.Timezone),
			CutClass:   cascadeCutClass(cutRaw),
			Raw:        raw,
			CutRaw:     cutRaw,
			WidthPct:   width,
			Final:      final,
			HasCut:     i > 0 && cutLabel != "",
			FillClass:  cascadeBarClass(final),
			LabelClass: cascadeLabelClass(final),
			ValueClass: cascadeValueClass(raw),
			WidthStyle: cascadeWidthStyle(width),
		})
	}
	return view
}

// cascadeMinusSign is U+2212 MINUS SIGN, used instead of a hyphen so signed
// cut values read as arithmetic rather than a hyphenated word.
const cascadeMinusSign = "−"

// cascadeCutValue renders a cascade stage's deduction with an explicit sign:
// a positive cut (money leaving the bridge) shows as "−<amount>", a negative
// cut (money added back) shows as "+<amount>", and a zero cut shows as a
// plain formatted zero.
func cascadeCutValue(cutRaw float64, formatter *format.Spec, locale, timezone string) string {
	switch {
	case cutRaw > 0:
		return cascadeMinusSign + formatValue(cutRaw, formatter, locale, timezone)
	case cutRaw < 0:
		return "+" + formatValue(-cutRaw, formatter, locale, timezone)
	default:
		return formatValue(0, formatter, locale, timezone)
	}
}

func cascadeCutClass(cutRaw float64) string {
	switch {
	case cutRaw > 0:
		return "text-red-600"
	case cutRaw < 0:
		return "text-green-600"
	default:
		return "text-slate-400"
	}
}

func cascadeLabelClass(final bool) string {
	if final {
		return "font-semibold text-slate-900"
	}
	return "font-medium text-slate-700"
}

// trendChipClass, trendArrow, and trendPercentText back the panel header's
// TrendSpec chip: a small signed-percent indicator colored/arrowed by sign.
func trendChipClass(percent float64) string {
	switch {
	case percent > 0:
		return "text-green-600"
	case percent < 0:
		return "text-red-600"
	default:
		return "text-slate-400"
	}
}

func trendArrow(percent float64) string {
	switch {
	case percent > 0:
		return "▲"
	case percent < 0:
		return "▼"
	default:
		return ""
	}
}

func trendPercentText(percent float64) string {
	sign := ""
	if percent > 0 {
		sign = "+"
	}
	return sign + strconv.FormatFloat(percent, 'f', 1, 64)
}

func firstField(fields ...panel.FieldRef) panel.FieldRef {
	for _, field := range fields {
		if !field.Empty() {
			return field
		}
	}
	return ""
}

func rowBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(v))
		return parsed
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case float32:
		return v != 0
	default:
		return false
	}
}

func cascadeBarClass(final bool) string {
	if final {
		return "h-full rounded-full bg-green-500"
	}
	return "h-full rounded-full bg-brand-500"
}

func cascadeValueClass(raw float64) string {
	if raw < 0 {
		return "text-red-600"
	}
	return "text-slate-900"
}

func cascadeWidthStyle(pct float64) templpkg.SafeCSS {
	return templpkg.SafeCSS("width:" + strconv.FormatFloat(pct, 'f', 4, 64) + "%")
}

func segmentLegendLabelClass(raw float64) string {
	if raw > 0 {
		return "truncate text-sm font-medium text-slate-600"
	}
	return "truncate text-sm font-medium text-slate-400"
}

func segmentLegendAmountClass(raw float64) string {
	if raw > 0 {
		return "text-sm font-semibold tabular-nums text-slate-900"
	}
	return "text-sm font-semibold tabular-nums text-slate-400"
}

func formatValue(value any, spec *format.Spec, locale, timezone string) string {
	if spec != nil {
		return format.Apply(spec, value, locale, timezone)
	}
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case time.Time:
		return v.Format("2006-01-02")
	case float64:
		return fmt.Sprintf("%.2f", v)
	case float32:
		return fmt.Sprintf("%.2f", v)
	default:
		return fmt.Sprint(v)
	}
}

func filterModel(result *runtime.Result) filter.Model {
	if result == nil {
		return filter.Model{}
	}
	return result.Filters
}

func actionURL(spec *action.Spec, row map[string]any, result *runtime.PanelResult) string {
	if spec == nil {
		return ""
	}
	switch spec.Kind {
	case action.KindNavigate, action.KindHtmxSwap, action.KindCubeDrill:
	case action.KindEmitEvent:
		return ""
	default:
		return ""
	}
	resolvedSpec := *spec
	if spec.URLSource != nil {
		if value, ok := action.ResolveValue(*spec.URLSource, row, resultVariables(result)); ok {
			resolvedURL, safe := action.SafeRelativeURL(fmt.Sprint(value))
			if !safe {
				return ""
			}
			resolvedSpec.URL = resolvedURL
		}
	}
	if strings.TrimSpace(resolvedSpec.URL) == "" {
		return ""
	}
	spec = &resolvedSpec
	if spec.Kind == action.KindCubeDrill {
		return cubeDrillActionURL(spec, row, result)
	}
	nextURL := interpolateActionURL(spec.URL, row, resultVariables(result))
	values := url.Values{}
	if spec.PreserveQuery && result != nil {
		values = cloneURLValues(result.Request)
	}
	if len(spec.Params) == 0 {
		if len(values) == 0 {
			return nextURL
		}
		return joinURLQuery(nextURL, values)
	}
	for _, param := range spec.Params {
		value, ok := actionValue(param.Source, row, resultVariables(result))
		if !ok {
			continue
		}
		assignQueryValue(values, param.Name, value)
	}
	query := values.Encode()
	if query == "" {
		return nextURL
	}
	separator := "?"
	if len(nextURL) > 0 && containsQuery(nextURL) {
		separator = "&"
	}
	return nextURL + separator + query
}

func actionOnClick(spec *action.Spec, row map[string]any, result *runtime.PanelResult) templpkg.ComponentScript {
	if spec == nil {
		return templpkg.ComponentScript{}
	}
	switch spec.Kind {
	case action.KindNavigate, action.KindCubeDrill:
		return templpkg.ComponentScript{}
	case action.KindHtmxSwap:
		href := actionURL(spec, row, result)
		if href == "" {
			return templpkg.ComponentScript{}
		}
		method := spec.Method
		if method == "" {
			method = "GET"
		}
		// Route through window.__lensDrillAjax so the htmx `source` is always
		// set (here `this`, the clicked element). htmx.ajax otherwise defaults
		// source to document.body, cascading the in-flight `htmx-request`
		// loading state onto every .btn on the page (nav tabs, sidebar, etc.).
		// See DashboardScripts() for the helper definition.
		return templpkg.JSUnsafeFuncCall(fmt.Sprintf("event.preventDefault(); window.__lensDrillAjax(%s, %s, %s, this);", js.MustToJS(method), js.MustToJS(href), js.MustToJS(spec.Target)))
	case action.KindEmitEvent:
		payload := actionPayload(spec, row, resultVariables(result))
		encoded, err := json.Marshal(payload)
		if err != nil {
			return templpkg.ComponentScript{}
		}
		return templpkg.JSUnsafeFuncCall(fmt.Sprintf("event.preventDefault(); document.dispatchEvent(new CustomEvent(%s, {detail: %s}));", js.MustToJS(spec.Event), encoded))
	default:
		return templpkg.ComponentScript{}
	}
}

func resultVariables(result *runtime.PanelResult) map[string]any {
	if result == nil {
		return nil
	}
	return result.Variables
}

func stopPropagationScript(script templpkg.ComponentScript) templpkg.ComponentScript {
	if script.Call == "" {
		return templpkg.JSUnsafeFuncCall("event.stopPropagation();")
	}
	return templpkg.JSUnsafeFuncCall("event.stopPropagation(); " + script.Call)
}

func actionPayload(spec *action.Spec, row map[string]any, variables map[string]any) map[string]any {
	if spec == nil || len(spec.Payload) == 0 {
		return nil
	}
	payload := make(map[string]any, len(spec.Payload))
	for key, source := range spec.Payload {
		value, ok := actionValue(source, row, variables)
		if !ok {
			continue
		}
		payload[key] = value
	}
	return payload
}

func cubeDrillActionURL(spec *action.Spec, row map[string]any, result *runtime.PanelResult) string {
	if spec == nil || result == nil {
		return ""
	}
	values := cloneURLValues(result.Request)
	for _, param := range spec.Params {
		value, ok := actionValue(param.Source, row, result.Variables)
		if !ok {
			continue
		}
		assignQueryValue(values, param.Name, value)
	}
	if spec.Drill != nil {
		if scopeValue, ok := actionValue(spec.Drill.Value, row, result.Variables); ok {
			text := strings.TrimSpace(fmt.Sprint(scopeValue))
			if text == "" {
				return joinURLQuery(interpolateActionURL(spec.URL, row, result.Variables), values)
			}
			ctx := cube.ParseDrillContext(values).ToggleFilter(spec.Drill.Dimension, text)
			values = ctx.WithValues(values)
		}
	}
	return joinURLQuery(interpolateActionURL(spec.URL, row, result.Variables), values)
}

func cloneURLValues(values url.Values) url.Values {
	cloned := url.Values{}
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func assignQueryValue(values url.Values, key string, value any) {
	if values == nil || strings.TrimSpace(key) == "" || value == nil {
		return
	}
	switch current := value.(type) {
	case []string:
		values.Del(key)
		for _, item := range current {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				values.Add(key, trimmed)
			}
		}
	case []any:
		values.Del(key)
		for _, item := range current {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				values.Add(key, text)
			}
		}
	default:
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return
		}
		values.Set(key, text)
	}
}

func actionValue(source action.ValueSource, row map[string]any, variables map[string]any) (any, bool) {
	return action.ResolveValue(source, row, variables)
}

func containsQuery(raw string) bool {
	return strings.ContainsRune(raw, '?')
}

func joinURLQuery(raw string, values url.Values) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	query := values.Encode()
	if query == "" {
		return raw
	}
	if containsQuery(raw) {
		return raw + "&" + query
	}
	return raw + "?" + query
}

func interpolateActionURL(raw string, row map[string]any, variables map[string]any) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}
	return actionPlaceholderPattern.ReplaceAllStringFunc(raw, func(token string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(token, "{"), "}")
		if row != nil {
			if value, ok := row[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
				return url.PathEscape(fmt.Sprint(value))
			}
		}
		if variables != nil {
			if value, ok := variables[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
				return url.PathEscape(fmt.Sprint(value))
			}
		}
		return ""
	})
}

func dateRangeState(input filter.Input) string {
	return js.MustToJS(struct {
		DateMode string `json:"dateMode"`
	}{
		DateMode: input.DateRange.Mode,
	})
}

func panelUsesLogScale(spec panel.Spec) bool {
	if spec.ValueAxis.Scale == panel.AxisScaleLogarithmic {
		return true
	}
	for _, child := range spec.Children {
		if panelUsesLogScale(child) {
			return true
		}
	}
	return false
}

type chartText struct {
	ExpandToFullscreen string
	CloseFullscreen    string
	LogScale           string
	LogScaleHint       string
	MetricInfo         string
	DrillBack          string
	ExportExcel        string
	ExportGenerating   string
	ExportFailed       string
}

// DashboardExportButtonProps configures the first-class Lens export action for
// a whole dashboard. It intentionally shares the same runtime and download
// handshake as panel exports; ParamsFormID lets filter forms contribute their
// current values without requiring the page URL to be up to date first.
type DashboardExportButtonProps struct {
	URL          string
	Label        string
	ParamsFormID string
	Class        string
}

type exportButtonProps struct {
	URL          string
	Label        string
	ParamsFormID string
	Class        string
	IconOnly     bool
}

func pageContext(ctx context.Context) types.PageContext {
	pageCtx, ok := ctx.Value(constants.PageContext).(types.PageContext)
	if !ok || pageCtx == nil {
		return nil
	}
	return pageCtx
}

func translate(ctx context.Context, key string, args ...map[string]interface{}) string {
	pageCtx := pageContext(ctx)
	if pageCtx == nil {
		return ""
	}
	return pageCtx.TSafe(key, args...)
}

func localizedChartText(ctx context.Context) chartText {
	return chartText{
		ExpandToFullscreen: translate(ctx, "Chart.ExpandToFullScreen"),
		CloseFullscreen:    translate(ctx, "Chart.CloseFullScreen"),
		LogScale:           translate(ctx, "Chart.LogScale"),
		LogScaleHint:       translate(ctx, "Chart.LogScaleHint"),
		MetricInfo:         translate(ctx, "Chart.MetricInfo"),
		DrillBack:          translate(ctx, "Lens.Drill.Back"),
		ExportExcel:        translate(ctx, "Chart.ExportExcel"),
		ExportGenerating:   translate(ctx, "Chart.ExportGenerating"),
		ExportFailed:       translate(ctx, "Chart.ExportFailed"),
	}
}

type lensText struct {
	FiltersTitle     string
	FiltersApply     string
	DefaultRange     string
	CustomRange      string
	AllTime          string
	All              string
	EmptyTitle       string
	EmptyDescription string
	ErrorTitle       string
	ErrorDescription string
	ErrorPanelLabel  string
	ErrorReasonLabel string
	ErrorLogsHint    string
}

func localizedLensText(ctx context.Context) lensText {
	return lensText{
		FiltersTitle:     translate(ctx, "Lens.Filters.Title"),
		FiltersApply:     translate(ctx, "Lens.Filters.Apply"),
		DefaultRange:     translate(ctx, "Lens.Filters.DefaultRange"),
		CustomRange:      translate(ctx, "Lens.Filters.CustomRange"),
		AllTime:          translate(ctx, "Lens.Filters.AllTime"),
		All:              translate(ctx, "Lens.Filters.All"),
		EmptyTitle:       translate(ctx, "Lens.Empty.Title"),
		EmptyDescription: translate(ctx, "Lens.Empty._Description"),
		ErrorTitle:       translate(ctx, "Lens.Error.Title"),
		ErrorDescription: translate(ctx, "Lens.Error._Description"),
		ErrorPanelLabel:  translate(ctx, "Lens.Error.PanelLabel"),
		ErrorReasonLabel: translate(ctx, "Lens.Error.ReasonLabel"),
		ErrorLogsHint:    translate(ctx, "Lens.Error.LogsHint"),
	}
}

func panelErrorDetails(result *runtime.PanelResult) panelErrorModel {
	if result == nil {
		return panelErrorModel{}
	}
	reason := ""
	if result.Error != nil {
		reason = normalizeErrorText(result.Error.Error())
	}
	return panelErrorModel{
		PanelID: strings.TrimSpace(result.Panel.ID),
		Reason:  reason,
	}
}

func panelErrorModelFor(spec panel.Spec, result *runtime.PanelResult, resolve PanelErrorActionResolver) panelErrorModel {
	details := panelErrorDetails(result)
	if resolve != nil {
		details.Action = normalizePanelErrorAction(resolve(spec, result))
	}
	return details
}

func normalizePanelErrorAction(action *PanelErrorAction) *PanelErrorAction {
	if action == nil {
		return nil
	}
	normalized := &PanelErrorAction{
		Label:   strings.TrimSpace(action.Label),
		URL:     strings.TrimSpace(action.URL),
		Method:  strings.ToLower(strings.TrimSpace(action.Method)),
		Target:  strings.TrimSpace(action.Target),
		Swap:    strings.TrimSpace(action.Swap),
		Include: strings.TrimSpace(action.Include),
		Confirm: strings.TrimSpace(action.Confirm),
	}
	if normalized.Label == "" || normalized.URL == "" {
		return nil
	}
	if normalized.Method != strings.ToLower(http.MethodPost) {
		normalized.Method = strings.ToLower(http.MethodGet)
	}
	return normalized
}

func inputTextValue(input filter.Input) string {
	if input.Kind == lens.VariableMultiSelect && len(input.Values) > 0 {
		return strings.Join(input.Values, ",")
	}
	return input.Value
}

func normalizeErrorText(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	message = strings.Join(strings.Fields(message), " ")
	const maxLen = 220
	if len(message) > maxLen {
		truncated := message[:maxLen-3]
		for len(truncated) > 0 && !utf8.ValidString(truncated) {
			truncated = truncated[:len(truncated)-1]
		}
		return truncated + "..."
	}
	return message
}

// tabsState seeds a Tabs panel's Alpine scope. It deliberately does NOT
// define `fullscreen`: that lives on the panel card's scope (see panelCard),
// shared with the header's fullscreen button, and Alpine scope chaining lets
// the overlay inside the tabs body read it from the ancestor.
func tabsState(spec panel.Spec) string {
	activeTab := ""
	if len(spec.Children) > 0 {
		activeTab = spec.Children[0].ID
	}
	return js.MustToJS(struct {
		ActiveTab string `json:"activeTab"`
	}{
		ActiveTab: activeTab,
	})
}

func jsStringLiteral(value string) string {
	return js.MustToJS(value)
}

func tabClassExpression(tabID string) string {
	literal := jsStringLiteral(tabID)
	return fmt.Sprintf("{ 'lens-seg__item--active': activeTab === %s }", literal)
}

func tabVisibilityExpression(tabID string) string {
	return "activeTab === " + jsStringLiteral(tabID)
}

func panelIcon(kind panel.Kind) templpkg.Component {
	iconProps := icons.Props{Size: "16"}
	switch kind {
	case panel.KindTimeSeries:
		return icons.ChartLine(iconProps)
	case panel.KindBar, panel.KindStackedBar, panel.KindHorizontalBar:
		return icons.ChartBar(iconProps)
	case panel.KindSegmentBar, panel.KindCascade:
		return icons.ChartBar(iconProps)
	case panel.KindPie, panel.KindDonut:
		return icons.ChartPie(iconProps)
	case panel.KindGauge:
		return icons.Gauge(iconProps)
	case panel.KindTable:
		return icons.Table(iconProps)
	case panel.KindStat, panel.KindStatGroup:
		return icons.HashStraight(iconProps)
	case panel.KindTabs:
		return icons.Tabs(iconProps)
	case panel.KindGrid:
		return icons.Rows(iconProps)
	case panel.KindSplit:
		return icons.Rows(iconProps)
	case panel.KindRepeat:
		return icons.Copy(iconProps)
	default:
		return icons.Question(iconProps)
	}
}

func statAriaLabel(spec panel.Spec) string {
	if trimmed := strings.TrimSpace(spec.Title); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(spec.Description); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(spec.ID)
}

func showPanelHeader(spec panel.Spec) bool {
	// Stat panels render their label inside the stat body (.lens-stat__label-row)
	// instead of a card header.
	if spec.Kind == panel.KindStat {
		return false
	}
	return spec.Title != ""
}

func metricInfoTooltipHTML(ctx context.Context, info string) string {
	if strings.TrimSpace(info) == "" {
		return ""
	}
	chartText := localizedChartText(ctx)
	body := html.EscapeString(strings.TrimSpace(info))
	body = strings.ReplaceAll(body, "\n", "<br>")
	return fmt.Sprintf(
		`<div class="max-w-xs space-y-1.5 p-1"><div class="text-xs font-semibold text-white">%s</div><div class="text-xs leading-5 text-white/85">%s</div></div>`,
		html.EscapeString(chartText.MetricInfo),
		body,
	)
}

func panelMetricInfoText(ctx context.Context, spec panel.Spec) string {
	if info := strings.TrimSpace(spec.Info); info != "" {
		return info
	}
	if !panelUsesMetricInfoFallback(spec) {
		return ""
	}
	return defaultMetricInfoText(ctx, spec)
}

func panelUsesMetricInfoFallback(spec panel.Spec) bool {
	// Apex charts plus the tabbed container surface a generic per-kind metric
	// info fallback; native leaves (segment bar/table) and the other
	// containers do not. Stat panels are included so their Description (which
	// stat v2 no longer renders as a visible line) still surfaces in the info
	// tooltip; metricInfoTemplateKey adds no generic template for stats.
	return spec.Kind.IsChart() || spec.Kind == panel.KindTabs || spec.Kind == panel.KindStat
}

func panelUsesRadialActionSurface(spec panel.Spec) bool {
	if spec.Action == nil {
		return false
	}
	switch spec.Kind {
	case panel.KindPie, panel.KindDonut, panel.KindGauge:
		return true
	case panel.KindStat,
		panel.KindTimeSeries,
		panel.KindBar,
		panel.KindHorizontalBar,
		panel.KindStackedBar,
		panel.KindSegmentBar,
		panel.KindCascade,
		panel.KindTable,
		panel.KindTabs,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
		return false
	}
	return false
}

func panelIsInteractive(spec panel.Spec) bool {
	return spec.Action != nil
}

func panelChartClass(spec panel.Spec, fullscreen bool) string {
	base := "w-full min-h-[240px]"
	if fullscreen {
		base = "h-full min-h-[420px] w-full flex-1"
	} else {
		base += " h-[320px]"
	}
	if panelIsInteractive(spec) {
		base += " cursor-pointer"
	}
	if spec.DrillHierarchy != nil {
		// Server-rendered counterpart of the data-lens-drill-hierarchy attribute
		// (set client-side on Mounted): gives the bars a pointer cursor from the
		// first paint instead of after the chart lifecycle hook runs.
		base += " lens-chart--drill"
	}
	if panelUsesRadialActionSurface(spec) {
		base += " lens-chart--radial-action"
	}
	return strings.TrimSpace(base)
}

func panelCardClass(spec panel.Spec) string {
	// .lens-card provides the full chrome (surface, hairline, radius, shadow,
	// flex column, h-100%, overflow clipping) — see LensThemeStyles().
	base := "lens-card"
	// Stat panels host an info (ⓘ) tooltip that pops outside the card body;
	// the default overflow-hidden would clip it (the tooltip mounts in-tree,
	// not portaled). Same for a StatGroup's per-child tooltips.
	if spec.Kind == panel.KindStat || spec.Kind == panel.KindStatGroup {
		base += " lens-card--overflow"
	}
	if panelIsInteractive(spec) {
		base += " lens-card--interactive"
	}
	return base
}

func panelHasClass(spec panel.Spec, token string) bool {
	for _, className := range strings.Fields(spec.ClassName) {
		if className == token {
			return true
		}
	}
	return false
}

func badgeStyle(color string) templpkg.SafeCSS {
	r, g, b := parseHexColor(color)
	safeColor := fmt.Sprintf("#%02x%02x%02x", r, g, b)
	return templpkg.SafeCSS(fmt.Sprintf(
		"background-color: rgba(%d, %d, %d, 0.12); border: 1px solid rgba(%d, %d, %d, 0.22); color: %s;",
		r, g, b, r, g, b, safeColor,
	))
}

func defaultMetricInfoText(ctx context.Context, spec panel.Spec) string {
	subject := metricInfoSubject(ctx, spec)
	parts := make([]string, 0, 3)
	if description := normalizedMetricDescription(spec.Description); description != "" {
		parts = append(parts, description)
	}
	if key := metricInfoTemplateKey(spec.Kind); key != "" {
		if summary := translate(ctx, key, map[string]interface{}{"Subject": subject}); summary != "" {
			parts = append(parts, summary)
		}
	}
	if spec.Action != nil && spec.Action.Kind == action.KindCubeDrill {
		if hint := translate(ctx, "Lens.Chart.Info.DrillHint"); hint != "" {
			parts = append(parts, hint)
		}
	}
	if panelUsesLogScale(spec) {
		if hint := translate(ctx, "Lens.Chart.Info.LogScaleHint"); hint != "" {
			parts = append(parts, hint)
		}
	}
	return strings.Join(parts, " ")
}

func metricInfoTemplateKey(kind panel.Kind) string {
	switch kind {
	case panel.KindTimeSeries:
		return "Lens.Chart.Info.TimeSeries"
	case panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar:
		return "Lens.Chart.Info.Category"
	case panel.KindPie, panel.KindDonut:
		return "Lens.Chart.Info.Distribution"
	case panel.KindGauge:
		return "Lens.Chart.Info.Gauge"
	case panel.KindTabs:
		return "Lens.Chart.Info.Tabs"
	case panel.KindStat,
		panel.KindSegmentBar,
		panel.KindCascade,
		panel.KindTable,
		panel.KindGrid,
		panel.KindSplit,
		panel.KindRepeat,
		panel.KindStatGroup:
		return ""
	}
	return ""
}

func metricInfoSubject(ctx context.Context, spec panel.Spec) string {
	if title := strings.TrimSpace(spec.Title); title != "" {
		return title
	}
	return firstNonEmptyString(
		translate(ctx, "Lens.Chart.Info.SubjectFallback"),
		strings.TrimSpace(spec.ID),
	)
}

func normalizedMetricDescription(description string) string {
	description = strings.TrimSpace(description)
	if description == "" {
		return ""
	}
	if len(description) < 40 && !strings.ContainsAny(description, ".!?") {
		return ""
	}
	if strings.HasSuffix(description, ".") || strings.HasSuffix(description, "!") || strings.HasSuffix(description, "?") {
		return description
	}
	return description + "."
}

func parseHexColor(color string) (int, int, int) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(color), "#")
	if len(trimmed) == 3 {
		trimmed = string([]byte{
			trimmed[0], trimmed[0],
			trimmed[1], trimmed[1],
			trimmed[2], trimmed[2],
		})
	}
	if len(trimmed) != 6 {
		return 100, 116, 139
	}
	value, err := strconv.ParseUint(trimmed, 16, 32)
	if err != nil {
		return 100, 116, 139
	}
	return int(value >> 16), int((value >> 8) & 0xff), int(value & 0xff)
}

func tableValueText(column panel.TableColumn, row map[string]any, result *runtime.PanelResult) string {
	if row == nil {
		return ""
	}
	return formatValue(row[column.Field.Name()], column.Formatter, result.Locale, result.Timezone)
}

func tableActionText(ctx context.Context, column panel.TableColumn, row map[string]any, result *runtime.PanelResult) string {
	if trimmed := strings.TrimSpace(column.Text); trimmed != "" {
		return trimmed
	}
	if value := strings.TrimSpace(tableValueText(column, row, result)); value != "" {
		return value
	}
	return translate(ctx, "Lens.Table.OpenRow")
}

func rowString(row map[string]any, key string) string {
	if row == nil {
		return ""
	}
	value, ok := row[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func rowAccentColor(row map[string]any) string {
	color := rowString(row, "accent_color")
	if color == "" {
		return "#6366f1"
	}
	return color
}

func rowMarker(text string) string {
	for _, r := range strings.TrimSpace(text) {
		return strings.ToUpper(string(r))
	}
	return "•"
}

func displayValueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func tablePrimaryText(ctx context.Context, column panel.TableColumn, row map[string]any, result *runtime.PanelResult) string {
	value := strings.TrimSpace(tableValueText(column, row, result))
	if value != "" {
		return value
	}
	return tableActionText(ctx, column, row, result)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func panelBodyClass(spec panel.Spec) string {
	switch spec.Kind {
	case panel.KindStat, panel.KindStatGroup:
		// Stat v2 (.lens-stat) owns its own padding.
		return "flex-1 p-0"
	case panel.KindTable:
		// .lens-th/.lens-td own the cell padding; the table sits flush.
		// The scrolling variant additionally needs min-h-0: when its card
		// has a bounded height (e.g. inside the fullscreen overlay) this flex
		// body must be allowed to shrink below the table's content height,
		// otherwise its default min-height:auto pins it to content size, the
		// table overflows the card, and the wrapper's own overflow-auto never
		// engages. Harmless for the normal auto-height in-page case.
		if panelHasClass(spec, "lens-table-scroll") {
			return "flex-1 p-0 min-h-0"
		}
		return "flex-1 p-0"
	case panel.KindTabs:
		return "flex-1 px-3 py-2"
	case panel.KindSegmentBar:
		return "flex-1 px-4 py-3"
	case panel.KindCascade:
		return "flex-1 px-4 py-3"
	case panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		return "flex-1 px-2 pb-2 pt-1"
	case panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return "flex-1 p-3"
	default:
		return "flex-1 p-3"
	}
}

func panelHasRenderableContent(spec panel.Spec, result *runtime.Result) bool {
	if spec.Kind.IsContainer() {
		for _, child := range spec.Children {
			if panelHasRenderableContent(child, result) {
				return true
			}
		}
		return false
	}
	if spec.Kind.IsChart() || spec.Kind.RendersNatively() {
		panelResult := panelResult(result, spec.ID)
		return panelResultHasContent(panelResult)
	}
	return false
}

func panelResultHasContent(result *runtime.PanelResult) bool {
	if result == nil || result.Error != nil || result.Frames == nil || result.Frames.Primary() == nil {
		return false
	}
	return result.Frames.Primary().RowCount > 0
}

func panelCanFullscreen(spec panel.Spec, result *runtime.Result) bool {
	// Only apex charts and the tabbed container offer a fullscreen affordance;
	// native leaves (stat/segment bar/table) and the plain layout containers
	// (grid/split/repeat) do not.
	if spec.Kind.IsChart() || spec.Kind == panel.KindTabs {
		return panelHasRenderableContent(spec, result)
	}
	return false
}

func panelFullscreenBodyClass(spec panel.Spec) string {
	return strings.TrimSpace("flex flex-1 min-h-0 flex-col " + panelBodyClass(spec) + " h-[calc(100dvh-8rem)] min-h-[70vh]")
}

func panelShellBodyClass(spec panel.Spec) string {
	return strings.TrimSpace(panelBodyClass(spec) + " relative min-h-0")
}

func panelIslandStyle(spec panel.Spec) templpkg.SafeCSS {
	minHeight := panelMinimumHeight(spec)
	if minHeight == "" {
		return ""
	}
	return templpkg.SafeCSS("min-height: " + minHeight + ";")
}

func panelMinimumHeight(spec panel.Spec) string {
	switch spec.Kind {
	case panel.KindStat:
		return "96px"
	case panel.KindStatGroup:
		if spec.GroupLayout == panel.GroupRows {
			children := len(spec.Children)
			if children < 1 {
				children = 1
			}
			return strconv.Itoa(children*96) + "px"
		}
		return "96px"
	case panel.KindTable:
		return "220px"
	case panel.KindSegmentBar:
		return "240px"
	case panel.KindCascade:
		return "280px"
	case panel.KindTabs:
		if childHeight := maxChildHeight(spec.Children); childHeight != "" {
			return "calc(" + childHeight + " + 5rem)"
		}
		return "420px"
	case panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		if childHeight := maxChildHeight(spec.Children); childHeight != "" {
			return childHeight
		}
		return "240px"
	case panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		if strings.TrimSpace(spec.Height) != "" {
			return strings.TrimSpace(spec.Height)
		}
		return "240px"
	}
	return "240px"
}

func maxChildHeight(children []panel.Spec) string {
	heights := make([]string, 0, len(children))
	for _, child := range children {
		if height := panelMinimumHeight(child); height != "" {
			heights = append(heights, height)
		}
	}
	switch len(heights) {
	case 0:
		return ""
	case 1:
		return heights[0]
	default:
		return "max(" + strings.Join(heights, ", ") + ")"
	}
}

func panelFragmentURL(basePath, panelID string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	if basePath == "" {
		return ""
	}
	return basePath + "/" + url.PathEscape(panelID)
}

func islandIncludeSelector(props AsyncProps) string {
	if strings.TrimSpace(props.IncludeSelector) != "" {
		return props.IncludeSelector
	}
	if strings.TrimSpace(props.FilterFormID) == "" {
		return ""
	}
	formID := "#" + props.FilterFormID
	return formID + " input, " + formID + " select, " + formID + " textarea"
}

func islandTrigger(props AsyncProps) string {
	if strings.TrimSpace(props.FilterFormID) == "" {
		return "load"
	}
	formID := "#" + props.FilterFormID
	delay := fmt.Sprintf("%dms", theme.DebounceMs)
	return "load, change delay:" + delay + " from:" + formID + ", dateRangeChange delay:" + delay + " from:" + formID
}

// formURLSyncDelayMs staggers the URL sync slightly behind the island
// debounce (theme.DebounceMs) so the address bar reflects the state the
// panels actually fetched.
func formURLSyncDelayMs() string {
	return strconv.Itoa(theme.DebounceMs + 20)
}

func shellIndicatorID(panelID string) string {
	return "lens-panel-indicator-" + panelID
}

func tabsPanelFrameClass(fullscreen bool) string {
	if fullscreen {
		return "flex h-full flex-1 min-h-0 flex-col"
	}
	return "flex-1"
}

func rerenderChartsScript(delayMs int) string {
	if delayMs <= 0 {
		delayMs = 180
	}
	return fmt.Sprintf("setTimeout(() => { const root = event && event.currentTarget && event.currentTarget.closest('[data-lens-rerender-scope]'); document.dispatchEvent(new CustomEvent('sdk:rerenderCharts', { detail: root ? { root } : {} })); window.dispatchEvent(new Event('resize')); }, %d)", delayMs)
}

func openFullscreenScript() string {
	return "fullscreen = true; requestAnimationFrame(() => { const root = event && event.currentTarget && event.currentTarget.closest('[data-lens-rerender-scope]'); if (root && root.__lensFullscreenRerenderTimer) { clearTimeout(root.__lensFullscreenRerenderTimer); } const rerender = () => { document.dispatchEvent(new CustomEvent('sdk:rerenderCharts', { detail: root ? { root } : {} })); window.dispatchEvent(new Event('resize')); if (root) { root.__lensFullscreenRerenderTimer = null; } }; const timer = setTimeout(rerender, 260); if (root) { root.__lensFullscreenRerenderTimer = timer; } });"
}

func swapTargetLoadingScript() templpkg.ComponentScript {
	return templpkg.JSUnsafeFuncCall("if (window.__lensSetSwapTargetLoading) { window.__lensSetSwapTargetLoading(this.closest('[data-lens-swap-target]'), true); }")
}

func activateTabScript(tabID string) string {
	return "activeTab = " + jsStringLiteral(tabID) + "; " + rerenderChartsScript(180)
}

func panelPlaceholderRows(spec panel.Spec) int {
	switch spec.Kind {
	case panel.KindStat, panel.KindStatGroup:
		return 2
	case panel.KindTable:
		return 5
	case panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return 4
	case panel.KindSegmentBar, panel.KindCascade, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		return 4
	}
	return 4
}
