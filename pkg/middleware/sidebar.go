// Package middleware provides this package.
package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	pkgsidebar "github.com/iota-uz/iota-sdk/pkg/sidebar"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// SidebarPropsDecorator allows host applications to adjust sidebar props after
// the SDK has built the default navigation-driven structure.
type SidebarPropsDecorator func(ctx context.Context, r *http.Request, props sidebar.Props) sidebar.Props

var sidebarPropsDecorator SidebarPropsDecorator = func(_ context.Context, _ *http.Request, props sidebar.Props) sidebar.Props {
	return props
}

// NavItemsDecorator allows host applications to adjust request-scoped
// navigation items before permission filtering, pin splitting, workspace
// grouping, sidebar mapping, and context storage.
type NavItemsDecorator func(ctx context.Context, r *http.Request, items []types.NavigationItem) []types.NavigationItem

var navItemsDecorator NavItemsDecorator = func(_ context.Context, _ *http.Request, items []types.NavigationItem) []types.NavigationItem {
	return items
}

// SetSidebarPropsDecorator overrides the default no-op sidebar props decorator.
func SetSidebarPropsDecorator(decorator SidebarPropsDecorator) {
	if decorator == nil {
		sidebarPropsDecorator = func(_ context.Context, _ *http.Request, props sidebar.Props) sidebar.Props {
			return props
		}
		return
	}
	sidebarPropsDecorator = decorator
}

// SetNavItemsDecorator overrides the default no-op nav item decorator.
func SetNavItemsDecorator(decorator NavItemsDecorator) {
	if decorator == nil {
		navItemsDecorator = func(_ context.Context, _ *http.Request, items []types.NavigationItem) []types.NavigationItem {
			return items
		}
		return
	}
	navItemsDecorator = decorator
}

func filterItems(items []types.NavigationItem, user user.User) []types.NavigationItem {
	filteredItems := make([]types.NavigationItem, 0, len(items))
	for _, item := range items {
		if item.HasPermission(user) {
			filteredChildren := filterItems(item.Children, user)
			// If item originally had children but all were filtered out, skip it
			if len(item.Children) > 0 && len(filteredChildren) == 0 {
				continue
			}
			filteredItems = append(filteredItems, types.NavigationItem{
				Key:         item.Key,
				Name:        item.Name,
				Workspace:   item.Workspace,
				Pinned:      item.Pinned,
				Href:        item.Href,
				Children:    filteredChildren,
				Keywords:    append([]string(nil), item.Keywords...),
				Icon:        item.Icon,
				Permissions: item.Permissions,
				IsBeta:      item.IsBeta,
			})
		}
	}
	return filteredItems
}

func getEnabledNavItems(items []types.NavigationItem) []types.NavigationItem {
	var out []types.NavigationItem
	for _, item := range items {
		if len(item.Children) > 0 {
			childrenWithInheritedWorkspace := make([]types.NavigationItem, len(item.Children))
			copy(childrenWithInheritedWorkspace, item.Children)
			for i := range childrenWithInheritedWorkspace {
				if childrenWithInheritedWorkspace[i].Workspace == "" {
					childrenWithInheritedWorkspace[i].Workspace = item.Workspace
				}
			}
			children := getEnabledNavItems(childrenWithInheritedWorkspace)
			childrenLen := len(children)
			if childrenLen == 0 {
				continue
			}
			if childrenLen == 1 {
				out = append(out, children[0])
			} else {
				item.Children = children
				out = append(out, item)
			}
		} else {
			out = append(out, item)
		}
	}

	return out
}

func splitPinnedItems(items []types.NavigationItem) ([]types.NavigationItem, []types.NavigationItem) {
	pinned := make([]types.NavigationItem, 0)
	unpinned := make([]types.NavigationItem, 0, len(items))
	for _, item := range items {
		if item.Pinned {
			pinned = append(pinned, item)
			continue
		}
		hadChildren := len(item.Children) > 0
		childPinned, childUnpinned := splitPinnedItems(item.Children)
		pinned = append(pinned, childPinned...)
		item.Children = childUnpinned
		if hadChildren && len(childUnpinned) == 0 {
			continue
		}
		unpinned = append(unpinned, item)
	}
	return pinned, unpinned
}

func normalizeTabGroups(collection sidebar.TabGroupCollection) sidebar.TabGroupCollection {
	groups := make([]sidebar.TabGroup, 0, len(collection.Groups))
	for _, group := range collection.Groups {
		pkgsidebar.AppendIfNotEmpty(&groups, group)
	}
	return sidebar.TabGroupCollection{
		Groups:       groups,
		DefaultValue: pkgsidebar.NormalizeDefaultTab(groups, collection.DefaultValue),
	}
}

// NavItemsWithInitialState provides navigation items and sidebar props.
// initialState controls the server-side sidebar default (collapsed/expanded/auto).
func NavItemsWithInitialState(initialState sidebar.SidebarState) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				app, err := application.UseApp(r.Context())
				if err != nil {
					panic(err.Error())
				}
				localizer, ok := intl.UseLocalizer(r.Context())
				if !ok {
					panic("localizer not found in context")
				}
				u, err := composables.UseUser(r.Context())
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				items := navItemsDecorator(r.Context(), r, app.NavItems(localizer))
				filtered := filterItems(items, u)
				if len(u.Roles()) == 0 {
					filtered = []types.NavigationItem{}
				}
				pinnedItems, unpinnedItems := splitPinnedItems(filtered)
				enabledPinnedItems := getEnabledNavItems(pinnedItems)
				enabledNavItems := getEnabledNavItems(unpinnedItems)

				// Build sidebar props with configurable tab groups
				tabGroups := normalizeTabGroups(pkgsidebar.BuildTabGroupsWithWorkspaces(
					enabledNavItems,
					app.NavWorkspaces(),
					localizer,
				))

				sidebarProps := sidebar.Props{
					Header:       layouts.DefaultSidebarHeader(),
					TabGroups:    tabGroups,
					PinnedItems:  layouts.MapNavItemsToSidebar(enabledPinnedItems),
					Footer:       layouts.DefaultSidebarFooter(),
					InitialState: initialState,
				}
				sidebarProps = sidebarPropsDecorator(r.Context(), r, sidebarProps)

				ctx := context.WithValue(r.Context(), constants.AllNavItemsKey, filtered)
				// Fresh allocation to avoid mutating enabledPinnedItems' backing
				// array (used above for PinnedItems sidebar props).
				allNavItems := append(append([]types.NavigationItem(nil), enabledPinnedItems...), enabledNavItems...)
				ctx = context.WithValue(ctx, constants.NavItemsKey, allNavItems)
				ctx = context.WithValue(ctx, constants.SidebarPropsKey, sidebarProps)
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}

// NavItems provides navigation items and sidebar props with the default behavior:
// respect localStorage unless overridden elsewhere.
func NavItems() mux.MiddlewareFunc {
	return NavItemsWithInitialState(sidebar.SidebarAuto)
}
