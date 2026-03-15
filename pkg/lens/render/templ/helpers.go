package templ

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	templpkg "github.com/a-h/templ"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/js"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

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

func panelResult(result *runtime.DashboardResult, panelID string) *runtime.PanelResult {
	if result == nil || result.Panels == nil {
		return nil
	}
	return result.Panels[panelID]
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

func filterModel(result *runtime.DashboardResult) filter.Model {
	if result == nil {
		return filter.Model{}
	}
	return result.Filters
}

func actionURL(spec *action.Spec, row map[string]any, variables map[string]any) string {
	if spec == nil {
		return ""
	}
	switch spec.Kind {
	case action.KindNavigate, action.KindHtmxSwap:
	case action.KindEmitEvent:
		return ""
	default:
		return ""
	}
	nextURL := spec.URL
	if len(spec.Params) == 0 {
		return nextURL
	}
	values := url.Values{}
	for _, param := range spec.Params {
		value, ok := actionValue(param.Source, row, variables)
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

func actionOnClick(spec *action.Spec, row map[string]any, variables map[string]any) templpkg.ComponentScript {
	if spec == nil {
		return templpkg.ComponentScript{}
	}
	switch spec.Kind {
	case action.KindNavigate:
		return templpkg.ComponentScript{}
	case action.KindHtmxSwap:
		href := actionURL(spec, row, variables)
		if href == "" {
			return templpkg.ComponentScript{}
		}
		method := spec.Method
		if method == "" {
			method = "GET"
		}
		return templpkg.JSUnsafeFuncCall(fmt.Sprintf("event.preventDefault(); htmx.ajax(%s, %s, {target: %s, swap: 'innerHTML'});", js.MustToJS(method), js.MustToJS(href), js.MustToJS(spec.Target)))
	case action.KindEmitEvent:
		payload := actionPayload(spec, row, variables)
		encoded, err := json.Marshal(payload)
		if err != nil {
			return templpkg.ComponentScript{}
		}
		return templpkg.JSUnsafeFuncCall(fmt.Sprintf("event.preventDefault(); document.dispatchEvent(new CustomEvent(%s, {detail: %s}));", js.MustToJS(spec.Event), encoded))
	default:
		return templpkg.ComponentScript{}
	}
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

func actionValue(source action.ValueSource, row map[string]any, variables map[string]any) (any, bool) {
	switch source.Kind {
	case action.SourceField:
		if row == nil {
			return nil, false
		}
		value, ok := row[source.Name]
		if !ok || value == nil || fmt.Sprint(value) == "" {
			if source.Fallback != nil {
				return source.Fallback, true
			}
			return nil, false
		}
		return value, true
	case action.SourcePoint:
		if row == nil {
			if source.Fallback != nil {
				return source.Fallback, true
			}
			return nil, false
		}
		value, ok := row[source.Name]
		if !ok || value == nil || fmt.Sprint(value) == "" {
			if source.Fallback != nil {
				return source.Fallback, true
			}
			return nil, false
		}
		return value, true
	case action.SourceLiteral:
		if source.Value == nil {
			return nil, false
		}
		return source.Value, true
	case action.SourceVariable:
		if variables == nil {
			if source.Fallback != nil {
				return source.Fallback, true
			}
			return nil, false
		}
		value, ok := variables[source.Name]
		if !ok || value == nil || fmt.Sprint(value) == "" {
			if source.Fallback != nil {
				return source.Fallback, true
			}
			return nil, false
		}
		return value, true
	default:
		return nil, false
	}
}

func containsQuery(raw string) bool {
	for _, ch := range raw {
		if ch == '?' {
			return true
		}
	}
	return false
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
	return spec.Icon != nil || spec.AccentColor != ""
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
	if color == "" {
		color = "#64748b"
	}
	r, g, b := parseHexColor(color)
	return templpkg.SafeCSS(fmt.Sprintf(
		"background-color: rgba(%d, %d, %d, 0.12); border: 1px solid rgba(%d, %d, %d, 0.22); color: %s;",
		r, g, b, r, g, b, color,
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

func panelFullscreenBodyClass(spec panel.Spec) string {
	return strings.TrimSpace(panelBodyClass(spec) + " h-[calc(100vh-8rem)] min-h-[70vh]")
}

func tabsPanelFrameClass(fullscreen bool) string {
	if fullscreen {
		return "flex flex-1 min-h-0"
	}
	return "flex-1"
}

func rerenderChartsScript(delayMs int) string {
	if delayMs <= 0 {
		delayMs = 180
	}
	return fmt.Sprintf("setTimeout(() => document.dispatchEvent(new Event('sdk:rerenderCharts')), %d)", delayMs)
}

func openFullscreenScript() string {
	return "fullscreen = true; " + rerenderChartsScript(180)
}

func activateTabScript(tabID string) string {
	return "activeTab = '" + tabID + "'; " + rerenderChartsScript(180)
}
