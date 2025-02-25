package hrm

import (
	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/pkg/types"
)

var EmployeesLink = types.NavigationItem{
	Name:     "NavigationLinks.Employees",
	Icon:     nil,
	Href:     "/hrm/employees",
	Children: nil,
}

var HRMLink = types.NavigationItem{
	Name: "NavigationLinks.HRM",
	Icon: icons.UsersThree(icons.Props{Size: "20"}),
	Href: "/hrm",
	Children: []types.NavigationItem{
		EmployeesLink,
	},
}

var NavItems = []types.NavigationItem{
	HRMLink,
}
