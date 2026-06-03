// Package core provides this package.
package core

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var DashboardLink = types.NavigationItem{
	Key:      "core.dashboard",
	Name:     "NavigationLinks.Dashboard",
	Icon:     icons.Gauge(icons.Props{Size: "20"}),
	Href:     "/",
	Children: nil,
}

var UsersLink = types.NavigationItem{
	Key:         "core.users",
	Name:        "NavigationLinks.Users",
	Icon:        nil,
	Href:        "/users",
	Permissions: []permission.Permission{permissions.UserRead},
	Children:    nil,
}

var RolesLink = types.NavigationItem{
	Key:         "core.roles",
	Name:        "NavigationLinks.Roles",
	Icon:        nil,
	Href:        "/roles",
	Permissions: []permission.Permission{permissions.RoleRead},
	Children:    nil,
}

var GroupsLink = types.NavigationItem{
	Key:         "core.groups",
	Name:        "NavigationLinks.Groups",
	Icon:        nil,
	Href:        "/groups",
	Permissions: []permission.Permission{permissions.GroupRead},
	Children:    nil,
}

var DepartmentsLink = types.NavigationItem{
	Key:         "core.departments",
	Name:        "NavigationLinks.Departments",
	Icon:        nil,
	Href:        "/departments",
	Permissions: []permission.Permission{permissions.DepartmentRead},
	Children:    nil,
}

var PositionsLink = types.NavigationItem{
	Key:         "core.positions",
	Name:        "NavigationLinks.Positions",
	Icon:        nil,
	Href:        "/positions",
	Permissions: []permission.Permission{permissions.PositionRead},
	Children:    nil,
}

var SettingsLink = types.NavigationItem{
	Key:      "core.settings",
	Name:     "NavigationLinks.Settings",
	Icon:     nil,
	Href:     "/settings/logo",
	Children: nil,
}

var AdministrationLink = types.NavigationItem{
	Key:  "core.administration",
	Name: "NavigationLinks.Administration",
	Icon: icons.AirTrafficControl(icons.Props{Size: "20"}),
	Href: "#",
	Children: []types.NavigationItem{
		UsersLink,
		RolesLink,
		GroupsLink,
		DepartmentsLink,
		PositionsLink,
		SettingsLink,
	},
}

// BuildNavItems returns core nav items with provided permission overrides.
func BuildNavItems(
	dashboardLinkPermissions []permission.Permission,
	settingsLinkPermissions []permission.Permission,
) []types.NavigationItem {
	dashboardLink := DashboardLink
	dashboardLink.Permissions = dashboardLinkPermissions

	settingsLink := SettingsLink
	settingsLink.Permissions = settingsLinkPermissions

	administrationLink := AdministrationLink
	administrationLink.Children = make([]types.NavigationItem, len(AdministrationLink.Children))
	copy(administrationLink.Children, AdministrationLink.Children)
	for i := range administrationLink.Children {
		if administrationLink.Children[i].Name == SettingsLink.Name {
			administrationLink.Children[i] = settingsLink
			break
		}
	}

	return []types.NavigationItem{
		dashboardLink,
		administrationLink,
	}
}
