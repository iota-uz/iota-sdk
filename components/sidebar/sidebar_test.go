package sidebar

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// renderContext mirrors what the page-context middleware installs in
// production: a real localizer carrying the keys the sidebar's own components
// look up. Rendering with a nil localizer is covered by the base components
// themselves, which fall back rather than panic.
func renderContext() context.Context {
	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English, &i18n.Message{ID: "Common.TabNavigation", Other: "Tab navigation"})
	return composables.WithPageCtx(
		context.Background(),
		types.NewPageContext(language.English, &url.URL{Path: "/"}, i18n.NewLocalizer(bundle, language.English.String())),
	)
}

func TestSidebar_CollapsedFlyoutUsesTeleportSafeStore(t *testing.T) {
	t.Parallel()

	ctx := renderContext()

	props := Props{
		TabGroups: TabGroupCollection{
			Groups: []TabGroup{
				{
					Label: "ERP",
					Value: "erp",
					Items: []Item{
						NewGroup("Analytics", nil, []Item{
							NewLink("/analytics", "Dashboard", nil),
						}),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, Sidebar(props).Render(ctx, &buf))
	html := buf.String()

	require.Contains(t, html, `x-teleport="body"`)
	require.Contains(t, html, `data-sidebar-nav-id="sidebar-navigation"`)
	require.Contains(t, html, `x-bind:data-sidebar-nav-instance-id="navInstanceID"`)
	require.Contains(t, html, `x-show="$store.sidebarCollapsedMenus.isOpenFor($el)"`)
	require.Contains(t, html, `:style="$store.sidebarCollapsedMenus.styleFor($el)"`)
	require.NotContains(t, html, `x-show="isCollapsedMenuOpenFor($el)"`)
	require.NotContains(t, html, `:style="collapsedMenuStyleFor($el)"`)
}

func TestSidebar_MainNavigationIDIsUniqueAcrossTabs(t *testing.T) {
	t.Parallel()

	ctx := renderContext()

	props := Props{
		TabGroups: TabGroupCollection{
			Groups: []TabGroup{
				{
					Label: "ERP",
					Value: "erp",
					Items: []Item{
						NewLink("/erp", "ERP Dashboard", nil),
					},
				},
				{
					Label: "CRM",
					Value: "crm",
					Items: []Item{
						NewLink("/crm", "CRM Dashboard", nil),
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	require.NoError(t, Sidebar(props).Render(ctx, &buf))
	html := buf.String()

	require.Equal(t, 1, strings.Count(html, `<nav id="sidebar-navigation"`))
	require.Contains(t, html, `<nav id="sidebar-navigation"`)
	require.NotContains(t, html, `<ul id="sidebar-navigation"`)
}
