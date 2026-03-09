package templ

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens/action"
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
		columns = append(columns, panel.TableColumn{Field: field.Name, Label: field.Name})
	}
	return columns
}

func statRawValue(spec panel.Spec, result *runtime.PanelResult) any {
	if result == nil || result.Frames == nil || result.Frames.Primary() == nil || result.Frames.Primary().RowCount == 0 {
		return "-"
	}
	rows := result.Frames.Primary().Rows()
	fieldName := spec.Fields.Value
	if fieldName == "" {
		fieldName = "value"
	}
	return rows[0][fieldName]
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

func formatDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
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

func formatVariableValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case []string:
		return strings.Join(v, ",")
	default:
		return fmt.Sprint(v)
	}
}

func variableValue(values map[string]any, name string) any {
	if values == nil {
		return nil
	}
	return values[name]
}

func variableBool(values map[string]any, name string) bool {
	value, ok := values[name]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1" || v == "on"
	default:
		return false
	}
}

func actionURL(spec *action.Spec, row map[string]any, variables map[string]any) string {
	if spec == nil || spec.Kind != action.KindNavigate {
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
