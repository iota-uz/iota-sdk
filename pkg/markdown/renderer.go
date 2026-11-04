// Package markdown provides this package.
package markdown

import (
	"bytes"
	"html/template"
	"regexp"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

type Renderer interface {
	Render(source []byte) (template.HTML, error)
}

type renderer struct {
	md        goldmark.Markdown
	sanitizer *bluemonday.Policy
}

// Option customizes the sanitizer of a renderer built by NewRendererWithOptions.
type Option func(policy *bluemonday.Policy)

// WithAllowedClasses keeps class attributes matching pattern on the given
// elements. Classes are otherwise stripped, so a caller opts in only the
// presentation hooks its own stylesheet owns.
func WithAllowedClasses(pattern *regexp.Regexp, elements ...string) Option {
	return func(policy *bluemonday.Policy) {
		policy.AllowAttrs("class").Matching(pattern).OnElements(elements...)
	}
}

var mermaidLanguagePattern = regexp.MustCompile(`^language-mermaid$`)

func NewRenderer() Renderer {
	return NewRendererWithOptions()
}

func NewRendererWithOptions(opts ...Option) Renderer {
	sanitizer := bluemonday.UGCPolicy()
	sanitizer.AllowAttrs("class").
		Matching(mermaidLanguagePattern).
		OnElements("code")
	for _, opt := range opts {
		opt(sanitizer)
	}

	return &renderer{
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithParserOptions(parser.WithAutoHeadingID()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		),
		sanitizer: sanitizer,
	}
}

func (r *renderer) Render(source []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(source, &buf); err != nil {
		return "", err
	}
	return template.HTML(r.sanitizer.SanitizeBytes(buf.Bytes())), nil
}
