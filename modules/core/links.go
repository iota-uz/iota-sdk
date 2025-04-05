package core

import (
	icons "github.com/iota-uz/icons/phosphor"
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

var GroupsLink = types.NavigationItem{
	Name:     "NavigationLinks.Groups",
	Icon:     nil,
	Href:     "/groups",
	Children: nil,
}

var AdministrationLink = types.NavigationItem{
	Name: "NavigationLinks.Administration",
	Icon: icons.AirTrafficControl(icons.Props{Size: "20"}),
	Href: "#",
	Children: []types.NavigationItem{
		UsersLink,
		RolesLink,
		GroupsLink,
	},
}

var NavItems = []types.NavigationItem{
	DashboardLink,
	AdministrationLink,
}
