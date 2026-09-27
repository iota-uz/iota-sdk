package application

import (
	"context"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/types"
	"github.com/stretchr/testify/require"
)

// Regression for the Portfolio-sidebar bug: NavItemsForScope runs translate(),
// which rebuilt each NavigationItem field-by-field and dropped Logic, reverting
// a RequireAny (OR) gate to the zero value PermissionLogicAll (AND). Downstream
// filterItems then hid the item from any user holding only one of its
// permissions. NavItemsForScope must preserve Logic on items and children.
func TestNavItemsForScope_PreservesAnyPermissionLogic(t *testing.T) {
	app, err := New(&ApplicationOptions{
		Bundle:             LoadBundle(),
		SupportedLanguages: []string{"en"},
	})
	require.NoError(t, err)

	permA := permission.New(permission.WithName("Portfolio.Read"))
	permB := permission.New(permission.WithName("Reinsurance.Contract.Read"))

	attachRuntimeSource(t, app, &testRuntimeSource{
		navItems: []types.NavigationItem{{
			Name:        "Spotlight.Badge.Action",
			Href:        "/portfolio/policies",
			Permissions: []permission.Permission{permA, permB},
			Logic:       types.PermissionLogicAny,
			Children: []types.NavigationItem{{
				Name:        "Spotlight.Badge.Action",
				Href:        "/portfolio/archive",
				Permissions: []permission.Permission{permA, permB},
				Logic:       types.PermissionLogicAny,
			}},
		}},
	})

	// NavItemsForScope is reached via interface assertion, exactly as the
	// sidebar middleware does it.
	type navScoper interface {
		NavItemsForScope(context.Context, *i18n.Localizer, NavScope) []types.NavigationItem
	}
	scoper, ok := app.(navScoper)
	require.True(t, ok, "application must expose NavItemsForScope")

	localizer := i18n.NewLocalizer(LoadBundle(), "en")
	items := scoper.NavItemsForScope(context.Background(), localizer, NavScope{})

	require.Len(t, items, 1)
	require.Equal(t, types.PermissionLogicAny, items[0].Logic, "translate() must preserve Logic on the parent")
	require.Len(t, items[0].Children, 1)
	require.Equal(t, types.PermissionLogicAny, items[0].Children[0].Logic, "translate() must preserve Logic on children")
}
