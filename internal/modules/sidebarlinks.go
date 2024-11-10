package modules

import (
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
	"github.com/iota-agency/iota-erp/internal/types"
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

func GetNavItems(localizer *i18n.Localizer, user *user.User) []types.NavigationItem {
	items := []types.NavigationItem{
		{
			Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Dashboard"}),
			Href:        "/",
			Icon:        icons.CirclesThreePlus(icons.Props{Size: "20"}),
			Children:    []types.NavigationItem{},
			Permissions: nil,
		},
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Operations"}),
			Href: "#",
			Children: []types.NavigationItem{
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Employees"}),
					Href:        "/operations/employees",
					Permissions: []permission.Permission{permission.EmployeeRead},
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Settings"}),
					Href:        "/settings",
					Permissions: []permission.Permission{permission.SettingsRead},
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Projects"}),
					Href:        "/projects",
					Permissions: []permission.Permission{permission.ProjectRead},
				},
			},
			Icon:        icons.Pulse(icons.Props{Size: "20"}),
			Permissions: nil,
		},
		{
			Name: localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Enums"}),
			Href: "#",
			Icon: icons.CheckCircle(icons.Props{Size: "20"}),
			Children: []types.NavigationItem{
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.TaskTypes"}),
					Href:        "/enums/task-types",
					Permissions: nil,
				},
				{
					Name:        localizer.MustLocalize(&i18n.LocalizeConfig{MessageID: "NavigationLinks.Positions"}),
					Href:        "/enums/positions",
					Permissions: nil,
				},
			},
		},
	}
	for _, m := range LoadedModules {
		items = append(items, m.NavigationItems(localizer)...)
	}
	return filterItems(items, user)
}
