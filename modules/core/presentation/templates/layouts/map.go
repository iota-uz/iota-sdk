package layouts

import (
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

func MapNavItemToSidebar(navItem types.NavigationItem) sidebar.Item {
	if len(navItem.Children) > 0 {
		return sidebar.NewGroup(
			navItem.Name,
			navItem.Icon,
			MapNavItemsToSidebar(navItem.Children),
		)
	}
	return sidebar.NewLink(
		navItem.Href,
		navItem.Name,
		navItem.Icon,
	)
}

func MapNavItemsToSidebar(navItems []types.NavigationItem) []sidebar.Item {
	sidebarItems := make([]sidebar.Item, 0, len(navItems))
	for _, item := range navItems {
		sidebarItems = append(sidebarItems, MapNavItemToSidebar(item))
	}
	return sidebarItems
}
