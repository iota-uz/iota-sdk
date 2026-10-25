package table

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestMobileFiltersReachableInPageAndEmbeddedTable(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	for _, key := range []string{"Scaffold.Filters.Title", "Scaffold.Table.NoDataTitle", "Scaffold.Table.NoDataDescription", "Search"} {
		bundle.MustAddMessages(language.English, &i18n.Message{ID: key, Other: key})
	}
	ctx := composables.WithPageCtx(context.Background(), types.NewPageContext(language.English, &url.URL{}, i18n.NewLocalizer(bundle, "en")))
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
				require.Equal(t, expected, strings.Count(output.String(), "$dispatch(&#39;open-filters&#39;)"))
				require.Equal(t, 1, strings.Count(output.String(), "<form"), "drawer close must not nest a form inside the filter form")
			})
		}
	}
}
