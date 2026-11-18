package core

import (
	"github.com/a-h/templ"
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/settings"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

// Settings page keys contributed by the core module. Pass them to
// composition.RemoveSettingsPagesByKey to hide or relocate an entry.
const (
	SettingsPageGeneral       = "core.general"
	SettingsPageNotifications = "core.notifications"
	SettingsPageUsers         = "core.users"
	SettingsPageRoles         = "core.roles"
	SettingsPageGroups        = "core.groups"
	SettingsPageDepartments   = "core.departments"
	SettingsPagePositions     = "core.positions"
)

func settingsIcon(icon func(icons.Props) templ.Component) templ.Component {
	return icon(icons.Props{Size: "20"})
}

func coreSettingsSections(userBasePath string, generalPermissions []permission.Permission) []types.SettingsSection {
	return []types.SettingsSection{
		settings.GeneralSection(
			types.SettingsPage{
				Key:         SettingsPageGeneral,
				Name:        "Settings.Hub.Pages.General.Name",
				Description: "Settings.Hub.Pages.General.Summary",
				Href:        "/settings/logo",
				Icon:        settingsIcon(icons.Buildings),
				Order:       10,
				Permissions: generalPermissions,
			},
			types.SettingsPage{
				Key:         SettingsPageNotifications,
				Name:        "NotificationSettings.Title",
				Description: "Settings.Hub.Pages.Notifications.Summary",
				Href:        "/settings/notifications",
				Icon:        settingsIcon(icons.Bell),
				Order:       20,
				Permissions: []permission.Permission{permissions.NotificationRulesRead},
			},
		),
		settings.AccessSection(
			types.SettingsPage{
				Key:         SettingsPageUsers,
				Name:        "NavigationLinks.Users",
				Description: "Settings.Hub.Pages.Users.Summary",
				Href:        userBasePath,
				Icon:        settingsIcon(icons.Users),
				Order:       10,
				Permissions: []permission.Permission{permissions.UserRead},
			},
			types.SettingsPage{
				Key:         SettingsPageRoles,
				Name:        "NavigationLinks.Roles",
				Description: "Settings.Hub.Pages.Roles.Summary",
				Href:        "/roles",
				Icon:        settingsIcon(icons.ShieldCheck),
				Order:       20,
				Permissions: []permission.Permission{permissions.RoleRead},
			},
			types.SettingsPage{
				Key:         SettingsPageGroups,
				Name:        "NavigationLinks.Groups",
				Description: "Settings.Hub.Pages.Groups.Summary",
				Href:        "/groups",
				Icon:        settingsIcon(icons.UsersThree),
				Order:       30,
				Permissions: []permission.Permission{permissions.GroupRead},
			},
		),
		settings.OrganizationSection(
			types.SettingsPage{
				Key:         SettingsPageDepartments,
				Name:        "NavigationLinks.Departments",
				Description: "Settings.Hub.Pages.Departments.Summary",
				Href:        "/departments",
				Icon:        settingsIcon(icons.TreeStructure),
				Order:       10,
				Permissions: []permission.Permission{permissions.DepartmentRead},
			},
			types.SettingsPage{
				Key:         SettingsPagePositions,
				Name:        "NavigationLinks.Positions",
				Description: "Settings.Hub.Pages.Positions.Summary",
				Href:        "/positions",
				Icon:        settingsIcon(icons.Briefcase),
				Order:       20,
				Permissions: []permission.Permission{permissions.PositionRead},
			},
		),
	}
}

func coreUserBasePath(opts []controllers.UserControllerOption) string {
	all := append([]controllers.UserControllerOption{controllers.WithUserControllerBasePath("/users")}, opts...)
	return controllers.UserControllerBasePath(all...)
}
