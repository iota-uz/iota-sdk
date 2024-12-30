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
	Icon:     icons.Users(icons.Props{Size: "20"}),
	Href:     "/users",
	Children: nil,
}

var EmployeesLink = types.NavigationItem{
	Name:     "NavigationLinks.Employees",
	Icon:     icons.Users(icons.Props{Size: "20"}),
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
	},
}

var NavItems = []types.NavigationItem{DashboardLink, AdministrationLink}
