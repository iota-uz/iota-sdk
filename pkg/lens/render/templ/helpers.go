package templ

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	templpkg "github.com/a-h/templ"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/js"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/drill"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
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

func panelResult(result *runtime.Result, panelID string) *runtime.PanelResult {
	if result == nil || result.Panels == nil {
		return nil
	}
	return result.Panels[panelID]
}

type drillNavModel struct {
	HasNav         bool
	CurrentTitle   string
	CurrentValue   string
	CurrentLabel   string
	CurrentDisplay string
	UpURL          string
	UpLabel        string
	Trail          []drillNavCrumb
	Summary        []drillSummaryItem
}

type drillNavCrumb struct {
	URL   string
	Label string
}

type drillSummaryItem struct {
	Label string
	Value string
}

func drillNavigationModel(result *runtime.Result) drillNavModel {
	if result == nil || result.Drill == nil {
		return drillNavModel{}
	}
	state := result.Drill
	model := drillNavModel{
		HasNav:         state.HasNavigation(),
		CurrentTitle:   state.Current.Title,
		CurrentValue:   state.Current.ScopeValue,
		CurrentLabel:   state.Current.ScopeLabel,
		CurrentDisplay: drillCurrentDisplay(state),
	}
	if up, ok := state.Up(); ok {
		model.UpURL = up.URL
		model.UpLabel = up.DisplayLabel()
	}
	for _, crumb := range state.Trail {
		model.Trail = append(model.Trail, drillNavCrumb{
			URL:   crumb.URL,
			Label: crumb.DisplayLabel(),
		})
		if strings.TrimSpace(crumb.ScopeValue) != "" {
			model.Summary = appendDrillSummary(model.Summary, firstNonEmptyString(crumb.ScopeLabel, crumb.Title), crumb.ScopeValue)
		}
	}
	if strings.TrimSpace(state.Current.ScopeValue) != "" {
		model.Summary = appendDrillSummary(model.Summary, firstNonEmptyString(state.Current.ScopeLabel, state.Current.Title), state.Current.ScopeValue)
	}
	return model
}

func drillCurrentDisplay(state *drill.State) string {
	if state == nil {
		return ""
	}
	display := firstNonEmptyString(state.Current.ScopeValue, state.Current.Title)
	if len(state.Trail) == 0 {
		return display
	}
	last := state.Trail[len(state.Trail)-1].DisplayLabel()
	if strings.TrimSpace(display) != "" && strings.EqualFold(strings.TrimSpace(display), strings.TrimSpace(last)) {
		return firstNonEmptyString(state.Current.Title, display)
	}
	return display
}

func appendDrillSummary(summary []drillSummaryItem, label, value string) []drillSummaryItem {
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
	return append(summary, drillSummaryItem{Label: label, Value: value})
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
	case action.KindNavigate, action.KindHtmxSwap, action.KindDrill:
	case action.KindEmitEvent:
		return ""
	default:
		return ""
	}
	if spec.Kind == action.KindDrill {
		return drillActionURL(spec, row, result)
	}
	nextURL := interpolateActionURL(spec.URL, row, resultVariables(result))
	if len(spec.Params) == 0 {
		return nextURL
	}
	values := url.Values{}
	for _, param := range spec.Params {
		value, ok := actionValue(param.Source, row, resultVariables(result))
		if !ok {
			continue
		}
		values.Add(param.Name, fmt.Sprint(value))
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
	case action.KindNavigate, action.KindDrill:
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
		return templpkg.JSUnsafeFuncCall(fmt.Sprintf("event.preventDefault(); htmx.ajax(%s, %s, {target: %s, swap: 'innerHTML'});", js.MustToJS(method), js.MustToJS(href), js.MustToJS(spec.Target)))
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

func drillActionURL(spec *action.Spec, row map[string]any, result *runtime.PanelResult) string {
	if spec == nil || result == nil {
		return ""
	}
	values := drill.Strip(result.Request)
	for _, param := range spec.Params {
		value, ok := actionValue(param.Source, row, result.Variables)
		if !ok {
			continue
		}
		assignQueryValue(values, param.Name, value)
	}
	if spec.Drill != nil {
		if result.Drill != nil {
			if trail := result.Drill.NextTrailEncoded(); trail != "" {
				values.Set(drill.QueryTrail, trail)
			}
		}
		if title := strings.TrimSpace(spec.Drill.PageTitle); title != "" {
			values.Set(drill.QueryPageTitle, title)
		}
		if label := strings.TrimSpace(spec.Drill.ScopeLabel); label != "" {
			values.Set(drill.QueryScopeLabel, label)
		}
		if scopeValue, ok := resolvedDrillScopeValue(spec, row, result); ok {
			values.Set(drill.QueryScopeValue, scopeValue)
		}
		if destination := strings.TrimSpace(string(spec.Drill.Destination)); destination != "" {
			values.Set(drill.QueryDestination, destination)
		}
	}
	return joinURLQuery(interpolateActionURL(spec.URL, row, result.Variables), values)
}

func resolvedDrillScopeValue(spec *action.Spec, row map[string]any, result *runtime.PanelResult) (string, bool) {
	if spec == nil || spec.Drill == nil {
		return "", false
	}
	source := spec.Drill.LabelSource
	if source.Kind != "" {
		value, ok := actionValue(source, row, result.Variables)
		if !ok {
			return "", false
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			return "", false
		}
		return text, true
	}
	for _, field := range []string{"label", "category"} {
		value, ok := actionValue(action.FieldValue(field), row, result.Variables)
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text != "" {
			return text, true
		}
	}
	return "", false
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
			assignQueryValue(values, key, item)
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
	DrillCurrentScope  string
}

func pageContext(ctx context.Context) types.PageContext {
	pageCtx, ok := ctx.Value(constants.PageContext).(types.PageContext)
	if !ok || pageCtx == nil {
		return nil
	}
	return pageCtx
}

func translateOrFallback(ctx context.Context, key, fallback string) string {
	pageCtx := pageContext(ctx)
	if pageCtx == nil {
		return fallback
	}
	if translated := pageCtx.TSafe(key); translated != "" {
		return translated
	}
	return fallback
}

func localizedChartText(ctx context.Context) chartText {
	return chartText{
		ExpandToFullscreen: translateOrFallback(ctx, "Chart.ExpandToFullScreen", "Expand to fullscreen"),
		CloseFullscreen:    translateOrFallback(ctx, "Chart.CloseFullScreen", "Close fullscreen"),
		LogScale:           translateOrFallback(ctx, "Chart.LogScale", "Log scale"),
		LogScaleHint:       translateOrFallback(ctx, "Chart.LogScaleHint", "Values are shown on a logarithmic scale"),
		MetricInfo:         translateOrFallback(ctx, "Chart.MetricInfo", "How this metric is calculated"),
		DrillBack:          translateOrFallback(ctx, "Lens.Drill.Back", "Back"),
		DrillCurrentScope:  translateOrFallback(ctx, "Lens.Drill.CurrentScope", "Current scope"),
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
}

func localizedLensText(ctx context.Context) lensText {
	return lensText{
		FiltersTitle:     translateOrFallback(ctx, "Lens.Filters.Title", "Filters"),
		FiltersApply:     translateOrFallback(ctx, "Lens.Filters.Apply", "Apply"),
		DefaultRange:     translateOrFallback(ctx, "Lens.Filters.DefaultRange", "Default range"),
		CustomRange:      translateOrFallback(ctx, "Lens.Filters.CustomRange", "Custom range"),
		AllTime:          translateOrFallback(ctx, "Lens.Filters.AllTime", "All time"),
		All:              translateOrFallback(ctx, "Lens.Filters.All", "All"),
		EmptyTitle:       translateOrFallback(ctx, "Lens.Empty.Title", "No data available"),
		EmptyDescription: translateOrFallback(ctx, "Lens.Empty._Description", "Try adjusting your filters"),
		ErrorTitle:       translateOrFallback(ctx, "Lens.Error.Title", "Unable to load data"),
		ErrorDescription: translateOrFallback(ctx, "Lens.Error._Description", "An error occurred while rendering this panel"),
	}
}

func tabsState(spec panel.Spec) string {
	activeTab := ""
	if len(spec.Children) > 0 {
		activeTab = spec.Children[0].ID
	}
	return js.MustToJS(struct {
		ActiveTab  string `json:"activeTab"`
		Fullscreen bool   `json:"fullscreen"`
	}{
		ActiveTab:  activeTab,
		Fullscreen: false,
	})
}

func jsStringLiteral(value string) string {
	return js.MustToJS(value)
}

func tabClassExpression(tabID string) string {
	literal := jsStringLiteral(tabID)
	return fmt.Sprintf(
		"{ 'bg-white text-slate-700 shadow-sm': activeTab === %s, 'text-slate-300 hover:text-white': activeTab !== %s }",
		literal,
		literal,
	)
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
	case panel.KindPie, panel.KindDonut:
		return icons.ChartPie(iconProps)
	case panel.KindGauge:
		return icons.Gauge(iconProps)
	case panel.KindTable:
		return icons.Table(iconProps)
	case panel.KindStat:
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

func panelDisplayIcon(spec panel.Spec) templpkg.Component {
	if spec.Icon != nil {
		return spec.Icon
	}
	return panelIcon(spec.Kind)
}

func showPanelHeader(spec panel.Spec) bool {
	if statUsesCustomChrome(spec) {
		return false
	}
	return spec.Title != "" || (spec.Description != "" && spec.Kind != panel.KindStat)
}

func statUsesCustomChrome(spec panel.Spec) bool {
	return spec.Kind == panel.KindStat && (spec.Icon != nil || strings.TrimSpace(spec.AccentColor) != "")
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

func metricInfoTooltipConfig(ctx context.Context, info string) string {
	html := metricInfoTooltipHTML(ctx, info)
	if html == "" {
		return ""
	}
	return fmt.Sprintf(`{content: %s, allowHTML: true, interactive: true, trigger: 'click focusin', theme: 'light', maxWidth: 360}`, js.MustToJS(html))
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
	return translateOrFallback(ctx, "Lens.Table.OpenRow", "Open details")
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
	case panel.KindStat:
		if statUsesCustomChrome(spec) {
			return "flex-1 p-0"
		}
		return "flex-1 px-5 py-2.5"
	case panel.KindTable:
		return "flex-1 p-4"
	case panel.KindTabs:
		return "flex-1 px-5 py-3"
	case panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindPie, panel.KindDonut, panel.KindGauge, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return "flex-1 p-3"
	default:
		return "flex-1 p-3"
	}
}

func panelHasRenderableContent(spec panel.Spec, result *runtime.Result) bool {
	switch spec.Kind {
	case panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		for _, child := range spec.Children {
			if panelHasRenderableContent(child, result) {
				return true
			}
		}
		return false
	default:
		panelResult := panelResult(result, spec.ID)
		return panelResultHasContent(panelResult)
	}
}

func panelResultHasContent(result *runtime.PanelResult) bool {
	if result == nil || result.Error != nil || result.Frames == nil || result.Frames.Primary() == nil {
		return false
	}
	return result.Frames.Primary().RowCount > 0
}

func panelCanFullscreen(spec panel.Spec, result *runtime.Result) bool {
	switch spec.Kind {
	case panel.KindTabs, panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		return panelHasRenderableContent(spec, result)
	default:
		return false
	}
}

func panelFullscreenBodyClass(spec panel.Spec) string {
	return strings.TrimSpace("flex flex-1 min-h-0 flex-col " + panelBodyClass(spec) + " h-[calc(100vh-8rem)] min-h-[70vh]")
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
		if statUsesCustomChrome(spec) {
			return "164px"
		}
		return "120px"
	case panel.KindTable:
		return "220px"
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
	return "load, change delay:800ms from:" + formID + ", dateRangeChange delay:800ms from:" + formID
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
	return "fullscreen = true; requestAnimationFrame(() => { const root = event && event.currentTarget && event.currentTarget.closest('[data-lens-rerender-scope]'); const rerender = () => { document.dispatchEvent(new CustomEvent('sdk:rerenderCharts', { detail: root ? { root } : {} })); window.dispatchEvent(new Event('resize')); }; setTimeout(rerender, 220); setTimeout(rerender, 420); setTimeout(rerender, 720); });"
}

func activateTabScript(tabID string) string {
	return "activeTab = " + jsStringLiteral(tabID) + "; " + rerenderChartsScript(180)
}

func panelPlaceholderRows(spec panel.Spec) int {
	switch spec.Kind {
	case panel.KindStat:
		return 2
	case panel.KindTable:
		return 5
	case panel.KindTabs, panel.KindGrid, panel.KindSplit, panel.KindRepeat:
		return 4
	case panel.KindTimeSeries, panel.KindBar, panel.KindHorizontalBar, panel.KindStackedBar, panel.KindPie, panel.KindDonut, panel.KindGauge:
		return 4
	}
	return 4
}
