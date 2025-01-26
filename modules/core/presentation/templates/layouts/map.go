package layouts

import (
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

func MapNavItemsToSidebar(navItems []types.NavigationItem) []sidebar.Item {
	items := make([]sidebar.Item, 0, len(navItems))
	for _, navItem := range navItems {
		if len(navItem.Children) > 0 {
			items = append(items, sidebar.NewGroup(
				navItem.Name,
				navItem.Icon,
				MapNavItemsToSidebar(navItem.Children),
			))
		} else {
			items = append(items, sidebar.NewLink(
				navItem.Href,
				navItem.Name,
				navItem.Icon,
			))
		}
	}
	return items
}
