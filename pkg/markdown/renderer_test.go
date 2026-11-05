package markdown

import (
	"regexp"
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

func TestRenderer_PreservesImageMetadata(t *testing.T) {
	renderer := NewRenderer()

	html, err := renderer.Render([]byte(`![Policy workflow](/help/media/assets/policy-workflow.svg "From quote to coverage")`))

	require.NoError(t, err)
	out := string(html)
	require.Contains(t, out, `src="/help/media/assets/policy-workflow.svg"`)
	require.Contains(t, out, `alt="Policy workflow"`)
	require.Contains(t, out, `title="From quote to coverage"`)
}

// False green: a pattern that also matches the stripped class, or an element
// list that silently keeps classes everywhere.
func TestRenderer_WithAllowedClassesKeepsOnlyMatchingClasses(t *testing.T) {
	renderer := NewRendererWithOptions(WithAllowedClasses(regexp.MustCompile(`^help-card$`), "div"))

	html, err := renderer.Render([]byte("<div class=\"help-card\">a</div>\n\n<div class=\"evil\">b</div>\n\n<span class=\"help-card\">c</span>"))

	require.NoError(t, err)
	out := string(html)
	require.Contains(t, out, `<div class="help-card">a</div>`)
	require.Contains(t, out, `<div>b</div>`)
	require.Contains(t, out, `<span>c</span>`)
}

func TestRenderer_DefaultStripsClasses(t *testing.T) {
	html, err := NewRenderer().Render([]byte(`<div class="help-card">a</div>`))

	require.NoError(t, err)
	require.Contains(t, string(html), `<div>a</div>`)
}

// False green: ASCII-only headings, which goldmark's default generator already
// turns into readable ids, or checking the generator without the sanitizer
// that used to strip non-ASCII ids.
func TestRenderer_HeadingIDsKeepEveryScript(t *testing.T) {
	html, err := NewRenderer().Render([]byte("## Шаги на экране\n\n## Шаги на экране\n\n### 2. Проверьте НАПП\n\n## Step one\n\n## ¿?"))

	require.NoError(t, err)
	out := string(html)
	require.Contains(t, out, `<h2 id="шаги-на-экране">`)
	require.Contains(t, out, `<h2 id="шаги-на-экране-1">`)
	require.Contains(t, out, `<h3 id="2-проверьте-напп">`)
	require.Contains(t, out, `<h2 id="step-one">`)
	require.Contains(t, out, `<h2 id="heading">`)
}
