// Package filter normalizes Lens variable specs into render-friendly filter models.
package filter

import (
	"fmt"
	"strings"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
)

type Model struct {
	Inputs []Input
}

type Input struct {
	Name        string
	Label       string
	Description string
	Kind        lens.VariableKind
	Required    bool
	Options     []Option
	Value       string
	Values      []string
	Checked     bool
	DateRange   DateRange
}

type Option struct {
	Label    string
	Value    string
	Selected bool
}

type DateRange struct {
	Mode         string
	AllowAllTime bool
	Start        string
	End          string
}

func Build(specs []lens.VariableSpec, values map[string]any) Model {
	inputs := make([]Input, 0, len(specs))
	for _, spec := range specs {
		value, ok := values[spec.Name]
		if !ok || value == nil {
			value = defaultValue(spec)
		}
		inputs = append(inputs, buildInput(spec, value))
	}
	return Model{Inputs: inputs}
}

func defaultValue(spec lens.VariableSpec) any {
	if spec.Kind != lens.VariableDateRange {
		return spec.Default
	}
	if value, ok := spec.Default.(lens.DateRangeValue); ok {
		return value
	}
	if spec.DefaultDuration <= 0 {
		return spec.Default
	}
	now := time.Now().UTC()
	start := now.Add(-spec.DefaultDuration)
	return lens.DateRangeValue{
		Mode:  "default",
		Start: &start,
		End:   &now,
	}
}

func buildInput(spec lens.VariableSpec, value any) Input {
	input := Input{
		Name:        spec.Name,
		Label:       spec.Label,
		Description: spec.Description,
		Kind:        spec.Kind,
		Required:    spec.Required,
		Options:     buildOptions(spec.Options, value),
	}
	switch spec.Kind {
	case lens.VariableDateRange:
		input.DateRange = buildDateRange(spec, value)
	case lens.VariableToggle:
		input.Checked = asBool(value)
	case lens.VariableMultiSelect:
		input.Values = asStrings(value)
	default:
		input.Value = asString(value)
	}
	return input
}

func buildOptions(specs []lens.VariableOption, value any) []Option {
	if len(specs) == 0 {
		return nil
	}
	selected := make(map[string]struct{}, len(asStrings(value)))
	for _, item := range asStrings(value) {
		selected[item] = struct{}{}
	}
	current := asString(value)
	options := make([]Option, 0, len(specs))
	for _, spec := range specs {
		_, picked := selected[spec.Value]
		options = append(options, Option{
			Label:    spec.Label,
			Value:    spec.Value,
			Selected: picked || current == spec.Value,
		})
	}
	return options
}

func buildDateRange(spec lens.VariableSpec, value any) DateRange {
	current, ok := value.(lens.DateRangeValue)
	if !ok {
		current = lens.DateRangeValue{Mode: "default"}
	}
	mode := normalizeDateRangeMode(current.Mode)
	if mode == "all" && !spec.AllowAllTime {
		mode = "default"
	}
	return DateRange{
		Mode:         mode,
		AllowAllTime: spec.AllowAllTime,
		Start:        formatDate(current.Start),
		End:          formatDate(current.End),
	}
}

func formatDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func normalizeDateRangeMode(mode string) string {
	normalized := strings.TrimSpace(mode)
	switch normalized {
	case "all", "bounded", "default":
		return normalized
	default:
		return "default"
	}
}

func asBool(value any) bool {
	switch current := value.(type) {
	case bool:
		return current
	case string:
		return current == "true" || current == "1" || current == "on"
	default:
		return false
	}
}

func asString(value any) string {
	switch current := value.(type) {
	case nil:
		return ""
	case string:
		return current
	case fmt.Stringer:
		return current.String()
	default:
		return fmt.Sprint(value)
	}
}

func asStrings(value any) []string {
	switch current := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string(nil), current...)
	case []any:
		values := make([]string, 0, len(current))
		for _, item := range current {
			values = append(values, asString(item))
		}
		return values
	case string:
		if current == "" {
			return nil
		}
		return []string{current}
	default:
		return []string{asString(value)}
	}
}
