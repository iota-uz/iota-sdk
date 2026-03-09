package templ

import (
	"fmt"
	"net/url"
	"time"

	templpkg "github.com/a-h/templ"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/js"
	"github.com/iota-uz/iota-sdk/pkg/lens/action"
	"github.com/iota-uz/iota-sdk/pkg/lens/filter"
	"github.com/iota-uz/iota-sdk/pkg/lens/format"
	"github.com/iota-uz/iota-sdk/pkg/lens/panel"
	"github.com/iota-uz/iota-sdk/pkg/lens/runtime"
)

var spanClasses = map[int]string{
	1:  "col-span-12 md:col-span-1",
	2:  "col-span-12 md:col-span-2",
	3:  "col-span-12 md:col-span-3",
	4:  "col-span-12 md:col-span-4",
	5:  "col-span-12 md:col-span-5",
	6:  "col-span-12 md:col-span-6",
	7:  "col-span-12 md:col-span-7",
	8:  "col-span-12 md:col-span-8",
	9:  "col-span-12 md:col-span-9",
	10: "col-span-12 md:col-span-10",
	11: "col-span-12 md:col-span-11",
	12: "col-span-12 md:col-span-12",
}

func spanClass(span int) string {
	if className, ok := spanClasses[span]; ok {
		return className
	}
	return spanClasses[6]
}

func defaultTab(spec panel.Spec) string {
	if spec.DefaultChild != "" {
		return spec.DefaultChild
	}
	if len(spec.Children) == 0 {
		return ""
	}
	return spec.Children[0].ID
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
		return templpkg.JSUnsafeFuncCall(fmt.Sprintf("event.preventDefault(); document.dispatchEvent(new CustomEvent(%s, {detail: %s}));", js.MustToJS(spec.Event), js.MustToJS(payload)))
	default:
		return templpkg.ComponentScript{}
	}
}

func rowActionOnClick(spec *action.Spec, row map[string]any, variables map[string]any) templpkg.ComponentScript {
	if spec == nil {
		return templpkg.ComponentScript{}
	}
	if onClick := actionOnClick(spec, row, variables); onClick.Call != "" {
		return onClick
	}
	href := actionURL(spec, row, variables)
	if href == "" {
		return templpkg.ComponentScript{}
	}
	return templpkg.JSUnsafeFuncCall(fmt.Sprintf("window.location.href = %s;", js.MustToJS(href)))
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

func tabsState(spec panel.Spec) string {
	return js.MustToJS(struct {
		ActiveTab string `json:"activeTab"`
	}{
		ActiveTab: defaultTab(spec),
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

func panelBodyClass(spec panel.Spec) string {
	switch spec.Kind {
	case panel.KindStat:
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
