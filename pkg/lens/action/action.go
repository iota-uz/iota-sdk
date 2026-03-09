package action

type Kind string

const (
	KindNavigate  Kind = "navigate"
	KindHtmxSwap  Kind = "htmx_swap"
	KindEmitEvent Kind = "emit_event"
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
			Kind:  SourceVariable,
			Name:  variable,
			Value: variable,
		},
	}
}
