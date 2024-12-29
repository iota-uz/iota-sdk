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

var BiChatLink = types.NavigationItem{
	Name:     "NavigationLinks.BiChat",
	Icon:     icons.ChatCircle(icons.Props{Size: "20"}),
	Href:     "/bi-chat",
	Children: nil,
}

var NavItems = []types.NavigationItem{DashboardLink, UsersLink, EmployeesLink, BiChatLink}
