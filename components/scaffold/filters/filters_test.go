package filters_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/components/scaffold/filters"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func render(t *testing.T, f *filters.TableFilter) string {
	t.Helper()
	ctx := composables.WithPageCtx(context.Background(), types.NewPageContext(language.English, &url.URL{}, i18n.NewLocalizer(i18n.NewBundle(language.English), "en")))
	var out strings.Builder
	require.NoError(t, f.Component().Render(ctx, &out))
	return out.String()
}

func TestSelectFilterShowsCaptionOnlyInTheMobileSheet(t *testing.T) {
	out := render(t, filters.NewFilter("category",
		filters.WithLabel("Category"),
		filters.WithPlaceholder("All"),
		filters.WithOptions(filters.Opt("1", "Rent")),
	))
	assert.Contains(t, out, `data-filter-label`)
	assert.Contains(t, out, ">Category<")
	assert.Contains(t, out, "md:hidden", "the desktop toolbar keeps showing the field alone")
}

func TestSelectFilterCaptionFallsBackToPlaceholder(t *testing.T) {
	out := render(t, filters.NewFilter("category",
		filters.WithPlaceholder("Category"),
		filters.WithOptions(filters.Opt("1", "Rent")),
	))
	assert.Contains(t, out, `data-filter-label`)
}

func TestSelectFilterWithoutLabelOrPlaceholderHasNoCaption(t *testing.T) {
	out := render(t, filters.NewFilter("category", filters.WithOptions(filters.Opt("1", "Rent"))))
	assert.NotContains(t, out, `data-filter-label`)
}
