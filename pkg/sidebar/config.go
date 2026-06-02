// Package sidebar provides this package.
package sidebar

import (
	"sort"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// TabGroupBuilder is a function that takes navigation items and returns tab groups
type TabGroupBuilder func(items []types.NavigationItem, localizer *i18n.Localizer) sidebar.TabGroupCollection

// BuildTabGroups is the global variable that users can override to customize tab grouping.
// Deprecated: use composition.AddNavWorkspaces and NavigationItem.Workspace instead.
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

func BuildTabGroupsWithWorkspaces(
	items []types.NavigationItem,
	workspaces []types.NavWorkspace,
	localizer *i18n.Localizer,
) sidebar.TabGroupCollection {
	if len(workspaces) == 0 {
		return BuildTabGroups(items, localizer)
	}
	return WorkspaceTabGroupBuilder(items, workspaces, localizer)
}

func WorkspaceTabGroupBuilder(
	items []types.NavigationItem,
	workspaces []types.NavWorkspace,
	localizer *i18n.Localizer,
) sidebar.TabGroupCollection {
	if len(workspaces) == 0 {
		return BuildTabGroups(items, localizer)
	}

	orderedWorkspaces := append([]types.NavWorkspace(nil), workspaces...)
	sort.SliceStable(orderedWorkspaces, func(i, j int) bool {
		if orderedWorkspaces[i].Order != orderedWorkspaces[j].Order {
			return orderedWorkspaces[i].Order < orderedWorkspaces[j].Order
		}
		// Deterministic secondary tie-break: equal-Order workspaces sort by Key
		// ascending so ordering is self-evident rather than relying on insertion.
		return orderedWorkspaces[i].Key < orderedWorkspaces[j].Key
	})

	defaultWorkspace := orderedWorkspaces[0]
	for _, workspace := range orderedWorkspaces {
		if workspace.Default {
			defaultWorkspace = workspace
			break
		}
	}

	workspaceByKey := make(map[string]types.NavWorkspace, len(orderedWorkspaces))
	groupItems := make(map[string][]sidebar.Item, len(orderedWorkspaces))
	for _, workspace := range orderedWorkspaces {
		workspaceByKey[workspace.Key] = workspace
	}

	for _, item := range items {
		workspaceKey := item.Workspace
		if _, ok := workspaceByKey[workspaceKey]; workspaceKey == "" || !ok {
			workspaceKey = defaultWorkspace.Key
		}
		if item.Workspace != "" && len(item.Children) > 0 {
			groupItems[workspaceKey] = append(groupItems[workspaceKey], layouts.MapNavItemsToSidebar(item.Children)...)
			continue
		}
		groupItems[workspaceKey] = append(groupItems[workspaceKey], layouts.MapNavItemToSidebar(item))
	}

	groups := make([]sidebar.TabGroup, 0, len(orderedWorkspaces))
	for _, workspace := range orderedWorkspaces {
		AppendIfNotEmpty(&groups, sidebar.TabGroup{
			Label:  localizeWorkspaceLabel(localizer, workspace),
			Value:  workspace.Key,
			Items:  groupItems[workspace.Key],
			IsBeta: workspace.IsBeta,
		})
	}

	return sidebar.TabGroupCollection{
		Groups:       groups,
		DefaultValue: NormalizeDefaultTab(groups, defaultWorkspace.Key),
	}
}

func localizeWorkspaceLabel(localizer *i18n.Localizer, workspace types.NavWorkspace) string {
	if localizer == nil || workspace.Label == "" {
		return workspace.Label
	}
	// Mirror nav-item name localization (pkg/application.translate): fail loudly
	// on a missing/typo'd NavWorkspaces.* key instead of silently shipping the
	// raw key as a visible sidebar tab label.
	return intl.MustLocalize(localizer, &i18n.LocalizeConfig{
		MessageID: workspace.Label,
	})
}

// DefaultTabGroupBuilder maintains current behavior.
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
