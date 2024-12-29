package core

import (
	"github.com/iota-uz/iota-sdk/pkg/presentation/templates/icons"
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

var NavItems = []types.NavigationItem{DashboardLink, UsersLink}
