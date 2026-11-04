package help

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// False green: splitting at the first </p> anywhere, which would put the meta
// line inside a callout or after the first section.
func TestSplitAfterLead(t *testing.T) {
	head, rest := splitAfterLead("<h1 id=\"t\">Title</h1>\n<p>Lead.</p>\n<h2>Section</h2>\n<p>Body.</p>")
	require.Equal(t, "<h1 id=\"t\">Title</h1>\n<p>Lead.</p>", head)
	require.Equal(t, "\n<h2>Section</h2>\n<p>Body.</p>", rest)

	head, rest = splitAfterLead("<h1>Title</h1>\n<blockquote><p>Note.</p></blockquote>")
	require.Equal(t, "<h1>Title</h1>", head)
	require.Equal(t, "\n<blockquote><p>Note.</p></blockquote>", rest)

	head, rest = splitAfterLead("<p>No title.</p>")
	require.Empty(t, head)
	require.Equal(t, "<p>No title.</p>", rest)

	head, rest = splitAfterLead("<p>Disclaimer.</p>\n<h1>Late title</h1>\n<p>Text.</p>")
	require.Empty(t, head)
	require.Equal(t, "<p>Disclaimer.</p>\n<h1>Late title</h1>\n<p>Text.</p>", rest)
}

func TestPartHTMLEscapesAndEmphasises(t *testing.T) {
	require.Equal(t, "<b>Глава 1</b> из 5", partHTML("**Глава 1** из 5"))
	require.Equal(t, "&lt;script&gt; <b>&amp;</b>", partHTML("<script> **&**"))
}
