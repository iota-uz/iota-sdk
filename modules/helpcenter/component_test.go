package helpcenter

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// False green: asserting the pattern string instead of rendered HTML, or a
// pattern that lets any help-* class through.
func TestArticleRendererKeepsOnlyHelpCenterHooks(t *testing.T) {
	html, err := newArticleRenderer().Render([]byte("<a class=\"help-task-card\" href=\"/help/doc/a.md\"><div class=\"help-task-title\">A</div></a>\n\n<div class=\"help-shell\">x</div>\n\n<div class=\"help-task-card evil\">y</div>"))

	require.NoError(t, err)
	out := string(html)
	require.Contains(t, out, `<a class="help-task-card" href="/help/doc/a.md"`)
	require.Contains(t, out, `<div class="help-task-title">A</div>`)
	require.Contains(t, out, `<div>x</div>`)
	require.Contains(t, out, `<div>y</div>`)
}
