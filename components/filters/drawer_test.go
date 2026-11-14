package filters_test

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/filters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderDrawer(t *testing.T, props filters.DrawerProps) string {
	t.Helper()
	var out strings.Builder
	body := templ.Raw(`<select name="category"></select>`)
	require.NoError(t, filters.Drawer(props).Render(templ.WithChildren(context.Background(), body), &out))
	return out.String()
}

func TestDrawerIsABottomSheetAsTallAsItsContent(t *testing.T) {
	out := renderDrawer(t, filters.DrawerProps{Heading: "Filters", Action: "open-filters"})
	assert.Contains(t, out, "dialog-btt")
	assert.Contains(t, out, "max-h-[85dvh]")
	assert.NotContains(t, out, "w-3/4")
	assert.NotContains(t, out, "h-full max-h-full w-full", "the sheet itself must not fill the screen")
	assert.Contains(t, out, `name="category"`)
}

func TestDrawerFooterNeedsLabels(t *testing.T) {
	bare := renderDrawer(t, filters.DrawerProps{Heading: "Filters", Action: "open-filters"})
	assert.NotContains(t, bare, "apply()")
	assert.NotContains(t, bare, "reset()")

	full := renderDrawer(t, filters.DrawerProps{Heading: "Filters", Action: "open-filters", ApplyLabel: "Apply", ResetLabel: "Reset"})
	assert.Contains(t, full, "apply()")
	assert.Contains(t, full, "reset()")
	assert.Contains(t, full, ">Apply<")
	assert.Contains(t, full, ">Reset<")
	assert.Contains(t, full, `x-data="filtersSheet"`)
}
