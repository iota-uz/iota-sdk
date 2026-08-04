// Package hrm provides this package.
package hrm

import (
	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

var HRMNavNode = application.NavNode{
	ID:       "hrm",
	TitleKey: "NavigationLinks.HRM",
	Icon:     icons.UsersThree(icons.Props{Size: "20"}),
	Order:    2,
}
