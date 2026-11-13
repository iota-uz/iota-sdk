package table

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every control of the form submits it through events that bubble to the form
// itself. A listener bound with from:<selector> would also hear same-named
// fields elsewhere on the page (a theme switch, another table's filter), so none
// may be left, and each clause that can come from a filter-builder popover needs
// the guard.
func TestFormTriggerListensOnlyToItsOwnForm(t *testing.T) {
	ctx := testPageCtx(t, "/accounts")

	cfg := NewTableConfig("Accounts", "/accounts")
	cfg.Configurable = false

	var output strings.Builder
	require.NoError(t, TableSection(cfg).Render(ctx, &output))

	match := regexp.MustCompile(`<form[^>]*\shx-trigger="([^"]*)"`).FindStringSubmatch(output.String())
	require.Len(t, match, 2, "the table form declares hx-trigger")

	clauses := strings.Split(html.UnescapeString(match[1]), ", ")
	require.NotEmpty(t, clauses)
	assert.Contains(t, clauses, "filter-changed", "Apply still submits the form")

	for _, clause := range clauses {
		assert.NotContains(t, clause, "from:", clause)
		assert.NotContains(t, clause, "changed delay", "htmx compares the form's own value for the changed modifier: "+clause)
		if clause == "filter-changed" {
			continue
		}
		assert.Contains(t, clause, "!target.closest('[data-fb-popover]')", clause)
		assert.Contains(t, clause, "detail.silent", clause)
	}
}

func TestCustomHxTriggerIsKept(t *testing.T) {
	cfg := NewTableConfig("Accounts", "/accounts")
	cfg.HxTrigger = "load"
	assert.Equal(t, "load", cfg.ResolvedHxTrigger())
}

func TestFormPushURL(t *testing.T) {
	ctx := testPageCtx(t, "/accounts")
	for name, tc := range map[string]struct {
		opts []TableConfigOpt
		want string
	}{
		"default":  {nil, "true"},
		"disabled": {[]TableConfigOpt{WithoutPushURL()}, "false"},
		"host url": {[]TableConfigOpt{WithPushURL("/finance/overview")}, "/finance/overview"},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := NewTableConfig("Accounts", "/accounts", tc.opts...)
			var output strings.Builder
			require.NoError(t, TableSection(cfg).Render(ctx, &output))
			assert.Contains(t, output.String(), `<form hx-get="/accounts" hx-push-url="`+tc.want+`"`)
		})
	}
}
