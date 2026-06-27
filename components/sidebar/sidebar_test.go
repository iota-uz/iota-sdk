package sidebar

import (
	"bytes"
	"context"
	"net/url"
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
	require.Contains(t, html, `x-show="$store.sidebarCollapsedMenus.isOpenFor($el)"`)
	require.Contains(t, html, `:style="$store.sidebarCollapsedMenus.styleFor($el)"`)
	require.NotContains(t, html, `x-show="isCollapsedMenuOpenFor($el)"`)
	require.NotContains(t, html, `:style="collapsedMenuStyleFor($el)"`)
}
