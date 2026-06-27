package sidebar

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

func TestSidebar_CollapsedFlyoutUsesTeleportSafeStore(t *testing.T) {
	t.Parallel()

	ctx := composables.WithPageCtx(
		context.Background(),
		types.NewPageContext(language.English, &url.URL{Path: "/"}, nil),
	)

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

	ctx := composables.WithPageCtx(
		context.Background(),
		types.NewPageContext(language.English, &url.URL{Path: "/"}, nil),
	)

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
