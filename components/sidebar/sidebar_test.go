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
	return renderContextForPath("/")
}

func renderContextForPath(path string) context.Context {
	bundle := i18n.NewBundle(language.English)
	bundle.MustAddMessages(language.English, &i18n.Message{ID: "Common.TabNavigation", Other: "Tab navigation"})
	return composables.WithPageCtx(
		context.Background(),
		types.NewPageContext(language.English, &url.URL{Path: path}, i18n.NewLocalizer(bundle, language.English.String())),
	)
}

func TestBuildSidebarNavTabs_SelectsMostSpecificMatchingRoute(t *testing.T) {
	t.Parallel()

	claims := NewGroup("Claims", nil, []Item{
		NewLink("/claims", "Current claims", nil),
		NewLink("/claims/archive", "Archived claims", nil),
	})
	testCases := []struct {
		name          string
		path          string
		currentActive bool
		archiveActive bool
	}{
		{name: "current claims list", path: "/claims", currentActive: true},
		{name: "current claim detail", path: "/claims/123", currentActive: true},
		{name: "archive list", path: "/claims/archive", archiveActive: true},
		{name: "archive claim detail", path: "/claims/archive/123", archiveActive: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			tabs := BuildSidebarNavTabs(renderContextForPath(testCase.path), TabGroupCollection{
				Groups: []TabGroup{{
					Label: "ERP",
					Value: "erp",
					Items: []Item{claims},
				}},
			})

			require.Len(t, tabs, 1)
			require.Len(t, tabs[0].Nodes, 1)
			require.True(t, tabs[0].Nodes[0].IsActive)
			require.Len(t, tabs[0].Nodes[0].Children, 2)
			require.Equal(t, testCase.currentActive, tabs[0].Nodes[0].Children[0].IsActive)
			require.Equal(t, testCase.archiveActive, tabs[0].Nodes[0].Children[1].IsActive)
		})
	}
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
