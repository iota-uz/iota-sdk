// Package sidebar provides this package.
package sidebar

import (
	"sort"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/components/sidebar"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
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

	resolveKey := func(workspace string) string {
		if _, ok := workspaceByKey[workspace]; workspace == "" || !ok {
			return defaultWorkspace.Key
		}
		return workspace
	}

	for _, item := range items {
		workspaceKey := resolveKey(item.Workspace)
		if item.Workspace != "" && len(item.Children) > 0 {
			// Tagged container: flatten its children into workspaces. A child
			// that declares its own (valid) Workspace is routed there; otherwise
			// it inherits the container's workspace.
			for _, child := range item.Children {
				childKey := workspaceKey
				if child.Workspace != "" {
					childKey = resolveKey(child.Workspace)
				}
				groupItems[childKey] = append(groupItems[childKey], layouts.MapNavItemToSidebar(child))
			}
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
	// Workspace labels are i18n keys declared by the consuming app, not the SDK.
	// Degrade gracefully (fall back to the raw key) rather than panicking on a
	// missing/typo'd NavWorkspaces.* key — a missing label must not take down the
	// whole authenticated sidebar. Consumers catch missing keys via their own
	// i18n key-consistency checks (e.g. `just check tr`).
	label, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: workspace.Label,
	})
	if err != nil {
		return workspace.Label
	}
	return label
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
