package table

import (
	"context"
	"html"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// A control inside a filter-builder popover is only a draft until Apply: a request
// fired by it would send the old filters and swap the open popover away. hx-trigger
// from:<selector> listens on the matching elements directly, so each clause that
// can match such a control needs its own guard.
func TestFormTriggerIgnoresFilterBuilderEditors(t *testing.T) {
	bundle := i18n.NewBundle(language.English)
	for _, key := range []string{"Scaffold.Filters.Title", "Scaffold.Table.NoDataTitle", "Scaffold.Table.NoDataDescription", "Search"} {
		bundle.MustAddMessages(language.English, &i18n.Message{ID: key, Other: key})
	}
	ctx := composables.WithPageCtx(context.Background(), types.NewPageContext(
		language.English,
		&url.URL{},
		i18n.NewLocalizer(bundle, "en"),
	))

	cfg := NewTableConfig("Accounts", "/accounts")
	cfg.Configurable = false

	var output strings.Builder
	require.NoError(t, TableSection(cfg).Render(ctx, &output))

	match := regexp.MustCompile(`<form[^>]*\shx-trigger="([^"]*)"`).FindStringSubmatch(output.String())
	require.Len(t, match, 2, "the table form declares hx-trigger")

	clauses := strings.Split(html.UnescapeString(match[1]), ", ")
	var fromControl []string
	for _, clause := range clauses {
		if strings.Contains(clause, "from:") {
			fromControl = append(fromControl, clause)
		}
	}
	require.NotEmpty(t, fromControl)

	for _, clause := range fromControl {
		assert.Contains(t, clause, "[!target.closest('[data-fb-popover]')]", clause)
	}
	assert.Contains(t, clauses, "filter-changed", "Apply still submits the form")
}
