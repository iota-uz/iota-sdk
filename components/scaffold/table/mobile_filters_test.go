package table

import (
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"
)

func TestMobileFiltersReachableInPageAndEmbeddedTable(t *testing.T) {
	ctx := testPageCtx(t, "/accounts")
	for _, layout := range []struct {
		name   string
		render func(*TableConfig) templ.Component
	}{{"embedded", EmbeddedContent}, {"page", Content}} {
		for _, filterKind := range []string{"none", "inline", "sidebar"} {
			t.Run(layout.name+"/"+filterKind, func(t *testing.T) {
				cfg := NewTableConfig("Accounts", "/accounts")
				cfg.Configurable = false
				filter := templ.Raw(`<input name="category" value="test">`)
				if filterKind == "inline" {
					cfg.Filters = []templ.Component{filter}
				}
				if filterKind == "sidebar" {
					cfg.SideFilter = filter
				}
				var output strings.Builder
				require.NoError(t, layout.render(cfg).Render(ctx, &output))
				expected := 1
				if filterKind == "none" {
					expected = 0
				}
				require.Equal(t, expected, strings.Count(output.String(), "$dispatch(&#39;open-filters-"))
				if expected > 0 {
					action := regexp.MustCompile(`\$dispatch\(&#39;(open-filters-[a-f0-9-]+)&#39;\)`).FindStringSubmatch(output.String())
					require.Len(t, action, 2)
					require.Contains(t, output.String(), "@"+action[1]+".window")
					var second strings.Builder
					require.NoError(t, layout.render(cfg).Render(ctx, &second))
					require.NotContains(t, second.String(), action[1], "even repeated configurations must have isolated drawer actions")
				}
				require.Equal(t, 1, strings.Count(output.String(), "<form"), "drawer close must not nest a form inside the filter form")
			})
		}
	}
}
