package help

import (
	"strings"

	"github.com/a-h/templ"
)

type ContextSection struct {
	Title string
	Items []string
}

type ContextProps struct {
	Title        string
	Summary      string
	ArticlePath  string
	ArticleLabel string
	ScreenPath   string
	ScreenLabel  string
	Label        string
	Sections     []ContextSection
	Class        any
	Attrs        templ.Attributes
}

type HintProps struct {
	Title        string
	Description  string
	Next         string
	ArticlePath  string
	ArticleLabel string
	Label        string
	Class        any
}

func contextLabel(props ContextProps) string {
	if strings.TrimSpace(props.Label) != "" {
		return props.Label
	}
	return defaultLabel
}

func withClass(base string, extra any) templ.CSSClasses {
	if extra == nil {
		return templ.Classes(base)
	}
	return templ.Classes(base, extra)
}

func contextClass(props ContextProps) templ.CSSClasses {
	return withClass("relative inline-flex shrink-0", props.Class)
}

func hintClass(props HintProps) templ.CSSClasses {
	return withClass("relative inline-flex align-middle", props.Class)
}

func contextAttrs(props ContextProps) templ.Attributes {
	attrs := templ.Attributes{}
	for key, value := range props.Attrs {
		attrs[key] = value
	}
	return attrs
}

func hintLabel(props HintProps) string {
	if strings.TrimSpace(props.Label) != "" {
		return props.Label
	}
	return props.Title
}
