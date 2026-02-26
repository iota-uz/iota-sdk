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

func AppendIfNotEmpty(groups *[]sidebar.TabGroup, group sidebar.TabGroup) {
	if len(group.Items) == 0 {
		return
	}
	*groups = append(*groups, group)
}

func NormalizeDefaultTab(groups []sidebar.TabGroup, preferred string) string {
	if len(groups) == 0 {
		return ""
	}
	for _, group := range groups {
		if group.Value == preferred {
			return preferred
		}
	}
	return groups[0].Value
}

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

	groups := make([]sidebar.TabGroup, 0, 2)
	AppendIfNotEmpty(&groups, sidebar.TabGroup{
		Label: "Core",
		Value: "core",
		Items: sidebarItems,
	})
	AppendIfNotEmpty(&groups, sidebar.TabGroup{
		Label: "CRM",
		Value: "crm",
		Items: crmItems,
	})

	return sidebar.TabGroupCollection{
		Groups:       groups,
		DefaultValue: NormalizeDefaultTab(groups, "core"),
	}
}
