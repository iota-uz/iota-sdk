package layouts

import (
	"github.com/iota-agency/iota-erp/internal/presentation/templates/icons"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

func getSidebarItems(pageCtx *composables.PageContext) []SidebarItem {
	return []SidebarItem{
		newSidebarItem(pageCtx.T("NavigationLinks.Dashboard"), "/", icons.CirclesThreePlus(icons.Props{Size: "20"}), []SidebarItem{}),
		newSidebarItem(
			pageCtx.T("NavigationLinks.Users"),
			"/users",
			icons.Users(icons.Props{Size: "20"}), []SidebarItem{},
		),
		newSidebarItem(
			pageCtx.T("NavigationLinks.Operations"), "#",
			icons.Pulse(icons.Props{Size: "20"}),
			[]SidebarItem{
				{name: pageCtx.T("NavigationLinks.Employees"), href: "/operations/employees"},
				{name: pageCtx.T("NavigationLinks.Settings"), href: "/settings"},
				{name: pageCtx.T("NavigationLinks.Projects"), href: "/projects"},
			},
		),
		newSidebarItem(
			pageCtx.T("NavigationLinks.Enums"), "#",
			icons.CheckCircle(icons.Props{Size: "20"}),
			[]SidebarItem{
				{name: pageCtx.T("NavigationLinks.TaskTypes"), href: "/enums/task-types"},
				{name: pageCtx.T("NavigationLinks.Positions"), href: "/enums/positions"},
			},
		),
		newSidebarItem(
			pageCtx.T("NavigationLinks.Finances"), "#",
			icons.Money(icons.Props{Size: "20"}),
			[]SidebarItem{
				{name: pageCtx.T("NavigationLinks.ExpenseCategories"), href: "/finance/expense-categories"},
				{name: pageCtx.T("NavigationLinks.Payments"), href: "/finance/payments"},
				{name: pageCtx.T("NavigationLinks.Expenses"), href: "/finance/expenses"},
				{name: pageCtx.T("NavigationLinks.Accounts"), href: "/finance/accounts"},
			},
		),
		newSidebarItem(
			pageCtx.T("NavigationLinks.Reports"), "#",
			icons.FileText(icons.Props{Size: "20"}),
			[]SidebarItem{
				{name: pageCtx.T("NavigationLinks.Finances"), href: "/reports/cash-flow"},
			},
		),
	}
}
