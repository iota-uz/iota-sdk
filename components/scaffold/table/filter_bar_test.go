package table

import (
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"
)

func TestFilterBarStaysVisibleAndOutOfDrawer(t *testing.T) {
	ctx := testPageCtx(t, "/accounts")
	for _, layout := range []struct {
		name   string
		render func(*TableConfig) templ.Component
	}{{"embedded", EmbeddedContent}, {"page", Content}} {
		t.Run(layout.name, func(t *testing.T) {
			cfg := NewTableConfig("Accounts", "/accounts",
				WithFilterBar(templ.Raw(`<div id="bar">chips</div>`), templ.Raw(`<div id="bar" hx-swap-oob="outerHTML">chips</div>`)))
			cfg.Configurable = false
			var out strings.Builder
			require.NoError(t, layout.render(cfg).Render(ctx, &out))
			html := out.String()
			require.Equal(t, 1, strings.Count(html, `id="bar"`), "the bar renders once, not again in the drawer")
			require.Contains(t, html, "data-filter-bar")
			require.NotContains(t, html, "$dispatch(&#39;open-filters-", "a chip bar alone needs no filters drawer")
			require.NotContains(t, html, `hx-swap-oob`, "the out-of-band twin is for row swaps only")
			require.Len(t, cfg.FiltersOOB, 1)
		})
	}
}
