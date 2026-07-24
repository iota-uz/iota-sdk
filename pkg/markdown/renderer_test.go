package markdown

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderer_SanitizesRawHTML(t *testing.T) {
	renderer := NewRenderer()

	html, err := renderer.Render([]byte("# Hello\n\n<script>alert(1)</script>\n\n[link](javascript:alert(1))"))

	require.NoError(t, err)
	out := string(html)
	require.Contains(t, out, "<h1")
	require.NotContains(t, out, "<script>")
	require.NotContains(t, out, "javascript:")
}

func TestRenderer_PreservesMermaidLanguageClass(t *testing.T) {
	renderer := NewRenderer()

	html, err := renderer.Render([]byte("```mermaid\ngraph LR\n  A --> B\n```"))

	require.NoError(t, err)
	require.Contains(t, string(html), `<code class="language-mermaid">`)
}
