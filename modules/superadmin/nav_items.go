package superadmin

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

var DashboardLink = types.NavigationItem{
	Name: "SuperAdmin.NavigationLinks.Dashboard",
	Icon: icons.ChartLineUp(icons.Props{Size: "20"}),
	Href: "/",
}

var TenantsLink = types.NavigationItem{
	Name: "SuperAdmin.NavigationLinks.Tenants",
	Icon: icons.Buildings(icons.Props{Size: "20"}),
	Href: "/superadmin/tenants",
}

var NavItems = []types.NavigationItem{
	DashboardLink,
	TenantsLink,
}
