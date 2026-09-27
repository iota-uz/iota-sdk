// Package crm provides this package.
package crm

import (
	icons "github.com/iota-uz/icons/phosphor"

	"github.com/iota-uz/iota-sdk/pkg/application"
)

var CRMLink = application.NavNode{
	ID:       "crm",
	TitleKey: "NavigationLinks.CRM",
	Icon:     icons.Handshake(icons.Props{Size: "20"}),
}
