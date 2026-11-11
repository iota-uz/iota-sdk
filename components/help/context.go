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
	// Class is appended to the wrapper classes. Accepts anything templ.Classes
	// accepts (string, templ.CSSClass, map[string]bool, templ.KV values) and
	// []any of those; nil entries are ignored.
	Class any
	Attrs templ.Attributes
}

type HintProps struct {
	Title        string
	Description  string
	Next         string
	ArticlePath  string
	ArticleLabel string
	Label        string
	// Class accepts the same values as ContextProps.Class.
	Class any
}

func contextLabel(props ContextProps) string {
	if strings.TrimSpace(props.Label) != "" {
		return props.Label
	}
	return defaultLabel
}

// withClass joins base with extra for templ's class processor, which rejects
// nil and []any with a --templ-css-class-unknown-type marker.
func withClass(base string, extra any) templ.CSSClasses {
	classes := []any{base}
	var add func(v any)
	add = func(v any) {
		switch value := v.(type) {
		case nil:
		case []any:
			for _, item := range value {
				add(item)
			}
		default:
			classes = append(classes, value)
		}
	}
	add(extra)
	return templ.Classes(classes...)
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
