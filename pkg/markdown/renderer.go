package markdown

import (
	"bytes"
	"html/template"

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

func NewRenderer() Renderer {
	return &renderer{
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithParserOptions(parser.WithAutoHeadingID()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		),
		sanitizer: bluemonday.UGCPolicy(),
	}
}

func (r *renderer) Render(source []byte) (template.HTML, error) {
	var buf bytes.Buffer
	if err := r.md.Convert(source, &buf); err != nil {
		return "", err
	}
	return template.HTML(r.sanitizer.SanitizeBytes(buf.Bytes())), nil
}
