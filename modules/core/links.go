// Package core provides this package.
package core

import (
	icons "github.com/iota-uz/icons/phosphor"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

var AdministrationLink = application.NavNode{
	ID:       "core.administration",
	TitleKey: "NavigationLinks.Administration",
	Icon:     icons.AirTrafficControl(icons.Props{Size: "20"}),
	Order:    20,
}
