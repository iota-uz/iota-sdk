package sidebar

import (
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// TabGroupBuilder is a function that takes navigation items and returns tab groups
type TabGroupBuilder func(items []types.NavigationItem, localizer *i18n.Localizer) sidebar.TabGroupCollection

// BuildTabGroups is the global variable that users can override to customize tab grouping
var BuildTabGroups TabGroupBuilder = DefaultTabGroupBuilder

// DefaultTabGroupBuilder maintains current behavior (single "Core" tab)
func DefaultTabGroupBuilder(items []types.NavigationItem, localizer *i18n.Localizer) sidebar.TabGroupCollection {
	sidebarItems := []sidebar.Item{}
	crmItems := []sidebar.Item{}
	for _, item := range items {
		if item.Href == "/crm" {
			crmItems = append(crmItems, layouts.MapNavItemsToSidebar(item.Children)...)
		} else {
			sidebarItems = append(sidebarItems, layouts.MapNavItemToSidebar(item))
		}
	}
	return sidebar.TabGroupCollection{
		Groups: []sidebar.TabGroup{
			{
				Label: "Core",
				Value: "core",
				Items: sidebarItems,
			},
			{
				Label: "CRM",
				Value: "crm",
				Items: crmItems,
			},
		},
		DefaultValue: "core",
	}
}
