package table

import (
	"context"
	"net/url"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

var testMessageIDs = []string{
	"Search",
	"Scaffold.Filters.Title",
	"Scaffold.Filters.Apply",
	"Scaffold.Filters.Reset",
	"Scaffold.Filters.Active",
	"Scaffold.Filters.RemoveChip",
	"Scaffold.Table.NoDataTitle",
	"Scaffold.Table.NoDataDescription",
	"Scaffold.Table.NothingFound",
	"Scaffold.Table.NothingFoundDescription",
	"Scaffold.Table.MoreActions",
	"Scaffold.Table.TableSettings",
	"Scaffold.Table.ResetSettings",
	"Scaffold.Table.Columns",
	"Scaffold.Table.Grid",
	"Scaffold.Table.GridVertical",
	"Scaffold.Table.GridHorizontal",
	"Actions",
	"Add",
}

// testPageCtx returns a context carrying a page context for target (path and
// query), with every message the scaffold table renders answering with its ID.
func testPageCtx(t *testing.T, target string) context.Context {
	t.Helper()
	u, err := url.Parse(target)
	require.NoError(t, err)
	bundle := i18n.NewBundle(language.English)
	for _, id := range testMessageIDs {
		bundle.MustAddMessages(language.English, &i18n.Message{ID: id, Other: id})
	}
	return composables.WithPageCtx(context.Background(), types.NewPageContext(language.English, u, i18n.NewLocalizer(bundle, "en")))
}
