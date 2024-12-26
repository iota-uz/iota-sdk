package core

import (
	"github.com/iota-agency/iota-sdk/pkg/presentation/templates/icons"
	"github.com/iota-agency/iota-sdk/pkg/types"
)

var DashboardLink = types.NavigationItem{
	Name:     "NavigationLinks.Dashboard",
	Icon:     icons.Gauge(icons.Props{Size: "20"}),
	Href:     "/",
	Children: nil,
}

var NavItems = []types.NavigationItem{DashboardLink}
