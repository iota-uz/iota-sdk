// Package action defines panel actions and action value sources for Lens dashboards.
package action

import "fmt"

type Kind string

const (
	KindNavigate  Kind = "navigate"
	KindHtmxSwap  Kind = "htmx_swap"
	KindEmitEvent Kind = "emit_event"
	KindCubeDrill Kind = "cube_drill"
)

type ValueSourceKind string

const (
	SourceLiteral  ValueSourceKind = "literal"
	SourceField    ValueSourceKind = "field"
	SourceVariable ValueSourceKind = "variable"
)

type ValueSource struct {
	Kind     ValueSourceKind
	Name     string
	Value    any
	Fallback any
}

type Param struct {
	Name   string
	Source ValueSource
}

type Spec struct {
	Kind    Kind
	Method  string
	URL     string
	Target  string
	Event   string
	Payload map[string]ValueSource
	Params  []Param
	Drill   *DrillSpec
}

func Navigate(url string, params ...Param) Spec {
	return Spec{
		Kind:   KindNavigate,
		URL:    url,
		Method: "GET",
		Params: params,
	}
}

func HtmxSwap(url, target string, params ...Param) Spec {
	return Spec{
		Kind:   KindHtmxSwap,
		URL:    url,
		Method: "GET",
		Target: target,
		Params: params,
	}
}

func CubeDrill(url, dimension string, params ...Param) Spec {
	return Spec{
		Kind:   KindCubeDrill,
		Method: "GET",
		URL:    url,
		Params: params,
		Drill: &DrillSpec{
			Dimension: dimension,
			Value:     FieldValue("filter_value"),
		},
	}
}

func (s Spec) withClonedDrill() Spec {
	if s.Drill == nil {
		return s
	}
	drill := *s.Drill
	s.Drill = &drill
	return s
}

func (s Spec) WithDrillValue(source ValueSource) Spec {
	if s.Drill == nil {
		return s
	}
	s = s.withClonedDrill()
	s.Drill.Value = source
	return s
}

func FieldParam(name, field string) Param {
	return Param{
		Name: name,
		Source: ValueSource{
			Kind: SourceField,
			Name: field,
		},
	}
}

func LiteralParam(name string, value any) Param {
	return Param{
		Name: name,
		Source: ValueSource{
			Kind:  SourceLiteral,
			Value: value,
		},
	}
}

func VariableParam(name, variable string) Param {
	return Param{
		Name: name,
		Source: ValueSource{
			Kind: SourceVariable,
			Name: variable,
		},
	}
}

func FieldValue(field string) ValueSource {
	return ValueSource{Kind: SourceField, Name: field}
}

func LiteralValue(value any) ValueSource {
	return ValueSource{Kind: SourceLiteral, Value: value}
}

func VariableValue(variable string) ValueSource {
	return ValueSource{Kind: SourceVariable, Name: variable}
}

// ResolveValue resolves a ValueSource against a data row and variable map,
// returning the resolved value and whether a non-empty value was found.
func ResolveValue(source ValueSource, row map[string]any, variables map[string]any) (any, bool) {
	switch source.Kind {
	case SourceField:
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
	case SourceLiteral:
		if source.Value == nil {
			return nil, false
		}
		return source.Value, true
	case SourceVariable:
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
