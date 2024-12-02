package modules

import (
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/domain/aggregates/user"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/tab"
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func filterItems(items []types.NavigationItem, user *user.User) []types.NavigationItem {
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
	for _, tab := range tabs {
		if tab.Href == href {
			return true
		}
	}
	return false
}

func getEnabledNavItems(items []types.NavigationItem, tabs []*tab.Tab) []types.NavigationItem {
	out := []types.NavigationItem{}
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

func GetNavItems(
	app application.Application,
	localizer *i18n.Localizer,
	user *user.User,
	tabs []*tab.Tab,
) ([]types.NavigationItem, []types.NavigationItem) {
	items := []types.NavigationItem{
		{
			Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Dashboard"}),
			Href:        "/",
			Icon:        icons.CirclesThreePlus(icons.Props{Size: "20"}),
			Children:    []types.NavigationItem{},
			Permissions: nil,
		},
	}
	for _, n := range app.NavigationItems(localizer) {
		items = append(items, n)
	}
	filtered := filterItems(items, user)
	return getEnabledNavItems(filtered, tabs), filtered
}
