package middleware

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/components/base/navtabs"
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/constants"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

func filterItems(items []types.NavigationItem, user user.User) []types.NavigationItem {
	filteredItems := make([]types.NavigationItem, 0, len(items))
	for _, item := range items {
		if item.HasPermission(user) {
			filteredItems = append(filteredItems, types.NavigationItem{
				Name:        item.Name,
				Href:        item.Href,
				Children:    filterItems(item.Children, user),
				Icon:        item.Icon,
				Permissions: item.Permissions,
			})
		}
	}
	return filteredItems
}

func hrefExists(href string, tabs []*tab.Tab) bool {
	for _, t := range tabs {
		if t.Href == href {
			return true
		}
	}
	return false
}

func getEnabledNavItems(items []types.NavigationItem, tabs []*tab.Tab) []types.NavigationItem {
	var out []types.NavigationItem
	for _, item := range items {
		if len(item.Children) > 0 {
			children := getEnabledNavItems(item.Children, tabs)
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
		} else if item.Href == "" || item.Href != "" && hrefExists(item.Href, tabs) {
			out = append(out, item)
		}
	}

	return out
}

func NavItems() mux.MiddlewareFunc {
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
				tabs, err := composables.UseTabs(r.Context())
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				filtered := filterItems(app.NavItems(localizer), u)
				enabledNavItems := getEnabledNavItems(filtered, tabs)

				// Build sidebar props with tab groups
				items := layouts.MapNavItemsToSidebar(enabledNavItems)
				tabGroups := sidebar.TabGroupCollection{
					Groups: []sidebar.TabGroup{
						{
							Tab: navtabs.Tab{
								Label: "Core",
								Value: "core",
							},
							Items: items,
						},
					},
					DefaultValue: "core",
				}

				sidebarProps := sidebar.Props{
					Header:    layouts.DefaultSidebarHeader(),
					TabGroups: tabGroups,
					Footer:    layouts.DefaultSidebarFooter(),
				}

				ctx := context.WithValue(r.Context(), constants.AllNavItemsKey, filtered)
				ctx = context.WithValue(ctx, constants.NavItemsKey, enabledNavItems)
				ctx = context.WithValue(ctx, constants.SidebarPropsKey, sidebarProps)
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}

func Tabs() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				u, err := composables.UseUser(r.Context())
				if err != nil {
					next.ServeHTTP(w, r)
					return
				}
				app, err := application.UseApp(r.Context())
				if err != nil {
					panic(err)
				}
				tabService := app.Service(services.TabService{}).(*services.TabService)
				tabs, err := tabService.GetUserTabs(r.Context(), u.ID())
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				ctx := context.WithValue(r.Context(), constants.TabsKey, tabs)
				next.ServeHTTP(w, r.WithContext(ctx))
			},
		)
	}
}
