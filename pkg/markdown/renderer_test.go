package markdown

import (
	"strings"
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
	require.False(t, strings.Contains(out, "javascript:"))
}
