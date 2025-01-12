package core

import (
	"github.com/iota-uz/iota-sdk/components/icons"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var DashboardLink = types.NavigationItem{
	Name:     "NavigationLinks.Dashboard",
	Icon:     icons.Gauge(icons.Props{Size: "20"}),
	Href:     "/",
	Children: nil,
}

var UsersLink = types.NavigationItem{
	Name:     "NavigationLinks.Users",
	Icon:     nil,
	Href:     "/users",
	Children: nil,
}

var RolesLink = types.NavigationItem{
	Name:     "NavigationLinks.Roles",
	Icon:     nil,
	Href:     "/roles",
	Children: nil,
}

var EmployeesLink = types.NavigationItem{
	Name:     "NavigationLinks.Employees",
	Icon:     nil,
	Href:     "/operations/employees",
	Children: nil,
}

var AdministrationLink = types.NavigationItem{
	Name: "NavigationLinks.Administration",
	Icon: icons.AirTrafficControl(icons.Props{Size: "20"}),
	Href: "#",
	Children: []types.NavigationItem{
		UsersLink,
		EmployeesLink,
		RolesLink,
	},
}

var BiChatLink = types.NavigationItem{
	Name:     "NavigationLinks.BiChat",
	Icon:     icons.ChatCircle(icons.Props{Size: "20"}),
	Href:     "/bi-chat",
	Children: nil,
}

var NavItems = []types.NavigationItem{
	DashboardLink,
	BiChatLink,
	AdministrationLink,
}
