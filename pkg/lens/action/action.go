// Package action defines panel actions and action value sources for Lens dashboards.
package action

type Kind string

const (
	KindNavigate  Kind = "navigate"
	KindHtmxSwap  Kind = "htmx_swap"
	KindEmitEvent Kind = "emit_event"
	KindDrill     Kind = "drill"
)

type ValueSourceKind string

const (
	SourceLiteral  ValueSourceKind = "literal"
	SourceField    ValueSourceKind = "field"
	SourceVariable ValueSourceKind = "variable"
	SourcePoint    ValueSourceKind = "point"
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

type Plugin interface {
	Name() string
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

func DrillDashboard(url, pageTitle, scopeLabel string, params ...Param) Spec {
	return Spec{
		Kind:   KindDrill,
		URL:    url,
		Method: "GET",
		Params: params,
		Drill: &DrillSpec{
			Destination: DestinationDashboard,
			PageTitle:   pageTitle,
			ScopeLabel:  scopeLabel,
			LabelSource: PointValue("label"),
		},
	}
}

func DrillRaw(url, pageTitle, scopeLabel string, params ...Param) Spec {
	return Spec{
		Kind:   KindDrill,
		URL:    url,
		Method: "GET",
		Params: params,
		Drill: &DrillSpec{
			Destination: DestinationRaw,
			PageTitle:   pageTitle,
			ScopeLabel:  scopeLabel,
			LabelSource: PointValue("label"),
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

func (s Spec) WithDrillLabel(source ValueSource) Spec {
	s = s.withClonedDrill()
	if s.Drill != nil {
		s.Drill.LabelSource = source
	}
	return s
}

func (s Spec) WithDrillScopeLabel(label string) Spec {
	s = s.withClonedDrill()
	if s.Drill != nil {
		s.Drill.ScopeLabel = label
	}
	return s
}

func (s Spec) WithDrillPageTitle(title string) Spec {
	s = s.withClonedDrill()
	if s.Drill != nil {
		s.Drill.PageTitle = title
	}
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

func PointParam(name, field string) Param {
	return Param{
		Name: name,
		Source: ValueSource{
			Kind: SourcePoint,
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

func PointValue(field string) ValueSource {
	return ValueSource{Kind: SourcePoint, Name: field}
}

func LiteralValue(value any) ValueSource {
	return ValueSource{Kind: SourceLiteral, Value: value}
}

func VariableValue(variable string) ValueSource {
	return ValueSource{Kind: SourceVariable, Name: variable}
}
