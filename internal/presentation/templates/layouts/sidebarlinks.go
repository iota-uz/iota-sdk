package layouts

import (
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/user"
	"github.com/iota-agency/iota-erp/internal/domain/entities/permission"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func filterItems(items []SidebarItem, user *user.User) []SidebarItem {
	filteredItems := make([]SidebarItem, 0, len(items))
	for _, item := range items {
		if item.HasPermission(user) {
			filteredItems = append(filteredItems, SidebarItem{
				name:        item.name,
				href:        item.href,
				children:    filterItems(item.children, user),
				icon:        item.icon,
				permissions: item.permissions,
			})
		}
	}
	return filteredItems
}

func getSidebarItems(pageCtx *composables.PageContext) []SidebarItem {
	items := []SidebarItem{
		{
			name:        pageCtx.T("NavigationLinks.Dashboard"),
			href:        "/",
			icon:        icons.CirclesThreePlus(icons.Props{Size: "20"}),
			children:    []SidebarItem{},
			permissions: nil,
		},
		{
			name:        pageCtx.T("NavigationLinks.Users"),
			href:        "/users",
			icon:        icons.Users(icons.Props{Size: "20"}),
			children:    []SidebarItem{},
			permissions: []permission.Permission{permission.UserRead},
		},
		{
			name: pageCtx.T("NavigationLinks.Operations"),
			href: "#",
			children: []SidebarItem{
				{
					name:        pageCtx.T("NavigationLinks.Employees"),
					href:        "/operations/employees",
					permissions: []permission.Permission{permission.EmployeeRead},
				},
				{
					name:        pageCtx.T("NavigationLinks.Settings"),
					href:        "/settings",
					permissions: []permission.Permission{permission.SettingsRead},
				},
				{
					name:        pageCtx.T("NavigationLinks.Projects"),
					href:        "/projects",
					permissions: []permission.Permission{permission.ProjectRead},
				},
			},
			icon:        icons.Pulse(icons.Props{Size: "20"}),
			permissions: nil,
		},
		{
			name: pageCtx.T("NavigationLinks.Enums"),
			href: "#",
			icon: icons.CheckCircle(icons.Props{Size: "20"}),
			children: []SidebarItem{
				{
					name:        pageCtx.T("NavigationLinks.TaskTypes"),
					href:        "/enums/task-types",
					permissions: nil,
				},
				{
					name:        pageCtx.T("NavigationLinks.Positions"),
					href:        "/enums/positions",
					permissions: nil,
				},
			},
		},
		{
			name: pageCtx.T("NavigationLinks.Finances"),
			href: "#",
			icon: icons.Money(icons.Props{Size: "20"}),
			children: []SidebarItem{
				{
					name:        pageCtx.T("NavigationLinks.ExpenseCategories"),
					href:        "/finance/expense-categories",
					permissions: nil,
				},
				{
					name:        pageCtx.T("NavigationLinks.Payments"),
					href:        "/finance/payments",
					permissions: nil,
				},
				{
					name:        pageCtx.T("NavigationLinks.Expenses"),
					href:        "/finance/expenses",
					permissions: nil,
				},
				{
					name:        pageCtx.T("NavigationLinks.Accounts"),
					href:        "/finance/moneyaccounts",
					permissions: nil,
				},
			},
		},
	}
	return filterItems(items, pageCtx.User)
}
